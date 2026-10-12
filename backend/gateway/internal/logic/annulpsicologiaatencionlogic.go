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

type AnnulPsicologiaAtencionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAnnulPsicologiaAtencionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AnnulPsicologiaAtencionLogic {
	return &AnnulPsicologiaAtencionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AnnulPsicologiaAtencionLogic) AnnulPsicologiaAtencion(req *types.AnnulPsicologiaAtencionRequest) (resp *types.PsicologiaAtencionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.AnnulAtencion(ctx, &psicologiaclient.AnnulAtencionRequest{Id: req.Id, Motivo: req.Motivo})
	if err != nil {
		return nil, err
	}
	return toAtencionResponse(out.Atencion), nil
}
