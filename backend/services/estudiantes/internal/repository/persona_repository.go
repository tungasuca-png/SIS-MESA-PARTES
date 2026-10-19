package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Modelo maestro, etapa 5 — PERSONAS (padres, madres, apoderados y otros
// familiares). Una persona es UNA fila aunque se relacione con varios
// estudiantes. Su auditoría va en auditoria_estudiantes con persona_id
// (migración 009): PERSONA_VIEW al consultarla con sus vínculos y
// PERSONA_UPDATE al registrarla (version_resultante = 1) o editarla. Nunca
// valores, solo nombres de campos.

const (
	AccionPersonaView   = "PERSONA_VIEW"
	AccionPersonaUpdate = "PERSONA_UPDATE"
)

var (
	ErrPersonaDocumentoExiste = errors.New("persona documento existe")
	ErrPersonaVersion         = errors.New("persona version conflict")
)

const personaColumns = `id, version, tipo_documento, numero_documento, nombres, apellido_paterno, apellido_materno`

func scanPersona(row pgx.Row) (*Persona, error) {
	p := &Persona{}
	if err := row.Scan(&p.ID, &p.Version, &p.TipoDocumento, &p.NumeroDocumento, &p.Nombres, &p.ApellidoPaterno, &p.ApellidoMaterno); err != nil {
		return nil, err
	}
	return p, nil
}

// VinculoPersona: relación de la persona con un estudiante (vista inversa).
type VinculoPersona struct {
	RelacionID   string
	EstudianteID string
	Parentesco   string
	EsApoderado  bool
	Vigente      bool
	Desde        *string
	Hasta        *string
}

func insertAuditoriaPersona(ctx context.Context, tx pgx.Tx, personaID string, version int, aud Auditoria) error {
	campos := aud.Campos
	if campos == nil {
		campos = []string{}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO auditoria_estudiantes (usuario_id, accion, estudiante_id, persona_id, version_resultante, campos)
		VALUES ($1, $2, NULL, $3, $4, $5)
	`, aud.UsuarioID, aud.Accion, personaID, version, campos)
	return err
}

// camposPersona: nombres de los campos informados (alta) o cambiados.
func camposPersona(a, b *PersonaDatos) []string {
	eq := func(x, y *string) bool {
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return *x == *y
	}
	var out []string
	if a.TipoDocumento != b.TipoDocumento {
		out = append(out, "tipo_documento")
	}
	if !eq(a.NumeroDocumento, b.NumeroDocumento) {
		out = append(out, "numero_documento")
	}
	if a.Nombres != b.Nombres {
		out = append(out, "nombres")
	}
	if !eq(a.ApellidoPaterno, b.ApellidoPaterno) {
		out = append(out, "apellido_paterno")
	}
	if !eq(a.ApellidoMaterno, b.ApellidoMaterno) {
		out = append(out, "apellido_materno")
	}
	return out
}

// CreatePersona registra una persona sin vincularla todavía. Con documento,
// si ya existe otra persona con ese documento se rechaza (no se duplica ni
// se fusiona).
func (r *EstudianteRepository) CreatePersona(ctx context.Context, d PersonaDatos, aud Auditoria) (*Persona, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	p, err := scanPersona(tx.QueryRow(ctx, `
		INSERT INTO personas (tipo_documento, numero_documento, nombres, apellido_paterno, apellido_materno)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+personaColumns, d.TipoDocumento, d.NumeroDocumento, d.Nombres, d.ApellidoPaterno, d.ApellidoMaterno))
	if err != nil {
		return nil, classifyPersonaError(err)
	}
	aud.Accion = AccionPersonaUpdate
	aud.Campos = camposPersona(&PersonaDatos{TipoDocumento: "SIN_DOCUMENTO"}, &d)
	if err := insertAuditoriaPersona(ctx, tx, p.ID, p.Version, aud); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, classifyPersonaError(err)
	}
	return p, nil
}

// GetPersona devuelve la persona y sus vínculos con estudiantes (vigentes, o
// todos con historial). La consulta se audita en la MISMA transacción.
func (r *EstudianteRepository) GetPersona(ctx context.Context, id string, historial bool, aud Auditoria) (*Persona, []VinculoPersona, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	p, err := scanPersona(tx.QueryRow(ctx, `SELECT `+personaColumns+` FROM personas WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrPersonaNotFound
		}
		return nil, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT f.id, f.estudiante_id, f.parentesco, f.es_apoderado, f.vigente,
			to_char(f.desde, 'YYYY-MM-DD'), to_char(f.hasta, 'YYYY-MM-DD')
		FROM estudiante_familiares f
		JOIN cat_parentescos c ON c.codigo = f.parentesco
		WHERE f.persona_id = $1 AND (f.vigente OR $2)
		ORDER BY f.vigente DESC, c.orden, f.desde NULLS LAST, f.created_at, f.id
	`, id, historial)
	if err != nil {
		return nil, nil, err
	}
	vinculos := make([]VinculoPersona, 0)
	for rows.Next() {
		var v VinculoPersona
		if err := rows.Scan(&v.RelacionID, &v.EstudianteID, &v.Parentesco, &v.EsApoderado, &v.Vigente, &v.Desde, &v.Hasta); err != nil {
			rows.Close()
			return nil, nil, err
		}
		vinculos = append(vinculos, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	aud.Accion = AccionPersonaView
	aud.Campos = []string{"familiares"}
	if err := insertAuditoriaPersona(ctx, tx, p.ID, p.Version, aud); err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return p, vinculos, nil
}

// PersonaSearch: Terms ya separados por la lógica (cualquier orden).
type PersonaSearch struct {
	Terms    []string
	Page     int
	PageSize int
}

// SearchPersonas: cada palabra debe aparecer en el documento, los nombres o
// los apellidos (normalizados sin tildes ni mayúsculas). Solo identidad: no
// devuelve vínculos (para eso GetPersona, auditado).
func (r *EstudianteRepository) SearchPersonas(ctx context.Context, s PersonaSearch) ([]Persona, int, error) {
	conds := make([]string, 0, len(s.Terms))
	args := make([]any, 0, len(s.Terms)+2)
	for _, term := range s.Terms {
		args = append(args, likeEscaper.Replace(term))
		conds = append(conds, fmt.Sprintf(`estudiantes_normalizar(COALESCE(numero_documento, '') || ' ' || nombres || ' ' ||
			COALESCE(apellido_paterno, '') || ' ' || COALESCE(apellido_materno, '')) LIKE '%%' || estudiantes_normalizar($%d) || '%%'`, len(args)))
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM personas `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, s.PageSize, (s.Page-1)*s.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT `+personaColumns+` FROM personas `+where+`
		ORDER BY apellido_paterno NULLS LAST, apellido_materno NULLS LAST, nombres, id
		LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Persona, 0)
	for rows.Next() {
		p, err := scanPersona(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// UpdatePersona aplica patch sobre los datos actuales (bloqueados) con
// control de versión. validar recibe el resultado final. Sin cambios reales
// devuelve ErrSinCambios (nada se escribe ni se audita).
func (r *EstudianteRepository) UpdatePersona(ctx context.Context, id string, expectedVersion int, aud Auditoria,
	patch func(cur *PersonaDatos) error) (*Persona, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	cur, err := scanPersona(tx.QueryRow(ctx, `SELECT `+personaColumns+` FROM personas WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPersonaNotFound
		}
		return nil, err
	}
	if cur.Version != expectedVersion {
		return nil, ErrPersonaVersion
	}
	antes := cur.PersonaDatos
	nuevo := cur.PersonaDatos
	if err := patch(&nuevo); err != nil {
		return nil, err
	}
	campos := camposPersona(&antes, &nuevo)
	if len(campos) == 0 {
		return nil, ErrSinCambios
	}
	p, err := scanPersona(tx.QueryRow(ctx, `
		UPDATE personas SET tipo_documento = $3, numero_documento = $4, nombres = $5, apellido_paterno = $6, apellido_materno = $7
		WHERE id = $1 AND version = $2
		RETURNING `+personaColumns, id, expectedVersion, nuevo.TipoDocumento, nuevo.NumeroDocumento, nuevo.Nombres,
		nuevo.ApellidoPaterno, nuevo.ApellidoMaterno))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPersonaVersion
	}
	if err != nil {
		return nil, classifyPersonaError(err)
	}
	aud.Accion = AccionPersonaUpdate
	aud.Campos = campos
	if err := insertAuditoriaPersona(ctx, tx, p.ID, p.Version, aud); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, classifyPersonaError(err)
	}
	return p, nil
}

// ActualizarFamiliar cambia solo el rol de apoderado de una relación vigente
// (el parentesco no se edita: se cierra y se registra otra relación). La
// regla de un solo apoderado vigente la garantiza el índice parcial.
func (r *EstudianteRepository) ActualizarFamiliar(ctx context.Context, estudianteID, relacionID string, esApoderado bool, aud Auditoria) (*Familiar, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	version, err := versionEstudiante(ctx, tx, estudianteID)
	if err != nil {
		return nil, err
	}
	var actual, vigente bool
	if err := tx.QueryRow(ctx, `SELECT es_apoderado, vigente FROM estudiante_familiares WHERE id = $1 AND estudiante_id = $2 FOR UPDATE`,
		relacionID, estudianteID).Scan(&actual, &vigente); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRelacionNotFound
		}
		return nil, err
	}
	if !vigente {
		return nil, ErrRelacionNoVigente
	}
	if actual == esApoderado {
		return nil, ErrSinCambios
	}
	if _, err := tx.Exec(ctx, `UPDATE estudiante_familiares SET es_apoderado = $2 WHERE id = $1`, relacionID, esApoderado); err != nil {
		return nil, classifyFamiliaError(err)
	}
	aud.Accion = AccionFamiliaUpdate
	aud.Campos = []string{"familiares", "es_apoderado"}
	if err := insertAuditoria(ctx, tx, estudianteID, version, aud); err != nil {
		return nil, err
	}
	f, err := scanFamiliar(tx.QueryRow(ctx, familiarSelect+` WHERE f.id = $1`, relacionID))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, classifyFamiliaError(err)
	}
	return f, nil
}

func classifyPersonaError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "uq_personas_documento" {
		return ErrPersonaDocumentoExiste
	}
	return classifyFamiliaError(err)
}
