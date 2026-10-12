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

type GetPsicologiaExpedienteByEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPsicologiaExpedienteByEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPsicologiaExpedienteByEstudianteLogic {
	return &GetPsicologiaExpedienteByEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPsicologiaExpedienteByEstudianteLogic) GetPsicologiaExpedienteByEstudiante(req *types.GetPsicologiaExpedienteByEstudianteRequest) (resp *types.PsicologiaExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.PsicologiaClient.GetExpedienteByEstudiante(ctx, &psicologiaclient.GetExpedienteByEstudianteRequest{EstudianteId: req.Id})
	if err != nil {
		return nil, err
	}

	return &types.PsicologiaExpedienteResponse{Expediente: toExpedientePsicologicoDTO(found.Expediente)}, nil
}
