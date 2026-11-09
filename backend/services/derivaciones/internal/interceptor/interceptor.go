package interceptor

import (
	"context"
	"strings"
	"time"

	"derivaciones/internal/security"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey struct{}

type userContext struct {
	UserID string
	Role   string
	// Token conserva el JWT crudo ya validado, para poder reenviarlo como
	// metadata saliente al consultar Auth.HasPermission (Paso 15B) — Auth
	// Service exige autenticación en ese RPC, igual que en Logout, y sin
	// esto no habría nada que reenviarle. Mismo patrón ya usado en
	// Expedientes (Paso 13B) y Documentos (Paso 14B).
	Token string
}

// MetodoAplicarMovimiento: ÚNICO RPC que acepta tokens de servicio (A + B).
// Solo el publicador de Expedientes lo invoca; ningún token de usuario
// (tampoco ADMIN) puede usarlo, y ningún token de servicio puede usar los
// demás RPC.
const MetodoAplicarMovimiento = "/derivaciones.Derivaciones/AplicarMovimiento"

type servicioKey struct{}

// Servicio identifica al sistema que invocó AplicarMovimiento.
type Servicio struct {
	ID       string
	ClientID string
}

// AuthenticationInterceptor exige un access token válido en la metadata
// "authorization" para TODAS las operaciones de Derivaciones: al igual que
// Expedientes/Documentos, este microservicio no tiene métodos públicos.
// AplicarMovimiento exige además un token de servicio con audiencia
// derivaciones y scope derivaciones.proyectar (serviceValidator nil = el
// RPC queda deshabilitado).
func AuthenticationInterceptor(validator *security.JWTValidator, serviceValidator *security.ServiceTokenValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == MetodoAplicarMovimiento {
			return autenticarServicio(ctx, req, info, handler, serviceValidator)
		}
		userID, role, token, err := authenticate(ctx, validator)
		if err != nil {
			return nil, err
		}

		return handler(context.WithValue(ctx, contextKey{}, userContext{UserID: userID, Role: role, Token: token}), req)
	}
}

func autenticarServicio(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler, v *security.ServiceTokenValidator) (any, error) {
	inicio := time.Now()
	raw, err := bearer(ctx)
	if err != nil {
		return nil, err
	}
	claims, err := v.Validate(raw)
	if err != nil {
		logx.WithContext(ctx).Infow("token de servicio rechazado", logx.Field("operacion", info.FullMethod))
		return nil, status.Error(codes.Unauthenticated, "token de servicio inválido")
	}
	if !claims.TienePermiso(security.ScopeProyectar) {
		return nil, status.Error(codes.PermissionDenied, "el sistema no tiene el permiso derivaciones.proyectar")
	}
	resp, err := handler(context.WithValue(ctx, servicioKey{}, Servicio{ID: claims.Subject, ClientID: claims.ClientID}), req)
	logx.WithContext(ctx).Infow("acceso de sistema consumidor",
		logx.Field("sistema_id", claims.Subject),
		logx.Field("client_id", claims.ClientID),
		logx.Field("operacion", info.FullMethod),
		logx.Field("resultado", status.Code(err).String()),
		logx.Field("duracion_ms", time.Since(inicio).Milliseconds()),
	)
	return resp, err
}

// ServicioFromContext expone el sistema autenticado (solo en AplicarMovimiento).
func ServicioFromContext(ctx context.Context) (Servicio, bool) {
	s, ok := ctx.Value(servicioKey{}).(Servicio)
	return s, ok && s.ID != ""
}

func bearer(ctx context.Context) (string, error) {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	authValues := values.Get("authorization")
	if len(authValues) == 0 {
		return "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	parts := strings.Fields(authValues[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	return parts[1], nil
}

func authenticate(ctx context.Context, validator *security.JWTValidator) (string, string, string, error) {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	authValues := values.Get("authorization")
	if len(authValues) == 0 {
		return "", "", "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	parts := strings.Fields(authValues[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", "", "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	claims, err := validator.Validate(parts[1])
	if err != nil {
		return "", "", "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}

	return claims.Subject, claims.Role, parts[1], nil
}

// UserFromContext expone la identidad autenticada a la capa de logic.
func UserFromContext(ctx context.Context) (userID, role string, ok bool) {
	u, ok := ctx.Value(contextKey{}).(userContext)
	if !ok || strings.TrimSpace(u.UserID) == "" {
		return "", "", false
	}
	return u.UserID, u.Role, true
}

// TokenFromContext expone el JWT crudo ya validado (Paso 15B) — solo para
// reenviarlo a Auth Service al consultar HasPermission, nunca para volver
// a validarlo acá.
func TokenFromContext(ctx context.Context) (string, bool) {
	u, ok := ctx.Value(contextKey{}).(userContext)
	if !ok || strings.TrimSpace(u.Token) == "" {
		return "", false
	}
	return u.Token, true
}
