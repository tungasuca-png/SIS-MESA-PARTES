package interceptor

import (
	"context"
	"testing"
	"time"

	"usuarios/internal/security"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func testValidator() *security.JWTValidator {
	return security.NewJWTValidator("test-secret")
}

func testHandler(ctx context.Context, _ any) (any, error) {
	userID, role, _ := UserFromContext(ctx)
	return map[string]string{"userID": userID, "role": role}, nil
}

func signToken(t *testing.T, secret, subject, role string, expiresIn time.Duration) string {
	t.Helper()
	claims := security.Claims{
		Username: "tester",
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestAuthenticationInterceptor_TokenValido(t *testing.T) {
	token := signToken(t, "test-secret", "user-1", "ADMIN", time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

	resp, err := AuthenticationInterceptor(testValidator())(ctx, nil, &grpc.UnaryServerInfo{}, testHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := resp.(map[string]string)
	if result["userID"] != "user-1" || result["role"] != "ADMIN" {
		t.Fatalf("unexpected identity: %+v", result)
	}
}

func TestAuthenticationInterceptor_SinToken(t *testing.T) {
	_, err := AuthenticationInterceptor(testValidator())(context.Background(), nil, &grpc.UnaryServerInfo{}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthenticationInterceptor_TokenExpirado(t *testing.T) {
	token := signToken(t, "test-secret", "user-1", "ADMIN", -time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

	_, err := AuthenticationInterceptor(testValidator())(ctx, nil, &grpc.UnaryServerInfo{}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthenticationInterceptor_TokenDeOtroSecreto(t *testing.T) {
	token := signToken(t, "otro-secreto", "user-1", "ADMIN", time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

	_, err := AuthenticationInterceptor(testValidator())(ctx, nil, &grpc.UnaryServerInfo{}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthenticationInterceptor_EsquemaIncorrecto(t *testing.T) {
	token := signToken(t, "test-secret", "user-1", "ADMIN", time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic "+token))

	_, err := AuthenticationInterceptor(testValidator())(ctx, nil, &grpc.UnaryServerInfo{}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}
