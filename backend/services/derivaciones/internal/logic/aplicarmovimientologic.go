package logic

import (
	"context"
	"strings"
	"time"

	"derivaciones/derivaciones"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/svc"
	"derivaciones/internal/validation"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AplicarMovimientoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAplicarMovimientoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AplicarMovimientoLogic {
	return &AplicarMovimientoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AplicarMovimiento: consumidor idempotente del Outbox de Expedientes (A + B).
// Solo lo invoca el publicador con un token de servicio (lo exige el
// interceptor). Entrega al menos una vez: un reenvío idéntico es DUPLICADO
// (éxito); un contenido distinto con el mismo id es DIVERGENTE (permanente,
// nunca se acepta en silencio); un hueco de secuencia es Aborted (el
// publicador reintenta más tarde, sin aplicar fuera de orden).
func (l *AplicarMovimientoLogic) AplicarMovimiento(in *derivaciones.AplicarMovimientoRequest) (*derivaciones.AplicarMovimientoResponse, error) {
	if l.svcCtx.ProyeccionRepository == nil {
		return nil, status.Error(codes.Unavailable, "la proyección de movimientos no está configurada")
	}
	if _, ok := interceptor.ServicioFromContext(l.ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere un token de servicio")
	}
	m, err := validarMovimiento(in.GetMovimiento())
	if err != nil {
		return nil, err
	}
	res, err := l.svcCtx.ProyeccionRepository.Aplicar(l.ctx, m)
	if err != nil {
		l.Errorf("aplicar movimiento %s: %v", m.MovimientoID, err)
		return nil, status.Error(codes.Internal, "no se pudo aplicar el movimiento")
	}
	switch res {
	case repository.ProyAplicado:
		return &derivaciones.AplicarMovimientoResponse{Resultado: derivaciones.AplicarMovimientoResponse_APLICADO}, nil
	case repository.ProyDuplicado:
		return &derivaciones.AplicarMovimientoResponse{Resultado: derivaciones.AplicarMovimientoResponse_DUPLICADO}, nil
	case repository.ProyHueco:
		return nil, status.Error(codes.Aborted, "falta la secuencia anterior del expediente; reintente más tarde")
	default:
		l.Errorf("ALERTA proyección divergente: movimiento %s (expediente %s, secuencia %d)", m.MovimientoID, m.ExpedienteID, m.Secuencia)
		return nil, status.Error(codes.FailedPrecondition, "movimiento divergente: ya existe con otro contenido o la secuencia está ocupada")
	}
}

func validarMovimiento(p *derivaciones.MovimientoProyectado) (repository.MovimientoProyectado, error) {
	invalido := func(msg string) (repository.MovimientoProyectado, error) {
		return repository.MovimientoProyectado{}, status.Error(codes.InvalidArgument, msg)
	}
	if p == nil {
		return invalido("el movimiento es obligatorio")
	}
	if !validation.IsValidUUID(p.MovimientoId) || !validation.IsValidUUID(p.ExpedienteId) || !validation.IsValidUUID(p.ActorId) {
		return invalido("identificadores del movimiento inválidos")
	}
	if p.Secuencia <= 0 {
		return invalido("la secuencia debe ser positiva")
	}
	if !validation.IsValidTipoMovimiento(p.Tipo) {
		return invalido("tipo de movimiento inválido")
	}
	if !validation.IsValidArea(p.AreaOrigen) || !validation.IsValidArea(p.AreaDestino) {
		return invalido("área del movimiento inválida")
	}
	if !validation.IsValidEstado(p.EstadoAnterior) || !validation.IsValidEstado(p.EstadoNuevo) {
		return invalido("estado del movimiento inválido")
	}
	rol := strings.TrimSpace(p.ActorRol)
	if rol == "" || len(rol) > 30 || len([]rune(p.Motivo)) > 500 || len([]rune(p.Condicion)) > 255 {
		return invalido("datos del movimiento inválidos")
	}
	ocurrido, err := time.Parse(time.RFC3339Nano, p.OcurridoEn)
	if err != nil {
		return invalido("fecha del movimiento inválida")
	}
	return repository.MovimientoProyectado{
		MovimientoID: strings.ToLower(p.MovimientoId), ExpedienteID: strings.ToLower(p.ExpedienteId), Secuencia: int(p.Secuencia),
		Tipo: p.Tipo, ActorID: strings.ToLower(p.ActorId), ActorRol: rol, AreaOrigen: p.AreaOrigen, AreaDestino: p.AreaDestino,
		EstadoAnterior: p.EstadoAnterior, EstadoNuevo: p.EstadoNuevo, Motivo: p.Motivo, Condicion: p.Condicion, OcurridoEn: ocurrido,
	}, nil
}
