// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"auth/authclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRolePermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRolePermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRolePermissionsLogic {
	return &GetRolePermissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRolePermissionsLogic) GetRolePermissions(req *types.GetRolePermissionsRequest) (resp *types.GetRolePermissionsResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.AuthClient.GetRolePermissions(ctx, &authclient.GetRolePermissionsRequest{
		RoleId: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.GetRolePermissionsResponse{Permissions: toPermissionDTOs(found.Permissions)}, nil
}
