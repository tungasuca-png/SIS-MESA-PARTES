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

type UpdateRolePermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateRolePermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRolePermissionsLogic {
	return &UpdateRolePermissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateRolePermissionsLogic) UpdateRolePermissions(req *types.UpdateRolePermissionsRequest) (resp *types.UpdateRolePermissionsResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	updated, err := l.svcCtx.AuthClient.UpdateRolePermissions(ctx, &authclient.UpdateRolePermissionsRequest{
		RoleId:        req.Id,
		PermissionIds: req.PermissionIds,
	})
	if err != nil {
		return nil, err
	}

	// gRPC no serializa un repeated field vacío, así que un rol sin permisos
	// vuelve de authclient con PermissionIds == nil; se normaliza a []
	// para que la respuesta HTTP nunca convierta "[]" en "null".
	permissionIDs := updated.PermissionIds
	if permissionIDs == nil {
		permissionIDs = []string{}
	}

	return &types.UpdateRolePermissionsResponse{
		RoleId:        updated.RoleId,
		PermissionIds: permissionIDs,
	}, nil
}
