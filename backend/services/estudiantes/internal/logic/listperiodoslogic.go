package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPeriodosLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPeriodosLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPeriodosLogic {
	return &ListPeriodosLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListPeriodos (etapa 5): periodos académicos, más reciente primero
// ("estudiantes.view").
func (l *ListPeriodosLogic) ListPeriodos(_ *estudiantes.ListPeriodosRequest) (*estudiantes.ListPeriodosResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, permisoView); err != nil {
		return nil, err
	}
	periodos, err := l.svcCtx.EstudianteRepository.ListPeriodos(l.ctx)
	if err != nil {
		return nil, matriculaError(l.Logger, "listar periodos", err)
	}
	out := make([]*estudiantes.PeriodoAcademico, 0, len(periodos))
	for i := range periodos {
		out = append(out, periodoToProto(&periodos[i]))
	}
	return &estudiantes.ListPeriodosResponse{Periodos: out}, nil
}
