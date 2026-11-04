package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConsultaReporteDoc(t *testing.T) {
	sql, args := construirConsultaReporteDoc(ReporteDocFiltro{ExpedienteIDs: []string{"a"}, Desde: "2026-01-01", ConRegistrador: true})
	for _, frag := range []string{"estado = 'ACTIVO'", "expediente_id = ANY($1::uuid[])", "fecha_registro >= ($2::date)::timestamp AT TIME ZONE 'America/Lima'", "subido_por::text"} {
		if !strings.Contains(sql, frag) {
			t.Errorf("falta %q", frag)
		}
	}
	if len(args) != 2 || strings.Contains(sql, "2026-01-01") {
		t.Fatal("los valores viajan como parámetros")
	}
	sinReg, _ := construirConsultaReporteDoc(ReporteDocFiltro{ExpedienteIDs: []string{"a"}})
	if strings.Contains(sinReg, "subido_por") {
		t.Fatal("sin incluir_registros no se agrupa por usuario")
	}
}

// SQL real contra una base AISLADA y desechable (REPORTES_DB_AISLADA_URL con
// las migraciones de Documentos). Inserta y borra filas: nunca la base real.
func TestReporteDoc_SQLReal(t *testing.T) {
	url := os.Getenv("REPORTES_DB_AISLADA_URL")
	if url == "" {
		t.Skip("REPORTES_DB_AISLADA_URL no definido (requiere una base aislada y desechable)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(ctx, "DELETE FROM documentos"); err != nil {
		t.Fatal(err)
	}
	const activo, deBaja = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	insertar := func(exp, tipo, estado string) {
		t.Helper()
		if _, err := db.Exec(ctx, `INSERT INTO documentos (expediente_id, nombre, tipo_documento, extension, tamano_bytes, subido_por, estado, contenido)
			VALUES ($1, 'archivo-de-prueba.pdf', $2, 'pdf', 10, gen_random_uuid(), $3, decode('00', 'hex'))`, exp, tipo, estado); err != nil {
			t.Fatal(err)
		}
	}
	insertar(activo, "ADJUNTO", "ACTIVO")
	insertar(activo, "PROVEIDO", "ACTIVO")
	insertar(activo, "ADJUNTO", "ELIMINADO")
	insertar(deBaja, "ADJUNTO", "ACTIVO")

	filas, err := NewDocumentoRepository(db).Reporte(ctx, ReporteDocFiltro{ExpedienteIDs: []string{activo}})
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, f := range filas {
		if f.ExpedienteID != activo {
			t.Fatal("documento de un expediente fuera del alcance")
		}
		total += f.Cantidad
	}
	if total != 2 {
		t.Fatalf("solo documentos ACTIVOS de expedientes visibles: %d", total)
	}
	_, _ = db.Exec(context.Background(), "DELETE FROM documentos")
}
