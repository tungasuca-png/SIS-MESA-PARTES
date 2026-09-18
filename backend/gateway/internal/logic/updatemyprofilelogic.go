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

type UpdateMyProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMyProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMyProfileLogic {
	return &UpdateMyProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateMyProfile reenvía la solicitud tal cual: no recibe ni decide ningún
// identificador de usuario — Usuarios Service determina de quién es el
// perfil exclusivamente a partir del JWT reenviado por withAuthorization.
func (l *UpdateMyProfileLogic) UpdateMyProfile(req *types.UpdateMyProfileRequest) (resp *types.UpdateMyProfileResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	updated, err := l.svcCtx.UsuariosClient.UpdateMyProfile(ctx, &usuariosclient.UpdateMyProfileRequest{
		Telefono:  req.Telefono,
		Correo:    req.Correo,
		Direccion: req.Direccion,
	})
	if err != nil {
		return nil, err
	}

	return &types.UpdateMyProfileResponse{Usuario: toUsuarioDTO(updated.Usuario)}, nil
}
