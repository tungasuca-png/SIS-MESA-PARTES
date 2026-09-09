package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Derivacion es un registro histórico (append-only) de un movimiento de un
// expediente: quién se lo pasó a quién, por qué, y de qué tipo fue (ver
// internal/validation para los tipos — derivación real, notificación,
// asignación, recepción o aprobación, según sección 15 del análisis
// funcional). No tiene estado propio ni se edita/borra: es una bitácora, no
// una entidad con ciclo de vida — el estado del trámite en sí vive en
// Expedientes Service.
type Derivacion struct {
	ID            string
	ExpedienteID  string
	Tipo          string
	Origen        string
	Destino       string
	Motivo        string
	Condicion     string
	RegistradoPor string
	FechaRegistro time.Time
}

type DerivacionRepository struct {
	db *pgxpool.Pool
}

func NewDerivacionRepository(db *pgxpool.Pool) *DerivacionRepository {
	return &DerivacionRepository{db: db}
}

const derivacionColumns = `
	id, expediente_id, tipo, origen, destino, motivo, condicion,
	registrado_por, fecha_registro
`

func scanDerivacion(row pgx.Row) (*Derivacion, error) {
	d := &Derivacion{}
	err := row.Scan(
		&d.ID, &d.ExpedienteID, &d.Tipo, &d.Origen, &d.Destino, &d.Motivo,
		&d.Condicion, &d.RegistradoPor, &d.FechaRegistro,
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *DerivacionRepository) Create(ctx context.Context, expedienteID, tipo, origen, destino, motivo, condicion, registradoPor string) (*Derivacion, error) {
	created, err := scanDerivacion(r.db.QueryRow(ctx, `
		INSERT INTO derivaciones (expediente_id, tipo, origen, destino, motivo, condicion, registrado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+derivacionColumns,
		expedienteID, tipo, origen, destino, motivo, condicion, registradoPor,
	))
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *DerivacionRepository) Get(ctx context.Context, id string) (*Derivacion, error) {
	found, err := scanDerivacion(r.db.QueryRow(ctx, `
		SELECT `+derivacionColumns+`
		FROM derivaciones
		WHERE id = $1
	`, id))
	if err != nil {
		return nil, classifyNotFound(err, ErrDerivacionNotFound)
	}
	return found, nil
}

// ListFilter filtra el listado de derivaciones. Los campos vacios no filtran.
type ListFilter struct {
	ExpedienteID string
	Tipo         string
	Page         int
	PageSize     int
}

func (r *DerivacionRepository) List(ctx context.Context, filter ListFilter) ([]*Derivacion, int, error) {
	conditions := []string{"TRUE"}
	args := make([]any, 0, 2)

	addCondition := func(column, value string) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if filter.ExpedienteID != "" {
		addCondition("expediente_id", filter.ExpedienteID)
	}
	if filter.Tipo != "" {
		addCondition("tipo", filter.Tipo)
	}
	whereClause := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM derivaciones WHERE "+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM derivaciones
		WHERE %s
		ORDER BY fecha_registro DESC
		LIMIT $%d OFFSET $%d
	`, derivacionColumns, whereClause, len(limitArgs)-1, len(limitArgs)), limitArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*Derivacion, 0)
	for rows.Next() {
		item, err := scanDerivacion(rows)
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
