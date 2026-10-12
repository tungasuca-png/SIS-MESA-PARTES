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

type ListPsicologiaAtencionesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPsicologiaAtencionesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPsicologiaAtencionesLogic {
	return &ListPsicologiaAtencionesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPsicologiaAtencionesLogic) ListPsicologiaAtenciones(req *types.ListPsicologiaAtencionesRequest) (resp *types.ListPsicologiaAtencionesResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.PsicologiaClient.ListAtenciones(ctx, &psicologiaclient.ListAtencionesRequest{
		ExpedienteId: req.Id, Estado: req.Estado, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]types.AtencionResumenDTO, 0, len(out.Atenciones))
	for _, a := range out.Atenciones {
		items = append(items, toAtencionResumenDTO(a))
	}
	return &types.ListPsicologiaAtencionesResponse{Atenciones: items, Total: out.Total}, nil
}
