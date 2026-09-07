package interceptor

import (
	"context"
	"strings"

	"expedientes/internal/security"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey struct{}

type userContext struct {
	UserID string
	Role   string
}

// AuthenticationInterceptor exige un access token válido en la metadata
// "authorization" para TODAS las operaciones de Expedientes: a diferencia
// de Auth Service, este microservicio no tiene métodos públicos.
func AuthenticationInterceptor(validator *security.JWTValidator) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		userID, role, err := authenticate(ctx, validator)
		if err != nil {
			return nil, err
		}

		return handler(context.WithValue(ctx, contextKey{}, userContext{UserID: userID, Role: role}), req)
	}
}

func authenticate(ctx context.Context, validator *security.JWTValidator) (string, string, error) {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	authValues := values.Get("authorization")
	if len(authValues) == 0 {
		return "", "", status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	parts := strings.Fields(authValues[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	claims, err := validator.Validate(parts[1])
	if err != nil {
		return "", "", status.Error(codes.Unauthenticated, "token de acceso inválido")
	}

	return claims.Subject, claims.Role, nil
}

// UserFromContext expone la identidad autenticada a la capa de logic.
func UserFromContext(ctx context.Context) (userID, role string, ok bool) {
	u, ok := ctx.Value(contextKey{}).(userContext)
	if !ok || strings.TrimSpace(u.UserID) == "" {
		return "", "", false
	}
	return u.UserID, u.Role, true
}
