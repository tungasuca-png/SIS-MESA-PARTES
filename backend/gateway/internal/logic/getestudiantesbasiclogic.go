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

type GetEstudiantesBasicLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetEstudiantesBasicLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetEstudiantesBasicLogic {
	return &GetEstudiantesBasicLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetEstudiantesBasicLogic) GetEstudiantesBasic(req *types.GetEstudiantesBasicRequest) (resp *types.GetEstudiantesBasicResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)

	found, err := l.svcCtx.EstudiantesClient.GetEstudiantesBasic(ctx, &estudiantesclient.GetEstudiantesBasicRequest{Ids: req.Ids})
	if err != nil {
		return nil, err
	}

	items := make([]types.EstudianteBasicoDTO, 0, len(found.Estudiantes))
	for _, e := range found.Estudiantes {
		items = append(items, toEstudianteBasicoDTO(e))
	}
	return &types.GetEstudiantesBasicResponse{Estudiantes: items}, nil
}
