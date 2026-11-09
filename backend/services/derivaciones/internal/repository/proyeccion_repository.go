package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MovimientoProyectado: copia exacta de un movimiento de Expedientes (fuente
// de verdad) en movimientos_proyeccion (migración 002).
type MovimientoProyectado struct {
	MovimientoID   string
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

// Resultado de aplicar un movimiento.
const (
	ProyAplicado   = "APLICADO"   // insertado
	ProyDuplicado  = "DUPLICADO"  // ya estaba, idéntico campo a campo
	ProyDivergente = "DIVERGENTE" // mismo id con otro contenido, u otro id con la misma secuencia
	ProyHueco      = "HUECO"      // falta la secuencia anterior (reintentable)
)

var errConflictoSinFila = errors.New("conflicto de movimiento_id sin fila existente")

// ProyeccionRepository usa el pool del rol derivaciones_proyector (solo
// SELECT e INSERT sobre movimientos_proyeccion).
type ProyeccionRepository struct {
	db *pgxpool.Pool
}

func NewProyeccionRepository(db *pgxpool.Pool) *ProyeccionRepository {
	return &ProyeccionRepository{db: db}
}

// Aplicar es idempotente y estricto (corrección H-E1-02 validada en E1):
//  1. si el movimiento_id ya existe, compara los 12 campos: idéntico =
//     DUPLICADO; cualquier diferencia = DIVERGENTE (nunca se acepta en
//     silencio un contenido distinto);
//  2. si no existe, inserta; el trigger de la base rechaza un hueco de
//     secuencia (TG001 = HUECO) y uq_proy_secuencia otro id con la misma
//     secuencia (DIVERGENTE);
//  3. si el INSERT no insertó (carrera en la PK), vuelve al paso 1.
func (r *ProyeccionRepository) Aplicar(ctx context.Context, m MovimientoProyectado) (string, error) {
	if res, err := r.compararExistente(ctx, m); err != nil || res != "" {
		return res, err
	}
	tag, err := r.db.Exec(ctx, `INSERT INTO movimientos_proyeccion (movimiento_id, expediente_id, secuencia, tipo, actor_id,
		actor_rol, area_origen, area_destino, estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (movimiento_id) DO NOTHING`,
		m.MovimientoID, m.ExpedienteID, m.Secuencia, m.Tipo, m.ActorID, m.ActorRol, m.AreaOrigen, m.AreaDestino,
		m.EstadoAnterior, m.EstadoNuevo, m.Motivo, m.Condicion, m.OcurridoEn)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == "TG001":
				return ProyHueco, nil
			case pgErr.Code == "23505" && pgErr.ConstraintName == "uq_proy_secuencia":
				return ProyDivergente, nil
			}
		}
		return "", err
	}
	if tag.RowsAffected() == 1 {
		return ProyAplicado, nil
	}
	res, err := r.compararExistente(ctx, m)
	if err == nil && res == "" {
		return "", errConflictoSinFila
	}
	return res, err
}

// compararExistente: "" si el movimiento no está proyectado.
func (r *ProyeccionRepository) compararExistente(ctx context.Context, m MovimientoProyectado) (string, error) {
	var identico bool
	err := r.db.QueryRow(ctx, `SELECT (expediente_id, secuencia, tipo, actor_id, actor_rol, area_origen, area_destino,
			estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en)
		IS NOT DISTINCT FROM ($2::uuid, $3::int, $4::varchar, $5::uuid, $6::varchar, $7::varchar, $8::varchar,
			$9::varchar, $10::varchar, $11::varchar, $12::varchar, $13::timestamptz)
		FROM movimientos_proyeccion WHERE movimiento_id = $1`,
		m.MovimientoID, m.ExpedienteID, m.Secuencia, m.Tipo, m.ActorID, m.ActorRol, m.AreaOrigen, m.AreaDestino,
		m.EstadoAnterior, m.EstadoNuevo, m.Motivo, m.Condicion, m.OcurridoEn).Scan(&identico)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if identico {
		return ProyDuplicado, nil
	}
	return ProyDivergente, nil
}
