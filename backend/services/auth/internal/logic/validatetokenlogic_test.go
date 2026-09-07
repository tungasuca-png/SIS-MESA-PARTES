package logic

import (
	"context"
	"testing"
	"time"

	"auth/auth"
	"auth/internal/security"
	"auth/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func validateTokenTestService() *ValidateTokenLogic {
	return NewValidateTokenLogic(context.Background(), &svc.ServiceContext{
		JWTManager: security.NewJWTManager("validate-token-test-secret", time.Hour),
	})
}

func TestValidateToken_ValidToken(t *testing.T) {
	logic := validateTokenTestService()
	token, _, err := logic.svcCtx.JWTManager.GenerateAccessToken("user-validate", "ana", "DOCENTE")
	if err != nil {
		t.Fatal(err)
	}

	response, err := logic.ValidateToken(&auth.ValidateTokenRequest{AccessToken: token})
	if err != nil {
		t.Fatalf("expected valid token, got %v", err)
	}
	if !response.Valid || response.UserId != "user-validate" || response.Username != "ana" || response.Role != "DOCENTE" {
		t.Fatalf("unexpected validation response: %+v", response)
	}
}

func TestValidateToken_RejectsInvalidTokens(t *testing.T) {
	logic := validateTokenTestService()
	validToken, _, err := logic.svcCtx.JWTManager.GenerateAccessToken("user-validate", "ana", "DOCENTE")
	if err != nil {
		t.Fatal(err)
	}

	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, security.JWTClaims{
		Username: "ana",
		Role:     "DOCENTE",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-validate",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	})
	expiredToken, err := expired.SignedString([]byte("validate-token-test-secret"))
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"expired":   expiredToken,
		"tampered":  validToken + "tampered",
		"empty":     "",
		"malformed": "not-a-jwt",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			response, err := logic.ValidateToken(&auth.ValidateTokenRequest{AccessToken: token})
			if response != nil || status.Code(err) != codes.Unauthenticated {
				t.Fatalf("expected Unauthenticated, response=%v err=%v", response, err)
			}
		})
	}
}
