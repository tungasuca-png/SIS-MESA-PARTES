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

type ClosePsicologiaExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewClosePsicologiaExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClosePsicologiaExpedienteLogic {
	return &ClosePsicologiaExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClosePsicologiaExpedienteLogic) ClosePsicologiaExpediente(req *types.ClosePsicologiaExpedienteRequest) (resp *types.PsicologiaExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.PsicologiaClient.CloseExpediente(ctx, &psicologiaclient.CloseExpedienteRequest{Id: req.Id, Motivo: req.Motivo})
	if err != nil {
		return nil, err
	}

	return &types.PsicologiaExpedienteResponse{Expediente: toExpedientePsicologicoDTO(found.Expediente)}, nil
}
