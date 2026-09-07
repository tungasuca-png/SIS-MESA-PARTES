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

type GetUsuarioLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUsuarioLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsuarioLogic {
	return &GetUsuarioLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUsuarioLogic) GetUsuario(req *types.GetUsuarioRequest) (resp *types.GetUsuarioResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.UsuariosClient.GetUsuario(ctx, &usuariosclient.GetUsuarioRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.GetUsuarioResponse{Usuario: toUsuarioDTO(found.Usuario)}, nil
}
