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

type ListPeriodosLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPeriodosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPeriodosLogic {
	return &ListPeriodosLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListPeriodos (etapa 5): periodos académicos (estudiantes.view). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *ListPeriodosLogic) ListPeriodos(req *types.ListPeriodosRequest) (resp *types.ListPeriodosResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ListPeriodos(ctx, &estudiantesclient.ListPeriodosRequest{})
	if err != nil {
		return nil, err
	}
	periodos := make([]types.PeriodoAcademicoDTO, 0, len(out.Periodos))
	for _, p := range out.Periodos {
		periodos = append(periodos, toPeriodoDTO(p))
	}
	return &types.ListPeriodosResponse{Periodos: periodos}, nil
}
