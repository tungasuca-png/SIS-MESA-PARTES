package repository

import (
	"context"
	"fmt"
	"strings"

	"estudiantes/internal/datos"
)

// Consultas y escrituras del motor de datos (importación/exportación
// universal). Todo parametrizado; los nombres de columna son constantes.

// Identificadores devuelve los estudiantes que coinciden con alguno de los
// documentos, códigos o códigos SIAGIE dados.
func (r *EstudianteRepository) Identificadores(ctx context.Context, docs [][2]string, codigos, siagies []string) ([]datos.Registrado, error) {
	tipos := make([]string, len(docs))
	numeros := make([]string, len(docs))
	for i, d := range docs {
		tipos[i], numeros[i] = d[0], d[1]
	}
	if codigos == nil {
		codigos = []string{}
	}
	if siagies == nil {
		siagies = []string{}
	}
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.codigo, e.tipo_documento, COALESCE(e.numero_documento, ''), COALESCE(e.codigo_siagie, ''), e.apellidos, e.nombres
		FROM estudiantes e
		WHERE (e.tipo_documento, e.numero_documento) IN (SELECT * FROM unnest($1::text[], $2::text[]))
		   OR e.codigo = ANY($3::text[])
		   OR e.codigo_siagie = ANY($4::text[])
	`, tipos, numeros, codigos, siagies)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []datos.Registrado
	for rows.Next() {
		var e datos.Registrado
		if err := rows.Scan(&e.ID, &e.Codigo, &e.TipoDocumento, &e.NumeroDocumento, &e.CodigoSiagie, &e.Apellidos, &e.Nombres); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CoincidenciasNombre: para cada nombre completo ("apellidos nombres"),
// cuántos estudiantes registrados tienen el mismo nombre normalizado
// (estudiantes_normalizar: sin tildes, minúsculas). Solo alimenta avisos.
func (r *EstudianteRepository) CoincidenciasNombre(ctx context.Context, nombres []string) (map[string]int, error) {
	out := map[string]int{}
	if len(nombres) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `
		WITH registrados AS (
			SELECT estudiantes_normalizar(apellidos || ' ' || nombres) AS k, count(*) AS n
			FROM estudiantes GROUP BY 1
		)
		SELECT x, registrados.n FROM unnest($1::text[]) AS x
		JOIN registrados ON registrados.k = estudiantes_normalizar(x)
	`, nombres)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var nombre string
		var n int
		if err := rows.Scan(&nombre, &n); err != nil {
			return nil, err
		}
		out[nombre] = n
	}
	return out, rows.Err()
}

// Secciones: combinaciones nivel/grado/sección definidas en cualquier periodo.
func (r *EstudianteRepository) Secciones(ctx context.Context) ([]datos.SeccionCatalogo, error) {
	rows, err := r.db.Query(ctx, `SELECT DISTINCT nivel, grado, seccion FROM secciones`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []datos.SeccionCatalogo
	for rows.Next() {
		var s datos.SeccionCatalogo
		if err := rows.Scan(&s.Nivel, &s.Grado, &s.Seccion); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CrearLote registra varios estudiantes en UNA transacción, con su auditoría
// CREATE cada uno. Si cualquiera falla (p. ej. otro usuario registró el mismo
// documento mientras tanto), no se guarda ninguno: la importación nunca queda
// a medias. Devuelve los ids en el mismo orden.
func (r *EstudianteRepository) CrearLote(ctx context.Context, lote []Maestro, aud Auditoria, campos func(Maestro) []string) ([]string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ids := make([]string, 0, len(lote))
	for i, m := range lote {
		var id string
		var version int
		err := tx.QueryRow(ctx, `
			INSERT INTO estudiantes (codigo, codigo_siagie, tipo_documento, numero_documento, nombres,
				apellido_paterno, apellido_materno, apellidos, estado, nivel, grado, seccion, turno)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id, version
		`, m.Codigo, m.CodigoSiagie, m.TipoDocumento, m.NumeroDocumento, m.Nombres,
			m.ApellidoPaterno, m.ApellidoMaterno, m.Apellidos, m.Estado, m.Nivel, m.Grado, m.Seccion, m.Turno).Scan(&id, &version)
		if err != nil {
			return nil, &ErrLote{Indice: i, Err: classifyWriteError(err)}
		}
		a := aud
		a.Campos = campos(m)
		if err := insertAuditoria(ctx, tx, id, version, a); err != nil {
			return nil, &ErrLote{Indice: i, Err: err}
		}
		ids = append(ids, id)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, classifyWriteError(err)
	}
	return ids, nil
}

// ErrLote indica qué registro del lote hizo fallar la transacción.
type ErrLote struct {
	Indice int
	Err    error
}

func (e *ErrLote) Error() string { return fmt.Sprintf("registro %d del lote: %v", e.Indice, e.Err) }
func (e *ErrLote) Unwrap() error { return e.Err }

// ExportarDatos recorre los estudiantes que cumplen el MISMO filtro del
// directorio (condicionesBusqueda) y en el mismo orden, con su matrícula del
// periodo EN_CURSO más reciente (una por estudiante: uq_matriculas_estudiante_periodo).
// Si el total supera limite no recorre nada y devuelve ErrExportDemasiados.
func (r *EstudianteRepository) ExportarDatos(ctx context.Context, filter SearchFilter, limite int, fn func(datos.FilaMaestro) error) (int, error) {
	args := []any{}
	param := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	where := ""
	if conds := condicionesBusqueda(filter, param); len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM estudiantes `+where, args...).Scan(&total); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, nil
	}
	if limite > 0 && total > limite {
		return total, ErrExportDemasiados
	}
	orden, ok := OrdenesBusqueda[filter.Orden]
	if !ok {
		orden = OrdenesBusqueda[""]
	}
	rows, err := r.db.Query(ctx, `
		WITH filtrados AS (
			SELECT e.*, row_number() OVER (ORDER BY `+orden+`) AS rn FROM estudiantes e `+where+`
		), vigente AS (
			SELECT id, anio FROM periodos_academicos WHERE estado = 'EN_CURSO' ORDER BY anio DESC LIMIT 1
		)
		SELECT f.codigo, COALESCE(f.codigo_siagie, ''), f.tipo_documento, COALESCE(f.numero_documento, ''),
		       COALESCE(f.apellido_paterno, ''), COALESCE(f.apellido_materno, ''), f.apellidos, f.nombres, f.estado,
		       f.nivel, f.grado, f.seccion, f.turno,
		       COALESCE(v.anio, 0), COALESCE(s.nivel, ''), COALESCE(s.grado, ''), COALESCE(s.seccion, ''),
		       COALESCE(s.turno, ''), COALESCE(m.estado_matricula, '')
		FROM filtrados f
		LEFT JOIN vigente v ON TRUE
		LEFT JOIN matriculas m ON m.estudiante_id = f.id AND m.periodo_id = v.id
		LEFT JOIN secciones s ON s.id = m.seccion_id
		ORDER BY f.rn
	`, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var x datos.FilaMaestro
		if err := rows.Scan(&x.Codigo, &x.CodigoSiagie, &x.TipoDocumento, &x.NumeroDocumento, &x.ApellidoPaterno,
			&x.ApellidoMaterno, &x.Apellidos, &x.Nombres, &x.Estado, &x.Nivel, &x.Grado, &x.Seccion, &x.Turno,
			&x.MatPeriodo, &x.MatNivel, &x.MatGrado, &x.MatSeccion, &x.MatTurno, &x.EstadoMatricula); err != nil {
			return 0, err
		}
		if x.EstadoMatricula == "" {
			x.MatPeriodo = 0 // hay periodo vigente, pero el estudiante no está matriculado en él
		}
		if err := fn(x); err != nil {
			return 0, err
		}
		n++
	}
	return n, rows.Err()
}
