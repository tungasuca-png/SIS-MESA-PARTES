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

type ListPsicologiaExpedientesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPsicologiaExpedientesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPsicologiaExpedientesLogic {
	return &ListPsicologiaExpedientesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPsicologiaExpedientesLogic) ListPsicologiaExpedientes(req *types.ListPsicologiaExpedientesRequest) (resp *types.ListPsicologiaExpedientesResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.PsicologiaClient.ListExpedientes(ctx, &psicologiaclient.ListExpedientesRequest{Estado: req.Estado, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}

	items := make([]types.ExpedientePsicologicoDTO, 0, len(found.Expedientes))
	for _, item := range found.Expedientes {
		items = append(items, toExpedientePsicologicoDTO(item))
	}
	return &types.ListPsicologiaExpedientesResponse{Expedientes: items, Total: found.Total}, nil
}
