package security

import (
	"errors"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidAccessToken = errors.New("invalid access token")

func (m *JWTManager) ValidateAccessToken(rawToken string) (*JWTClaims, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || strings.TrimSpace(m.secret) == "" {
		return nil, ErrInvalidAccessToken
	}

	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidAccessToken
		}
		return []byte(m.secret), nil
	})
	if err != nil || !token.Valid || claims.Subject == "" {
		return nil, ErrInvalidAccessToken
	}

	return claims, nil
}
