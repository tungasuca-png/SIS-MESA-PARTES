package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Modelo maestro — MATRÍCULA. Etapa 3: lectura (historial académico).
// Etapa 5: escritura de periodos, secciones y matrículas. Cada operación
// sobre una matrícula deja un movimiento append-only y una fila de auditoría
// MATRICULA_UPDATE en el estudiante, en la MISMA transacción. Nada se borra:
// un cambio de sección, un retiro o un reingreso nunca sobrescribe el
// historial (queda en matricula_movimientos).

const AccionMatriculaUpdate = "MATRICULA_UPDATE"

// Estados (CHECK de la migración 007; estados D-M4 aprobados).
const (
	PeriodoPlanificado = "PLANIFICADO"
	PeriodoEnCurso     = "EN_CURSO"
	PeriodoCerrado     = "CERRADO"

	MatriculaMatriculado = "MATRICULADO"
	MatriculaRetirado    = "RETIRADO"
	MatriculaTrasladado  = "TRASLADADO"
	MatriculaConcluido   = "CONCLUIDO"
)

// Tipos de movimiento (CHECK de 007 + CONCLUSION de 009).
const (
	MovCambioSeccion  = "CAMBIO_SECCION"
	MovCambioTurno    = "CAMBIO_TURNO"
	MovRetiro         = "RETIRO"
	MovReingreso      = "REINGRESO"
	MovTrasladoSalida = "TRASLADO_SALIDA"
	MovConclusion     = "CONCLUSION"
)

var (
	ErrPeriodoNotFound        = errors.New("periodo not found")
	ErrPeriodoDuplicado       = errors.New("periodo duplicado")
	ErrPeriodoCerrado         = errors.New("periodo cerrado")
	ErrTransicionInvalida     = errors.New("transicion invalida")
	ErrSeccionNotFound        = errors.New("seccion not found")
	ErrSeccionDuplicada       = errors.New("seccion duplicada")
	ErrSinVacantes            = errors.New("sin vacantes")
	ErrMatriculaNotFound      = errors.New("matricula not found")
	ErrMatriculaDuplicada     = errors.New("matricula duplicada")
	ErrEstudianteInactivo     = errors.New("estudiante inactivo")
	ErrMatriculaVersion       = errors.New("matricula version conflict")
	ErrFechaInconsistente     = errors.New("fecha inconsistente")
	ErrSinCambios             = errors.New("sin cambios")
)

type Periodo struct {
	ID          string
	Anio        int
	Nombre      string
	Estado      string
	FechaInicio *string
	FechaFin    *string
}

type Seccion struct {
	ID           string
	PeriodoID    string
	Nivel        string
	Grado        string
	Seccion      string
	Turno        *string
	Aula         *string
	Vacantes     *int
	Matriculados int
}

type Movimiento struct {
	Tipo              string
	Fecha             string
	SeccionAnteriorID *string
	SeccionNuevaID    *string
}

type Matricula struct {
	ID              string
	EstudianteID    string
	Periodo         Periodo
	Seccion         *Seccion // nil = sin sección asignada
	EstadoMatricula string
	FechaMatricula  *string
	FechaRetiro     *string
	Origen          string
	Version         int
	Movimientos     []Movimiento
}

// querier: lo común de pgxpool.Pool y pgx.Tx que usan las lecturas
// compartidas (dentro o fuera de una transacción).
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// ---- Periodos ----

const periodoColumns = `id, anio, nombre, estado, to_char(fecha_inicio, 'YYYY-MM-DD'), to_char(fecha_fin, 'YYYY-MM-DD')`

func scanPeriodo(row pgx.Row) (*Periodo, error) {
	p := &Periodo{}
	var anio int16
	if err := row.Scan(&p.ID, &anio, &p.Nombre, &p.Estado, &p.FechaInicio, &p.FechaFin); err != nil {
		return nil, err
	}
	p.Anio = int(anio)
	return p, nil
}

type PeriodoInput struct {
	Anio        int
	Nombre      string
	FechaInicio *string
	FechaFin    *string
}

// CreatePeriodo registra un periodo PLANIFICADO (uno por año).
func (r *EstudianteRepository) CreatePeriodo(ctx context.Context, in PeriodoInput) (*Periodo, error) {
	p, err := scanPeriodo(r.db.QueryRow(ctx, `
		INSERT INTO periodos_academicos (anio, nombre, fecha_inicio, fecha_fin)
		VALUES ($1, $2, $3::date, $4::date)
		RETURNING `+periodoColumns, in.Anio, in.Nombre, in.FechaInicio, in.FechaFin))
	if err != nil {
		return nil, classifyMatriculaError(err)
	}
	return p, nil
}

func (r *EstudianteRepository) ListPeriodos(ctx context.Context) ([]Periodo, error) {
	rows, err := r.db.Query(ctx, `SELECT `+periodoColumns+` FROM periodos_academicos ORDER BY anio DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Periodo, 0)
	for rows.Next() {
		p, err := scanPeriodo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// siguienteEstadoPeriodo: el ciclo de un periodo solo avanza.
var siguienteEstadoPeriodo = map[string]string{
	PeriodoPlanificado: PeriodoEnCurso,
	PeriodoEnCurso:     PeriodoCerrado,
}

// ChangeEstadoPeriodo avanza el estado (PLANIFICADO -> EN_CURSO -> CERRADO).
// Cualquier otro cambio (retroceder, saltar, repetir) es ErrTransicionInvalida.
func (r *EstudianteRepository) ChangeEstadoPeriodo(ctx context.Context, id, estado string) (*Periodo, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var actual string
	if err := tx.QueryRow(ctx, `SELECT estado FROM periodos_academicos WHERE id = $1 FOR UPDATE`, id).Scan(&actual); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPeriodoNotFound
		}
		return nil, err
	}
	if siguienteEstadoPeriodo[actual] != estado {
		return nil, ErrTransicionInvalida
	}
	p, err := scanPeriodo(tx.QueryRow(ctx, `UPDATE periodos_academicos SET estado = $2 WHERE id = $1 RETURNING `+periodoColumns, id, estado))
	if err != nil {
		return nil, classifyMatriculaError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

// estadoPeriodo lee el estado con FOR SHARE: impide que el periodo cambie de
// estado (p. ej. se cierre) mientras dura la operación.
func estadoPeriodo(ctx context.Context, tx pgx.Tx, id string) (string, error) {
	var estado string
	if err := tx.QueryRow(ctx, `SELECT estado FROM periodos_academicos WHERE id = $1 FOR SHARE`, id).Scan(&estado); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrPeriodoNotFound
		}
		return "", err
	}
	return estado, nil
}

// ---- Secciones ----

type SeccionInput struct {
	PeriodoID string
	Nivel     string
	Grado     string
	Seccion   string
	Turno     *string
	Aula      *string
	Vacantes  *int
}

const seccionColumns = `s.id, s.periodo_id, s.nivel, s.grado, s.seccion, s.turno, s.aula, s.vacantes`

func scanSeccion(row pgx.Row, extra ...any) (*Seccion, error) {
	s := &Seccion{}
	var vacantes *int16
	dest := append([]any{&s.ID, &s.PeriodoID, &s.Nivel, &s.Grado, &s.Seccion, &s.Turno, &s.Aula, &vacantes}, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	if vacantes != nil {
		v := int(*vacantes)
		s.Vacantes = &v
	}
	return s, nil
}

// CreateSeccion registra una sección en un periodo que no esté CERRADO.
func (r *EstudianteRepository) CreateSeccion(ctx context.Context, in SeccionInput) (*Seccion, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	estado, err := estadoPeriodo(ctx, tx, in.PeriodoID)
	if err != nil {
		return nil, err
	}
	if estado == PeriodoCerrado {
		return nil, ErrPeriodoCerrado
	}
	s, err := scanSeccion(tx.QueryRow(ctx, `
		INSERT INTO secciones AS s (periodo_id, nivel, grado, seccion, turno, aula, vacantes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+seccionColumns, in.PeriodoID, in.Nivel, in.Grado, in.Seccion, in.Turno, in.Aula, in.Vacantes))
	if err != nil {
		return nil, classifyMatriculaError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, classifyMatriculaError(err)
	}
	return s, nil
}

// ListSecciones devuelve las secciones del periodo con el número de
// matrículas MATRICULADO de cada una.
func (r *EstudianteRepository) ListSecciones(ctx context.Context, periodoID string) ([]Seccion, error) {
	var existe bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM periodos_academicos WHERE id = $1)`, periodoID).Scan(&existe); err != nil {
		return nil, err
	}
	if !existe {
		return nil, ErrPeriodoNotFound
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+seccionColumns+`,
			(SELECT count(*) FROM matriculas m WHERE m.seccion_id = s.id AND m.estado_matricula = 'MATRICULADO')
		FROM secciones s
		WHERE s.periodo_id = $1
		ORDER BY array_position(ARRAY['INICIAL', 'PRIMARIA', 'SECUNDARIA']::varchar[], s.nivel), s.grado, s.seccion, s.turno NULLS FIRST, s.id
	`, periodoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Seccion, 0)
	for rows.Next() {
		var n int64
		s, err := scanSeccion(rows, &n)
		if err != nil {
			return nil, err
		}
		s.Matriculados = int(n)
		out = append(out, *s)
	}
	return out, rows.Err()
}

// reservarSeccion bloquea la sección (FOR UPDATE: serializa las altas
// simultáneas en la misma sección), verifica que sea del periodo indicado y,
// si tiene límite de vacantes, que quede al menos una. La matrícula que se
// está moviendo no cuenta (aún no es MATRICULADO en esa sección).
func reservarSeccion(ctx context.Context, tx pgx.Tx, seccionID, periodoID string) (*Seccion, error) {
	s, err := scanSeccion(tx.QueryRow(ctx, `SELECT `+seccionColumns+` FROM secciones s WHERE s.id = $1 FOR UPDATE`, seccionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSeccionNotFound
		}
		return nil, err
	}
	if s.PeriodoID != periodoID {
		// Una sección de otro periodo no existe para esta matrícula.
		return nil, ErrSeccionNotFound
	}
	if s.Vacantes != nil {
		var ocupadas int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM matriculas WHERE seccion_id = $1 AND estado_matricula = 'MATRICULADO'`,
			seccionID).Scan(&ocupadas); err != nil {
			return nil, err
		}
		if ocupadas >= *s.Vacantes {
			return nil, ErrSinVacantes
		}
	}
	return s, nil
}

// ---- Matrículas: lectura ----

const matriculaSelect = `
	SELECT m.id, m.estudiante_id, p.id, p.anio, p.nombre, p.estado, to_char(p.fecha_inicio, 'YYYY-MM-DD'), to_char(p.fecha_fin, 'YYYY-MM-DD'),
		s.id, s.periodo_id, s.nivel, s.grado, s.seccion, s.turno, s.aula, s.vacantes,
		m.estado_matricula, to_char(m.fecha_matricula, 'YYYY-MM-DD'), to_char(m.fecha_retiro, 'YYYY-MM-DD'),
		m.origen, m.version
	FROM matriculas m
	JOIN periodos_academicos p ON p.id = m.periodo_id
	LEFT JOIN secciones s ON s.id = m.seccion_id
`

func scanMatricula(row pgx.Row) (*Matricula, error) {
	var m Matricula
	var sID, sPeriodo, sNivel, sGrado, sSeccion, sTurno, sAula *string
	var sVacantes *int16
	var anio int16
	if err := row.Scan(&m.ID, &m.EstudianteID, &m.Periodo.ID, &anio, &m.Periodo.Nombre, &m.Periodo.Estado,
		&m.Periodo.FechaInicio, &m.Periodo.FechaFin,
		&sID, &sPeriodo, &sNivel, &sGrado, &sSeccion, &sTurno, &sAula, &sVacantes,
		&m.EstadoMatricula, &m.FechaMatricula, &m.FechaRetiro, &m.Origen, &m.Version); err != nil {
		return nil, err
	}
	m.Periodo.Anio = int(anio)
	if sID != nil {
		m.Seccion = &Seccion{ID: *sID, PeriodoID: *sPeriodo, Nivel: *sNivel, Grado: *sGrado, Seccion: *sSeccion, Turno: sTurno, Aula: sAula}
		if sVacantes != nil {
			v := int(*sVacantes)
			m.Seccion.Vacantes = &v
		}
	}
	return &m, nil
}

// cargarMovimientos agrega a cada matrícula sus movimientos en orden
// cronológico.
func cargarMovimientos(ctx context.Context, q querier, matriculas []Matricula) error {
	if len(matriculas) == 0 {
		return nil
	}
	idx := make(map[string]int, len(matriculas))
	ids := make([]string, 0, len(matriculas))
	for i, m := range matriculas {
		idx[m.ID] = i
		ids = append(ids, m.ID)
	}
	rows, err := q.Query(ctx, `
		SELECT matricula_id, tipo, to_char(fecha, 'YYYY-MM-DD'), seccion_anterior_id::text, seccion_nueva_id::text
		FROM matricula_movimientos
		WHERE matricula_id = ANY($1)
		ORDER BY created_at, id
	`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var matriculaID string
		var mv Movimiento
		if err := rows.Scan(&matriculaID, &mv.Tipo, &mv.Fecha, &mv.SeccionAnteriorID, &mv.SeccionNuevaID); err != nil {
			return err
		}
		i := idx[matriculaID]
		matriculas[i].Movimientos = append(matriculas[i].Movimientos, mv)
	}
	return rows.Err()
}

func getMatricula(ctx context.Context, q querier, id string) (*Matricula, error) {
	m, err := scanMatricula(q.QueryRow(ctx, matriculaSelect+` WHERE m.id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMatriculaNotFound
		}
		return nil, err
	}
	list := []Matricula{*m}
	if err := cargarMovimientos(ctx, q, list); err != nil {
		return nil, err
	}
	return &list[0], nil
}

// ListMatriculas devuelve el historial académico del estudiante: una
// matrícula por periodo (más reciente primero) con sus movimientos en orden
// cronológico. ErrEstudianteNotFound si el estudiante no existe (una lista
// vacía significa "sin matrículas registradas").
func (r *EstudianteRepository) ListMatriculas(ctx context.Context, estudianteID string) ([]Matricula, error) {
	var existe bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM estudiantes WHERE id = $1)`, estudianteID).Scan(&existe); err != nil {
		return nil, err
	}
	if !existe {
		return nil, ErrEstudianteNotFound
	}

	rows, err := r.db.Query(ctx, matriculaSelect+`
		WHERE m.estudiante_id = $1
		ORDER BY p.anio DESC, m.id
	`, estudianteID)
	if err != nil {
		return nil, err
	}
	matriculas := make([]Matricula, 0)
	for rows.Next() {
		m, err := scanMatricula(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		matriculas = append(matriculas, *m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := cargarMovimientos(ctx, r.db, matriculas); err != nil {
		return nil, err
	}
	return matriculas, nil
}

// NominaFilter: SeccionID y Estado vacíos = sin filtro.
type NominaFilter struct {
	PeriodoID string
	SeccionID string
	Estado    string
	Page      int
	PageSize  int
}

// ListMatriculasPeriodo: nómina de un periodo (sin movimientos), ordenada por
// apellidos y nombres del estudiante (cada matrícula trae su EstudianteID).
func (r *EstudianteRepository) ListMatriculasPeriodo(ctx context.Context, f NominaFilter) ([]Matricula, int, error) {
	var existe bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM periodos_academicos WHERE id = $1)`, f.PeriodoID).Scan(&existe); err != nil {
		return nil, 0, err
	}
	if !existe {
		return nil, 0, ErrPeriodoNotFound
	}
	where := ` WHERE m.periodo_id = $1 AND ($2 = '' OR m.seccion_id::text = $2) AND ($3 = '' OR m.estado_matricula = $3)`
	args := []any{f.PeriodoID, f.SeccionID, f.Estado}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM matriculas m`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, matriculaSelect+` JOIN estudiantes e ON e.id = m.estudiante_id`+where+`
		ORDER BY e.apellidos, e.nombres, m.id
		LIMIT $4 OFFSET $5`, append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Matricula, 0)
	for rows.Next() {
		m, err := scanMatricula(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *m)
	}
	return out, total, rows.Err()
}

// ---- Matrículas: escritura ----

// MatriculaInput: SeccionID nil = sin sección asignada todavía.
type MatriculaInput struct {
	EstudianteID   string
	PeriodoID      string
	SeccionID      *string
	FechaMatricula string
}

// MatricularEstudiante registra la matrícula del estudiante en un periodo.
// Exige estudiante con registro ACTIVO, periodo no CERRADO, sección del mismo
// periodo con vacantes y que no exista otra matrícula en ese periodo.
func (r *EstudianteRepository) MatricularEstudiante(ctx context.Context, in MatriculaInput, aud Auditoria) (*Matricula, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// FOR SHARE: el estado del registro no cambia mientras se matricula.
	var estado string
	var version int
	if err := tx.QueryRow(ctx, `SELECT estado, version FROM estudiantes WHERE id = $1 FOR SHARE`, in.EstudianteID).Scan(&estado, &version); err != nil {
		return nil, classifyNotFound(err)
	}
	if estado != "ACTIVO" {
		return nil, ErrEstudianteInactivo
	}
	estadoP, err := estadoPeriodo(ctx, tx, in.PeriodoID)
	if err != nil {
		return nil, err
	}
	if estadoP == PeriodoCerrado {
		return nil, ErrPeriodoCerrado
	}
	campos := []string{"matricula"}
	if in.SeccionID != nil {
		if _, err := reservarSeccion(ctx, tx, *in.SeccionID, in.PeriodoID); err != nil {
			return nil, err
		}
		campos = append(campos, "seccion")
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO matriculas (estudiante_id, periodo_id, seccion_id, fecha_matricula)
		VALUES ($1, $2, $3, $4::date)
		RETURNING id
	`, in.EstudianteID, in.PeriodoID, in.SeccionID, in.FechaMatricula).Scan(&id); err != nil {
		return nil, classifyMatriculaError(err)
	}
	aud.Accion = AccionMatriculaUpdate
	aud.Campos = campos
	if err := insertAuditoria(ctx, tx, in.EstudianteID, version, aud); err != nil {
		return nil, err
	}
	m, err := getMatricula(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, classifyMatriculaError(err)
	}
	return m, nil
}

// matriculaActual: lo que las transiciones necesitan saber, bloqueado.
type matriculaActual struct {
	EstudianteID   string
	PeriodoID      string
	PeriodoEstado  string
	SeccionID      *string
	Estado         string
	FechaMatricula *string
	FechaRetiro    *string
}

// bloquearMatricula toma la matrícula FOR UPDATE (y su periodo FOR SHARE) y
// verifica la versión esperada (concurrencia optimista).
func bloquearMatricula(ctx context.Context, tx pgx.Tx, id string, expectedVersion int) (*matriculaActual, error) {
	var cur matriculaActual
	var version int
	err := tx.QueryRow(ctx, `
		SELECT estudiante_id, periodo_id, seccion_id::text, estado_matricula,
			to_char(fecha_matricula, 'YYYY-MM-DD'), to_char(fecha_retiro, 'YYYY-MM-DD'), version
		FROM matriculas WHERE id = $1 FOR UPDATE
	`, id).Scan(&cur.EstudianteID, &cur.PeriodoID, &cur.SeccionID, &cur.Estado, &cur.FechaMatricula, &cur.FechaRetiro, &version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMatriculaNotFound
		}
		return nil, err
	}
	if version != expectedVersion {
		return nil, ErrMatriculaVersion
	}
	if cur.PeriodoEstado, err = estadoPeriodo(ctx, tx, cur.PeriodoID); err != nil {
		return nil, err
	}
	return &cur, nil
}

// noAnterior: fecha (AAAA-MM-DD) no anterior a otra fecha registrada; el
// formato fijo permite comparar como texto.
func noAnterior(fecha string, referencia *string) error {
	if referencia != nil && fecha < *referencia {
		return ErrFechaInconsistente
	}
	return nil
}

// cambioMatricula: una transición aplicada a una matrícula ya bloqueada.
type cambioMatricula struct {
	set        string // SET de UPDATE (parámetros desde $3)
	args       []any
	movimiento string
	anterior   *string
	nueva      *string
	fecha      string
	motivo     *string
	campos     []string
}

// aplicarCambio escribe la transición (versión esperada), su movimiento y la
// auditoría, y devuelve la matrícula resultante.
func (r *EstudianteRepository) aplicarCambio(ctx context.Context, tx pgx.Tx, id string, expectedVersion int, cur *matriculaActual,
	c cambioMatricula, aud Auditoria) (*Matricula, error) {
	args := append([]any{id, expectedVersion}, c.args...)
	tag, err := tx.Exec(ctx, `UPDATE matriculas SET `+c.set+` WHERE id = $1 AND version = $2`, args...)
	if err != nil {
		return nil, classifyMatriculaError(err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrMatriculaVersion
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO matricula_movimientos (matricula_id, tipo, seccion_anterior_id, seccion_nueva_id, fecha, motivo, registrado_por)
		VALUES ($1, $2, $3, $4, $5::date, $6, $7)
	`, id, c.movimiento, c.anterior, c.nueva, c.fecha, c.motivo, aud.UsuarioID); err != nil {
		return nil, classifyMatriculaError(err)
	}
	version, err := versionEstudiante(ctx, tx, cur.EstudianteID)
	if err != nil {
		return nil, err
	}
	aud.Accion = AccionMatriculaUpdate
	aud.Campos = c.campos
	if err := insertAuditoria(ctx, tx, cur.EstudianteID, version, aud); err != nil {
		return nil, err
	}
	m, err := getMatricula(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, classifyMatriculaError(err)
	}
	return m, nil
}

// transicion ejecuta fn sobre la matrícula bloqueada dentro de una transacción.
func (r *EstudianteRepository) transicion(ctx context.Context, id string, expectedVersion int, aud Auditoria,
	fn func(tx pgx.Tx, cur *matriculaActual) (*cambioMatricula, error)) (*Matricula, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cur, err := bloquearMatricula(ctx, tx, id, expectedVersion)
	if err != nil {
		return nil, err
	}
	c, err := fn(tx, cur)
	if err != nil {
		return nil, err
	}
	return r.aplicarCambio(ctx, tx, id, expectedVersion, cur, *c, aud)
}

// CambiarSeccion asigna o cambia la sección de una matrícula MATRICULADO de
// un periodo abierto. Si la nueva sección solo difiere en el turno, el
// movimiento es CAMBIO_TURNO.
func (r *EstudianteRepository) CambiarSeccion(ctx context.Context, id string, expectedVersion int, seccionID, fecha string, motivo *string, aud Auditoria) (*Matricula, error) {
	return r.transicion(ctx, id, expectedVersion, aud, func(tx pgx.Tx, cur *matriculaActual) (*cambioMatricula, error) {
		if cur.Estado != MatriculaMatriculado {
			return nil, ErrTransicionInvalida
		}
		if cur.PeriodoEstado == PeriodoCerrado {
			return nil, ErrPeriodoCerrado
		}
		if cur.SeccionID != nil && *cur.SeccionID == seccionID {
			return nil, ErrSinCambios
		}
		if err := noAnterior(fecha, cur.FechaMatricula); err != nil {
			return nil, err
		}
		nueva, err := reservarSeccion(ctx, tx, seccionID, cur.PeriodoID)
		if err != nil {
			return nil, err
		}
		tipo := MovCambioSeccion
		if cur.SeccionID != nil {
			var nivel, grado, seccion string
			if err := tx.QueryRow(ctx, `SELECT nivel, grado, seccion FROM secciones WHERE id = $1`, *cur.SeccionID).Scan(&nivel, &grado, &seccion); err != nil {
				return nil, err
			}
			if nivel == nueva.Nivel && grado == nueva.Grado && seccion == nueva.Seccion {
				tipo = MovCambioTurno
			}
		}
		return &cambioMatricula{
			set: "seccion_id = $3", args: []any{seccionID},
			movimiento: tipo, anterior: cur.SeccionID, nueva: &seccionID, fecha: fecha, motivo: motivo,
			campos: []string{"matricula", "seccion"},
		}, nil
	})
}

// Retirar cierra una matrícula MATRICULADO como RETIRADO (movimiento RETIRO)
// o TRASLADADO (TRASLADO_SALIDA). La sección se conserva como referencia.
func (r *EstudianteRepository) Retirar(ctx context.Context, id string, expectedVersion int, traslado bool, fecha string, motivo string, aud Auditoria) (*Matricula, error) {
	return r.transicion(ctx, id, expectedVersion, aud, func(_ pgx.Tx, cur *matriculaActual) (*cambioMatricula, error) {
		if cur.Estado != MatriculaMatriculado {
			return nil, ErrTransicionInvalida
		}
		if cur.PeriodoEstado == PeriodoCerrado {
			return nil, ErrPeriodoCerrado
		}
		if err := noAnterior(fecha, cur.FechaMatricula); err != nil {
			return nil, err
		}
		estado, mov := MatriculaRetirado, MovRetiro
		if traslado {
			estado, mov = MatriculaTrasladado, MovTrasladoSalida
		}
		return &cambioMatricula{
			set: "estado_matricula = $3, fecha_retiro = $4::date, motivo_retiro = $5", args: []any{estado, fecha, motivo},
			movimiento: mov, anterior: cur.SeccionID, fecha: fecha, motivo: &motivo,
			campos: []string{"matricula", "estado_matricula"},
		}, nil
	})
}

// Reingresar devuelve a MATRICULADO una matrícula RETIRADO o TRASLADADO del
// mismo periodo (abierto). El retiro anterior queda en los movimientos; la
// sección debe tener vacantes.
func (r *EstudianteRepository) Reingresar(ctx context.Context, id string, expectedVersion int, fecha string, motivo *string, aud Auditoria) (*Matricula, error) {
	return r.transicion(ctx, id, expectedVersion, aud, func(tx pgx.Tx, cur *matriculaActual) (*cambioMatricula, error) {
		if cur.Estado != MatriculaRetirado && cur.Estado != MatriculaTrasladado {
			return nil, ErrTransicionInvalida
		}
		if cur.PeriodoEstado == PeriodoCerrado {
			return nil, ErrPeriodoCerrado
		}
		if err := noAnterior(fecha, cur.FechaRetiro); err != nil {
			return nil, err
		}
		if cur.SeccionID != nil {
			if _, err := reservarSeccion(ctx, tx, *cur.SeccionID, cur.PeriodoID); err != nil {
				return nil, err
			}
		}
		return &cambioMatricula{
			set: "estado_matricula = 'MATRICULADO', fecha_retiro = NULL, motivo_retiro = NULL",
			movimiento: MovReingreso, nueva: cur.SeccionID, fecha: fecha, motivo: motivo,
			campos: []string{"matricula", "estado_matricula"},
		}, nil
	})
}

// Concluir marca CONCLUIDO una matrícula MATRICULADO de un periodo EN_CURSO o
// CERRADO (cierre del periodo). No decide aprobación ni promoción.
func (r *EstudianteRepository) Concluir(ctx context.Context, id string, expectedVersion int, fecha string, aud Auditoria) (*Matricula, error) {
	return r.transicion(ctx, id, expectedVersion, aud, func(_ pgx.Tx, cur *matriculaActual) (*cambioMatricula, error) {
		if cur.Estado != MatriculaMatriculado || cur.PeriodoEstado == PeriodoPlanificado {
			return nil, ErrTransicionInvalida
		}
		if err := noAnterior(fecha, cur.FechaMatricula); err != nil {
			return nil, err
		}
		return &cambioMatricula{
			set:        "estado_matricula = 'CONCLUIDO'",
			movimiento: MovConclusion, anterior: cur.SeccionID, fecha: fecha,
			campos: []string{"matricula", "estado_matricula"},
		}, nil
	})
}

// classifyMatriculaError: restricciones del dominio académico -> errores de
// dominio (solo la restricción, nunca valores).
func classifyMatriculaError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case "uq_periodos_academicos_anio":
		return ErrPeriodoDuplicado
	case "uq_secciones_periodo_grupo":
		return ErrSeccionDuplicada
	case "uq_matriculas_estudiante_periodo":
		return ErrMatriculaDuplicada
	case "fk_matriculas_seccion_mismo_periodo", "fk_movimientos_seccion_nueva", "fk_movimientos_seccion_anterior":
		return ErrSeccionNotFound
	case "fk_secciones_periodo", "fk_matriculas_periodo":
		return ErrPeriodoNotFound
	case "fk_matriculas_estudiante":
		return ErrEstudianteNotFound
	case "chk_matriculas_fechas", "chk_periodos_academicos_fechas":
		return ErrFechaInconsistente
	}
	return classifyWriteError(err)
}
