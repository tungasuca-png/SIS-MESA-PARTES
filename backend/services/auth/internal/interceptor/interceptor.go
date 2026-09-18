package interceptor

import (
	"context"
	"strings"

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

		// Paso 21B: primer consumidor real de "roles.view" (sembrado desde
		// la migración 005, sin ningún RPC que lo usara hasta ahora — ver
		// Paso 21A). ProtectedMethod ya resuelve JWT + permiso vía
		// AuthorizationInterceptor/AuthorizationService.HasPermission, sin
		// necesitar ningún chequeo adicional en el Logic de cada RPC.
		"/auth.Auth/ListRoles":             ProtectedMethod("roles.view"),
		"/auth.Auth/ListPermissions":       ProtectedMethod("roles.view"),
		"/auth.Auth/GetRolePermissions":    ProtectedMethod("roles.view"),
		"/auth.Auth/UpdateRolePermissions": ProtectedMethod("roles.view"),
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

func AuthenticationInterceptor(jwtManager *security.JWTManager, policies map[string]MethodPolicy) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		policy := policies[info.FullMethod]
		if policy.Public && !policy.RequireAuthentication {
			return handler(ctx, req)
		}

		userID, err := authenticatedUserID(ctx, jwtManager)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
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

func authenticatedUserID(ctx context.Context, jwtManager *security.JWTManager) (string, error) {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	authorizationValues := values.Get("authorization")
	if len(authorizationValues) == 0 {
		return "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	parts := strings.Fields(authorizationValues[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	claims, err := jwtManager.ValidateAccessToken(parts[1])
	if err != nil {
		return "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	return claims.Subject, nil
}

var _ PermissionChecker = (*authorization.AuthorizationService)(nil)
