package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Movimiento es el registro inmutable de un cambio de área y/o estado de un
// expediente (tabla expediente_movimientos, migración 004).
type Movimiento struct {
	ID             string
	ExpedienteID   string
	Secuencia      int
	Tipo           string
	ActorID        string
	ActorRol       string
	AreaOrigen     string
	AreaDestino    string
	EstadoAnterior string
	EstadoNuevo    string
	Motivo         string
	Condicion      string
	OcurridoEn     time.Time
}

// Tipos de movimiento (chk_mov_tipo).
const (
	MovDerivacion   = "DERIVACION"
	MovRechazo      = "RECHAZO"
	MovCambioEstado = "CAMBIO_ESTADO"
	MovResolucion   = "RESOLUCION"
	MovCorreccion   = "CORRECCION"
)

var (
	// ErrExpedienteOcupado: otra operación tiene bloqueada la fila del
	// expediente más allá de lock_timeout. Reintentable.
	ErrExpedienteOcupado = errors.New("expediente ocupado por otra operación")
	// ErrClaveReutilizada: la misma (actor, idempotency_key) ya se usó con
	// una solicitud de contenido distinto.
	ErrClaveReutilizada = errors.New("clave de idempotencia reutilizada con otro contenido")
	// ErrResultadoIncierto: el COMMIT pudo o no confirmarse (deadline o
	// conexión perdida durante el COMMIT). Se resuelve reintentando con la
	// misma clave: si se confirmó, se devuelve el mismo movimiento.
	ErrResultadoIncierto = errors.New("no se pudo confirmar el resultado de la operación")
)

// Decision es lo que las reglas de negocio deciden sobre el expediente YA
// bloqueado: área destino y estado nuevo. NuevaDescripcion solo la usa
// CorregirExpediente (se actualiza en el mismo UPDATE).
type Decision struct {
	AreaDestino      string
	EstadoNuevo      string
	NuevaDescripcion *string
}

// SolicitudMovimiento describe una operación que mueve área o estado.
type SolicitudMovimiento struct {
	IDOrCodigo     string
	Tipo           string
	ActorID        string
	ActorRol       string
	IdempotencyKey string
	// Contenido: representación canónica de los datos de la solicitud (sin
	// el id del expediente, que se agrega ya resuelto) para la huella.
	Contenido string
	Motivo    string
	Condicion string
	// Decidir aplica autorización y reglas de negocio sobre el expediente
	// BLOQUEADO (sin carreras entre la comprobación y la escritura). Un error
	// devuelto aquí aborta la transacción sin escribir nada.
	Decidir func(actual *Expediente) (Decision, error)
}

// ResultadoMovimiento: expediente tras la operación y su movimiento.
// Repetido = true cuando la clave ya se había usado con el mismo contenido
// (se devuelve el movimiento original; el expediente es el estado actual).
type ResultadoMovimiento struct {
	Expediente *Expediente
	Movimiento *Movimiento
	Repetido   bool
}

const movimientoColumns = `
	id, expediente_id, secuencia, tipo, actor_id, actor_rol, area_origen, area_destino,
	estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en
`

func scanMovimiento(row pgx.Row) (*Movimiento, error) {
	m := &Movimiento{}
	err := row.Scan(&m.ID, &m.ExpedienteID, &m.Secuencia, &m.Tipo, &m.ActorID, &m.ActorRol, &m.AreaOrigen,
		&m.AreaDestino, &m.EstadoAnterior, &m.EstadoNuevo, &m.Motivo, &m.Condicion, &m.OcurridoEn)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Huella: SHA-256 (hex) del expediente ya resuelto, el tipo y el contenido.
func Huella(expedienteID, tipo, contenido string) string {
	h := sha256.Sum256([]byte(expedienteID + "|" + tipo + "|" + contenido))
	return hex.EncodeToString(h[:])
}

// Presupuesto de tiempo dentro de la transacción: menor que el deadline
// restante del RPC, para que la base corte antes que el cliente.
func presupuestoSentencia(ctx context.Context) time.Duration {
	const maximo = 1000 * time.Millisecond
	d, ok := ctx.Deadline()
	if !ok {
		return maximo
	}
	resto := time.Until(d) - 300*time.Millisecond
	if resto < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	if resto > maximo {
		return maximo
	}
	return resto
}

// RegistrarMovimiento es el ÚNICO camino para cambiar área o estado:
//  1. bloquea la fila del expediente (FOR UPDATE, lock_timeout 500 ms);
//  2. idempotencia por (actor, clave), comprobada tras el bloqueo;
//  3. reglas de negocio sobre el estado bloqueado (Decidir);
//  4. UPDATE del expediente + INSERT del movimiento + INSERT del outbox;
//  5. COMMIT (los triggers diferidos verifican la coherencia).
//
// Cualquier error antes del COMMIT revierte todo. Si el COMMIT queda
// incierto se devuelve ErrResultadoIncierto (el reintento con la misma clave
// resuelve el resultado real).
func (r *ExpedienteRepository) RegistrarMovimiento(ctx context.Context, s SolicitudMovimiento) (*ResultadoMovimiento, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		rc, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(rc)
	}()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`SET LOCAL lock_timeout = '500ms'; SET LOCAL statement_timeout = '%dms'`,
		presupuestoSentencia(ctx).Milliseconds())); err != nil {
		return nil, err
	}

	var ultimaSecuencia int
	actual := &Expediente{}
	err = tx.QueryRow(ctx, `SELECT `+expedienteColumns+`, ultima_secuencia FROM expedientes
		WHERE (id::text = $1 OR codigo = $1) AND activo = TRUE FOR UPDATE`, s.IDOrCodigo).Scan(
		&actual.ID, &actual.Codigo, &actual.Tipo, &actual.Asunto, &actual.Descripcion, &actual.SolicitanteID,
		&actual.Estado, &actual.Prioridad, &actual.FechaRegistro, &actual.FechaActualizacion, &actual.Activo,
		&actual.AreaActual, &ultimaSecuencia)
	if err != nil {
		return nil, clasificarError(err)
	}

	huella := Huella(actual.ID, s.Tipo, s.Contenido)
	if previo, err := buscarIdempotente(ctx, tx, s.ActorID, s.IdempotencyKey, huella); err != nil || previo != nil {
		if err != nil {
			return nil, err
		}
		return &ResultadoMovimiento{Expediente: actual, Movimiento: previo, Repetido: true}, nil
	}

	decision, err := s.Decidir(actual)
	if err != nil {
		return nil, err
	}

	secuencia := ultimaSecuencia + 1
	var actualizado *Expediente
	if decision.NuevaDescripcion != nil {
		actualizado, err = scanExpediente(tx.QueryRow(ctx, `UPDATE expedientes
			SET area_actual = $2, estado = $3, ultima_secuencia = $4, descripcion = $5, fecha_actualizacion = NOW()
			WHERE id = $1 RETURNING `+expedienteColumns,
			actual.ID, decision.AreaDestino, decision.EstadoNuevo, secuencia, *decision.NuevaDescripcion))
	} else {
		actualizado, err = scanExpediente(tx.QueryRow(ctx, `UPDATE expedientes
			SET area_actual = $2, estado = $3, ultima_secuencia = $4, fecha_actualizacion = NOW()
			WHERE id = $1 RETURNING `+expedienteColumns,
			actual.ID, decision.AreaDestino, decision.EstadoNuevo, secuencia))
	}
	if err != nil {
		return nil, clasificarError(err)
	}

	mov, err := scanMovimiento(tx.QueryRow(ctx, `INSERT INTO expediente_movimientos
		(expediente_id, secuencia, tipo, actor_id, actor_rol, area_origen, area_destino, estado_anterior, estado_nuevo,
		 motivo, condicion, idempotency_key, huella_solicitud)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING `+movimientoColumns,
		actual.ID, secuencia, s.Tipo, s.ActorID, s.ActorRol, actual.AreaActual, decision.AreaDestino,
		actual.Estado, decision.EstadoNuevo, s.Motivo, s.Condicion, s.IdempotencyKey, huella))
	if err != nil {
		return r.resolverCarrera(ctx, s, huella, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO expediente_outbox (movimiento_id, expediente_id, secuencia) VALUES ($1, $2, $3)`,
		mov.ID, actual.ID, secuencia); err != nil {
		return nil, clasificarError(err)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err() // vencido antes del COMMIT: el defer revierte
	}
	if err := tx.Commit(ctx); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			// Sin respuesta del servidor (deadline, cancelación o conexión
			// perdida con el COMMIT ya enviado): pudo confirmarse. Se comprueba
			// con un contexto nuevo solo para dejar constancia en el log.
			return nil, fmt.Errorf("%w (movimiento %s, confirmado=%s): %v", ErrResultadoIncierto, mov.ID, r.confirmado(mov.ID), err)
		}
		// El servidor respondió con un error: la transacción se revirtió.
		if pgErr.Code == "23505" {
			return r.resolverCarrera(ctx, s, huella, err)
		}
		return nil, clasificarError(err)
	}
	return &ResultadoMovimiento{Expediente: actualizado, Movimiento: mov}, nil
}

// buscarIdempotente devuelve el movimiento previo de (actor, clave) si existe
// y coincide la huella; ErrClaveReutilizada si existe con otra huella.
func buscarIdempotente(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, actorID, clave, huella string) (*Movimiento, error) {
	var h string
	m := &Movimiento{}
	err := q.QueryRow(ctx, `SELECT `+movimientoColumns+`, huella_solicitud FROM expediente_movimientos
		WHERE actor_id = $1 AND idempotency_key = $2`, actorID, clave).Scan(
		&m.ID, &m.ExpedienteID, &m.Secuencia, &m.Tipo, &m.ActorID, &m.ActorRol, &m.AreaOrigen, &m.AreaDestino,
		&m.EstadoAnterior, &m.EstadoNuevo, &m.Motivo, &m.Condicion, &m.OcurridoEn, &h)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if h != huella {
		return nil, ErrClaveReutilizada
	}
	return m, nil
}

// resolverCarrera: otra transacción con la misma (actor, clave) confirmó
// primero (23505 en uq_mov_idempotencia). Se lee lo confirmado fuera de la
// transacción abortada.
func (r *ExpedienteRepository) resolverCarrera(ctx context.Context, s SolicitudMovimiento, huella string, causa error) (*ResultadoMovimiento, error) {
	var pgErr *pgconn.PgError
	if !errors.As(causa, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "uq_mov_idempotencia" {
		return nil, clasificarError(causa)
	}
	previo, err := buscarIdempotente(ctx, r.db, s.ActorID, s.IdempotencyKey, huella)
	if err != nil {
		return nil, err
	}
	if previo == nil {
		return nil, clasificarError(causa)
	}
	exp, err := r.FindByIDOrCodigo(ctx, previo.ExpedienteID)
	if err != nil {
		return nil, err
	}
	return &ResultadoMovimiento{Expediente: exp, Movimiento: previo, Repetido: true}, nil
}

func (r *ExpedienteRepository) confirmado(movimientoID string) string {
	c, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var n int
	if err := r.db.QueryRow(c, `SELECT count(*) FROM expediente_movimientos WHERE id = $1`, movimientoID).Scan(&n); err != nil {
		return "desconocido"
	}
	if n == 1 {
		return "sí"
	}
	return "no"
}

func clasificarError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrExpedienteNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "55P03": // lock_not_available
			return ErrExpedienteOcupado
		case "57014": // query_canceled (statement_timeout)
			return ErrExpedienteOcupado
		}
	}
	if strings.Contains(err.Error(), "context deadline exceeded") {
		return context.DeadlineExceeded
	}
	return err
}
