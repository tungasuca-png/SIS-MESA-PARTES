// Package clinicalcrypto cifra el contenido clínico de Psicología (FASE 6N-3,
// C1) con AES-256-GCM, solo con la biblioteca estándar.
//
// Formato de cada bloque cifrado (BYTEA en PostgreSQL):
//
//	0x01 ‖ nonce (12 bytes aleatorios) ‖ texto cifrado ‖ etiqueta GCM (16 bytes)
//
// El identificador de la clave (key_id) se guarda en una columna aparte.
// Cada bloque va ligado a su fila mediante datos autenticados (AAD): un bloque
// copiado a otra fila o alterado no se puede descifrar.
//
// Las claves solo viven en memoria del proceso de Psicología. Ningún error ni
// mensaje de este paquete incluye claves, texto claro ni texto cifrado.
package clinicalcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	formatoV1     byte = 0x01
	largoClave         = 32 // AES-256
	largoNonce         = 12
	largoEtiqueta      = 16
)

var (
	// ErrConfiguracion: clave ausente o mal formada. El servicio no debe iniciar.
	ErrConfiguracion = errors.New("configuración de cifrado clínico inválida")
	// ErrClaveDesconocida: el bloque se cifró con un key_id que no está configurado.
	ErrClaveDesconocida = errors.New("clave de cifrado clínico no configurada para este dato")
	// ErrDescifrado: bloque alterado, copiado a otra fila o clave incorrecta.
	ErrDescifrado = errors.New("no se pudo descifrar el contenido clínico")
)

var keyIDValido = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// Cipher cifra con la clave activa y descifra con la activa o con las anteriores.
type Cipher struct {
	activeID string
	aeads    map[string]cipher.AEAD
}

// New construye el cifrador. activeKey es la clave activa en base64 (32 bytes);
// activeID su identificador; previous (opcional) es "id:base64,id:base64" con
// claves anteriores, que solo se usan para descifrar durante una rotación.
func New(activeKey, activeID, previous string) (*Cipher, error) {
	c := &Cipher{aeads: map[string]cipher.AEAD{}}
	activeID = strings.TrimSpace(activeID)
	if err := c.agregar(activeID, activeKey); err != nil {
		return nil, err
	}
	c.activeID = activeID
	for _, par := range strings.Split(previous, ",") {
		par = strings.TrimSpace(par)
		if par == "" {
			continue
		}
		id, clave, ok := strings.Cut(par, ":")
		if !ok {
			return nil, fmt.Errorf("%w: CLINICAL_DATA_PREVIOUS_KEYS debe tener el formato id:clave", ErrConfiguracion)
		}
		if err := c.agregar(strings.TrimSpace(id), clave); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Cipher) agregar(id, claveB64 string) error {
	if !keyIDValido.MatchString(id) {
		return fmt.Errorf("%w: identificador de clave vacío o inválido", ErrConfiguracion)
	}
	if _, repetido := c.aeads[id]; repetido {
		return fmt.Errorf("%w: identificador de clave repetido", ErrConfiguracion)
	}
	clave, err := base64.StdEncoding.DecodeString(strings.TrimSpace(claveB64))
	if err != nil || len(clave) != largoClave {
		// Nunca se incluye la clave (ni su largo real) en el error.
		return fmt.Errorf("%w: la clave debe ser base64 de exactamente 32 bytes", ErrConfiguracion)
	}
	bloque, err := aes.NewCipher(clave)
	if err != nil {
		return fmt.Errorf("%w: clave inválida", ErrConfiguracion)
	}
	aead, err := cipher.NewGCM(bloque)
	if err != nil {
		return fmt.Errorf("%w: no se pudo inicializar AES-GCM", ErrConfiguracion)
	}
	c.aeads[id] = aead
	return nil
}

// ActiveKeyID devuelve el identificador de la clave con la que se cifra.
func (c *Cipher) ActiveKeyID() string { return c.activeID }

// KeyIDs devuelve todos los identificadores configurados (activa y anteriores).
func (c *Cipher) KeyIDs() []string {
	out := make([]string, 0, len(c.aeads))
	out = append(out, c.activeID)
	for id := range c.aeads {
		if id != c.activeID {
			out = append(out, id)
		}
	}
	return out
}

// Encrypt cifra plaintext con la clave activa ligándolo a aad. Devuelve el
// bloque (formato v1) y el key_id usado.
func (c *Cipher) Encrypt(plaintext, aad []byte) ([]byte, string, error) {
	aead := c.aeads[c.activeID]
	nonce := make([]byte, largoNonce)
	if _, err := rand.Read(nonce); err != nil {
		return nil, "", errors.New("no se pudo generar el nonce de cifrado")
	}
	out := make([]byte, 0, 1+largoNonce+len(plaintext)+largoEtiqueta)
	out = append(out, formatoV1)
	out = append(out, nonce...)
	out = aead.Seal(out, nonce, plaintext, aad)
	return out, c.activeID, nil
}

// Decrypt descifra un bloque cifrado con keyID y ligado a aad.
func (c *Cipher) Decrypt(blob []byte, keyID string, aad []byte) ([]byte, error) {
	aead, ok := c.aeads[keyID]
	if !ok {
		return nil, ErrClaveDesconocida
	}
	if len(blob) < 1+largoNonce+largoEtiqueta || blob[0] != formatoV1 {
		return nil, ErrDescifrado
	}
	nonce := blob[1 : 1+largoNonce]
	plaintext, err := aead.Open(nil, nonce, blob[1+largoNonce:], aad)
	if err != nil {
		return nil, ErrDescifrado
	}
	return plaintext, nil
}

// ---------------------------------------------------------------------------
// Datos autenticados (AAD): ligan cada bloque a su fila y campo. Se usan
// identificadores INMUTABLES; nunca numero_version, que cambia de 0 a 1 al
// registrar una atención.
// ---------------------------------------------------------------------------

func AADVersion(versionID, atencionID string) []byte {
	return []byte("psicologia/atencion_versiones/" + versionID + "/" + atencionID)
}

func AADMotivoAnulacion(atencionID string) []byte {
	return []byte("psicologia/atenciones_psicologicas/" + atencionID + "/motivo_anulacion")
}

func AADResumenCaso(expedienteID string) []byte {
	return []byte("psicologia/expedientes_psicologicos/" + expedienteID + "/resumen_caso")
}

func AADMotivoCierre(expedienteID string) []byte {
	return []byte("psicologia/expedientes_psicologicos/" + expedienteID + "/motivo_cierre")
}

func AADCanario(keyID string) []byte {
	return []byte("psicologia/cifrado_canario/" + keyID)
}

// TextoCanario es el valor conocido que se cifra en cifrado_canario para
// comprobar al arrancar que la clave configurada es la correcta.
const TextoCanario = "tungasuca/psicologia/canario/v1"
