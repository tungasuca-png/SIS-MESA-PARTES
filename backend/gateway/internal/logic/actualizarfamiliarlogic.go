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

type ActualizarFamiliarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewActualizarFamiliarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ActualizarFamiliarLogic {
	return &ActualizarFamiliarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ActualizarFamiliar (etapa 5): cambia solo el rol de apoderado (estudiantes.familia.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *ActualizarFamiliarLogic) ActualizarFamiliar(req *types.ActualizarFamiliarRequest) (resp *types.FamiliarResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ActualizarFamiliar(ctx, &estudiantesclient.ActualizarFamiliarRequest{
		EstudianteId: req.Id, RelacionId: req.RelacionId, EsApoderado: req.EsApoderado,
	})
	if err != nil {
		return nil, err
	}
	return &types.FamiliarResponse{Familiar: toFamiliarDTO(out.Familiar)}, nil
}
