package repository

import (
	"context"
	"fmt"
	"strings"

	"expedientes/reportes"
)

// ReporteDerivFila: un grupo de la consulta agregada del reporte. Origen y
// destino llegan CRUDOS (texto histórico) y la lógica los normaliza; nunca se
// devuelven tal cual.
type ReporteDerivFila struct {
	ExpedienteID, Tipo, Origen, Destino, Dia, Registrador string
	Cantidad                                              int
}

// ReporteDerivFiltro: filtros ya validados. ExpedienteIDs = expedientes
// ACTIVOS y visibles (lo entrega Expedientes); vacío = sin resultados.
type ReporteDerivFiltro struct {
	ExpedienteIDs  []string
	Desde, Hasta   string
	ConRegistrador bool
}

func construirConsultaReporteDeriv(f ReporteDerivFiltro) (string, []any) {
	args := []any{f.ExpedienteIDs}
	condiciones := []string{"expediente_id = ANY($1::uuid[])"}
	if f.Desde != "" {
		args = append(args, f.Desde)
		condiciones = append(condiciones, fmt.Sprintf("fecha_registro >= ($%d::date)::timestamp AT TIME ZONE '%s'", len(args), reportes.ZonaSQL))
	}
	if f.Hasta != "" {
		args = append(args, f.Hasta)
		condiciones = append(condiciones, fmt.Sprintf("fecha_registro < ($%d::date + 1)::timestamp AT TIME ZONE '%s'", len(args), reportes.ZonaSQL))
	}
	registrador := "''"
	if f.ConRegistrador {
		registrador = "registrado_por::text"
	}
	return `
		SELECT expediente_id::text, tipo, origen, destino,
			to_char((fecha_registro AT TIME ZONE '` + reportes.ZonaSQL + `')::date, 'YYYY-MM-DD') AS dia,
			` + registrador + ` AS registrador,
			COUNT(*)::int
		FROM derivaciones
		WHERE ` + strings.Join(condiciones, " AND ") + `
		GROUP BY 1, 2, 3, 4, 5, 6`, args
}

// Reporte: una sola consulta agregada sobre la base de Derivaciones.
func (r *DerivacionRepository) Reporte(ctx context.Context, f ReporteDerivFiltro) ([]ReporteDerivFila, error) {
	if len(f.ExpedienteIDs) == 0 {
		return []ReporteDerivFila{}, nil
	}
	consulta, args := construirConsultaReporteDeriv(f)
	rows, err := r.db.Query(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReporteDerivFila{}
	for rows.Next() {
		var fila ReporteDerivFila
		if err := rows.Scan(&fila.ExpedienteID, &fila.Tipo, &fila.Origen, &fila.Destino, &fila.Dia, &fila.Registrador, &fila.Cantidad); err != nil {
			return nil, err
		}
		out = append(out, fila)
	}
	return out, rows.Err()
}
