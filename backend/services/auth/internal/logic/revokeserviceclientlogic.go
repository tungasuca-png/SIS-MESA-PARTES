package logic

import (
	"context"
	"errors"
	"strings"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RevokeServiceClientLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevokeServiceClientLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeServiceClientLogic {
	return &RevokeServiceClientLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RevokeServiceClientLogic) RevokeServiceClient(in *auth.RevokeServiceClientRequest) (*auth.RevokeServiceClientResponse, error) {
	adminID, _ := interceptor.UserIDFromContext(l.ctx)
	id := strings.TrimSpace(in.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "el id o client_id es obligatorio")
	}
	revoked, err := l.svcCtx.ServiceClientRepository.Revoke(l.ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrServiceClientNotFound) {
			return nil, status.Error(codes.NotFound, "sistema consumidor no encontrado")
		}
		l.Errorf("revocar sistema consumidor: %v", err)
		return nil, status.Error(codes.Internal, "no se pudo revocar el sistema")
	}
	l.Infof("sistema consumidor revocado: id=%s client_id=%s por=%s", revoked.ID, revoked.ClientID, adminID)
	return &auth.RevokeServiceClientResponse{Client: serviceClientProto(revoked)}, nil
}
