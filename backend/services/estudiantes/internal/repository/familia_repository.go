package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Modelo maestro, etapa 4 — FAMILIA. Personas normalizadas (una sola fila
// aunque se relacione con varios estudiantes) y relaciones con parentesco,
// apoderado y vigencia. Toda lectura y modificación se audita en
// auditoria_estudiantes (FAMILIA_VIEW / FAMILIA_UPDATE) con el campo
// "familiares", nunca con valores.

const (
	AccionFamiliaView   = "FAMILIA_VIEW"
	AccionFamiliaUpdate = "FAMILIA_UPDATE"
)

var (
	ErrPersonaNotFound       = errors.New("persona not found")
	ErrPersonaDatosDistintos = errors.New("persona con otros datos")
	ErrPersonaDuplicada      = errors.New("persona duplicada")
	ErrFamiliarYaVinculado   = errors.New("familiar ya vinculado")
	ErrProgenitorDuplicado   = errors.New("progenitor duplicado")
	ErrApoderadoDuplicado    = errors.New("apoderado duplicado")
	ErrParentescoInvalido    = errors.New("parentesco invalido")
	ErrRelacionNotFound      = errors.New("relacion not found")
	ErrRelacionNoVigente     = errors.New("relacion no vigente")
)

// PersonaDatos: nil = no registrado (NULL). Llegan ya normalizados.
type PersonaDatos struct {
	TipoDocumento   string
	NumeroDocumento *string
	Nombres         string
	ApellidoPaterno *string
	ApellidoMaterno *string
}

type Persona struct {
	ID string
	PersonaDatos
}

type Familiar struct {
	RelacionID  string
	Persona     Persona
	Parentesco  string
	EsApoderado bool
	Vigente     bool
	Desde       *string
	Hasta       *string
}

// VinculoInput: PersonaID (persona existente) o Persona (datos).
type VinculoInput struct {
	PersonaID   string
	Persona     *PersonaDatos
	Parentesco  string
	EsApoderado bool
	Desde       *string
}

const familiarSelect = `
	SELECT f.id, p.id, p.tipo_documento, p.numero_documento, p.nombres, p.apellido_paterno, p.apellido_materno,
		f.parentesco, f.es_apoderado, f.vigente, to_char(f.desde, 'YYYY-MM-DD'), to_char(f.hasta, 'YYYY-MM-DD')
	FROM estudiante_familiares f
	JOIN personas p ON p.id = f.persona_id
	JOIN cat_parentescos c ON c.codigo = f.parentesco
`

func scanFamiliar(row pgx.Row) (*Familiar, error) {
	f := &Familiar{}
	err := row.Scan(&f.RelacionID, &f.Persona.ID, &f.Persona.TipoDocumento, &f.Persona.NumeroDocumento,
		&f.Persona.Nombres, &f.Persona.ApellidoPaterno, &f.Persona.ApellidoMaterno,
		&f.Parentesco, &f.EsApoderado, &f.Vigente, &f.Desde, &f.Hasta)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func versionEstudiante(ctx context.Context, tx pgx.Tx, estudianteID string) (int, error) {
	var version int
	if err := tx.QueryRow(ctx, `SELECT version FROM estudiantes WHERE id = $1`, estudianteID).Scan(&version); err != nil {
		return 0, classifyNotFound(err)
	}
	return version, nil
}

// ListFamiliares: relaciones del estudiante (solo vigentes, o todas con
// historial). La lectura se audita en la MISMA transacción (fail-closed);
// version_resultante = versión observada del estudiante.
func (r *EstudianteRepository) ListFamiliares(ctx context.Context, estudianteID string, historial bool, aud Auditoria) ([]Familiar, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	version, err := versionEstudiante(ctx, tx, estudianteID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, familiarSelect+`
		WHERE f.estudiante_id = $1 AND (f.vigente OR $2)
		ORDER BY f.vigente DESC, c.orden, f.desde NULLS LAST, f.created_at, f.id
	`, estudianteID, historial)
	if err != nil {
		return nil, err
	}
	familiares := make([]Familiar, 0)
	for rows.Next() {
		f, err := scanFamiliar(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		familiares = append(familiares, *f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	aud.Accion = AccionFamiliaView
	aud.Campos = []string{"familiares"}
	if err := insertAuditoria(ctx, tx, estudianteID, version, aud); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return familiares, nil
}

// VincularFamiliar crea la relación (y la persona si hace falta) con su
// auditoría en una sola transacción. Con documento, una persona existente se
// REUTILIZA solo si sus nombres coinciden; si no, se rechaza sin modificar
// nada (no se sobrescribe ni se fusiona). Sin documento se registra una
// persona nueva: no hay forma segura de deduplicarla automáticamente.
func (r *EstudianteRepository) VincularFamiliar(ctx context.Context, estudianteID string, in VinculoInput, aud Auditoria) (*Familiar, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	version, err := versionEstudiante(ctx, tx, estudianteID)
	if err != nil {
		return nil, err
	}
	personaID, err := resolverPersona(ctx, tx, in)
	if err != nil {
		return nil, err
	}
	var relacionID string
	err = tx.QueryRow(ctx, `
		INSERT INTO estudiante_familiares (estudiante_id, persona_id, parentesco, es_apoderado, desde)
		VALUES ($1, $2, $3, $4, $5::date)
		RETURNING id
	`, estudianteID, personaID, in.Parentesco, in.EsApoderado, in.Desde).Scan(&relacionID)
	if err != nil {
		return nil, classifyFamiliaError(err)
	}
	aud.Accion = AccionFamiliaUpdate
	aud.Campos = []string{"familiares"}
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

func resolverPersona(ctx context.Context, tx pgx.Tx, in VinculoInput) (string, error) {
	if in.PersonaID != "" {
		var id string
		if err := tx.QueryRow(ctx, `SELECT id FROM personas WHERE id = $1`, in.PersonaID).Scan(&id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", ErrPersonaNotFound
			}
			return "", err
		}
		return id, nil
	}
	d := in.Persona
	if d.NumeroDocumento != nil {
		var existente Persona
		err := tx.QueryRow(ctx, `
			SELECT id, nombres, apellido_paterno, apellido_materno FROM personas
			WHERE tipo_documento = $1 AND numero_documento = $2
		`, d.TipoDocumento, *d.NumeroDocumento).Scan(&existente.ID, &existente.Nombres,
			&existente.ApellidoPaterno, &existente.ApellidoMaterno)
		switch {
		case err == nil:
			if !mismosNombres(d, &existente.PersonaDatos) {
				return "", ErrPersonaDatosDistintos
			}
			return existente.ID, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return "", err
		}
	}
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO personas (tipo_documento, numero_documento, nombres, apellido_paterno, apellido_materno)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, d.TipoDocumento, d.NumeroDocumento, d.Nombres, d.ApellidoPaterno, d.ApellidoMaterno).Scan(&id)
	if err != nil {
		return "", classifyFamiliaError(err)
	}
	return id, nil
}

// mismosNombres compara sin distinguir mayúsculas (los valores ya llegan con
// espacios normalizados). Un apellido ausente en la solicitud no se compara.
func mismosNombres(nuevo, guardado *PersonaDatos) bool {
	eq := func(a, b *string) bool {
		if a == nil {
			return true
		}
		return b != nil && strings.EqualFold(*a, *b)
	}
	return strings.EqualFold(nuevo.Nombres, guardado.Nombres) &&
		eq(nuevo.ApellidoPaterno, guardado.ApellidoPaterno) &&
		eq(nuevo.ApellidoMaterno, guardado.ApellidoMaterno)
}

// DesvincularFamiliar cierra la relación vigente (vigente=false, hasta) sin
// borrar nada: el historial se conserva. Auditoría en la misma transacción.
func (r *EstudianteRepository) DesvincularFamiliar(ctx context.Context, estudianteID, relacionID, hasta string, aud Auditoria) (*Familiar, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	version, err := versionEstudiante(ctx, tx, estudianteID)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE estudiante_familiares SET vigente = FALSE, hasta = $3::date
		WHERE id = $1 AND estudiante_id = $2 AND vigente
	`, relacionID, estudianteID, hasta)
	if err != nil {
		return nil, classifyFamiliaError(err)
	}
	if tag.RowsAffected() == 0 {
		var vigente bool
		err := tx.QueryRow(ctx, `SELECT vigente FROM estudiante_familiares WHERE id = $1 AND estudiante_id = $2`,
			relacionID, estudianteID).Scan(&vigente)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRelacionNotFound
		}
		if err != nil {
			return nil, err
		}
		return nil, ErrRelacionNoVigente
	}
	aud.Accion = AccionFamiliaUpdate
	aud.Campos = []string{"familiares"}
	if err := insertAuditoria(ctx, tx, estudianteID, version, aud); err != nil {
		return nil, err
	}
	f, err := scanFamiliar(tx.QueryRow(ctx, familiarSelect+` WHERE f.id = $1`, relacionID))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return f, nil
}

// classifyFamiliaError: restricciones del dominio familiar -> errores de
// dominio (identificando solo la restricción, nunca valores).
func classifyFamiliaError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.ConstraintName {
	case "uq_estudiante_familiares_vigente":
		return ErrFamiliarYaVinculado
	case "uq_estudiante_familiares_progenitor":
		return ErrProgenitorDuplicado
	case "uq_estudiante_familiares_apoderado":
		return ErrApoderadoDuplicado
	case "uq_personas_documento":
		return ErrPersonaDuplicada
	case "fk_estudiante_familiares_parentesco":
		return ErrParentescoInvalido
	case "fk_personas_tipo_documento", "chk_personas_documento":
		return ErrDocumentoInvalido
	}
	return classifyWriteError(err)
}
