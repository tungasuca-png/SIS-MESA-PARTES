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

type GetExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetExpedienteLogic {
	return &GetExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetExpedienteLogic) GetExpediente(req *types.GetExpedienteRequest) (resp *types.GetExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.ExpedientesClient.GetExpediente(ctx, &expedientesclient.GetExpedienteRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.GetExpedienteResponse{Expediente: toExpedienteDTO(found.Expediente)}, nil
}
