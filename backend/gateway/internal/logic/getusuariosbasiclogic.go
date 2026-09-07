// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUsuariosBasicLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUsuariosBasicLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsuariosBasicLogic {
	return &GetUsuariosBasicLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUsuariosBasicLogic) GetUsuariosBasic(req *types.GetUsuariosBasicRequest) (resp *types.GetUsuariosBasicResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.UsuariosClient.GetUsuariosBasic(ctx, &usuariosclient.GetUsuariosBasicRequest{
		Ids: req.Ids,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.UsuarioBasicDTO, 0, len(found.Usuarios))
	for _, item := range found.Usuarios {
		items = append(items, toUsuarioBasicDTO(item))
	}

	return &types.GetUsuariosBasicResponse{Usuarios: items}, nil
}
