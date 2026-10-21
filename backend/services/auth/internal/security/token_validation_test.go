package security

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Secreto exclusivo de estas pruebas (no es el de ningún entorno).
const secretoPrueba = "secreto-de-prueba-unitaria-auth"

func firmar(t *testing.T, method jwt.SigningMethod, key any, claims jwt.Claims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestValidateAccessToken_TokenEmitidoEsValido(t *testing.T) {
	m := NewJWTManager(secretoPrueba, time.Minute)
	tok, _, err := m.GenerateAccessToken("u-1", "ana", "SECRETARIA")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.ValidateAccessToken(tok)
	if err != nil || claims.Subject != "u-1" || claims.Role != "SECRETARIA" || claims.ExpiresAt == nil || claims.ID == "" {
		t.Fatalf("el token emitido debe validar con sus claims: %+v, %v", claims, err)
	}
}

func TestValidateAccessToken_Rechazos(t *testing.T) {
	m := NewJWTManager(secretoPrueba, time.Minute)
	futuro := jwt.NewNumericDate(time.Now().Add(time.Hour))
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	casos := map[string]string{
		"ausente":     "",
		"espacios":    "   ",
		"malformado":  "no.es.un-jwt",
		"basura":      strings.Repeat("A", 500),
		"vencido":     firmar(t, jwt.SigningMethodHS256, []byte(secretoPrueba), jwt.RegisteredClaims{Subject: "u-1", ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute))}),
		"sin exp":     firmar(t, jwt.SigningMethodHS256, []byte(secretoPrueba), jwt.MapClaims{"sub": "u-1", "role": "ADMIN"}),
		"sin sub":     firmar(t, jwt.SigningMethodHS256, []byte(secretoPrueba), jwt.RegisteredClaims{ExpiresAt: futuro}),
		"otro secret": firmar(t, jwt.SigningMethodHS256, []byte("otro-secreto"), jwt.RegisteredClaims{Subject: "u-1", ExpiresAt: futuro}),
		"alg none":    firmar(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, jwt.RegisteredClaims{Subject: "u-1", ExpiresAt: futuro}),
		"HS512":       firmar(t, jwt.SigningMethodHS512, []byte(secretoPrueba), jwt.RegisteredClaims{Subject: "u-1", ExpiresAt: futuro}),
		"RS256":       firmar(t, jwt.SigningMethodRS256, rsaKey, jwt.RegisteredClaims{Subject: "u-1", ExpiresAt: futuro}),
	}
	// Token válido con la firma alterada (mismo header/payload).
	valido, _, _ := m.GenerateAccessToken("u-1", "ana", "ADMIN")
	casos["firma alterada"] = valido[:len(valido)-2] + "xx"
	// Misma firma en base64 NO canónico: el último carácter de una firma
	// HS256 (32 bytes, 43 caracteres) lleva 2 bits de relleno; cambiarlos no
	// altera los bytes decodificados. Con decodificación estricta se rechaza.
	const alfabeto = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	ultimo := strings.IndexByte(alfabeto, valido[len(valido)-1])
	casos["base64 no canónico"] = valido[:len(valido)-1] + string(alfabeto[ultimo^1])
	// Payload alterado (rol escalado) sin volver a firmar.
	partes := strings.Split(valido, ".")
	casos["payload alterado"] = partes[0] + "." + strings.TrimRight(partes[1], "=") + "e30" + "." + partes[2]

	for nombre, tok := range casos {
		if claims, err := m.ValidateAccessToken(tok); err == nil {
			t.Errorf("%s: se aceptó un token inválido (claims %+v)", nombre, claims)
		}
	}
}

func TestValidateAccessToken_SinSecretoConfigurado(t *testing.T) {
	tok := firmar(t, jwt.SigningMethodHS256, []byte(""), jwt.RegisteredClaims{Subject: "u-1", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))})
	if _, err := NewJWTManager("", time.Minute).ValidateAccessToken(tok); err == nil {
		t.Fatal("sin secreto configurado no debe validarse ningún token")
	}
}
