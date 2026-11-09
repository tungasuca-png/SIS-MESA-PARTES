package security

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Tokens de servicio (client credentials) para Derivaciones (A + B). Mismo
// formato que valida Estudiantes, con una audiencia y un secreto EXCLUSIVOS
// de Derivaciones: este servicio solo conoce su propio secreto, así que no
// puede emitir ni aceptar tokens de otra audiencia.
const (
	ServiceTokenType            = "service"
	DefaultServiceTokenIssuer   = "tungasuca-auth"
	DefaultServiceTokenAudience = "derivaciones"
	// ScopeProyectar: único permiso que habilita AplicarMovimiento.
	ScopeProyectar = "derivaciones.proyectar"

	maxServiceTokenLifetime = 15 * time.Minute
	minServiceSecretLength  = 32
)

var (
	ErrInvalidServiceToken = errors.New("invalid service token")
	errServiceSecretWeak   = errors.New("ServiceJWTSecret debe tener al menos 32 caracteres")
	errServiceSecretShared = errors.New("ServiceJWTSecret no puede ser igual a JWTSecret")
	uuidPattern            = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type ServiceClaims struct {
	TokenType string `json:"token_type"`
	ClientID  string `json:"client_id"`
	Scope     string `json:"scope"`
	jwt.RegisteredClaims
}

// TienePermiso: el scope es una lista de permisos separada por espacios.
func (c *ServiceClaims) TienePermiso(permiso string) bool {
	for _, p := range strings.Fields(c.Scope) {
		if p == permiso {
			return true
		}
	}
	return false
}

type ServiceTokenValidator struct {
	secret   string
	issuer   string
	audience string
}

// NewServiceTokenValidator devuelve nil (sin error) si no hay secreto
// configurado: AplicarMovimiento queda deshabilitado (Unauthenticated).
func NewServiceTokenValidator(secret, userSecret, issuer, audience string) (*ServiceTokenValidator, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, nil
	}
	if len(secret) < minServiceSecretLength {
		return nil, errServiceSecretWeak
	}
	if secret == strings.TrimSpace(userSecret) {
		return nil, errServiceSecretShared
	}
	if strings.TrimSpace(issuer) == "" {
		issuer = DefaultServiceTokenIssuer
	}
	if strings.TrimSpace(audience) == "" {
		audience = DefaultServiceTokenAudience
	}
	return &ServiceTokenValidator{secret: secret, issuer: issuer, audience: audience}, nil
}

func (v *ServiceTokenValidator) Validate(rawToken string) (*ServiceClaims, error) {
	if v == nil {
		return nil, ErrInvalidServiceToken
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, ErrInvalidServiceToken
	}
	claims := &ServiceClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidServiceToken
		}
		return []byte(v.secret), nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithLeeway(30*time.Second),
		jwt.WithStrictDecoding(),
	)
	if err != nil || !token.Valid {
		return nil, ErrInvalidServiceToken
	}
	if claims.TokenType != ServiceTokenType || !uuidPattern.MatchString(claims.Subject) ||
		strings.TrimSpace(claims.ClientID) == "" || strings.TrimSpace(claims.Scope) == "" {
		return nil, ErrInvalidServiceToken
	}
	if claims.IssuedAt == nil || claims.ExpiresAt.Sub(claims.IssuedAt.Time) > maxServiceTokenLifetime {
		return nil, ErrInvalidServiceToken
	}
	return claims, nil
}
