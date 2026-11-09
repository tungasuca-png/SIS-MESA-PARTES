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

type ChangeEstadoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangeEstadoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEstadoLogic {
	return &ChangeEstadoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChangeEstadoLogic) ChangeEstado(req *types.ChangeEstadoRequest) (resp *types.ChangeEstadoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	changed, err := l.svcCtx.ExpedientesClient.ChangeEstado(ctx, &expedientesclient.ChangeEstadoRequest{
		Id:             req.Id,
		NuevoEstado:    req.NuevoEstado,
		EstadoEsperado: req.EstadoEsperado,
		IdempotencyKey: req.IdempotencyKey,
		Motivo:         req.Motivo,
	})
	if err != nil {
		return nil, errorMovimiento(err)
	}

	return &types.ChangeEstadoResponse{
		Expediente: toExpedienteDTO(changed.Expediente),
		Movimiento: toMovimientoDTO(changed.Movimiento),
		Repetido:   changed.Repetido,
	}, nil
}
