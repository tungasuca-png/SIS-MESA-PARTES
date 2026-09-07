// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strings"

	"gateway/internal/svc"
	"gateway/internal/types"
	"usuarios/usuariosclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListUsuariosLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListUsuariosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsuariosLogic {
	return &ListUsuariosLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListUsuarios usa una sola ruta para listar y buscar: con "q" delega en
// SearchUsuarios y sin "q" en ListUsuarios, evitando una ruta extra que
// colisione con /api/usuarios/:id.
func (l *ListUsuariosLogic) ListUsuarios(req *types.ListUsuariosRequest) (resp *types.ListUsuariosResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	if strings.TrimSpace(req.Q) != "" {
		found, err := l.svcCtx.UsuariosClient.SearchUsuarios(ctx, &usuariosclient.SearchUsuariosRequest{
			Query:       req.Q,
			TipoUsuario: req.TipoUsuario,
			Page:        req.Page,
			PageSize:    req.PageSize,
		})
		if err != nil {
			return nil, err
		}
		return &types.ListUsuariosResponse{Usuarios: toUsuarioDTOs(found.Usuarios), Total: found.Total}, nil
	}

	found, err := l.svcCtx.UsuariosClient.ListUsuarios(ctx, &usuariosclient.ListUsuariosRequest{
		TipoUsuario: req.TipoUsuario,
		Page:        req.Page,
		PageSize:    req.PageSize,
	})
	if err != nil {
		return nil, err
	}

	return &types.ListUsuariosResponse{Usuarios: toUsuarioDTOs(found.Usuarios), Total: found.Total}, nil
}

func toUsuarioDTOs(items []*usuariosclient.Usuario) []types.UsuarioDTO {
	result := make([]types.UsuarioDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toUsuarioDTO(item))
	}
	return result
}
