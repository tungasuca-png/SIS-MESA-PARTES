package interceptor

import (
	"context"
	"errors"
	"testing"
	"time"

	"auth/internal/security"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type stubPermissionChecker struct {
	allowed bool
	err     error
	userID  string
}

func (s *stubPermissionChecker) HasPermission(_ context.Context, userID string, _ string) (bool, error) {
	s.userID = userID
	return s.allowed, s.err
}

func testJWTManager() *security.JWTManager {
	return security.NewJWTManager("test-secret", time.Hour)
}

func testContextWithToken(t *testing.T) context.Context {
	t.Helper()
	manager := testJWTManager()
	token, _, err := manager.GenerateAccessToken("user-1", "ana", "ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
}

func testContextWithRawToken(rawToken string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+rawToken))
}

func testHandler(_ context.Context, _ any) (any, error) {
	return "handled", nil
}

func TestAuthenticationInterceptor_PublicRPCContinuesWithoutToken(t *testing.T) {
	policies := map[string]MethodPolicy{"/auth.Auth/Login": PublicMethod()}
	response, err := AuthenticationInterceptor(testJWTManager(), policies)(
		context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/auth.Auth/Login"}, testHandler,
	)
	if err != nil || response != "handled" {
		t.Fatalf("expected public RPC to continue, response=%v err=%v", response, err)
	}
}

func TestAuthenticationInterceptor_UsesJWTSubject(t *testing.T) {
	manager := testJWTManager()
	token, _, err := manager.GenerateAccessToken("subject-user", "ana", "ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	policies := map[string]MethodPolicy{"/mesa.Expedientes/Create": ProtectedMethod("expedientes.create")}
	var userID string
	_, err = AuthenticationInterceptor(manager, policies)(testContextWithRawToken(token), nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, func(ctx context.Context, _ any) (any, error) {
		userID, _ = UserIDFromContext(ctx)
		return nil, nil
	})
	if err != nil || userID != "subject-user" {
		t.Fatalf("expected JWT subject to identify user, userID=%q err=%v", userID, err)
	}
}

func TestAuthorizationInterceptor_AllowsUserWithPermission(t *testing.T) {
	checker := &stubPermissionChecker{allowed: true}
	policies := map[string]MethodPolicy{"/mesa.Expedientes/Create": ProtectedMethod("expedientes.create")}
	ctx := testContextWithToken(t)

	var authenticated context.Context
	_, err := AuthenticationInterceptor(testJWTManager(), policies)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, func(ctx context.Context, _ any) (any, error) {
		authenticated = ctx
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	response, err := AuthorizationInterceptor(checker, policies)(authenticated, nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, testHandler)
	if err != nil || response != "handled" || checker.userID != "user-1" {
		t.Fatalf("expected authorized request to continue, response=%v user=%q err=%v", response, checker.userID, err)
	}
}

func TestAuthorizationInterceptor_DeniesWithoutPermission(t *testing.T) {
	checker := &stubPermissionChecker{}
	policies := map[string]MethodPolicy{"/mesa.Expedientes/Create": ProtectedMethod("expedientes.create")}
	ctx := testContextWithToken(t)
	var authenticated context.Context
	_, err := AuthenticationInterceptor(testJWTManager(), policies)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, func(ctx context.Context, _ any) (any, error) {
		authenticated = ctx
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = AuthorizationInterceptor(checker, policies)(authenticated, nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, testHandler)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestAuthenticationInterceptor_RejectsMissingToken(t *testing.T) {
	policies := map[string]MethodPolicy{"/mesa.Expedientes/Create": ProtectedMethod("expedientes.create")}
	_, err := AuthenticationInterceptor(testJWTManager(), policies)(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthenticationInterceptor_RejectsInvalidTokenForms(t *testing.T) {
	manager := testJWTManager()
	validToken, _, err := manager.GenerateAccessToken("user-1", "ana", "ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	policies := map[string]MethodPolicy{"/mesa.Expedientes/Create": ProtectedMethod("expedientes.create")}
	cases := map[string]context.Context{
		"empty":        metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer ")),
		"wrong scheme": metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Invalid "+validToken)),
		"tampered":     testContextWithRawToken(validToken[:len(validToken)-1] + "x"),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := AuthenticationInterceptor(manager, policies)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, testHandler)
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("expected Unauthenticated, got %v", err)
			}
		})
	}
}

func TestAuthenticationInterceptor_RejectsExpiredToken(t *testing.T) {
	claims := security.JWTClaims{
		Username: "ana",
		Role:     "ADMIN",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	rawToken, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	policies := map[string]MethodPolicy{"/mesa.Expedientes/Create": ProtectedMethod("expedientes.create")}
	_, err = AuthenticationInterceptor(testJWTManager(), policies)(testContextWithRawToken(rawToken), nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, testHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected expired token to be rejected as Unauthenticated, got %v", err)
	}
}

func TestAuthorizationInterceptor_PropagatesCheckerError(t *testing.T) {
	checker := &stubPermissionChecker{err: errors.New("repository unavailable")}
	policies := map[string]MethodPolicy{"/mesa.Expedientes/Create": ProtectedMethod("expedientes.create")}
	ctx := context.WithValue(context.Background(), contextKey{}, "user-1")

	_, err := AuthorizationInterceptor(checker, policies)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/mesa.Expedientes/Create"}, testHandler)
	if !errors.Is(err, checker.err) {
		t.Fatalf("expected checker error to propagate, got %v", err)
	}
}
