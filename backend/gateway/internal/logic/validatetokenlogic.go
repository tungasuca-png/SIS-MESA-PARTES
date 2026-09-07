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
)

type ValidateTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewValidateTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateTokenLogic {
	return &ValidateTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ValidateTokenLogic) ValidateToken(req *types.ValidateTokenRequest) (resp *types.ValidateTokenResponse, err error) {
	authResp, err := l.svcCtx.AuthClient.ValidateToken(l.ctx, &authclient.ValidateTokenRequest{
		AccessToken: bearerToken(req.Authorization),
	})
	if err != nil {
		return nil, err
	}

	return &types.ValidateTokenResponse{
		Valid:    authResp.Valid,
		UserId:   authResp.UserId,
		Username: authResp.Username,
		Role:     authResp.Role,
	}, nil
}

func bearerToken(authorization string) string {
	parts := strings.Fields(authorization)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}
	return authorization
}
