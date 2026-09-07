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

type ListExpedientesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListExpedientesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListExpedientesLogic {
	return &ListExpedientesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListExpedientesLogic) ListExpedientes(req *types.ListExpedientesRequest) (resp *types.ListExpedientesResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	listed, err := l.svcCtx.ExpedientesClient.ListExpedientes(ctx, &expedientesclient.ListExpedientesRequest{
		Estado:    req.Estado,
		Prioridad: req.Prioridad,
		Tipo:      req.Tipo,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.ExpedienteDTO, 0, len(listed.Expedientes))
	for _, item := range listed.Expedientes {
		items = append(items, toExpedienteDTO(item))
	}

	return &types.ListExpedientesResponse{Expedientes: items, Total: listed.Total}, nil
}
