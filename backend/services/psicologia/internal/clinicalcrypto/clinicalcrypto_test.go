package clinicalcrypto

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
)

func claveAleatoria(t *testing.T) string {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(k)
}

func TestCifrarDescifrar(t *testing.T) {
	c, err := New(claveAleatoria(t), "k1", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	aad := AADVersion("v-1", "a-1")
	texto := []byte("MARCADOR_CLINICO_TEST_123 — ñ, tildes y 😀")
	blob, kid, err := c.Encrypt(texto, aad)
	if err != nil || kid != "k1" {
		t.Fatalf("Encrypt: kid=%q err=%v", kid, err)
	}
	if blob[0] != 0x01 || len(blob) != 1+12+len(texto)+16 {
		t.Fatalf("formato inesperado: primer byte %x, largo %d", blob[0], len(blob))
	}
	if bytes.Contains(blob, []byte("MARCADOR")) {
		t.Fatal("el bloque cifrado contiene el texto claro")
	}
	got, err := c.Decrypt(blob, kid, aad)
	if err != nil || !bytes.Equal(got, texto) {
		t.Fatalf("Decrypt: %v", err)
	}
	otro, _, _ := c.Encrypt(texto, aad)
	if bytes.Equal(blob, otro) {
		t.Fatal("dos cifrados del mismo texto deben usar nonces distintos")
	}
}

func TestBloqueAlteradoOCopiadoSeRechaza(t *testing.T) {
	c, _ := New(claveAleatoria(t), "k1", "")
	aad := AADVersion("v-1", "a-1")
	blob, kid, _ := c.Encrypt([]byte("contenido"), aad)

	alterado := append([]byte(nil), blob...)
	alterado[len(alterado)-1] ^= 0xFF
	if _, err := c.Decrypt(alterado, kid, aad); !errors.Is(err, ErrDescifrado) {
		t.Fatalf("bloque alterado: se esperaba ErrDescifrado, got %v", err)
	}
	if _, err := c.Decrypt(blob, kid, AADVersion("v-2", "a-1")); !errors.Is(err, ErrDescifrado) {
		t.Fatalf("bloque copiado a otra fila: se esperaba ErrDescifrado, got %v", err)
	}
	if _, err := c.Decrypt(blob[:10], kid, aad); !errors.Is(err, ErrDescifrado) {
		t.Fatalf("bloque truncado: se esperaba ErrDescifrado, got %v", err)
	}
	sinVersion := append([]byte{0x02}, blob[1:]...)
	if _, err := c.Decrypt(sinVersion, kid, aad); !errors.Is(err, ErrDescifrado) {
		t.Fatalf("formato desconocido: se esperaba ErrDescifrado, got %v", err)
	}
}

func TestClaveIncorrectaYDesconocida(t *testing.T) {
	a, _ := New(claveAleatoria(t), "k1", "")
	b, _ := New(claveAleatoria(t), "k1", "") // mismo id, otra clave
	aad := AADCanario("k1")
	blob, kid, _ := a.Encrypt([]byte(TextoCanario), aad)
	if _, err := b.Decrypt(blob, kid, aad); !errors.Is(err, ErrDescifrado) {
		t.Fatalf("clave incorrecta: se esperaba ErrDescifrado, got %v", err)
	}
	if _, err := a.Decrypt(blob, "k9", aad); !errors.Is(err, ErrClaveDesconocida) {
		t.Fatalf("key_id desconocido: se esperaba ErrClaveDesconocida, got %v", err)
	}
}

func TestRotacionConClavesAnteriores(t *testing.T) {
	vieja := claveAleatoria(t)
	c1, _ := New(vieja, "k1", "")
	aad := AADResumenCaso("e-1")
	blob, kid, _ := c1.Encrypt([]byte("resumen"), aad)

	c2, err := New(claveAleatoria(t), "k2", "k1:"+vieja)
	if err != nil {
		t.Fatalf("New con anteriores: %v", err)
	}
	if got, err := c2.Decrypt(blob, kid, aad); err != nil || string(got) != "resumen" {
		t.Fatalf("una clave anterior debe seguir descifrando: %v", err)
	}
	if _, nuevo, _ := c2.Encrypt([]byte("x"), aad); nuevo != "k2" {
		t.Fatalf("se debe cifrar con la clave activa, got %q", nuevo)
	}
}

func TestConfiguracionInvalida(t *testing.T) {
	casos := map[string][3]string{
		"clave vacía":          {"", "k1", ""},
		"base64 inválido":      {"no-es-base64!!", "k1", ""},
		"clave corta":          {base64.StdEncoding.EncodeToString(make([]byte, 16)), "k1", ""},
		"key_id vacío":         {claveAleatoria(t), "", ""},
		"key_id inválido":      {claveAleatoria(t), "k 1/x", ""},
		"anterior sin id":      {claveAleatoria(t), "k1", claveAleatoria(t)},
		"anterior repetida":    {claveAleatoria(t), "k1", "k1:" + claveAleatoria(t)},
		"anterior mal formada": {claveAleatoria(t), "k1", "k0:corta"},
	}
	for nombre, c := range casos {
		if _, err := New(c[0], c[1], c[2]); !errors.Is(err, ErrConfiguracion) {
			t.Errorf("%s: se esperaba ErrConfiguracion, got %v", nombre, err)
		}
	}
}
