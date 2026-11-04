package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConsultaReporteDeriv(t *testing.T) {
	sql, args := construirConsultaReporteDeriv(ReporteDerivFiltro{ExpedienteIDs: []string{"a"}, Desde: "2026-01-01", Hasta: "2026-01-31", ConRegistrador: true})
	for _, frag := range []string{"expediente_id = ANY($1::uuid[])", "fecha_registro >= ($2::date)::timestamp AT TIME ZONE 'America/Lima'",
		"fecha_registro < ($3::date + 1)::timestamp AT TIME ZONE 'America/Lima'", "registrado_por::text", "GROUP BY 1, 2, 3, 4, 5, 6"} {
		if !strings.Contains(sql, frag) {
			t.Errorf("falta %q", frag)
		}
	}
	if len(args) != 3 || strings.Contains(sql, "2026-01-01") {
		t.Fatal("los valores viajan como parámetros")
	}
	sinReg, _ := construirConsultaReporteDeriv(ReporteDerivFiltro{ExpedienteIDs: []string{"a"}})
	if strings.Contains(sinReg, "registrado_por") {
		t.Fatal("sin incluir_registros no se agrupa por usuario")
	}
}

// SQL real contra una base AISLADA y desechable (REPORTES_DB_AISLADA_URL con
// las migraciones de Derivaciones). Inserta y borra filas: nunca la base real.
func TestReporteDeriv_SQLReal(t *testing.T) {
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
	if _, err := db.Exec(ctx, "DELETE FROM derivaciones"); err != nil {
		t.Fatal(err)
	}
	const activo, deBaja = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	lima := time.FixedZone("Lima", -5*3600)
	insertar := func(exp, tipo, origen, destino string, cuando time.Time) {
		t.Helper()
		if _, err := db.Exec(ctx, `INSERT INTO derivaciones (expediente_id, tipo, origen, destino, motivo, registrado_por, fecha_registro)
			VALUES ($1, $2, $3, $4, 'motivo de prueba', gen_random_uuid(), $5)`, exp, tipo, origen, destino, cuando); err != nil {
			t.Fatal(err)
		}
	}
	insertar(activo, "DERIVACION", "SECRETARIA", "DIRECTOR", time.Date(2026, 1, 31, 23, 30, 0, 0, lima))
	insertar(activo, "DERIVACION", "Secretaría", "Dirección", time.Date(2026, 2, 2, 10, 0, 0, 0, lima))
	insertar(activo, "NOTIFICACION", "usuario.prueba (Docente)", "Secretaría", time.Date(2026, 2, 3, 10, 0, 0, 0, lima))
	insertar(deBaja, "DERIVACION", "SECRETARIA", "DIRECTOR", time.Date(2026, 2, 3, 10, 0, 0, 0, lima))

	repo := NewDerivacionRepository(db)
	filas, err := repo.Reporte(ctx, ReporteDerivFiltro{ExpedienteIDs: []string{activo}, ConRegistrador: true})
	if err != nil {
		t.Fatal(err)
	}
	total, dias := 0, map[string]int{}
	for _, f := range filas {
		if f.ExpedienteID != activo {
			t.Fatal("una derivación de un expediente fuera del alcance (dado de baja) no debe incluirse")
		}
		if f.Registrador == "" {
			t.Fatal("registrador vacío con ConRegistrador")
		}
		total += f.Cantidad
		dias[f.Dia] += f.Cantidad
	}
	if total != 3 || dias["2026-01-31"] != 1 {
		t.Fatalf("total=%d días=%v (31/01 23:30 en Lima es enero)", total, dias)
	}
	enero, _ := repo.Reporte(ctx, ReporteDerivFiltro{ExpedienteIDs: []string{activo}, Desde: "2026-01-01", Hasta: "2026-01-31"})
	if len(enero) != 1 {
		t.Fatalf("hasta inclusivo en hora de Lima: %d", len(enero))
	}
	if vacio, _ := repo.Reporte(ctx, ReporteDerivFiltro{}); len(vacio) != 0 {
		t.Fatal("sin expedientes visibles: vacío")
	}
	_, _ = db.Exec(context.Background(), "DELETE FROM derivaciones")
}
