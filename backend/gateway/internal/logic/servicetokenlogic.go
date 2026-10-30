// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"

	"auth/authclient"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServiceTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceTokenLogic {
	return &ServiceTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceTokenLogic) ServiceToken(req *types.ServiceTokenRequest) (resp *types.ServiceTokenResponse, err error) {
	// Solo el flujo client credentials; Auth valida las credenciales.
	if strings.TrimSpace(req.GrantType) != "client_credentials" {
		return nil, status.Error(codes.InvalidArgument, "grant_type debe ser client_credentials")
	}
	authResp, err := l.svcCtx.AuthClient.IssueServiceToken(l.ctx, &authclient.IssueServiceTokenRequest{
		ClientId:     req.ClientId,
		ClientSecret: req.ClientSecret,
	})
	if err != nil {
		return nil, err
	}
	return &types.ServiceTokenResponse{
		AccessToken: authResp.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   authResp.ExpiresIn,
		Scope:       authResp.Scope,
	}, nil
}
