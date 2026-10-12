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

type ChangeEstadoEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangeEstadoEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEstadoEstudianteLogic {
	return &ChangeEstadoEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ChangeEstadoEstudiante (FASE E-05): ACTIVO/INACTIVO del registro.
func (l *ChangeEstadoEstudianteLogic) ChangeEstadoEstudiante(req *types.ChangeEstadoEstudianteRequest) (resp *types.EstudianteWriteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	out, err := l.svcCtx.EstudiantesClient.ChangeEstadoEstudiante(ctx, &estudiantesclient.ChangeEstadoEstudianteRequest{
		Id:              req.Id,
		ExpectedVersion: req.ExpectedVersion,
		Estado:          req.Estado,
	})
	if err != nil {
		return nil, err
	}
	return &types.EstudianteWriteResponse{Estudiante: toEstudianteBasicoDTO(out.Estudiante)}, nil
}
