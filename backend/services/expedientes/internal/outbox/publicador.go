// Package outbox: publicador del Transactional Outbox de Expedientes (A + B).
//
// Corre dentro del proceso de Expedientes con su PROPIA conexión (rol
// expedientes_publicador: reclama, reprograma y cierra eventos; nunca crea
// eventos ni toca expedientes o movimientos). Entrega al menos una vez a
// Derivaciones.AplicarMovimiento, cuyo consumidor es idempotente: no hay
// entrega exactamente una vez entre bases independientes.
//
// Orden: solo se reclama la CABEZA (menor secuencia no publicada) de cada
// expediente; un evento posterior nunca se envía antes que el anterior.
// Fencing: cada reclamo lleva un reclamo_id aleatorio y todo cierre exige
// id + reclamo_id + estado PENDIENTE; si un cierre afecta 0 filas, el reclamo
// se perdió (lease vencido y retomado por otra instancia) y el evento no se
// vuelve a tocar.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	derivacionespb "derivaciones/derivaciones"
	"derivaciones/derivacionesclient"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Config del publicador (valores ya validados por svc).
type Config struct {
	Lote          int
	Lease         time.Duration
	Intervalo     time.Duration
	MaxIntentos   int
	TimeoutRPC    time.Duration
	RetencionDias int
}

// Proveedor de tokens de servicio (Auth.IssueServiceToken con caché).
type Proveedor interface {
	Token(ctx context.Context) (string, error)
	Invalidar()
}

type Publicador struct {
	db      *pgxpool.Pool
	deriv   derivacionesclient.Derivaciones
	tokens  Proveedor
	cfg     Config
	aviso   chan struct{}
	ahora   func() time.Time
	logger  logx.Logger
	ultPurg time.Time
	ultStat time.Time
}

func New(db *pgxpool.Pool, deriv derivacionesclient.Derivaciones, tokens Proveedor, cfg Config) *Publicador {
	return &Publicador{db: db, deriv: deriv, tokens: tokens, cfg: cfg, aviso: make(chan struct{}, 1),
		ahora: time.Now, logger: logx.WithContext(context.Background())}
}

// Avisar despierta al publicador tras un COMMIT (no bloquea).
func (p *Publicador) Avisar() {
	if p == nil {
		return
	}
	select {
	case p.aviso <- struct{}{}:
	default:
	}
}

// Correr procesa eventos hasta que ctx se cancele. Al arrancar hace un barrido
// inmediato (recuperación tras un reinicio: los leases vencidos se retoman).
func (p *Publicador) Correr(ctx context.Context) {
	p.logger.Infof("publicador del outbox iniciado (lote=%d lease=%s intervalo=%s)", p.cfg.Lote, p.cfg.Lease, p.cfg.Intervalo)
	esperaToken := time.Second
	for ctx.Err() == nil {
		if _, err := p.tokens.Token(ctx); err != nil {
			// Sin token no se reclama nada: los intentos de los eventos no crecen.
			p.logger.Errorf("publicador: no se pudo obtener el token de servicio (reintento en %s): %v", esperaToken, err)
			if !dormir(ctx, esperaToken, nil) {
				break
			}
			esperaToken = min(esperaToken*2, time.Minute)
			continue
		}
		esperaToken = time.Second
		n, err := p.Ciclo(ctx)
		if err != nil && ctx.Err() == nil {
			p.logger.Errorf("publicador: ciclo: %v", err)
		}
		p.mantenimiento(ctx)
		if n == 0 && !dormir(ctx, p.cfg.Intervalo, p.aviso) {
			break
		}
	}
	p.logger.Infof("publicador del outbox detenido")
}

func dormir(ctx context.Context, d time.Duration, aviso <-chan struct{}) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
	case <-aviso:
	}
	return true
}

// Reclamo de un evento.
type Reclamo struct {
	ID       int64
	Reclamo  string
	Mov      string
	Intentos int
}

const sqlReclamo = `
WITH cabezas AS (
    SELECT DISTINCT ON (expediente_id) id, estado, proximo_intento_en, reclamado_hasta
      FROM expediente_outbox WHERE estado <> 'PUBLICADO'
     ORDER BY expediente_id, secuencia),
elegibles AS (
    SELECT id FROM cabezas
     WHERE estado = 'PENDIENTE' AND proximo_intento_en <= clock_timestamp()
       AND (reclamado_hasta IS NULL OR reclamado_hasta < clock_timestamp())
     ORDER BY proximo_intento_en, id LIMIT $1),
bloqueados AS (
    SELECT o.id FROM expediente_outbox o
     WHERE o.id IN (SELECT id FROM elegibles) AND o.estado = 'PENDIENTE'
       AND (o.reclamado_hasta IS NULL OR o.reclamado_hasta < clock_timestamp())
     FOR UPDATE SKIP LOCKED)
UPDATE expediente_outbox o
   SET reclamo_id = gen_random_uuid(), reclamado_hasta = clock_timestamp() + $2::interval, intentos = o.intentos + 1
  FROM bloqueados b WHERE o.id = b.id
RETURNING o.id, o.reclamo_id::text, o.movimiento_id::text, o.intentos`

// Ciclo: un reclamo y su procesamiento. Devuelve cuántos eventos reclamó.
func (p *Publicador) Ciclo(ctx context.Context) (int, error) {
	inicio := p.ahora()
	reclamos, err := p.reclamar(ctx)
	if err != nil || len(reclamos) == 0 {
		return 0, err
	}
	for _, r := range reclamos {
		// Se procesa solo si queda lease suficiente; el resto se deja vencer
		// y otra iteración (o instancia) lo retoma.
		if p.ahora().Sub(inicio) > p.cfg.Lease-p.cfg.TimeoutRPC-2*time.Second {
			break
		}
		p.procesar(ctx, r)
	}
	return len(reclamos), nil
}

func (p *Publicador) reclamar(ctx context.Context) ([]Reclamo, error) {
	rows, err := p.db.Query(ctx, sqlReclamo, p.cfg.Lote, fmt.Sprintf("%d milliseconds", p.cfg.Lease.Milliseconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Reclamo
	for rows.Next() {
		var r Reclamo
		if err := rows.Scan(&r.ID, &r.Reclamo, &r.Mov, &r.Intentos); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Resultado de un envío según el código gRPC del consumidor.
type decision int

const (
	publicar decision = iota
	reintentar
	fallar
)

// Clasificar: OK = publicado; divergente o inválido = permanente (FALLIDO);
// cualquier otro error (hueco, Unavailable, timeout, Unauthenticated,
// Unimplemented, PermissionDenied por configuración...) = reintento con backoff.
func Clasificar(err error) decision {
	if err == nil {
		return publicar
	}
	switch status.Code(err) {
	case codes.FailedPrecondition, codes.InvalidArgument:
		return fallar
	}
	return reintentar
}

// Backoff exponencial con jitter ±20 %, de 1 s a 5 min.
func Backoff(intentos int) time.Duration {
	d := time.Second << uint(min(max(intentos-1, 0), 9))
	d = min(d, 5*time.Minute)
	j := 0.8 + rand.Float64()*0.4
	return time.Duration(float64(d) * j)
}

func (p *Publicador) procesar(ctx context.Context, r Reclamo) {
	mov, err := p.leerMovimiento(ctx, r.Mov)
	if err != nil {
		p.logger.Errorf("publicador: leer movimiento %s: %v", r.Mov, err)
		p.cerrar(ctx, r, reintentar, "no se pudo leer el movimiento")
		return
	}
	token, err := p.tokens.Token(ctx)
	if err != nil {
		p.cerrar(ctx, r, reintentar, "sin token de servicio")
		return
	}
	cctx, cancel := context.WithTimeout(metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token), p.cfg.TimeoutRPC)
	resp, err := p.deriv.AplicarMovimiento(cctx, &derivacionesclient.AplicarMovimientoRequest{Movimiento: mov})
	cancel()
	if status.Code(err) == codes.Unauthenticated {
		p.tokens.Invalidar()
	}
	d := Clasificar(err)
	if d == reintentar && r.Intentos >= p.cfg.MaxIntentos {
		d = fallar
	}
	motivo := ""
	if err != nil {
		motivo = fmt.Sprintf("%s: %s", status.Code(err), status.Convert(err).Message())
	}
	if d == publicar && resp.GetResultado() == derivacionespb.AplicarMovimientoResponse_DUPLICADO {
		p.logger.Infof("publicador: movimiento %s ya estaba proyectado (DUPLICADO)", r.Mov)
	}
	if d == fallar {
		p.logger.Errorf("ALERTA publicador: evento %d (movimiento %s) FALLIDO tras %d intentos: %s", r.ID, r.Mov, r.Intentos, motivo)
	}
	p.cerrar(ctx, r, d, motivo)
}

// cerrar aplica el fencing: id + reclamo_id + estado. 0 filas = reclamo perdido.
func (p *Publicador) cerrar(ctx context.Context, r Reclamo, d decision, motivo string) {
	if len(motivo) > 480 {
		motivo = motivo[:480]
	}
	// El cierre usa un contexto propio para no perderlo si ctx se cancela justo ahora.
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var sql string
	var args []any
	switch d {
	case publicar:
		sql = `UPDATE expediente_outbox SET estado = 'PUBLICADO', reclamo_id = NULL, reclamado_hasta = NULL, ultimo_error = NULL
			WHERE id = $1 AND reclamo_id = $2 AND estado = 'PENDIENTE'`
		args = []any{r.ID, r.Reclamo}
	case reintentar:
		sql = `UPDATE expediente_outbox SET proximo_intento_en = clock_timestamp() + $3::interval, reclamo_id = NULL,
			reclamado_hasta = NULL, ultimo_error = $4 WHERE id = $1 AND reclamo_id = $2 AND estado = 'PENDIENTE'`
		args = []any{r.ID, r.Reclamo, fmt.Sprintf("%d milliseconds", Backoff(r.Intentos).Milliseconds()), motivo}
	default:
		sql = `UPDATE expediente_outbox SET estado = 'FALLIDO', reclamo_id = NULL, reclamado_hasta = NULL, ultimo_error = $3
			WHERE id = $1 AND reclamo_id = $2 AND estado = 'PENDIENTE'`
		args = []any{r.ID, r.Reclamo, motivo}
	}
	tag, err := p.db.Exec(c, sql, args...)
	if err != nil {
		p.logger.Errorf("publicador: cerrar evento %d: %v", r.ID, err)
		return
	}
	if tag.RowsAffected() == 0 {
		p.logger.Infof("publicador: reclamo perdido del evento %d (lease vencido y retomado); no se modifica", r.ID)
	}
}

func (p *Publicador) leerMovimiento(ctx context.Context, id string) (*derivacionesclient.MovimientoProyectado, error) {
	m := &derivacionesclient.MovimientoProyectado{}
	var ocurrido time.Time
	err := p.db.QueryRow(ctx, `SELECT id::text, expediente_id::text, secuencia, tipo, actor_id::text, actor_rol, area_origen,
		area_destino, estado_anterior, estado_nuevo, motivo, condicion, ocurrido_en FROM expediente_movimientos WHERE id = $1`, id).Scan(
		&m.MovimientoId, &m.ExpedienteId, &m.Secuencia, &m.Tipo, &m.ActorId, &m.ActorRol, &m.AreaOrigen, &m.AreaDestino,
		&m.EstadoAnterior, &m.EstadoNuevo, &m.Motivo, &m.Condicion, &ocurrido)
	if err != nil {
		return nil, err
	}
	// Se serializa el valor LEÍDO de PostgreSQL (microsegundos), no time.Now():
	// así el consumidor puede comparar ocurrido_en exactamente.
	m.OcurridoEn = ocurrido.UTC().Format(time.RFC3339Nano)
	return m, nil
}

// mantenimiento: estado del outbox (cada minuto) y purga (cada hora).
func (p *Publicador) mantenimiento(ctx context.Context) {
	ahora := p.ahora()
	if ahora.Sub(p.ultStat) >= time.Minute {
		p.ultStat = ahora
		var pendientes, fallidos int
		var antiguedad float64
		err := p.db.QueryRow(ctx, `SELECT count(*) FILTER (WHERE estado = 'PENDIENTE'), count(*) FILTER (WHERE estado = 'FALLIDO'),
			coalesce(extract(epoch FROM clock_timestamp() - min(creado_en) FILTER (WHERE estado = 'PENDIENTE')), 0)
			FROM expediente_outbox WHERE estado <> 'PUBLICADO'`).Scan(&pendientes, &fallidos, &antiguedad)
		if err == nil {
			p.logger.Infow("estado del outbox", logx.Field("pendientes", pendientes), logx.Field("fallidos", fallidos),
				logx.Field("antiguedad_pendiente_s", int64(antiguedad)))
			if fallidos > 0 || antiguedad > 300 {
				p.logger.Errorf("ALERTA outbox: %d evento(s) FALLIDO, pendiente más antiguo %.0f s", fallidos, antiguedad)
			}
		}
	}
	if p.cfg.RetencionDias > 0 && ahora.Sub(p.ultPurg) >= time.Hour {
		p.ultPurg = ahora
		tag, err := p.db.Exec(ctx, `DELETE FROM expediente_outbox WHERE id IN (
			SELECT id FROM expediente_outbox WHERE estado = 'PUBLICADO' AND publicado_en < clock_timestamp() - $1::interval
			ORDER BY id LIMIT 500)`, fmt.Sprintf("%d days", p.cfg.RetencionDias))
		if err != nil && !errors.Is(err, context.Canceled) {
			p.logger.Errorf("publicador: purga: %v", err)
		} else if err == nil && tag.RowsAffected() > 0 {
			p.logger.Infof("publicador: purgados %d eventos publicados", tag.RowsAffected())
		}
	}
}
