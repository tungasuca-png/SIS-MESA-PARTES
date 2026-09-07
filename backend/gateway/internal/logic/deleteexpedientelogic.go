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

type DeleteExpedienteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteExpedienteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteExpedienteLogic {
	return &DeleteExpedienteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteExpedienteLogic) DeleteExpediente(req *types.DeleteExpedienteRequest) (resp *types.DeleteExpedienteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	deleted, err := l.svcCtx.ExpedientesClient.DeleteExpediente(ctx, &expedientesclient.DeleteExpedienteRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.DeleteExpedienteResponse{Success: deleted.Success}, nil
}
