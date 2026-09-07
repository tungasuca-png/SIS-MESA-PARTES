package interceptor

import (
	"context"
	"testing"
	"time"

	"documentos/internal/security"

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

func signToken(t *testing.T, secret, subject, username, role string, expiresIn time.Duration) string {
	t.Helper()
	claims := security.Claims{
		Username: username,
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

func TestAuthenticationInterceptor_ValidToken(t *testing.T) {
	token := signToken(t, "test-secret", "user-1", "ana", "ADMIN", time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

	resp, err := AuthenticationInterceptor(testValidator())(ctx, nil, &grpc.UnaryServerInfo{}, testHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result := resp.(map[string]string)
	if result["userID"] != "user-1" || result["role"] != "ADMIN" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAuthenticationInterceptor_MissingToken(t *testing.T) {
	_, err := AuthenticationInterceptor(testValidator())(context.Background(), nil, &grpc.UnaryServerInfo{}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthenticationInterceptor_ExpiredToken(t *testing.T) {
	token := signToken(t, "test-secret", "user-1", "ana", "ADMIN", -time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))

	_, err := AuthenticationInterceptor(testValidator())(ctx, nil, &grpc.UnaryServerInfo{}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthenticationInterceptor_WrongScheme(t *testing.T) {
	token := signToken(t, "test-secret", "user-1", "ana", "ADMIN", time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic "+token))

	_, err := AuthenticationInterceptor(testValidator())(ctx, nil, &grpc.UnaryServerInfo{}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthenticationInterceptor_TamperedToken(t *testing.T) {
	token := signToken(t, "test-secret", "user-1", "ana", "ADMIN", time.Hour)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token+"x"))

	_, err := AuthenticationInterceptor(testValidator())(ctx, nil, &grpc.UnaryServerInfo{}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}
