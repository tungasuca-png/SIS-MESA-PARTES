package logic

import (
	"context"

	"auth/auth"
	"auth/internal/interceptor"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrantServiceClientPermissionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGrantServiceClientPermissionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GrantServiceClientPermissionLogic {
	return &GrantServiceClientPermissionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GrantServiceClientPermissionLogic) GrantServiceClientPermission(in *auth.ServiceClientPermissionRequest) (*auth.ServiceClientPermissionResponse, error) {
	adminID, ok := interceptor.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "se requiere autenticación")
	}
	return cambiarPermisoSistema(l.ctx, l.svcCtx.ServiceClientRepository, adminID, in.GetClientId(), in.GetPermissionCode(), true)
}
