package repository

import (
	"context"
	"fmt"
	"strings"

	"expedientes/reportes"
)

// ReporteDocFila: un grupo de la consulta agregada (sin nombres de archivo).
type ReporteDocFila struct {
	ExpedienteID, TipoDocumento, Dia, Registrador string
	Cantidad                                      int
}

// ReporteDocFiltro: ExpedienteIDs = expedientes ACTIVOS y visibles (lo
// entrega Expedientes); vacío = sin resultados.
type ReporteDocFiltro struct {
	ExpedienteIDs  []string
	Desde, Hasta   string
	ConRegistrador bool
}

func construirConsultaReporteDoc(f ReporteDocFiltro) (string, []any) {
	args := []any{f.ExpedienteIDs}
	condiciones := []string{"estado = 'ACTIVO'", "expediente_id = ANY($1::uuid[])"}
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
		registrador = "subido_por::text"
	}
	return `
		SELECT expediente_id::text, tipo_documento,
			to_char((fecha_registro AT TIME ZONE '` + reportes.ZonaSQL + `')::date, 'YYYY-MM-DD') AS dia,
			` + registrador + ` AS registrador,
			COUNT(*)::int
		FROM documentos
		WHERE ` + strings.Join(condiciones, " AND ") + `
		GROUP BY 1, 2, 3, 4`, args
}

// Reporte: una sola consulta agregada sobre la base de Documentos.
func (r *DocumentoRepository) Reporte(ctx context.Context, f ReporteDocFiltro) ([]ReporteDocFila, error) {
	if len(f.ExpedienteIDs) == 0 {
		return []ReporteDocFila{}, nil
	}
	consulta, args := construirConsultaReporteDoc(f)
	rows, err := r.db.Query(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReporteDocFila{}
	for rows.Next() {
		var fila ReporteDocFila
		if err := rows.Scan(&fila.ExpedienteID, &fila.TipoDocumento, &fila.Dia, &fila.Registrador, &fila.Cantidad); err != nil {
			return nil, err
		}
		out = append(out, fila)
	}
	return out, rows.Err()
}
