package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Tokens de servicio (client credentials): identidad de un SISTEMA
// consumidor, nunca de un usuario. Se separan de los access tokens humanos
// por tres vías independientes:
//   - Clave distinta (ServiceJWTSecret): los validadores de usuario de todos
//     los servicios los rechazan por firma.
//   - token_type="service": ValidateAccessToken rechaza cualquier token con
//     token_type, aunque alguien reutilizara la clave.
//   - iss/aud: solo la audiencia configurada (Estudiantes) los acepta.

const (
	ServiceTokenType            = "service"
	DefaultServiceTokenIssuer   = "tungasuca-auth"
	DefaultServiceTokenAudience = "estudiantes"
	DefaultServiceTokenDuration = 5 * time.Minute
	// MaxServiceTokenDuration: tope de vida de un token de servicio; la
	// revocación de un cliente surte efecto, como máximo, en este plazo.
	MaxServiceTokenDuration = 15 * time.Minute
	minServiceSecretLength  = 32
	clientIDPrefix          = "cli_"
	clientSecretPrefix      = "cs_"
)

var (
	ErrServiceTokenDisabled = errors.New("service tokens are not configured")
	errServiceSecretWeak    = errors.New("ServiceJWTSecret debe tener al menos 32 caracteres")
	errServiceSecretShared  = errors.New("ServiceJWTSecret no puede ser igual a JWTSecret")
)

// AssignableServicePermissions: únicos permisos que un sistema consumidor
// puede recibir (mínimo privilegio, nunca permisos administrativos). Son
// códigos reales del catálogo de Auth; estudiantes.basic.view lo agrega la
// migración 019.
var AssignableServicePermissions = map[string]bool{
	"estudiantes.view":             true,
	"estudiantes.basic.view":       true,
	"estudiantes.personal.view":    true,
	"estudiantes.create":           true,
	"estudiantes.update":           true,
	"estudiantes.matricula.update": true,
}

type ServiceClaims struct {
	TokenType string `json:"token_type"`
	ClientID  string `json:"client_id"`
	Scope     string `json:"scope"`
	jwt.RegisteredClaims
}

type ServiceTokenIssuer struct {
	secret   string
	issuer   string
	audience string
	duration time.Duration
}

// NewServiceTokenIssuer devuelve (nil, nil) si no hay clave configurada: los
// tokens de servicio quedan deshabilitados y el resto de Auth sigue igual.
// Una clave débil o igual a la de usuarios es un error de configuración.
func NewServiceTokenIssuer(secret, userSecret, issuer, audience string, duration time.Duration) (*ServiceTokenIssuer, error) {
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
	if duration <= 0 {
		duration = DefaultServiceTokenDuration
	}
	if duration > MaxServiceTokenDuration {
		duration = MaxServiceTokenDuration
	}
	return &ServiceTokenIssuer{secret: secret, issuer: issuer, audience: audience, duration: duration}, nil
}

// Issue firma un token para el sistema serviceID (UUID del registro) con los
// permisos dados, ordenados y sin repetir, como claim "scope".
func (i *ServiceTokenIssuer) Issue(serviceID, clientID string, permisos []string) (string, string, time.Duration, error) {
	if i == nil {
		return "", "", 0, ErrServiceTokenDisabled
	}
	scope := NormalizeScope(permisos)
	now := time.Now()
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", "", 0, err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, ServiceClaims{
		TokenType: ServiceTokenType,
		ClientID:  clientID,
		Scope:     scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			Subject:   serviceID,
			Audience:  jwt.ClaimStrings{i.audience},
			ID:        hex.EncodeToString(jti),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.duration)),
		},
	})
	signed, err := token.SignedString([]byte(i.secret))
	if err != nil {
		return "", "", 0, err
	}
	return signed, scope, i.duration, nil
}

func NormalizeScope(permisos []string) string {
	vistos := make(map[string]bool, len(permisos))
	lista := make([]string, 0, len(permisos))
	for _, p := range permisos {
		p = strings.TrimSpace(p)
		if p == "" || vistos[p] {
			continue
		}
		vistos[p] = true
		lista = append(lista, p)
	}
	sort.Strings(lista)
	return strings.Join(lista, " ")
}

// GenerateClientCredentials crea un client_id público y un client_secret de
// 256 bits; solo el hash SHA-256 del secreto se guarda (el secreto es
// aleatorio y largo, no una contraseña: no necesita bcrypt).
func GenerateClientCredentials() (clientID, secret string, secretHash []byte, err error) {
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return "", "", nil, err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", nil, err
	}
	clientID = clientIDPrefix + hex.EncodeToString(id)
	secret = clientSecretPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return clientID, secret, HashClientSecret(secret), nil
}

func HashClientSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// ValidClientIDFormat: solo para decidir si un client_id puede escribirse en
// un log (nunca se registra texto arbitrario que podría ser un secreto).
func ValidClientIDFormat(clientID string) bool {
	if len(clientID) != len(clientIDPrefix)+32 || !strings.HasPrefix(clientID, clientIDPrefix) {
		return false
	}
	_, err := hex.DecodeString(clientID[len(clientIDPrefix):])
	return err == nil
}
