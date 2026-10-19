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

type ListMatriculasPeriodoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMatriculasPeriodoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMatriculasPeriodoLogic {
	return &ListMatriculasPeriodoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListMatriculasPeriodo (etapa 5): nómina paginada del periodo (estudiantes.view). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *ListMatriculasPeriodoLogic) ListMatriculasPeriodo(req *types.ListMatriculasPeriodoRequest) (resp *types.ListMatriculasPeriodoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ListMatriculasPeriodo(ctx, &estudiantesclient.ListMatriculasPeriodoRequest{
		PeriodoId: req.PeriodoId, SeccionId: req.SeccionId, EstadoMatricula: req.EstadoMatricula, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	matriculas := make([]types.MatriculaNominaDTO, 0, len(out.Matriculas))
	for _, m := range out.Matriculas {
		matriculas = append(matriculas, types.MatriculaNominaDTO{Matricula: toMatriculaDTO(m.GetMatricula()), Estudiante: toEstudianteBasicoPtr(m.GetEstudiante())})
	}
	return &types.ListMatriculasPeriodoResponse{Matriculas: matriculas, Total: out.Total}, nil
}
