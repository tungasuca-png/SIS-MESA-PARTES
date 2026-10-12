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

type ListPosiblesDuplicadosLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPosiblesDuplicadosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPosiblesDuplicadosLogic {
	return &ListPosiblesDuplicadosLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListPosiblesDuplicados (modelo maestro): reporte para revisión humana;
// permiso (estudiantes.update) en Estudiantes Service.
func (l *ListPosiblesDuplicadosLogic) ListPosiblesDuplicados(req *types.ListPosiblesDuplicadosRequest) (resp *types.ListPosiblesDuplicadosResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ListPosiblesDuplicados(ctx, &estudiantesclient.ListPosiblesDuplicadosRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	grupos := make([]types.GrupoPosibleDuplicadoDTO, 0, len(out.Grupos))
	for _, g := range out.Grupos {
		items := make([]types.EstudianteBasicoDTO, 0, len(g.Estudiantes))
		for _, e := range g.Estudiantes {
			items = append(items, toEstudianteBasicoDTO(e))
		}
		grupos = append(grupos, types.GrupoPosibleDuplicadoDTO{Criterio: g.Criterio, Estudiantes: items})
	}
	return &types.ListPosiblesDuplicadosResponse{Grupos: grupos, Total: out.Total}, nil
}
