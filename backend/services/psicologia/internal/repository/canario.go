package repository

import (
	"context"
	"errors"
	"fmt"

	"psicologia/internal/clinicalcrypto"

	"github.com/jackc/pgx/v5"
)

// CipherConClaves: cifrador que además informa qué claves tiene configuradas.
type CipherConClaves interface {
	Cifrador
	ActiveKeyID() string
	KeyIDs() []string
}

type querierRow interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// VerificarCanario comprueba, para CADA clave configurada (activa y
// anteriores), que su fila en cifrado_canario existe y se descifra al valor
// conocido. Psicología lo ejecuta al arrancar y no inicia si falla: una clave
// ausente, incorrecta o no aprovisionada nunca llega a leer ni escribir datos.
// Los errores nombran el key_id, nunca la clave.
func VerificarCanario(ctx context.Context, db querierRow, c CipherConClaves) error {
	for _, keyID := range c.KeyIDs() {
		var blob []byte
		err := db.QueryRow(ctx, `SELECT canario FROM cifrado_canario WHERE key_id = $1`, keyID).Scan(&blob)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no existe canario para la clave %q: aprovisionarlo con cmd/cifrar-contenido --inicializar-canario", keyID)
		}
		if err != nil {
			return fmt.Errorf("no se pudo leer el canario de cifrado: %w", err)
		}
		plano, err := c.Decrypt(blob, keyID, clinicalcrypto.AADCanario(keyID))
		if err != nil || string(plano) != clinicalcrypto.TextoCanario {
			return fmt.Errorf("la clave configurada como %q no descifra su canario: clave incorrecta", keyID)
		}
	}
	return nil
}

// InicializarCanario crea el canario de la clave ACTIVA si no existe (lo usa
// cmd/cifrar-contenido; el servicio nunca crea canarios). Si ya existe, lo
// verifica: nunca lo reemplaza.
func InicializarCanario(ctx context.Context, tx pgx.Tx, c CipherConClaves) (creado bool, err error) {
	keyID := c.ActiveKeyID()
	var existe bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cifrado_canario WHERE key_id = $1)`, keyID).Scan(&existe); err != nil {
		return false, err
	}
	if existe {
		return false, VerificarCanario(ctx, tx, soloActiva{c})
	}
	blob, _, err := c.Encrypt([]byte(clinicalcrypto.TextoCanario), clinicalcrypto.AADCanario(keyID))
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO cifrado_canario (key_id, canario) VALUES ($1, $2)`, keyID, blob)
	return err == nil, err
}

// soloActiva restringe la verificación a la clave activa.
type soloActiva struct{ CipherConClaves }

func (s soloActiva) KeyIDs() []string { return []string{s.ActiveKeyID()} }
