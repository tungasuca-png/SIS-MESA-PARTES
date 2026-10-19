// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"estudiantes/estudiantesclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchPersonasLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchPersonasLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchPersonasLogic {
	return &SearchPersonasLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SearchPersonas (etapa 5): busca personas por documento o nombre (estudiantes.familia.view). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *SearchPersonasLogic) SearchPersonas(req *types.SearchPersonasRequest) (resp *types.SearchPersonasResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.SearchPersonas(ctx, &estudiantesclient.SearchPersonasRequest{Q: req.Q, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, err
	}
	personas := make([]types.PersonaDTO, 0, len(out.Personas))
	for _, p := range out.Personas {
		personas = append(personas, toPersonaDTO(p))
	}
	return &types.SearchPersonasResponse{Personas: personas, Total: out.Total}, nil
}
