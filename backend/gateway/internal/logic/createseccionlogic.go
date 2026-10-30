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

type CreateSeccionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateSeccionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSeccionLogic {
	return &CreateSeccionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateSeccion (etapa 5): registra una sección del periodo (estudiantes.matricula.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *CreateSeccionLogic) CreateSeccion(req *types.CreateSeccionRequest) (resp *types.SeccionResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.CreateSeccion(ctx, &estudiantesclient.CreateSeccionRequest{
		PeriodoId: req.PeriodoId, Nivel: req.Nivel, Grado: req.Grado, Seccion: req.Seccion, Turno: req.Turno, Aula: req.Aula, Vacantes: req.Vacantes,
	})
	if err != nil {
		return nil, err
	}
	return &types.SeccionResponse{Seccion: toSeccionDTO(out.Seccion)}, nil
}
