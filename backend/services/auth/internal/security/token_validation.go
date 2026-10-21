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
	},
		// Endurecimiento: el algoritmo se fija también en el parser y un token
		// sin "exp" se rechaza (antes se aceptaba y no vencía nunca).
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		// Base64 canónico: sin esto, variantes de la misma firma (bits de relleno
		// distintos en el último carácter) también validaban.
		jwt.WithStrictDecoding(),
	)
	if err != nil || !token.Valid || claims.Subject == "" {
		return nil, ErrInvalidAccessToken
	}

	return claims, nil
}
