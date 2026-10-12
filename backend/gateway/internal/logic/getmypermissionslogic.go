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

type GetMyPermissionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMyPermissionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyPermissionsLogic {
	return &GetMyPermissionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetMyPermissions reenvía el JWT tal cual y devuelve exactamente lo que
// Auth.GetMyPermissions responda — el Gateway no calcula ni interpreta
// permisos, y no tiene forma de indicar otro user_id (el request no lo
// tiene, ver GetMyPermissionsRequest).
func (l *GetMyPermissionsLogic) GetMyPermissions(req *types.GetMyPermissionsRequest) (resp *types.GetMyPermissionsResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	authResp, err := l.svcCtx.AuthClient.GetMyPermissions(ctx, &authclient.GetMyPermissionsRequest{})
	if err != nil {
		return nil, err
	}

	// gRPC no serializa un repeated field vacío, así que un usuario sin
	// roles vuelve con Permissions == nil; se normaliza a [] para que la
	// respuesta HTTP nunca confunda "sin permisos" con "null" (mismo
	// criterio ya aplicado en UpdateRolePermissions).
	permissions := authResp.Permissions
	if permissions == nil {
		permissions = []string{}
	}

	return &types.GetMyPermissionsResponse{Permissions: permissions}, nil
}
