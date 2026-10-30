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

type MatricularEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMatricularEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MatricularEstudianteLogic {
	return &MatricularEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// MatricularEstudiante (etapa 5): matrícula del estudiante en un periodo (estudiantes.matricula.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *MatricularEstudianteLogic) MatricularEstudiante(req *types.MatricularEstudianteRequest) (resp *types.MatriculaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.MatricularEstudiante(ctx, &estudiantesclient.MatricularEstudianteRequest{
		EstudianteId: req.Id, PeriodoId: req.PeriodoId, SeccionId: req.SeccionId, FechaMatricula: req.FechaMatricula,
	})
	if err != nil {
		return nil, err
	}
	return toMatriculaResponse(out), nil
}
