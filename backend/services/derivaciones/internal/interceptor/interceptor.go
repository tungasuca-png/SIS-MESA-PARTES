package interceptor

import (
	"context"
	"strings"

	"derivaciones/internal/security"

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

// AuthenticationInterceptor exige un access token válido en la metadata
// "authorization" para TODAS las operaciones de Derivaciones: al igual que
// Expedientes/Documentos, este microservicio no tiene métodos públicos.
func AuthenticationInterceptor(validator *security.JWTValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		userID, role, token, err := authenticate(ctx, validator)
		if err != nil {
			return nil, err
		}

		return handler(context.WithValue(ctx, contextKey{}, userContext{UserID: userID, Role: role, Token: token}), req)
	}
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
