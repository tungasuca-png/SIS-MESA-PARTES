package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Prueba del SQL del reporte contra PostgreSQL. Inserta y BORRA expedientes,
// así que exige una base AISLADA y desechable: REPORTES_DB_AISLADA_URL (nunca
// la base real). Sin la variable, se omite.
func TestReporte_SQLReal(t *testing.T) {
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
	if _, err := db.Exec(ctx, "DELETE FROM expedientes"); err != nil {
		t.Fatal(err)
	}

	lima := time.FixedZone("Lima", -5*3600)
	insertar := func(codigo, tipo, estado, prioridad, area string, registro time.Time, activo bool) {
		t.Helper()
		_, err := db.Exec(ctx, `INSERT INTO expedientes (codigo, tipo, asunto, solicitante_id, estado, prioridad, area_actual, fecha_registro, activo)
			VALUES ($1, $2, 'Asunto de prueba', gen_random_uuid(), $3, $4, $5, $6, $7)`, codigo, tipo, estado, prioridad, area, registro, activo)
		if err != nil {
			t.Fatalf("insertar %s: %v", codigo, err)
		}
	}
	// 31/01 23:30 en Lima = 01/02 04:30 UTC: debe contarse en ENERO.
	insertar("T-1", "CONSTANCIA", "PENDIENTE", "NORMAL", "SECRETARIA", time.Date(2026, 1, 31, 23, 30, 0, 0, lima), true)
	insertar("T-2", "PERMISO", "EN_PROCESO", "URGENTE", "DIRECTOR", time.Date(2026, 2, 1, 0, 30, 0, 0, lima), true)
	insertar("T-3", "PERMISO", "ATENDIDO", "NORMAL", "DOCENTE", time.Date(2026, 2, 10, 10, 0, 0, 0, lima), true)
	insertar("T-4", "OTRO", "OBSERVADO", "NORMAL", "SECRETARIA", time.Date(2026, 3, 5, 10, 0, 0, 0, lima), true)
	insertar("T-5", "CONSTANCIA", "PENDIENTE", "NORMAL", "SECRETARIA", time.Date(2026, 3, 5, 10, 0, 0, 0, lima), false) // baja lógica
	defer db.Exec(context.Background(), "DELETE FROM expedientes WHERE codigo LIKE 'T-%'")

	repo := NewExpedienteRepository(db)
	contar := func(f ReporteFilter) (total int, porMes map[string]int, dias map[string]int) {
		t.Helper()
		f.Hoy = "2026-03-02"
		filas, err := repo.Reporte(ctx, f)
		if err != nil {
			t.Fatalf("%+v: %v", f, err)
		}
		porMes, dias = map[string]int{}, map[string]int{}
		for _, fila := range filas {
			total += fila.Cantidad
			porMes[fila.Mes] += fila.Cantidad
			if fila.DiasPendiente != nil {
				dias[fila.Estado] = *fila.DiasPendiente
			}
		}
		return
	}

	if total, porMes, dias := contar(ReporteFilter{}); total != 4 || porMes["2026-01"] != 1 || porMes["2026-02"] != 2 || porMes["2026-03"] != 1 {
		t.Fatalf("sin filtros (sin bajas, meses en hora de Lima): total=%d meses=%v", total, porMes)
	} else if dias["PENDIENTE"] != 30 || dias["EN_PROCESO"] != 29 || len(dias) != 2 {
		// 31/01 -> 02/03 = 30 días; 01/02 -> 02/03 = 29 días; ATENDIDO/OBSERVADO sin antigüedad.
		t.Fatalf("antigüedad de pendientes: %v", dias)
	}
	// Hasta inclusivo y límite de día en Lima.
	if total, _, _ := contar(ReporteFilter{Desde: "2026-01-31", Hasta: "2026-01-31"}); total != 1 {
		t.Fatalf("31/01 (hora de Lima) debe incluir solo T-1: %d", total)
	}
	if total, _, _ := contar(ReporteFilter{Desde: "2026-02-01", Hasta: "2026-02-10"}); total != 2 {
		t.Fatalf("febrero hasta el 10 inclusive: %d", total)
	}
	// Filtros individuales y combinados, y alcance por rol.
	for nombre, c := range map[string]struct {
		f     ReporteFilter
		total int
	}{
		"estado":              {ReporteFilter{Estado: "PENDIENTE"}, 1},
		"tipo":                {ReporteFilter{Tipo: "PERMISO"}, 2},
		"prioridad":           {ReporteFilter{Prioridad: "URGENTE"}, 1},
		"área":                {ReporteFilter{Area: "SECRETARIA"}, 2},
		"combinado":           {ReporteFilter{Tipo: "PERMISO", Estado: "ATENDIDO", Desde: "2026-02-01"}, 1},
		"alcance DIRECTOR":    {ReporteFilter{AreaAlcance: "DIRECTOR"}, 1},
		"alcance y otra área": {ReporteFilter{AreaAlcance: "DIRECTOR", Area: "SECRETARIA"}, 0},
		"sin resultados":      {ReporteFilter{Desde: "2026-04-01"}, 0},
	} {
		if total, _, _ := contar(c.f); total != c.total {
			t.Errorf("%s: total=%d, se esperaba %d", nombre, total, c.total)
		}
	}

	// Fase 2: día de registro (hora de Lima) y registrador opcionales.
	filas, err := repo.Reporte(ctx, ReporteFilter{Hoy: "2026-03-02", ConDia: true, ConRegistrador: true})
	if err != nil {
		t.Fatal(err)
	}
	porDia := map[string]int{}
	for _, f := range filas {
		porDia[f.Dia] += f.Cantidad
		if f.Registrador == "" {
			t.Fatal("registrador vacío con ConRegistrador")
		}
	}
	if porDia["2026-01-31"] != 1 || porDia["2026-02-01"] != 1 || porDia["2026-03-05"] != 1 {
		t.Fatalf("días en hora de Lima: %v", porDia)
	}
	sinOpc, _ := repo.Reporte(ctx, ReporteFilter{Hoy: "2026-03-02"})
	for _, f := range sinOpc {
		if f.Dia != "" || f.Registrador != "" {
			t.Fatal("sin opciones, día y registrador deben venir vacíos (Fase 1)")
		}
	}
	// Alcance: solo activos, por tipo y área.
	refs, err := repo.ReporteAlcance(ctx, "", "")
	if err != nil || len(refs) != 4 {
		t.Fatalf("alcance (sin bajas): %d %v", len(refs), err)
	}
	if refs, _ := repo.ReporteAlcance(ctx, "PERMISO", "DIRECTOR"); len(refs) != 1 || refs[0].Tipo != "PERMISO" {
		t.Fatalf("alcance filtrado: %v", refs)
	}
}
