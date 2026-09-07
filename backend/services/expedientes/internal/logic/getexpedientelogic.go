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

type GetExpedienteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetExpedienteLogic {
	return &GetExpedienteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetExpedienteLogic) GetExpediente(in *expedientes.GetExpedienteRequest) (*expedientes.GetExpedienteResponse, error) {
	userID, role, ok := interceptor.UserFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, status.Error(codes.InvalidArgument, "el identificador es obligatorio")
	}

	found, err := l.svcCtx.ExpedienteRepository.FindByIDOrCodigo(l.ctx, strings.TrimSpace(in.Id))
	if err != nil {
		if errors.Is(err, repository.ErrExpedienteNotFound) {
			return nil, status.Error(codes.NotFound, "expediente no encontrado")
		}
		return nil, status.Error(codes.Internal, "no se pudo consultar el expediente")
	}

	if !authorization.CanViewAll(role) && found.SolicitanteID != userID {
		return nil, status.Error(codes.PermissionDenied, "no tiene acceso a este expediente")
	}

	return &expedientes.GetExpedienteResponse{Expediente: toProto(found)}, nil
}
