package logic

import (
	"context"

	"auth/authclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginRequest) (resp *types.LoginResponse, err error) {

	authResp, err := l.svcCtx.AuthClient.Login(
		l.ctx,
		&authclient.LoginRequest{
			Username: req.Username,
			Password: req.Password,
		},
	)

	if err != nil {
		return nil, err
	}

	return &types.LoginResponse{
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		ExpiresIn:    authResp.ExpiresIn,
		UserId:       authResp.UserId,
		Username:     authResp.Username,
		Email:        authResp.Email,
		Role:         authResp.Role,
	}, nil
}
