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

type RefreshTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshTokenLogic {
	return &RefreshTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshTokenLogic) RefreshToken(req *types.RefreshTokenRequest) (resp *types.RefreshTokenResponse, err error) {
	authResp, err := l.svcCtx.AuthClient.RefreshToken(l.ctx, &authclient.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		return nil, err
	}

	return &types.RefreshTokenResponse{
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresIn:    authResp.ExpiresIn,
	}, nil
}
