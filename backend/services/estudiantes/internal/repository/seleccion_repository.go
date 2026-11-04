package repository

import (
	"context"
	"fmt"
	"strings"
)

// Búsqueda MÍNIMA para seleccionar un estudiante (sistemas consumidores con
// estudiantes.search.basic). Independiente de Search: no usa texto_busqueda
// (que incluye código interno y SIAGIE) ni devuelve otros datos.
//
//   - Términos de nombre: deben aparecer (sin tildes ni mayúsculas) en nombres
//     o apellidos, incluidos los apellidos antiguos combinados.
//   - Términos DNI: coincidencia EXACTA del DNI completo (tipo DNI); nunca
//     CE/pasaporte ni coincidencias parciales.
//   - Solo registros ACTIVO. Todos los términos deben cumplirse.
//   - SQL parametrizado; la consulta nunca se registra.

type Seleccion struct {
	ID             string
	NombreCompleto string
	Grado          string
	Seccion        string
}

type SeleccionFilter struct {
	Nombres  []string // términos de nombre/apellido
	DNIs     []string // DNI completos (8 dígitos), ya validados
	Page     int
	PageSize int
}

const exprNombreSeleccion = `estudiantes_normalizar(nombres || ' ' || apellidos || ' ' || ` +
	`COALESCE(apellido_paterno, '') || ' ' || COALESCE(apellido_materno, ''))`

func condicionesSeleccion(f SeleccionFilter, param func(any) string) []string {
	conds := []string{`estado = 'ACTIVO'`}
	for _, dni := range f.DNIs {
		conds = append(conds, `(tipo_documento = 'DNI' AND numero_documento = `+param(dni)+`)`)
	}
	for _, term := range f.Nombres {
		conds = append(conds, exprNombreSeleccion+` LIKE '%' || estudiantes_normalizar(`+param(likeEscaper.Replace(term))+`) || '%'`)
	}
	return conds
}

func (r *EstudianteRepository) BuscarSeleccion(ctx context.Context, f SeleccionFilter) ([]Seleccion, int, error) {
	args := make([]any, 0, len(f.Nombres)+len(f.DNIs)+2)
	param := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	where := "WHERE " + strings.Join(condicionesSeleccion(f, param), " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM estudiantes `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := param(f.PageSize)
	offset := param((f.Page - 1) * f.PageSize)
	rows, err := r.db.Query(ctx, `
		SELECT id::text, nombres, apellidos, grado, seccion
		FROM estudiantes `+where+`
		ORDER BY apellidos, nombres, id
		LIMIT `+limit+` OFFSET `+offset, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Seleccion, 0)
	for rows.Next() {
		var s Seleccion
		e := Estudiante{}
		if err := rows.Scan(&s.ID, &e.Nombres, &e.Apellidos, &s.Grado, &s.Seccion); err != nil {
			return nil, 0, err
		}
		s.NombreCompleto = e.NombreCompleto()
		out = append(out, s)
	}
	return out, total, rows.Err()
}
