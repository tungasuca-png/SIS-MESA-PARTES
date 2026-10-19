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

type ChangeEstadoPeriodoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangeEstadoPeriodoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeEstadoPeriodoLogic {
	return &ChangeEstadoPeriodoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ChangeEstadoPeriodo (etapa 5): avanza el estado del periodo (estudiantes.matricula.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *ChangeEstadoPeriodoLogic) ChangeEstadoPeriodo(req *types.ChangeEstadoPeriodoRequest) (resp *types.PeriodoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ChangeEstadoPeriodo(ctx, &estudiantesclient.ChangeEstadoPeriodoRequest{Id: req.PeriodoId, Estado: req.Estado})
	if err != nil {
		return nil, err
	}
	return &types.PeriodoResponse{Periodo: toPeriodoDTO(out.Periodo)}, nil
}
