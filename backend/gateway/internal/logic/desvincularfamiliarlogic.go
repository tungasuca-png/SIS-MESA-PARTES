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

type DesvincularFamiliarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDesvincularFamiliarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DesvincularFamiliarLogic {
	return &DesvincularFamiliarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DesvincularFamiliar (modelo maestro): cierra la relación sin borrarla;
// permiso (estudiantes.familia.update) en Estudiantes Service.
func (l *DesvincularFamiliarLogic) DesvincularFamiliar(req *types.DesvincularFamiliarRequest) (resp *types.FamiliarResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.DesvincularFamiliar(ctx, &estudiantesclient.DesvincularFamiliarRequest{
		EstudianteId: req.Id,
		RelacionId:   req.RelacionId,
		Hasta:        req.Hasta,
	})
	if err != nil {
		return nil, err
	}
	return &types.FamiliarResponse{Familiar: toFamiliarDTO(out.Familiar)}, nil
}
