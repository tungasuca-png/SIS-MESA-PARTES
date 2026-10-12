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

type GetPsicologiaAtencionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPsicologiaAtencionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPsicologiaAtencionLogic {
	return &GetPsicologiaAtencionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPsicologiaAtencionLogic) GetPsicologiaAtencion(req *types.GetPsicologiaAtencionRequest) (resp *types.PsicologiaAtencionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.GetAtencion(ctx, &psicologiaclient.GetAtencionRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return toAtencionResponse(out.Atencion), nil
}
