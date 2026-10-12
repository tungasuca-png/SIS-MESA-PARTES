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

type GetPsicologiaExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPsicologiaExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPsicologiaExpedienteLogic {
	return &GetPsicologiaExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPsicologiaExpedienteLogic) GetPsicologiaExpediente(req *types.GetPsicologiaExpedienteRequest) (resp *types.PsicologiaExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.PsicologiaClient.GetExpediente(ctx, &psicologiaclient.GetExpedienteRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}

	return &types.PsicologiaExpedienteResponse{Expediente: toExpedientePsicologicoDTO(found.Expediente)}, nil
}
