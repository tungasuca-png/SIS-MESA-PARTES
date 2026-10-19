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

type CambiarSeccionMatriculaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCambiarSeccionMatriculaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CambiarSeccionMatriculaLogic {
	return &CambiarSeccionMatriculaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CambiarSeccionMatricula (etapa 5): asigna o cambia la sección (estudiantes.matricula.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *CambiarSeccionMatriculaLogic) CambiarSeccionMatricula(req *types.CambiarSeccionMatriculaRequest) (resp *types.MatriculaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.CambiarSeccionMatricula(ctx, &estudiantesclient.CambiarSeccionMatriculaRequest{
		MatriculaId: req.MatriculaId, ExpectedVersion: req.ExpectedVersion, SeccionId: req.SeccionId, Fecha: req.Fecha, Motivo: req.Motivo,
	})
	if err != nil {
		return nil, err
	}
	return toMatriculaResponse(out), nil
}
