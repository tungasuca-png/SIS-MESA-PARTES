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

type UpdateAreaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAreaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAreaLogic {
	return &UpdateAreaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAreaLogic) UpdateArea(req *types.UpdateAreaRequest) (resp *types.UpdateAreaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	updated, err := l.svcCtx.ExpedientesClient.UpdateArea(ctx, &expedientesclient.UpdateAreaRequest{
		Id:   req.Id,
		Area: req.Area,
	})
	if err != nil {
		return nil, err
	}

	return &types.UpdateAreaResponse{Expediente: toExpedienteDTO(updated.Expediente)}, nil
}
