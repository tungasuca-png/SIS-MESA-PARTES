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

type ReingresarMatriculaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReingresarMatriculaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReingresarMatriculaLogic {
	return &ReingresarMatriculaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReingresarMatricula (etapa 5): reingreso de una matrícula retirada o trasladada (estudiantes.matricula.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *ReingresarMatriculaLogic) ReingresarMatricula(req *types.ReingresarMatriculaRequest) (resp *types.MatriculaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ReingresarMatricula(ctx, &estudiantesclient.ReingresarMatriculaRequest{
		MatriculaId: req.MatriculaId, ExpectedVersion: req.ExpectedVersion, Fecha: req.Fecha, Motivo: req.Motivo,
	})
	if err != nil {
		return nil, err
	}
	return toMatriculaResponse(out), nil
}
