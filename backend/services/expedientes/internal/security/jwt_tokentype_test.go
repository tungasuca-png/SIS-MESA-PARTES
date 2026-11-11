package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const secretoPrueba = "secreto-de-usuarios-de-prueba-0123456789abcdef"

func firmar(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secretoPrueba))
	if err != nil {
		t.Fatalf("firmar: %v", err)
	}
	return s
}

// B-01: aunque un token de servicio llegara firmado con la clave de usuarios
// (configuración errónea o clave reutilizada), el validador humano lo rechaza
// por token_type; un access token humano normal sigue siendo válido.
func TestValidateRechazaTokenConTokenType(t *testing.T) {
	v := NewJWTValidator(secretoPrueba)
	base := func() jwt.MapClaims {
		return jwt.MapClaims{"sub": "6f1c2a4e-1b2c-4d3e-8f90-0a1b2c3d4e5f", "role": "SECRETARIA",
			"exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix()}
	}
	if _, err := v.Validate(firmar(t, base())); err != nil {
		t.Fatalf("access token humano rechazado: %v", err)
	}
	for _, tipo := range []string{"service", "refresh", "x"} {
		c := base()
		c["token_type"] = tipo
		if _, err := v.Validate(firmar(t, c)); err == nil {
			t.Fatalf("token con token_type=%q aceptado como humano", tipo)
		}
	}
	vencido := base()
	vencido["exp"] = time.Now().Add(-time.Minute).Unix()
	if _, err := v.Validate(firmar(t, vencido)); err == nil {
		t.Fatalf("token vencido aceptado")
	}
}
