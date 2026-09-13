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
	AreaActual         string
}

type ListFilter struct {
	Estado        string
	Prioridad     string
	Tipo          string
	SolicitanteID string
	AreaActual    string
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
	fecha_registro, fecha_actualizacion, activo, area_actual
`

func scanExpediente(row pgx.Row) (*Expediente, error) {
	e := &Expediente{}
	err := row.Scan(
		&e.ID, &e.Codigo, &e.Tipo, &e.Asunto, &e.Descripcion, &e.SolicitanteID,
		&e.Estado, &e.Prioridad, &e.FechaRegistro, &e.FechaActualizacion, &e.Activo, &e.AreaActual,
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
	if filter.AreaActual != "" {
		addCondition("area_actual", filter.AreaActual)
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

// Delete es una baja logica: marca activo = FALSE en vez de borrar la fila,
// para no perder el historial de un expediente que es un registro oficial.
// A partir de ahi, el resto de los metodos (que ya filtran activo = TRUE)
// dejan de encontrarlo.
func (r *ExpedienteRepository) Delete(ctx context.Context, idOrCodigo string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE expedientes
		SET activo = FALSE, fecha_actualizacion = NOW()
		WHERE (id::text = $1 OR codigo = $1) AND activo = TRUE
	`, idOrCodigo)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrExpedienteNotFound
	}
	return nil
}

// UpdateArea cambia el area interna responsable del expediente (a quien le
// corresponde atenderlo ahora). Se llama cuando se registra una derivacion
// real (ver Derivaciones Service) hacia otra area. Exige el area_actual
// esperada (areaAnterior) como guarda de concurrencia optimista — mismo
// patron que UpdateEstado: si el area cambio entre la lectura y la
// escritura, no encuentra la fila y devuelve ErrExpedienteNotFound.
func (r *ExpedienteRepository) UpdateArea(ctx context.Context, idOrCodigo, areaAnterior, nuevaArea string) (*Expediente, error) {
	updated, err := scanExpediente(r.db.QueryRow(ctx, `
		UPDATE expedientes
		SET area_actual = $1, fecha_actualizacion = NOW()
		WHERE (id::text = $2 OR codigo = $2) AND activo = TRUE AND area_actual = $3
		RETURNING `+expedienteColumns,
		nuevaArea, idOrCodigo, areaAnterior,
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

// RechazarYDevolver (Etapa 4): Dirección o Subdirección rechazan un F4 y el
// expediente vuelve a Secretaría para que el solicitante corrija — cambia
// estado Y área en una sola escritura (ambos viven en la misma fila/tabla,
// no hay problema de transacción distribuida acá). Exige ambos valores
// anteriores como guarda de concurrencia optimista, mismo patrón que
// UpdateArea/UpdateEstado.
func (r *ExpedienteRepository) RechazarYDevolver(ctx context.Context, idOrCodigo, estadoActual, areaActual, nuevaArea string) (*Expediente, error) {
	updated, err := scanExpediente(r.db.QueryRow(ctx, `
		UPDATE expedientes
		SET estado = 'OBSERVADO', area_actual = $1, fecha_actualizacion = NOW()
		WHERE (id::text = $2 OR codigo = $2) AND activo = TRUE AND estado = $3 AND area_actual = $4
		RETURNING `+expedienteColumns,
		nuevaArea, idOrCodigo, estadoActual, areaActual,
	))
	if err != nil {
		return nil, classifyNotFound(err, ErrExpedienteNotFound)
	}
	return updated, nil
}

// CorregirYReenviar (Etapa 4): el solicitante corrige un expediente
// OBSERVADO y vuelve a entrar a la revisión de Secretaría (mismo código,
// mismo expediente — no se crea uno nuevo). Actualiza la descripción y
// regresa el estado a PENDIENTE. Exige estado=OBSERVADO como guarda.
func (r *ExpedienteRepository) CorregirYReenviar(ctx context.Context, idOrCodigo, descripcionNueva string) (*Expediente, error) {
	updated, err := scanExpediente(r.db.QueryRow(ctx, `
		UPDATE expedientes
		SET estado = 'PENDIENTE', descripcion = $1, fecha_actualizacion = NOW()
		WHERE (id::text = $2 OR codigo = $2) AND activo = TRUE AND estado = 'OBSERVADO'
		RETURNING `+expedienteColumns,
		descripcionNueva, idOrCodigo,
	))
	if err != nil {
		return nil, classifyNotFound(err, ErrExpedienteNotFound)
	}
	return updated, nil
}

// DerivarConCambioDeEstado (Etapa 4): caso especial de derivación que
// además cambia el estado en la MISMA escritura — necesario cuando mover
// el área y cambiar el estado por separado dejaría una ventana donde el
// actor original ya no es dueño del área nueva para poder completar el
// segundo paso (ver DerivarExpedienteLogic para los dos casos reales:
// Secretaría -> Dirección pasa PENDIENTE -> EN_PROCESO para F2/F4, y
// Subdirección -> Docente en F4 además cierra a ATENDIDO). Exige área Y
// estado anteriores como guarda de concurrencia optimista.
func (r *ExpedienteRepository) DerivarConCambioDeEstado(ctx context.Context, idOrCodigo, areaActual, nuevaArea, estadoActual, nuevoEstado string) (*Expediente, error) {
	updated, err := scanExpediente(r.db.QueryRow(ctx, `
		UPDATE expedientes
		SET area_actual = $1, estado = $2, fecha_actualizacion = NOW()
		WHERE (id::text = $3 OR codigo = $3) AND activo = TRUE AND area_actual = $4 AND estado = $5
		RETURNING `+expedienteColumns,
		nuevaArea, nuevoEstado, idOrCodigo, areaActual, estadoActual,
	))
	if err != nil {
		return nil, classifyNotFound(err, ErrExpedienteNotFound)
	}
	return updated, nil
}
