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

type SearchEstudiantesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSearchEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchEstudiantesLogic {
	return &SearchEstudiantesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SearchEstudiantesLogic) SearchEstudiantes(req *types.SearchEstudiantesRequest) (resp *types.SearchEstudiantesResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.EstudiantesClient.SearchEstudiantes(ctx, &estudiantesclient.SearchEstudiantesRequest{
		Q:        req.Q,
		Page:     req.Page,
		PageSize: req.PageSize,
		Estado:   req.Estado,
		Nivel:    req.Nivel,
		Grado:    req.Grado,
		Seccion:  req.Seccion,
		Turno:    req.Turno,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.EstudianteBasicoDTO, 0, len(found.Estudiantes))
	for _, e := range found.Estudiantes {
		items = append(items, toEstudianteBasicoDTO(e))
	}
	return &types.SearchEstudiantesResponse{Estudiantes: items, Total: found.Total}, nil
}
