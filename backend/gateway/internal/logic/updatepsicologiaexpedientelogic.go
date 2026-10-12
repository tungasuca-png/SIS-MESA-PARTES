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

type UpdatePsicologiaExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePsicologiaExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePsicologiaExpedienteLogic {
	return &UpdatePsicologiaExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePsicologiaExpedienteLogic) UpdatePsicologiaExpediente(req *types.UpdatePsicologiaExpedienteRequest) (resp *types.PsicologiaExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.PsicologiaClient.UpdateExpediente(ctx, &psicologiaclient.UpdateExpedienteRequest{Id: req.Id, ResumenCaso: req.ResumenCaso, ResponsableId: req.ResponsableId, Estado: req.Estado})
	if err != nil {
		return nil, err
	}

	return &types.PsicologiaExpedienteResponse{Expediente: toExpedientePsicologicoDTO(found.Expediente)}, nil
}
