package logic

import (
	"context"
	"errors"
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

func (l *ChangeEstadoLogic) ChangeEstado(in *expedientes.ChangeEstadoRequest) (*expedientes.ChangeEstadoResponse, error) {
	_, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanChangeEstado(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede cambiar el estado del expediente")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	nuevoEstado := strings.ToUpper(strings.TrimSpace(in.NuevoEstado))
	if !estados.IsValidEstado(nuevoEstado) {
		return nil, status.Error(codes.InvalidArgument, "el estado indicado no es válido")
	}

	current, err := l.svcCtx.ExpedienteRepository.FindByIDOrCodigo(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el expediente")
	}

	if !estados.CanTransition(current.Estado, nuevoEstado) {
		return nil, status.Error(codes.FailedPrecondition, "la transición de estado no está permitida")
	}

	updated, err := l.svcCtx.ExpedienteRepository.UpdateEstado(l.ctx, current.ID, current.Estado, nuevoEstado)
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.Aborted, "el estado del expediente cambió, intente nuevamente")
		}
		return nil, status.Error(codes.Internal, "no se pudo actualizar el estado")
	}

	return &expedientes.ChangeEstadoResponse{Expediente: toProto(updated)}, nil
}
