package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrJWTConfiguration = errors.New("invalid JWT configuration")

type JWTClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret   string
	duration time.Duration
}

func NewJWTManager(secret string, duration time.Duration) *JWTManager {
	return &JWTManager{
		secret:   secret,
		duration: duration,
	}
}

func (m *JWTManager) GenerateAccessToken(userID, username, role string) (string, time.Duration, error) {
	if strings.TrimSpace(m.secret) == "" || m.duration <= 0 {
		return "", 0, ErrJWTConfiguration
	}

	now := time.Now()
	jtiBytes := make([]byte, 16)
	if _, err := rand.Read(jtiBytes); err != nil {
		return "", 0, err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, JWTClaims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        hex.EncodeToString(jtiBytes),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.duration)),
		},
	})

	signed, err := token.SignedString([]byte(m.secret))
	if err != nil {
		return "", 0, err
	}

	return signed, m.duration, nil
}
