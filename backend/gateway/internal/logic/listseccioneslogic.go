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

type ListSeccionesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSeccionesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSeccionesLogic {
	return &ListSeccionesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListSecciones (etapa 5): secciones del periodo con su ocupación (estudiantes.view). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *ListSeccionesLogic) ListSecciones(req *types.ListSeccionesRequest) (resp *types.ListSeccionesResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ListSecciones(ctx, &estudiantesclient.ListSeccionesRequest{PeriodoId: req.PeriodoId})
	if err != nil {
		return nil, err
	}
	secciones := make([]types.SeccionAcademicaDTO, 0, len(out.Secciones))
	for _, s := range out.Secciones {
		secciones = append(secciones, toSeccionDTO(s))
	}
	return &types.ListSeccionesResponse{Secciones: secciones}, nil
}
