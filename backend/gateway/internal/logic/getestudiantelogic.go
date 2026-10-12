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

type GetEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEstudianteLogic {
	return &GetEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetEstudianteLogic) GetEstudiante(req *types.GetEstudianteRequest) (resp *types.GetEstudianteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.EstudiantesClient.GetEstudiante(ctx, &estudiantesclient.GetEstudianteRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}

	return &types.GetEstudianteResponse{Estudiante: toEstudianteDTO(found.Estudiante)}, nil
}
