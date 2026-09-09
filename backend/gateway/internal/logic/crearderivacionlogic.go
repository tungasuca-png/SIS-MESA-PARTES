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

type CrearDerivacionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCrearDerivacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CrearDerivacionLogic {
	return &CrearDerivacionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CrearDerivacionLogic) CrearDerivacion(req *types.CrearDerivacionRequest) (resp *types.CrearDerivacionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	created, err := l.svcCtx.DerivacionesClient.CrearDerivacion(ctx, &derivacionesclient.CrearDerivacionRequest{
		ExpedienteId: req.ExpedienteId,
		Tipo:         req.Tipo,
		Origen:       req.Origen,
		Destino:      req.Destino,
		Motivo:       req.Motivo,
		Condicion:    req.Condicion,
	})
	if err != nil {
		return nil, err
	}

	return &types.CrearDerivacionResponse{Derivacion: toDerivacionDTO(created.Derivacion)}, nil
}
