// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"expedientes/expedientesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateExpedienteLogic {
	return &UpdateExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateExpedienteLogic) UpdateExpediente(req *types.UpdateExpedienteRequest) (resp *types.UpdateExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	updated, err := l.svcCtx.ExpedientesClient.UpdateExpediente(ctx, &expedientesclient.UpdateExpedienteRequest{
		Id:          req.Id,
		Asunto:      req.Asunto,
		Descripcion: req.Descripcion,
		Prioridad:   req.Prioridad,
	})
	if err != nil {
		return nil, err
	}

	return &types.UpdateExpedienteResponse{Expediente: toExpedienteDTO(updated.Expediente)}, nil
}
