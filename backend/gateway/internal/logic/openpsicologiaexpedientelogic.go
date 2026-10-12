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

type OpenPsicologiaExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenPsicologiaExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenPsicologiaExpedienteLogic {
	return &OpenPsicologiaExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenPsicologiaExpedienteLogic) OpenPsicologiaExpediente(req *types.OpenPsicologiaExpedienteRequest) (resp *types.PsicologiaExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.PsicologiaClient.OpenExpediente(ctx, &psicologiaclient.OpenExpedienteRequest{EstudianteId: req.Id, ResponsableId: req.ResponsableId, ResumenCaso: req.ResumenCaso})
	if err != nil {
		return nil, err
	}

	return &types.PsicologiaExpedienteResponse{Expediente: toExpedientePsicologicoDTO(found.Expediente)}, nil
}
