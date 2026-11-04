package repository

import (
	"strings"
	"testing"
)

func TestConstruirConsultaReporte_FiltrosIndividualesYCombinados(t *testing.T) {
	sql, args := construirConsultaReporte(ReporteFilter{Hoy: "2026-10-30"})
	if !strings.Contains(sql, "activo = TRUE") || len(args) != 1 || args[0] != "2026-10-30" {
		t.Fatalf("sin filtros: solo activos y la fecha de referencia: %v", args)
	}
	if !strings.Contains(sql, "GROUP BY 1, 2, 3, 4, 5, 6") {
		t.Fatal("debe ser una sola consulta agregada")
	}

	casos := []struct {
		nombre string
		f      ReporteFilter
		frag   []string
	}{
		{"estado", ReporteFilter{Estado: "PENDIENTE"}, []string{"estado = $1"}},
		{"tipo", ReporteFilter{Tipo: "CONSTANCIA"}, []string{"tipo = $1"}},
		{"prioridad", ReporteFilter{Prioridad: "URGENTE"}, []string{"prioridad = $1"}},
		{"área", ReporteFilter{Area: "DIRECTOR"}, []string{"area_actual = $1"}},
		{"alcance por rol", ReporteFilter{AreaAlcance: "DOCENTE"}, []string{"area_actual = $1"}},
		{"desde", ReporteFilter{Desde: "2026-01-01"}, []string{"fecha_registro >= ($1::date)::timestamp AT TIME ZONE 'America/Lima'"}},
		{"hasta inclusivo", ReporteFilter{Hasta: "2026-01-31"}, []string{"fecha_registro < ($1::date + 1)::timestamp AT TIME ZONE 'America/Lima'"}},
		{"combinados", ReporteFilter{Estado: "EN_PROCESO", Tipo: "PERMISO", Prioridad: "URGENTE", Area: "DIRECTOR", AreaAlcance: "DIRECTOR", Desde: "2026-01-01", Hasta: "2026-06-30"},
			[]string{"estado = $1", "tipo = $2", "prioridad = $3", "area_actual = $4", "area_actual = $5", "fecha_registro >= ($6", "fecha_registro < ($7"}},
	}
	for _, c := range casos {
		c.f.Hoy = "2026-10-30"
		sql, args := construirConsultaReporte(c.f)
		for _, frag := range c.frag {
			if !strings.Contains(sql, frag) {
				t.Errorf("%s: falta %q en la consulta", c.nombre, frag)
			}
		}
		if len(args) != len(c.frag)+1 {
			t.Errorf("%s: %d argumentos, se esperaban %d", c.nombre, len(args), len(c.frag)+1)
		}
		// Los valores del usuario viajan como parámetros, nunca dentro del SQL.
		for _, v := range []string{"CONSTANCIA", "PERMISO", "DIRECTOR", "DOCENTE", "URGENTE", "2026-01-01", "2026-06-30"} {
			if strings.Contains(sql, v) {
				t.Errorf("%s: el valor %s aparece literal en la consulta", c.nombre, v)
			}
		}
	}
}

func TestConstruirConsultaReporte_PendientesYMeses(t *testing.T) {
	sql, _ := construirConsultaReporte(ReporteFilter{Hoy: "2026-10-30"})
	for _, frag := range []string{
		"CASE WHEN estado IN ('PENDIENTE', 'EN_PROCESO')",
		"($1::date - (fecha_registro AT TIME ZONE 'America/Lima')::date)",
		"to_char(date_trunc('month', (fecha_registro AT TIME ZONE 'America/Lima')), 'YYYY-MM')",
	} {
		if !strings.Contains(sql, frag) {
			t.Errorf("falta %q", frag)
		}
	}
}
