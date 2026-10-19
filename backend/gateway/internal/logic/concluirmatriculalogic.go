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

type ConcluirMatriculaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConcluirMatriculaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConcluirMatriculaLogic {
	return &ConcluirMatriculaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ConcluirMatricula (etapa 5): conclusión del periodo (estudiantes.matricula.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *ConcluirMatriculaLogic) ConcluirMatricula(req *types.ConcluirMatriculaRequest) (resp *types.MatriculaResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ConcluirMatricula(ctx, &estudiantesclient.ConcluirMatriculaRequest{
		MatriculaId: req.MatriculaId, ExpectedVersion: req.ExpectedVersion, Fecha: req.Fecha,
	})
	if err != nil {
		return nil, err
	}
	return toMatriculaResponse(out), nil
}
