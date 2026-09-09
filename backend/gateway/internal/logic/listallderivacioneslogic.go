// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"derivaciones/derivacionesclient"
	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAllDerivacionesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAllDerivacionesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAllDerivacionesLogic {
	return &ListAllDerivacionesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAllDerivacionesLogic) ListAllDerivaciones(req *types.ListAllDerivacionesRequest) (resp *types.ListDerivacionesResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.DerivacionesClient.ListDerivaciones(ctx, &derivacionesclient.ListDerivacionesRequest{
		ExpedienteId: req.ExpedienteId,
		Tipo:         req.Tipo,
		Page:         req.Page,
		PageSize:     req.PageSize,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.DerivacionDTO, 0, len(found.Derivaciones))
	for _, item := range found.Derivaciones {
		items = append(items, toDerivacionDTO(item))
	}

	return &types.ListDerivacionesResponse{Derivaciones: items, Total: found.Total}, nil
}
