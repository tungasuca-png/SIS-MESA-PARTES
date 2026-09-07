// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"auth/authclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/metadata"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(req *types.LogoutRequest) (resp *types.LogoutResponse, err error) {
	ctx := l.ctx
	if req.Authorization != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", req.Authorization)
	}

	authResp, err := l.svcCtx.AuthClient.Logout(ctx, &authclient.LogoutRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		return nil, err
	}

	return &types.LogoutResponse{
		Success: authResp.Success,
		Message: authResp.Message,
	}, nil
}
