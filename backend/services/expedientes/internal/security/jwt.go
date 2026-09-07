// Package security valida los access tokens emitidos por Auth Service.
// Expedientes Service NO emite, renueva ni conoce refresh tokens: solo
// verifica la firma HS256 del access token con el mismo JWTSecret que Auth,
// que el Gateway reenvía como metadata "authorization" en cada llamada gRPC.
package security

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidAccessToken = errors.New("invalid access token")

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type JWTValidator struct {
	secret string
}

func NewJWTValidator(secret string) *JWTValidator {
	return &JWTValidator{secret: secret}
}

func (v *JWTValidator) Validate(rawToken string) (*Claims, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || strings.TrimSpace(v.secret) == "" {
		return nil, ErrInvalidAccessToken
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidAccessToken
		}
		return []byte(v.secret), nil
	})
	if err != nil || !token.Valid || claims.Subject == "" {
		return nil, ErrInvalidAccessToken
	}

	return claims, nil
}
