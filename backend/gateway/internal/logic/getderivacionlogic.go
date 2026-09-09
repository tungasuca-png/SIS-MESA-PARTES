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

type GetDerivacionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDerivacionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDerivacionLogic {
	return &GetDerivacionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetDerivacionLogic) GetDerivacion(req *types.GetDerivacionRequest) (resp *types.GetDerivacionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.DerivacionesClient.GetDerivacion(ctx, &derivacionesclient.GetDerivacionRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.GetDerivacionResponse{Derivacion: toDerivacionDTO(found.Derivacion)}, nil
}
