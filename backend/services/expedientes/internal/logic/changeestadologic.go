package logic

import (
	"context"
	"strings"

	"expedientes/expedientes"
	"expedientes/internal/authorization"
	"expedientes/internal/estados"
	"expedientes/internal/interceptor"
	"expedientes/internal/repository"
	"expedientes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ChangeEstadoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeEstadoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEstadoLogic {
	return &ChangeEstadoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ChangeEstado (reglas sin cambios: permiso expedientes.change_estado,
// propiedad de área y estados.CanTransition). Desde A + B: movimiento
// CAMBIO_ESTADO en la misma transacción, estado esperado e idempotencia.
func (l *ChangeEstadoLogic) ChangeEstado(in *expedientes.ChangeEstadoRequest) (*expedientes.ChangeEstadoResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "expedientes.change_estado"); err != nil {
		return nil, err
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}
	nuevoEstado := strings.ToUpper(strings.TrimSpace(in.NuevoEstado))
	if !estados.IsValidEstado(nuevoEstado) {
		return nil, status.Error(codes.InvalidArgument, "el estado indicado no es válido")
	}
	estadoEsperado := strings.ToUpper(strings.TrimSpace(in.EstadoEsperado))
	if !estados.IsValidEstado(estadoEsperado) {
		return nil, status.Error(codes.InvalidArgument, "el estado esperado es obligatorio")
	}
	clave, err := validarClave(in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	motivo := strings.TrimSpace(in.Motivo)
	if len([]rune(motivo)) > 500 {
		return nil, status.Error(codes.InvalidArgument, "el motivo no puede superar los 500 caracteres")
	}
	if err := validarRolActor(role); err != nil {
		return nil, err
	}

	res, err := l.svcCtx.ExpedienteRepository.RegistrarMovimiento(l.ctx, repository.SolicitudMovimiento{
		IDOrCodigo:     strings.TrimSpace(in.Id),
		Tipo:           repository.MovCambioEstado,
		ActorID:        userID,
		ActorRol:       role,
		IdempotencyKey: clave,
		Contenido:      contenido(nuevoEstado, estadoEsperado, motivo),
		Motivo:         motivo,
		Decidir: func(actual *repository.Expediente) (repository.Decision, error) {
			if !authorization.CanViewAll(role) && actual.AreaActual != authorization.AreaDelRol(role) {
				return repository.Decision{}, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
			}
			if actual.Estado != estadoEsperado {
				return repository.Decision{}, status.Errorf(codes.Aborted, "el expediente ya no está %s; actualice y vuelva a intentarlo", estadoEsperado)
			}
			if !estados.CanTransition(actual.Estado, nuevoEstado) {
				return repository.Decision{}, status.Error(codes.FailedPrecondition, "la transición de estado no está permitida")
			}
			return repository.Decision{AreaDestino: actual.AreaActual, EstadoNuevo: nuevoEstado}, nil
		},
	})
	if err != nil {
		return nil, errorMovimiento(l.ctx, "cambiar estado", err)
	}
	if !res.Repetido {
		l.svcCtx.Publicador.Avisar() // proyección inmediata (si no, el sondeo periódico)
	}
	return &expedientes.ChangeEstadoResponse{
		Expediente: toProto(res.Expediente),
		Movimiento: movimientoToProto(res.Movimiento),
		Repetido:   res.Repetido,
	}, nil
}
