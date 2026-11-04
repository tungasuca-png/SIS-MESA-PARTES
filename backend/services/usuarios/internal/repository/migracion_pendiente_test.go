package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Base SIN la migración 003 (dni_declarado), como una instalación que todavía
// no la aplicó: los perfiles se leen y escriben como antes y las operaciones
// del DNI declarado responden ErrEsquemaPendiente. Luego aplica la 003 en la
// MISMA base desechable y comprueba que se detecta sin reiniciar. Solo con
// USUARIOS_DB_SIN_003=1 (scratchpad/usuarios-aislado.sh sin003).
func TestMigracion003Pendiente_PerfilesSiguenFuncionando(t *testing.T) {
	if os.Getenv("USUARIOS_DB_SIN_003") != "1" {
		t.Skip("solo con USUARIOS_DB_SIN_003=1 (base sin la migración 003)")
	}
	r, pool := baseAislada(t)
	ctx := context.Background()
	if r.tieneDeclarado(ctx) {
		t.Fatal("la base de esta prueba no debe tener la columna dni_declarado")
	}
	// Preparación del registro público: NO lista sin la 003 (el Gateway
	// responde 503 antes de que Auth cree ninguna cuenta).
	if err := r.RegistroPublicoDisponible(ctx); err == nil {
		t.Fatal("sin la 003 el registro público no debe estar disponible")
	}

	// Lectura y escritura de perfiles como antes de la 003.
	inst := perfil(t, r, "SECRETARIA", dniNuevo(), "")
	sol := perfil(t, r, "SOLICITANTE", dniNuevo(), "")
	if u, err := r.FindByID(ctx, inst); err != nil || u.DNI == "" || u.DNIDeclarado != "" || u.IdentidadPendiente() {
		t.Fatalf("FindByID: %+v %v", u, err)
	}
	if lista, total, err := r.List(ctx, ListFilter{Page: 1, PageSize: 100}); err != nil || total < 2 || len(lista) < 2 {
		t.Fatalf("List: %d %v", total, err)
	}
	if lista, err := r.FindManyByIDs(ctx, []string{inst, sol}); err != nil || len(lista) != 2 {
		t.Fatalf("FindManyByIDs: %v", err)
	}
	if _, err := r.UpdateOwnProfile(ctx, sol, "987654321", "s@correo.example", "Av. Siempre Viva"); err != nil {
		t.Fatalf("UpdateOwnProfile: %v", err)
	}
	if _, err := r.UpdateTipoUsuario(ctx, inst, "DIRECTOR"); err != nil {
		t.Fatalf("UpdateTipoUsuario: %v", err)
	}
	if _, err := r.Upsert(ctx, &Usuario{ID: uuidNuevo(), Nombres: "Luis", Apellidos: "Mamani", DNI: dniNuevo(), TipoUsuario: "DOCENTE"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Operaciones del DNI declarado: controladas, sin escribir nada.
	id := uuidNuevo()
	if _, _, err := r.CreateIfNotExists(ctx, &Usuario{ID: id, Nombres: "Ana", Apellidos: "Quispe", TipoUsuario: "SOLICITANTE", DNIDeclarado: dniNuevo()}); !errors.Is(err, ErrEsquemaPendiente) {
		t.Fatalf("crear con DNI declarado sin la 003: %v", err)
	}
	if _, err := r.FindByID(ctx, id); !errors.Is(err, ErrUsuarioNotFound) {
		t.Fatal("no debe haberse creado el perfil")
	}
	if _, err := r.AsignarDNIVerificado(ctx, sol, dniNuevo()); !errors.Is(err, ErrEsquemaPendiente) {
		t.Fatalf("asignar sin la 003: %v", err)
	}
	if _, err := r.RevertirDNIVerificado(ctx, sol, dniNuevo()); !errors.Is(err, ErrEsquemaPendiente) {
		t.Fatalf("revertir sin la 003: %v", err)
	}

	// Se aplica la 003 en esta base desechable: se detecta sin reiniciar
	// (pasada la ventana de 30 s; aquí se adelanta el reloj de la caché).
	sql, err := os.ReadFile("../../migrations/003_add_dni_declarado.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("aplicar 003: %v", err)
	}
	r.esquemaMu.Lock()
	r.comprobado = time.Now().Add(-time.Minute)
	r.esquemaMu.Unlock()
	if !r.tieneDeclarado(ctx) {
		t.Fatal("tras aplicar la 003, la columna debe detectarse")
	}
	// La preparación se comprueba en cada llamada (sin caché ni reinicio).
	if err := r.RegistroPublicoDisponible(ctx); err != nil {
		t.Fatalf("tras la 003 el registro público debe estar disponible: %v", err)
	}
	declarado := dniNuevo()
	if _, _, err := r.CreateIfNotExists(ctx, &Usuario{ID: id, Nombres: "Ana", Apellidos: "Quispe", TipoUsuario: "SOLICITANTE", DNIDeclarado: declarado}); err != nil {
		t.Fatalf("crear con DNI declarado tras la 003: %v", err)
	}
	if u, err := r.FindByID(ctx, id); err != nil || u.DNIDeclarado != declarado || !u.IdentidadPendiente() {
		t.Fatalf("perfil con DNI declarado: %+v %v", u, err)
	}
}
