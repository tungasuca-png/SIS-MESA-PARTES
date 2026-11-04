package repository

import (
	"context"
	"fmt"
	"strings"
)

// ZonaReportes: los reportes cuentan días y meses en hora de Perú
// (fecha_registro se guarda como TIMESTAMPTZ). PostgreSQL resuelve la zona
// con su propia base tz.
const ZonaReportes = "America/Lima"

// EstadosPendientes: expedientes abiertos dentro de la institución (aún sin
// atender). OBSERVADO no se incluye: espera la corrección del solicitante.
var EstadosPendientes = []string{"PENDIENTE", "EN_PROCESO"}

// ReporteFilter: filtros ya validados. Desde/Hasta en AAAA-MM-DD (inclusivos,
// sobre fecha_registro en ZonaReportes); Hoy, la fecha de referencia para la
// antigüedad de los pendientes. AreaAlcance es la restricción por rol
// (independiente del filtro Area elegido por el usuario).
type ReporteFilter struct {
	Desde, Hasta                  string
	Estado, Tipo, Prioridad, Area string
	AreaAlcance                   string
	Hoy                           string
	// Fase 2 (opcionales): agrupar también por día de registro y por usuario
	// registrador. Sin ellos, la consulta agrupa igual que en la Fase 1.
	ConDia, ConRegistrador bool
}

// ReporteFila: un grupo de la consulta agregada. DiasPendiente es nil para
// expedientes que no están pendientes.
type ReporteFila struct {
	Estado, Tipo, Prioridad, Area, Mes string
	DiasPendiente                      *int
	Cantidad                           int
	// Fase 2: "" si no se pidió.
	Dia, Registrador string
}

// construirConsultaReporte arma UNA consulta agregada (GROUP BY) con todos los
// filtros como parámetros. Devuelve también los argumentos en orden.
func construirConsultaReporte(f ReporteFilter) (string, []any) {
	condiciones := []string{"activo = TRUE"}
	args := []any{}
	param := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	igual := func(columna, valor string) {
		if valor != "" {
			condiciones = append(condiciones, columna+" = "+param(valor))
		}
	}
	igual("estado", f.Estado)
	igual("tipo", f.Tipo)
	igual("prioridad", f.Prioridad)
	igual("area_actual", f.Area)
	igual("area_actual", f.AreaAlcance)
	if f.Desde != "" {
		// Inicio del día en Lima, convertido a instante.
		condiciones = append(condiciones, "fecha_registro >= ("+param(f.Desde)+"::date)::timestamp AT TIME ZONE '"+ZonaReportes+"'")
	}
	if f.Hasta != "" {
		// Hasta inclusivo: antes del inicio del día siguiente.
		condiciones = append(condiciones, "fecha_registro < ("+param(f.Hasta)+"::date + 1)::timestamp AT TIME ZONE '"+ZonaReportes+"'")
	}

	local := "(fecha_registro AT TIME ZONE '" + ZonaReportes + "')"
	pendientes := make([]string, len(EstadosPendientes))
	for i, e := range EstadosPendientes {
		pendientes[i] = "'" + e + "'"
	}
	hoy := param(f.Hoy)
	dia, registrador := "''", "''"
	if f.ConDia {
		dia = "to_char(" + local + "::date, 'YYYY-MM-DD')"
	}
	if f.ConRegistrador {
		registrador = "solicitante_id::text"
	}
	consulta := `
		SELECT estado, tipo, prioridad, area_actual,
			to_char(date_trunc('month', ` + local + `), 'YYYY-MM') AS mes,
			CASE WHEN estado IN (` + strings.Join(pendientes, ", ") + `)
				THEN (` + hoy + `::date - ` + local + `::date) END AS dias_pendiente,
			COUNT(*)::int AS cantidad,
			` + dia + ` AS dia, ` + registrador + ` AS registrador
		FROM expedientes
		WHERE ` + strings.Join(condiciones, " AND ") + `
		GROUP BY 1, 2, 3, 4, 5, 6, 8, 9`
	return consulta, args
}

// Reporte devuelve los grupos de la consulta agregada (una sola ida a la
// base). La lógica arma con ellos cada agrupación.
func (r *ExpedienteRepository) Reporte(ctx context.Context, f ReporteFilter) ([]ReporteFila, error) {
	consulta, args := construirConsultaReporte(f)
	rows, err := r.db.Query(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReporteFila{}
	for rows.Next() {
		var fila ReporteFila
		if err := rows.Scan(&fila.Estado, &fila.Tipo, &fila.Prioridad, &fila.Area, &fila.Mes, &fila.DiasPendiente, &fila.Cantidad, &fila.Dia, &fila.Registrador); err != nil {
			return nil, err
		}
		out = append(out, fila)
	}
	return out, rows.Err()
}

// ExpedienteRef: identificador y tipo de un expediente (sin otros datos).
type ExpedienteRef struct {
	ID, Tipo string
}

// ReporteAlcance devuelve los expedientes ACTIVOS (opcionalmente de un tipo y
// del área de alcance del rol). Solo id y tipo.
func (r *ExpedienteRepository) ReporteAlcance(ctx context.Context, tipo, areaAlcance string) ([]ExpedienteRef, error) {
	condiciones := []string{"activo = TRUE"}
	args := []any{}
	if tipo != "" {
		args = append(args, tipo)
		condiciones = append(condiciones, fmt.Sprintf("tipo = $%d", len(args)))
	}
	if areaAlcance != "" {
		args = append(args, areaAlcance)
		condiciones = append(condiciones, fmt.Sprintf("area_actual = $%d", len(args)))
	}
	rows, err := r.db.Query(ctx, "SELECT id::text, tipo FROM expedientes WHERE "+strings.Join(condiciones, " AND "), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExpedienteRef{}
	for rows.Next() {
		var e ExpedienteRef
		if err := rows.Scan(&e.ID, &e.Tipo); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
