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

type CreateExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateExpedienteLogic {
	return &CreateExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateExpedienteLogic) CreateExpediente(req *types.CreateExpedienteRequest) (resp *types.CreateExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	created, err := l.svcCtx.ExpedientesClient.CreateExpediente(ctx, &expedientesclient.CreateExpedienteRequest{
		Tipo:        req.Tipo,
		Asunto:      req.Asunto,
		Descripcion: req.Descripcion,
		Prioridad:   req.Prioridad,
	})
	if err != nil {
		return nil, err
	}

	return &types.CreateExpedienteResponse{Expediente: toExpedienteDTO(created.Expediente)}, nil
}
