package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DNI declarado y verificado en los perfiles (migración 003), contra un
// PostgreSQL DESECHABLE (USUARIOS_DB_AISLADA_URL; ver
// scratchpad/usuarios-aislado.sh). Nunca contra usuarios_db real.

func baseAislada(t *testing.T) (*UsuarioRepository, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("USUARIOS_DB_AISLADA_URL")
	if url == "" {
		t.Skip("USUARIOS_DB_AISLADA_URL no definida: solo corre contra un PostgreSQL desechable")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewUsuarioRepository(pool), pool
}

func uuidNuevo() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func dniNuevo() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(89999999))
	return fmt.Sprintf("%08d", 10000000+n.Int64())
}

func perfil(t *testing.T, r *UsuarioRepository, tipo, dni, declarado string) string {
	t.Helper()
	id := uuidNuevo()
	if _, _, err := r.CreateIfNotExists(context.Background(), &Usuario{ID: id, Nombres: "Ana", Apellidos: "Quispe",
		DNI: dni, TipoUsuario: tipo, DNIDeclarado: declarado}); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestDNIDeclaradoYVerificado(t *testing.T) {
	r, _ := baseAislada(t)
	ctx := context.Background()
	dni := dniNuevo()

	// Declarar no reserva: dos solicitantes declaran el mismo DNI.
	a := perfil(t, r, "SOLICITANTE", "", dni)
	b := perfil(t, r, "SOLICITANTE", "", dni)
	pa, _ := r.FindByID(ctx, a)
	if pa.DNI != "" || pa.DNIDeclarado != dni || !pa.IdentidadPendiente() {
		t.Fatalf("perfil declarado: %+v", pa)
	}

	// Un perfil institucional con otro DNI: nunca se toca.
	dniInst := dniNuevo()
	inst := perfil(t, r, "SECRETARIA", dniInst, "")
	c := perfil(t, r, "SOLICITANTE", "", dniInst)
	if _, err := r.AsignarDNIVerificado(ctx, c, dniInst); !errors.Is(err, ErrDniExists) {
		t.Fatalf("DNI de un perfil institucional: %v", err)
	}
	if pc, _ := r.FindByID(ctx, c); pc.DNI != "" {
		t.Fatal("el conflicto no deja cambios")
	}
	if pi, _ := r.FindByID(ctx, inst); pi.DNI != dniInst || pi.IdentidadPendiente() {
		t.Fatal("el perfil institucional no cambia")
	}
	// Solo perfiles SOLICITANTE y solo el DNI declarado.
	if _, err := r.AsignarDNIVerificado(ctx, inst, dniInst); !errors.Is(err, ErrDNINoAsignable) {
		t.Fatalf("perfil institucional: %v", err)
	}
	if _, err := r.AsignarDNIVerificado(ctx, a, dniNuevo()); !errors.Is(err, ErrDNINoAsignable) {
		t.Fatalf("DNI no declarado: %v", err)
	}
	if _, err := r.AsignarDNIVerificado(ctx, uuidNuevo(), dni); !errors.Is(err, ErrUsuarioNotFound) {
		t.Fatalf("perfil inexistente: %v", err)
	}

	// Concurrencia: a y b compiten por el mismo DNI; gana exactamente uno.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, id := range []string{a, b} {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, errs[i] = r.AsignarDNIVerificado(ctx, id, dni)
		}(i, id)
	}
	wg.Wait()
	ganador := ""
	for i, err := range errs {
		switch {
		case err == nil:
			ganador = []string{a, b}[i]
		case !errors.Is(err, ErrDniExists):
			t.Fatalf("error inesperado: %v", err)
		}
	}
	if ganador == "" || (errs[0] == nil) == (errs[1] == nil) {
		t.Fatalf("exactamente uno debe ganar: %v", errs)
	}
	// Idempotente para el ganador; la reversión vuelve a dejarlo pendiente.
	if u, err := r.AsignarDNIVerificado(ctx, ganador, dni); err != nil || u.DNI != dni || u.IdentidadPendiente() {
		t.Fatalf("asignación repetida: %v", err)
	}
	if u, err := r.RevertirDNIVerificado(ctx, ganador, dni); err != nil || u.DNI != "" || !u.IdentidadPendiente() {
		t.Fatalf("reversión: %+v %v", u, err)
	}
	// Revertir otra vez o sobre un institucional: no cambia nada.
	if _, err := r.RevertirDNIVerificado(ctx, ganador, dni); err != nil {
		t.Fatalf("reversión repetida: %v", err)
	}
	if u, _ := r.RevertirDNIVerificado(ctx, inst, dniInst); u.DNI != dniInst {
		t.Fatal("la reversión nunca toca un perfil institucional")
	}
}
