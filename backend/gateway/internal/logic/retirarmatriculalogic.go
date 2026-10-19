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

type RetirarMatriculaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetirarMatriculaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetirarMatriculaLogic {
	return &RetirarMatriculaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// RetirarMatricula (etapa 5): retiro o traslado de la matrícula (estudiantes.matricula.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *RetirarMatriculaLogic) RetirarMatricula(req *types.RetirarMatriculaRequest) (resp *types.MatriculaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.RetirarMatricula(ctx, &estudiantesclient.RetirarMatriculaRequest{
		MatriculaId: req.MatriculaId, ExpectedVersion: req.ExpectedVersion, Tipo: req.Tipo, Fecha: req.Fecha, Motivo: req.Motivo,
	})
	if err != nil {
		return nil, err
	}
	return toMatriculaResponse(out), nil
}
