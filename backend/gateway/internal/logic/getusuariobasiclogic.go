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

type GetUsuarioBasicLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUsuarioBasicLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUsuarioBasicLogic {
	return &GetUsuarioBasicLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUsuarioBasicLogic) GetUsuarioBasic(req *types.GetUsuarioBasicRequest) (resp *types.GetUsuarioBasicResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.UsuariosClient.GetUsuarioBasic(ctx, &usuariosclient.GetUsuarioBasicRequest{
		Id: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.GetUsuarioBasicResponse{Usuario: toUsuarioBasicDTO(found.Usuario)}, nil
}
