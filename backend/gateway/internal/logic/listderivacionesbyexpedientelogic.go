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

type ListDerivacionesByExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDerivacionesByExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDerivacionesByExpedienteLogic {
	return &ListDerivacionesByExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListDerivacionesByExpedienteLogic) ListDerivacionesByExpediente(req *types.ListDerivacionesByExpedienteRequest) (resp *types.ListDerivacionesResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.DerivacionesClient.ListDerivaciones(ctx, &derivacionesclient.ListDerivacionesRequest{
		ExpedienteId: req.ExpedienteId,
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
