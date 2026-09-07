package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Expediente struct {
	ID                 string
	Codigo             string
	Tipo               string
	Asunto             string
	Descripcion        string
	SolicitanteID      string
	Estado             string
	Prioridad          string
	FechaRegistro      time.Time
	FechaActualizacion time.Time
	Activo             bool
}

type ListFilter struct {
	Estado        string
	Prioridad     string
	Tipo          string
	SolicitanteID string
	Page          int
	PageSize      int
}

type ExpedienteRepository struct {
	db *pgxpool.Pool
}

func NewExpedienteRepository(db *pgxpool.Pool) *ExpedienteRepository {
	return &ExpedienteRepository{db: db}
}

const expedienteColumns = `
	id, codigo, tipo, asunto, descripcion, solicitante_id, estado, prioridad,
	fecha_registro, fecha_actualizacion, activo
`

func scanExpediente(row pgx.Row) (*Expediente, error) {
	e := &Expediente{}
	err := row.Scan(
		&e.ID, &e.Codigo, &e.Tipo, &e.Asunto, &e.Descripcion, &e.SolicitanteID,
		&e.Estado, &e.Prioridad, &e.FechaRegistro, &e.FechaActualizacion, &e.Activo,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Create genera el codigo (EXP-<anio>-<consecutivo>) y crea el expediente
// en una sola transacción, para que el consecutivo nunca quede huérfano.
func (r *ExpedienteRepository) Create(ctx context.Context, e *Expediente) (*Expediente, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	codigo, err := nextCodigo(ctx, tx, time.Now().Year())
	if err != nil {
		return nil, err
	}

	created, err := scanExpediente(tx.QueryRow(ctx, `
		INSERT INTO expedientes (codigo, tipo, asunto, descripcion, solicitante_id, estado, prioridad)
		VALUES ($1, $2, $3, $4, $5, 'PENDIENTE', $6)
		RETURNING `+expedienteColumns,
		codigo, e.Tipo, e.Asunto, e.Descripcion, e.SolicitanteID, e.Prioridad,
	))
	if err != nil {
		return nil, classifyUniqueViolation(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return created, nil
}

func nextCodigo(ctx context.Context, tx pgx.Tx, year int) (string, error) {
	var ultimo int
	err := tx.QueryRow(ctx, `
		INSERT INTO expediente_codigo_counters (anio, ultimo)
		VALUES ($1, 1)
		ON CONFLICT (anio) DO UPDATE SET ultimo = expediente_codigo_counters.ultimo + 1
		RETURNING ultimo
	`, year).Scan(&ultimo)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("EXP-%d-%06d", year, ultimo), nil
}

// FindByIDOrCodigo acepta tanto el UUID interno como el codigo legible.
func (r *ExpedienteRepository) FindByIDOrCodigo(ctx context.Context, value string) (*Expediente, error) {
	found, err := scanExpediente(r.db.QueryRow(ctx, `
		SELECT `+expedienteColumns+`
		FROM expedientes
		WHERE (id::text = $1 OR codigo = $1) AND activo = TRUE
	`, value))
	if err != nil {
		return nil, classifyNotFound(err, ErrExpedienteNotFound)
	}
	return found, nil
}

func (r *ExpedienteRepository) List(ctx context.Context, filter ListFilter) ([]*Expediente, int, error) {
	conditions := []string{"activo = TRUE"}
	args := make([]any, 0, 4)

	addCondition := func(column, value string) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if filter.Estado != "" {
		addCondition("estado", filter.Estado)
	}
	if filter.Prioridad != "" {
		addCondition("prioridad", filter.Prioridad)
	}
	if filter.Tipo != "" {
		addCondition("tipo", filter.Tipo)
	}
	if filter.SolicitanteID != "" {
		addCondition("solicitante_id", filter.SolicitanteID)
	}
	whereClause := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM expedientes WHERE "+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM expedientes
		WHERE %s
		ORDER BY fecha_registro DESC
		LIMIT $%d OFFSET $%d
	`, expedienteColumns, whereClause, len(limitArgs)-1, len(limitArgs)), limitArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*Expediente, 0)
	for rows.Next() {
		item, err := scanExpediente(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Update solo permite modificar los campos no críticos (asunto, descripcion,
// prioridad). El estado se cambia exclusivamente mediante UpdateEstado.
func (r *ExpedienteRepository) Update(ctx context.Context, idOrCodigo, asunto, descripcion, prioridad string) (*Expediente, error) {
	updated, err := scanExpediente(r.db.QueryRow(ctx, `
		UPDATE expedientes
		SET asunto = $1, descripcion = $2, prioridad = $3, fecha_actualizacion = NOW()
		WHERE (id::text = $4 OR codigo = $4) AND activo = TRUE
		RETURNING `+expedienteColumns,
		asunto, descripcion, prioridad, idOrCodigo,
	))
	if err != nil {
		return nil, classifyNotFound(err, ErrExpedienteNotFound)
	}
	return updated, nil
}

// UpdateEstado exige el estado actual esperado (estadoActual) como guarda de
// concurrencia optimista: si el expediente cambió de estado entre la lectura
// y la escritura, no encuentra la fila y devuelve ErrExpedienteNotFound.
func (r *ExpedienteRepository) UpdateEstado(ctx context.Context, idOrCodigo, estadoActual, nuevoEstado string) (*Expediente, error) {
	updated, err := scanExpediente(r.db.QueryRow(ctx, `
		UPDATE expedientes
		SET estado = $1, fecha_actualizacion = NOW()
		WHERE (id::text = $2 OR codigo = $2) AND activo = TRUE AND estado = $3
		RETURNING `+expedienteColumns,
		nuevoEstado, idOrCodigo, estadoActual,
	))
	if err != nil {
		return nil, classifyNotFound(err, ErrExpedienteNotFound)
	}
	return updated, nil
}
