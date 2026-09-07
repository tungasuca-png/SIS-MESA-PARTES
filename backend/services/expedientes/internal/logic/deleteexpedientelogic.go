package logic

import (
	"context"
	"errors"
	"strings"

	"expedientes/expedientes"
	"expedientes/internal/authorization"
	"expedientes/internal/interceptor"
	"expedientes/internal/repository"
	"expedientes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteExpedienteLogic {
	return &DeleteExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteExpedienteLogic) DeleteExpediente(in *expedientes.DeleteExpedienteRequest) (*expedientes.DeleteExpedienteResponse, error) {
	_, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanDelete(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede eliminar expedientes")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	if err := l.svcCtx.ExpedienteRepository.Delete(l.ctx, strings.TrimSpace(in.Id)); err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo eliminar el expediente")
	}

	return &expedientes.DeleteExpedienteResponse{Success: true}, nil
}
