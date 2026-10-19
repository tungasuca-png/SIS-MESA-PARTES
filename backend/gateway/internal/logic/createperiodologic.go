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

type CreatePeriodoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePeriodoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePeriodoLogic {
	return &CreatePeriodoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreatePeriodo (etapa 5): registra un periodo PLANIFICADO (estudiantes.matricula.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *CreatePeriodoLogic) CreatePeriodo(req *types.CreatePeriodoRequest) (resp *types.PeriodoResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.CreatePeriodo(ctx, &estudiantesclient.CreatePeriodoRequest{
		Anio: req.Anio, Nombre: req.Nombre, FechaInicio: req.FechaInicio, FechaFin: req.FechaFin,
	})
	if err != nil {
		return nil, err
	}
	return &types.PeriodoResponse{Periodo: toPeriodoDTO(out.Periodo)}, nil
}
