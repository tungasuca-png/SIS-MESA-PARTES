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

type ReopenPsicologiaExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReopenPsicologiaExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReopenPsicologiaExpedienteLogic {
	return &ReopenPsicologiaExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReopenPsicologiaExpedienteLogic) ReopenPsicologiaExpediente(req *types.ReopenPsicologiaExpedienteRequest) (resp *types.PsicologiaExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.PsicologiaClient.ReopenExpediente(ctx, &psicologiaclient.ReopenExpedienteRequest{Id: req.Id, Estado: req.Estado})
	if err != nil {
		return nil, err
	}

	return &types.PsicologiaExpedienteResponse{Expediente: toExpedientePsicologicoDTO(found.Expediente)}, nil
}
