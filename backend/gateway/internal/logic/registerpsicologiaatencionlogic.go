// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"psicologia/psicologiaclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterPsicologiaAtencionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterPsicologiaAtencionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterPsicologiaAtencionLogic {
	return &RegisterPsicologiaAtencionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterPsicologiaAtencionLogic) RegisterPsicologiaAtencion(req *types.RevisionPsicologiaAtencionRequest) (resp *types.PsicologiaAtencionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.RegisterAtencion(ctx, &psicologiaclient.RegisterAtencionRequest{Id: req.Id, Revision: req.Revision})
	if err != nil {
		return nil, err
	}
	return toAtencionResponse(out.Atencion), nil
}
