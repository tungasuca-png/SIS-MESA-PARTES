package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Etapa 6 — historial de cambios del estudiante y cambio de parentesco.

// AccionesHistorial: eventos de CAMBIO que se muestran en el historial. Las
// lecturas auditadas (PERSONAL_VIEW, FAMILIA_VIEW) no son historial del
// estudiante: revelarían quién consultó sus datos.
var AccionesHistorial = []string{AccionCreate, AccionUpdate, AccionChangeEstado, "PERSONAL_UPDATE", AccionFamiliaUpdate, AccionMatriculaUpdate}

type EventoHistorial struct {
	ID                string
	Accion            string
	FechaHora         time.Time
	UsuarioID         string
	Campos            []string
	EstadoAnterior    *string
	EstadoNuevo       *string
	VersionResultante int
}

// ListHistorial devuelve los eventos de cambio del estudiante, más reciente
// primero. Solo nombres de campos: la auditoría nunca guarda valores.
func (r *EstudianteRepository) ListHistorial(ctx context.Context, estudianteID string, page, pageSize int) ([]EventoHistorial, int, error) {
	var existe bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM estudiantes WHERE id = $1)`, estudianteID).Scan(&existe); err != nil {
		return nil, 0, err
	}
	if !existe {
		return nil, 0, ErrEstudianteNotFound
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM auditoria_estudiantes WHERE estudiante_id = $1 AND accion = ANY($2)`,
		estudianteID, AccionesHistorial).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, accion, fecha_hora, usuario_id, campos, estado_anterior, estado_nuevo, version_resultante
		FROM auditoria_estudiantes
		WHERE estudiante_id = $1 AND accion = ANY($2)
		ORDER BY fecha_hora DESC, id
		LIMIT $3 OFFSET $4
	`, estudianteID, AccionesHistorial, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]EventoHistorial, 0)
	for rows.Next() {
		var e EventoHistorial
		if err := rows.Scan(&e.ID, &e.Accion, &e.FechaHora, &e.UsuarioID, &e.Campos, &e.EstadoAnterior, &e.EstadoNuevo, &e.VersionResultante); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// CambiarParentesco corrige el parentesco de una relación vigente sin perder
// el historial, en UNA transacción: cierra la relación (vigente=false,
// hasta=desde) y crea otra con la misma persona, el nuevo parentesco y el
// mismo rol de apoderado. Las reglas de la base (un PADRE/MADRE y un
// apoderado vigentes) se aplican igual; si algo falla no cambia nada.
func (r *EstudianteRepository) CambiarParentesco(ctx context.Context, estudianteID, relacionID, parentesco, desde string, aud Auditoria) (*Familiar, *Familiar, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	version, err := versionEstudiante(ctx, tx, estudianteID)
	if err != nil {
		return nil, nil, err
	}
	var personaID, actual string
	var esApoderado, vigente bool
	var inicio *string
	if err := tx.QueryRow(ctx, `
		SELECT persona_id, parentesco, es_apoderado, vigente, to_char(desde, 'YYYY-MM-DD')
		FROM estudiante_familiares WHERE id = $1 AND estudiante_id = $2 FOR UPDATE
	`, relacionID, estudianteID).Scan(&personaID, &actual, &esApoderado, &vigente, &inicio); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrRelacionNotFound
		}
		return nil, nil, err
	}
	if !vigente {
		return nil, nil, ErrRelacionNoVigente
	}
	if actual == parentesco {
		return nil, nil, ErrSinCambios
	}
	if inicio != nil && desde < *inicio {
		return nil, nil, ErrFechaInconsistente
	}
	if _, err := tx.Exec(ctx, `UPDATE estudiante_familiares SET vigente = FALSE, hasta = $2::date WHERE id = $1`, relacionID, desde); err != nil {
		return nil, nil, classifyFamiliaError(err)
	}
	var nuevaID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO estudiante_familiares (estudiante_id, persona_id, parentesco, es_apoderado, desde)
		VALUES ($1, $2, $3, $4, $5::date)
		RETURNING id
	`, estudianteID, personaID, parentesco, esApoderado, desde).Scan(&nuevaID); err != nil {
		return nil, nil, classifyFamiliaError(err)
	}
	aud.Accion = AccionFamiliaUpdate
	aud.Campos = []string{"familiares"}
	if err := insertAuditoria(ctx, tx, estudianteID, version, aud); err != nil {
		return nil, nil, err
	}
	anterior, err := scanFamiliar(tx.QueryRow(ctx, familiarSelect+` WHERE f.id = $1`, relacionID))
	if err != nil {
		return nil, nil, err
	}
	nueva, err := scanFamiliar(tx.QueryRow(ctx, familiarSelect+` WHERE f.id = $1`, nuevaID))
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, classifyFamiliaError(err)
	}
	return anterior, nueva, nil
}
