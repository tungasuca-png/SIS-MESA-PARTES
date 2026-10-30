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

type ListMatriculasEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMatriculasEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMatriculasEstudianteLogic {
	return &ListMatriculasEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListMatriculasEstudiante (modelo maestro): historial académico; permiso
// (estudiantes.view) en Estudiantes Service.
func (l *ListMatriculasEstudianteLogic) ListMatriculasEstudiante(req *types.ListMatriculasEstudianteRequest) (resp *types.ListMatriculasEstudianteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ListMatriculasEstudiante(ctx, &estudiantesclient.ListMatriculasEstudianteRequest{EstudianteId: req.Id})
	if err != nil {
		return nil, err
	}
	matriculas := make([]types.MatriculaDTO, 0, len(out.Matriculas))
	for _, m := range out.Matriculas {
		matriculas = append(matriculas, toMatriculaDTO(m))
	}
	return &types.ListMatriculasEstudianteResponse{Matriculas: matriculas}, nil
}
