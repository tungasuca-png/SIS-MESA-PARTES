package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
)

// Códigos de verificación del registro manual: 6 dígitos de crypto/rand
// (uniformes, sin sesgo de módulo). En la base solo queda
// HMAC-SHA256(clave, id_del_registro ":" código); la clave deriva de
// JWT_SECRET y nunca se guarda, así que una copia de la base no permite
// probar el millón de códigos posibles fuera de línea.

const etiquetaCodigos = "sis-mesa-partes/registro-correo/v1"

var maxCodigo = big.NewInt(1_000_000)

// GenerarCodigoVerificacion devuelve un código de exactamente 6 dígitos.
func GenerarCodigoVerificacion() (string, error) {
	n, err := rand.Int(rand.Reader, maxCodigo)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// NuevoIDRegistro: UUID v4 aleatorio (el hash del código se liga a él, así
// que se genera antes de insertar la fila).
func NuevoIDRegistro() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// CodigosVerificacion calcula y compara los HMAC.
type CodigosVerificacion struct {
	clave []byte
}

func NewCodigosVerificacion(secreto string) (*CodigosVerificacion, error) {
	// Mismo requisito que el login (JWT_SECRET obligatorio): no se añade una
	// condición de arranque nueva.
	if secreto == "" {
		return nil, errors.New("JWT_SECRET no está configurado")
	}
	m := hmac.New(sha256.New, []byte(secreto))
	m.Write([]byte(etiquetaCodigos))
	return &CodigosVerificacion{clave: m.Sum(nil)}, nil
}

// Hash: 32 bytes ligados al registro.
func (c *CodigosVerificacion) Hash(registroID, codigo string) []byte {
	m := hmac.New(sha256.New, c.clave)
	m.Write([]byte(registroID))
	m.Write([]byte{':'})
	m.Write([]byte(codigo))
	return m.Sum(nil)
}

// Coincide compara en tiempo constante.
func (c *CodigosVerificacion) Coincide(hash []byte, registroID, codigo string) bool {
	return hmac.Equal(hash, c.Hash(registroID, codigo))
}
