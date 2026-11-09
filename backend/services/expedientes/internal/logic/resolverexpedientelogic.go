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

type ResolverExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolverExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolverExpedienteLogic {
	return &ResolverExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolverExpediente (Etapa 4, reglas sin cambios): exclusivo de F2, solo
// Secretaría con el expediente de vuelta y EN_PROCESO; cierra a ATENDIDO.
// Desde A + B: movimiento RESOLUCION en la misma transacción e idempotencia.
func (l *ResolverExpedienteLogic) ResolverExpediente(in *expedientes.ResolverExpedienteRequest) (*expedientes.ResolverExpedienteResponse, error) {
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
	clave, err := validarClave(in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if err := validarRolActor(role); err != nil {
		return nil, err
	}

	res, err := l.svcCtx.ExpedienteRepository.RegistrarMovimiento(l.ctx, repository.SolicitudMovimiento{
		IDOrCodigo:     strings.TrimSpace(in.Id),
		Tipo:           repository.MovResolucion,
		ActorID:        userID,
		ActorRol:       role,
		IdempotencyKey: clave,
		Contenido:      contenido(),
		Decidir: func(actual *repository.Expediente) (repository.Decision, error) {
			if !authorization.CanViewAll(role) && actual.AreaActual != authorization.AreaDelRol(role) {
				return repository.Decision{}, status.Error(codes.PermissionDenied, "el expediente no pertenece al área del usuario")
			}
			if !estados.EsF2(actual.Tipo) {
				return repository.Decision{}, status.Error(codes.FailedPrecondition, "resolver no aplica a este tipo de trámite")
			}
			if actual.AreaActual != estados.AreaSecretaria {
				return repository.Decision{}, status.Error(codes.FailedPrecondition, "solo Secretaría puede cerrar el expediente, y solo cuando lo tiene de vuelta")
			}
			if actual.Estado != estados.EstadoEnProceso {
				return repository.Decision{}, status.Error(codes.FailedPrecondition, "solo se puede resolver un expediente en proceso")
			}
			return repository.Decision{AreaDestino: actual.AreaActual, EstadoNuevo: estados.EstadoAtendido}, nil
		},
	})
	if err != nil {
		return nil, errorMovimiento(l.ctx, "resolver", err)
	}
	if !res.Repetido {
		l.svcCtx.Publicador.Avisar() // proyección inmediata (si no, el sondeo periódico)
	}
	return &expedientes.ResolverExpedienteResponse{
		Expediente: toProto(res.Expediente),
		Movimiento: movimientoToProto(res.Movimiento),
		Repetido:   res.Repetido,
	}, nil
}
