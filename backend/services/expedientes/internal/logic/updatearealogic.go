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

type UpdateAreaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAreaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAreaLogic {
	return &UpdateAreaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAreaLogic) UpdateArea(in *expedientes.UpdateAreaRequest) (*expedientes.UpdateAreaResponse, error) {
	_, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if !authorization.CanUpdateArea(role) {
		return nil, status.Error(codes.PermissionDenied, "el rol no puede cambiar el área del expediente")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	area := strings.ToUpper(strings.TrimSpace(in.Area))
	if !estados.IsValidArea(area) {
		return nil, status.Error(codes.InvalidArgument, "el área indicada no es válida")
	}

	updated, err := l.svcCtx.ExpedienteRepository.UpdateArea(l.ctx, strings.TrimSpace(in.Id), area)
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo actualizar el área del expediente")
	}

	return &expedientes.UpdateAreaResponse{Expediente: toProto(updated)}, nil
}
