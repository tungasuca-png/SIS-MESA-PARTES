package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateIfNotExists contra un PostgreSQL DESECHABLE con las migraciones de
// Usuarios (REPORTES_DB_AISLADA_URL; ver scratchpad/reportes-aislado-srv.sh).
func TestCreateIfNotExists_SQLReal(t *testing.T) {
	url := os.Getenv("REPORTES_DB_AISLADA_URL")
	if url == "" {
		t.Skip("REPORTES_DB_AISLADA_URL no definida: solo corre contra un PostgreSQL desechable")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	r := NewUsuarioRepository(pool)
	sufijo := time.Now().UnixNano() % 1e7
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", sufijo)
	dni := fmt.Sprintf("7%07d", sufijo)

	u, creado, err := r.CreateIfNotExists(ctx, &Usuario{ID: id, Nombres: "Ana", Apellidos: "Quispe", DNI: dni, Correo: "ana@gmail.example", TipoUsuario: "SOLICITANTE"})
	if err != nil || !creado || u.DNI != dni || u.TipoUsuario != "SOLICITANTE" {
		t.Fatalf("crear: %+v %v %v", u, creado, err)
	}
	// Reintento (doble envío): devuelve el existente y NO sobrescribe.
	u2, creado, err := r.CreateIfNotExists(ctx, &Usuario{ID: id, Nombres: "Otro", Apellidos: "Nombre", DNI: dni, TipoUsuario: "SOLICITANTE"})
	if err != nil || creado || u2.Nombres != "Ana" {
		t.Fatalf("reintento: %+v %v %v", u2, creado, err)
	}
	// Otro usuario con el mismo DNI: lo rechaza el índice único.
	otro := fmt.Sprintf("00000000-0000-4000-9000-%012d", sufijo)
	if _, _, err := r.CreateIfNotExists(ctx, &Usuario{ID: otro, Nombres: "X", Apellidos: "Y", DNI: dni, TipoUsuario: "SOLICITANTE"}); !errors.Is(err, ErrDniExists) {
		t.Fatalf("DNI duplicado: %v", err)
	}
}
