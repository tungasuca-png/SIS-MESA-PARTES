package repository

import (
	"context"
)

// Modelo maestro, etapa 1 — resolución exacta y posibles duplicados.

// GetByCodigo resuelve por código institucional EXACTO (único).
func (r *EstudianteRepository) GetByCodigo(ctx context.Context, codigo string) (*Estudiante, error) {
	found, err := scanEstudiante(r.db.QueryRow(ctx, `SELECT `+estudianteColumns+` FROM estudiantes WHERE codigo = $1`, codigo))
	if err != nil {
		return nil, classifyNotFound(err)
	}
	return found, nil
}

// GetByDocumento resuelve por tipo y número de documento EXACTOS (únicos
// cuando el número existe: uq_estudiantes_documento).
func (r *EstudianteRepository) GetByDocumento(ctx context.Context, tipo, numero string) (*Estudiante, error) {
	found, err := scanEstudiante(r.db.QueryRow(ctx, `SELECT `+estudianteColumns+`
		FROM estudiantes WHERE tipo_documento = $1 AND numero_documento = $2`, tipo, numero))
	if err != nil {
		return nil, classifyNotFound(err)
	}
	return found, nil
}

const (
	CriterioMismoNombreYFecha       = "MISMO_NOMBRE_Y_FECHA"
	CriterioMismoNombreSinDocumento = "MISMO_NOMBRE_SIN_DOCUMENTO"
)

// GrupoDuplicado: estudiantes que PODRÍAN ser la misma persona. Solo para
// revisión humana: nunca se fusiona nada automáticamente.
type GrupoDuplicado struct {
	Criterio string
	IDs      []string
}

// posiblesDuplicadosSQL agrupa por el nombre completo NORMALIZADO (mismo
// criterio que la búsqueda: sin tildes, minúsculas, espacios colapsados):
//   - MISMO_NOMBRE_Y_FECHA: además, misma fecha de nacimiento;
//   - MISMO_NOMBRE_SIN_DOCUMENTO: al menos uno sin documento, donde la
//     unicidad por documento no puede proteger.
//
// El nombre normalizado solo se usa para agrupar y ordenar; no se devuelve.
const posiblesDuplicadosSQL = `
	WITH base AS (
		SELECT id, estudiantes_normalizar(nombres || ' ' || apellidos) AS nombre, fecha_nacimiento, tipo_documento
		FROM estudiantes
	), grupos AS (
		SELECT 'MISMO_NOMBRE_Y_FECHA'::text AS criterio, nombre, array_agg(id::text ORDER BY id) AS ids
		FROM base WHERE fecha_nacimiento IS NOT NULL
		GROUP BY nombre, fecha_nacimiento HAVING count(*) > 1
		UNION ALL
		SELECT 'MISMO_NOMBRE_SIN_DOCUMENTO'::text, nombre, array_agg(id::text ORDER BY id)
		FROM base GROUP BY nombre
		HAVING count(*) > 1 AND bool_or(tipo_documento = 'SIN_DOCUMENTO')
	)
`

// ListPosiblesDuplicados devuelve una página de grupos (orden estable) y el
// total de grupos.
func (r *EstudianteRepository) ListPosiblesDuplicados(ctx context.Context, page, pageSize int) ([]GrupoDuplicado, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, posiblesDuplicadosSQL+`SELECT count(*) FROM grupos`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, posiblesDuplicadosSQL+`
		SELECT criterio, ids FROM grupos ORDER BY criterio, nombre, ids LIMIT $1 OFFSET $2
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	grupos := make([]GrupoDuplicado, 0)
	for rows.Next() {
		var g GrupoDuplicado
		if err := rows.Scan(&g.Criterio, &g.IDs); err != nil {
			return nil, 0, err
		}
		grupos = append(grupos, g)
	}
	return grupos, total, rows.Err()
}
