package security

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Tokens de SISTEMAS consumidores (client credentials), emitidos por Auth
// con una clave propia (ServiceJWTSecret) distinta de la de usuarios. Se
// verifican firma, iss, aud, exp, token_type="service", sub (UUID del
// sistema), client_id y scope. Nunca se convierten en una identidad de
// usuario: no llevan rol.

const (
	ServiceTokenType            = "service"
	DefaultServiceTokenIssuer   = "tungasuca-auth"
	DefaultServiceTokenAudience = "estudiantes"
	// maxServiceTokenLifetime: un token con una vida mayor (exp - iat) se
	// rechaza aunque esté bien firmado (Auth nunca emite más de 15 min).
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

// Permisos devuelve el scope como conjunto.
func (c *ServiceClaims) Permisos() map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.Fields(c.Scope) {
		out[p] = true
	}
	return out
}

type ServiceTokenValidator struct {
	secret   string
	issuer   string
	audience string
}

// NewServiceTokenValidator devuelve (nil, nil) sin clave: los tokens de
// servicio quedan deshabilitados y solo se aceptan tokens de usuario.
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
