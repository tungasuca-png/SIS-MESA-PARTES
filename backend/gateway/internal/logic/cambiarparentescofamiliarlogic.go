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

type CambiarParentescoFamiliarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCambiarParentescoFamiliarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CambiarParentescoFamiliarLogic {
	return &CambiarParentescoFamiliarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CambiarParentescoFamiliar (etapa 6): corrige el parentesco conservando el historial (estudiantes.familia.update). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *CambiarParentescoFamiliarLogic) CambiarParentescoFamiliar(req *types.CambiarParentescoFamiliarRequest) (resp *types.CambiarParentescoFamiliarResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.CambiarParentescoFamiliar(ctx, &estudiantesclient.CambiarParentescoFamiliarRequest{
		EstudianteId: req.Id, RelacionId: req.RelacionId, Parentesco: req.Parentesco, Desde: req.Desde,
	})
	if err != nil {
		return nil, err
	}
	return &types.CambiarParentescoFamiliarResponse{Anterior: toFamiliarDTO(out.Anterior), Familiar: toFamiliarDTO(out.Familiar)}, nil
}
