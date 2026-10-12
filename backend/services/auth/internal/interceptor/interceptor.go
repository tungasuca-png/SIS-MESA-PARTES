package interceptor

import (
	"context"
	"strings"
	"time"

	"auth/internal/authorization"
	"auth/internal/security"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey struct{}

type MethodPolicy struct {
	Public                bool
	RequireAuthentication bool
	Permission            string
}

type PermissionChecker interface {
	HasPermission(ctx context.Context, userID string, permissionCode string) (bool, error)
}

func DefaultMethodPolicies() map[string]MethodPolicy {
	return map[string]MethodPolicy{
		"/auth.Auth/Ping":          PublicMethod(),
		"/auth.Auth/Register":      PublicMethod(),
		"/auth.Auth/Login":         PublicMethod(),
		"/auth.Auth/RefreshToken":  PublicMethod(),
		"/auth.Auth/ValidateToken": PublicMethod(),
		"/auth.Auth/Logout":        AuthenticatedMethod(),
		// HasPermission consulta si existe un permiso: no puede depender de
		// un permiso para poder consultarlo. Solo exige autenticación, igual
		// que Logout — la restricción real (solo el propio usuario) la
		// aplica HasPermissionLogic comparando contra el sub del JWT.
		"/auth.Auth/HasPermission": AuthenticatedMethod(),
		// GetMyPermissions: cada usuario consulta SUS PROPIOS permisos —
		// solo exige autenticación, igual que HasPermission (nunca un
		// permiso administrativo como "roles.view": esto no es
		// configuración, es autoconsulta).
		"/auth.Auth/GetMyPermissions": AuthenticatedMethod(),

		// Paso 21B: primer consumidor real de "roles.view" (sembrado desde
		// la migración 005, sin ningún RPC que lo usara hasta ahora — ver
		// Paso 21A). ProtectedMethod ya resuelve JWT + permiso vía
		// AuthorizationInterceptor/AuthorizationService.HasPermission, sin
		// necesitar ningún chequeo adicional en el Logic de cada RPC.
		"/auth.Auth/ListRoles":             ProtectedMethod("roles.view"),
		"/auth.Auth/ListPermissions":       ProtectedMethod("roles.view"),
		"/auth.Auth/GetRolePermissions":    ProtectedMethod("roles.view"),
		"/auth.Auth/UpdateRolePermissions": ProtectedMethod("roles.view"),

		// UpdateUserRole (Usuario -> Rol): permiso propio y distinto de
		// "roles.view" (que protege Rol -> Permisos, un dominio separado).
		// Solo ADMIN lo tiene hoy (migración 006), sin tocar el resto del
		// catálogo.
		"/auth.Auth/UpdateUserRole": ProtectedMethod("usuarios.change_role"),

		// CreateUser (administrativo, distinto de Register): el catálogo
		// real NO tiene "usuarios.create" (Fase "Creación de usuarios desde
		// Configuración") — se reutiliza "usuarios.change_role" a propósito,
		// sin agregar un permiso nuevo: crear un usuario incluye
		// necesariamente decidir su rol inicial, la misma responsabilidad
		// que ese permiso ya protege, y hoy solo ADMIN lo tiene.
		"/auth.Auth/CreateUser": ProtectedMethod("usuarios.change_role"),
	}
}

func PublicMethod() MethodPolicy {
	return MethodPolicy{Public: true}
}

func AuthenticatedMethod() MethodPolicy {
	return MethodPolicy{RequireAuthentication: true}
}

func ProtectedMethod(permission string) MethodPolicy {
	return MethodPolicy{RequireAuthentication: true, Permission: strings.TrimSpace(permission)}
}

// SessionFreshnessChecker detecta un access token emitido ANTES del último
// cambio de rol del usuario (su claim "role" quedó desactualizado). Es
// opcional: sin checker el interceptor se comporta como antes.
type SessionFreshnessChecker interface {
	RoleAssignedAfter(ctx context.Context, userID string, issuedAt time.Time) (bool, error)
}

func AuthenticationInterceptor(jwtManager *security.JWTManager, policies map[string]MethodPolicy, freshness ...SessionFreshnessChecker) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		policy := policies[info.FullMethod]
		if policy.Public && !policy.RequireAuthentication {
			return handler(ctx, req)
		}

		claims, err := authenticatedClaims(ctx, jwtManager)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
		}
		userID := claims.Subject

		// Token emitido antes del último cambio de rol: se rechaza (401) para
		// que el cliente renueve la sesión — RefreshToken relee el rol real
		// en la BD. Todo microservicio autoriza vía Auth.HasPermission con
		// este mismo JWT, así que un token viejo deja de servir en todos
		// lados, no solo en Auth. Fail-closed: si no se puede verificar, se
		// rechaza. Un token sin iat no lo emite Auth y no se puede fechar.
		if len(freshness) > 0 && freshness[0] != nil && claims.IssuedAt != nil {
			stale, err := freshness[0].RoleAssignedAfter(ctx, userID, claims.IssuedAt.Time)
			if err != nil {
				return nil, status.Error(codes.Unavailable, "no se pudo verificar la vigencia de la sesión")
			}
			if stale {
				return nil, status.Error(codes.Unauthenticated, "la sesión está desactualizada por un cambio de rol, renueve la sesión")
			}
		}

		return handler(context.WithValue(ctx, contextKey{}, userID), req)
	}
}

func AuthorizationInterceptor(checker PermissionChecker, policies map[string]MethodPolicy) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		policy := policies[info.FullMethod]
		if policy.Permission == "" {
			return handler(ctx, req)
		}

		userID, ok := UserIDFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
		}
		allowed, err := checker.HasPermission(ctx, userID, policy.Permission)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, status.Error(codes.PermissionDenied, "permiso insuficiente")
		}

		return handler(ctx, req)
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(contextKey{}).(string)
	return userID, ok && strings.TrimSpace(userID) != ""
}

func authenticatedClaims(ctx context.Context, jwtManager *security.JWTManager) (*security.JWTClaims, error) {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	authorizationValues := values.Get("authorization")
	if len(authorizationValues) == 0 {
		return nil, status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	parts := strings.Fields(authorizationValues[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	claims, err := jwtManager.ValidateAccessToken(parts[1])
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	return claims, nil
}

var _ PermissionChecker = (*authorization.AuthorizationService)(nil)
