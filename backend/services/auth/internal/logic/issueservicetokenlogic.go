package logic

import (
	"context"

	"auth/auth"
	"auth/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type IssueServiceTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewIssueServiceTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IssueServiceTokenLogic {
	return &IssueServiceTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *IssueServiceTokenLogic) IssueServiceToken(in *auth.IssueServiceTokenRequest) (*auth.IssueServiceTokenResponse, error) {
	return emitirTokenServicio(l.ctx, l.svcCtx.ServiceClientRepository, l.svcCtx.ServiceTokenIssuer, in.GetClientId(), in.GetClientSecret())
}
