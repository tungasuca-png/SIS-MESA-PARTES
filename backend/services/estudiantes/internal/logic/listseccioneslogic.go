package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSeccionesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSeccionesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSeccionesLogic {
	return &ListSeccionesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListSecciones (etapa 5): secciones del periodo con su ocupación
// (matrículas MATRICULADO) — "estudiantes.view".
func (l *ListSeccionesLogic) ListSecciones(in *estudiantes.ListSeccionesRequest) (*estudiantes.ListSeccionesResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, permisoView); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ListSeccionesRequest{}
	}
	periodoID, err := validarID(in.PeriodoId)
	if err != nil {
		return nil, matriculaError(l.Logger, "listar secciones", err)
	}
	secciones, err := l.svcCtx.EstudianteRepository.ListSecciones(l.ctx, periodoID)
	if err != nil {
		return nil, matriculaError(l.Logger, "listar secciones", err)
	}
	out := make([]*estudiantes.SeccionAcademica, 0, len(secciones))
	for i := range secciones {
		out = append(out, seccionToProto(&secciones[i]))
	}
	return &estudiantes.ListSeccionesResponse{Secciones: out}, nil
}
