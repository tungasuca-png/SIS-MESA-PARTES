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

type CorregirExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCorregirExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CorregirExpedienteLogic {
	return &CorregirExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CorregirExpediente (Etapa 4, reglas sin cambios): el solicitante dueño
// corrige la descripción de un expediente OBSERVADO y lo reenvía (PENDIENTE,
// misma área). Desde A + B: movimiento CORRECCION en la misma transacción
// (descripción incluida en el mismo UPDATE) e idempotencia.
func (l *CorregirExpedienteLogic) CorregirExpediente(in *expedientes.CorregirExpedienteRequest) (*expedientes.CorregirExpedienteResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}
	descripcion := strings.TrimSpace(in.Descripcion)
	if descripcion == "" {
		return nil, status.Error(codes.InvalidArgument, "la descripción corregida es obligatoria")
	}
	if err := authorization.RequirePermission(l.ctx, l.svcCtx.AuthClient, userID, "expedientes.view_own"); err != nil {
		return nil, err
	}
	clave, err := validarClave(in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	// Regla vigente: corrige el DUEÑO (solicitante_id), sea solicitante externo
	// o personal interno que registró el expediente; se verifica abajo, sobre
	// la fila bloqueada.
	if err := validarRolActor(role); err != nil {
		return nil, err
	}

	res, err := l.svcCtx.ExpedienteRepository.RegistrarMovimiento(l.ctx, repository.SolicitudMovimiento{
		IDOrCodigo:     strings.TrimSpace(in.Id),
		Tipo:           repository.MovCorreccion,
		ActorID:        userID,
		ActorRol:       role,
		IdempotencyKey: clave,
		Contenido:      contenido(descripcion),
		Decidir: func(actual *repository.Expediente) (repository.Decision, error) {
			if actual.SolicitanteID != userID {
				return repository.Decision{}, status.Error(codes.PermissionDenied, "solo el solicitante dueño del expediente puede corregirlo")
			}
			if actual.Estado != estados.EstadoObservado {
				return repository.Decision{}, status.Error(codes.FailedPrecondition, "solo se puede corregir un expediente observado")
			}
			return repository.Decision{AreaDestino: actual.AreaActual, EstadoNuevo: estados.EstadoPendiente, NuevaDescripcion: &descripcion}, nil
		},
	})
	if err != nil {
		return nil, errorMovimiento(l.ctx, "corregir", err)
	}
	if !res.Repetido {
		l.svcCtx.Publicador.Avisar() // proyección inmediata (si no, el sondeo periódico)
	}
	return &expedientes.CorregirExpedienteResponse{
		Expediente: toProto(res.Expediente),
		Movimiento: movimientoToProto(res.Movimiento),
		Repetido:   res.Repetido,
	}, nil
}
