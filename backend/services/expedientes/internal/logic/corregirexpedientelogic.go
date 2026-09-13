package logic

import (
	"context"
	"errors"
	"strings"

	"expedientes/expedientes"
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

// CorregirExpediente (Etapa 4): exclusivo del solicitante dueño del propio
// expediente — a diferencia de las demás acciones de esta etapa, no se
// basa en área (el solicitante no tiene un área interna), sino en
// solicitante_id == quien llama. Mismo expediente, mismo código: no se
// crea uno nuevo, se actualiza la descripción (la corrección) y se
// reingresa a la revisión de Secretaría (OBSERVADO -> PENDIENTE).
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

	current, err := l.svcCtx.ExpedienteRepository.FindByIDOrCodigo(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el expediente")
	}

	if role != "SOLICITANTE" || current.SolicitanteID != userID {
		return nil, status.Error(codes.PermissionDenied, "solo el solicitante dueño del expediente puede corregirlo")
	}
	if current.Estado != estados.EstadoObservado {
		return nil, status.Error(codes.FailedPrecondition, "solo se puede corregir un expediente observado")
	}

	updated, err := l.svcCtx.ExpedienteRepository.CorregirYReenviar(l.ctx, strings.TrimSpace(in.Id), descripcion)
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.Aborted, "el expediente cambió, intente nuevamente")
		}
		return nil, status.Error(codes.Internal, "no se pudo corregir el expediente")
	}

	return &expedientes.CorregirExpedienteResponse{Expediente: toProto(updated)}, nil
}
