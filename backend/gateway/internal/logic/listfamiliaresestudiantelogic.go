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

type ListFamiliaresEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFamiliaresEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFamiliaresEstudianteLogic {
	return &ListFamiliaresEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListFamiliaresEstudiante (modelo maestro): permiso (estudiantes.familia.view)
// y auditoría de la lectura en Estudiantes Service.
func (l *ListFamiliaresEstudianteLogic) ListFamiliaresEstudiante(req *types.ListFamiliaresEstudianteRequest) (resp *types.ListFamiliaresEstudianteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ListFamiliaresEstudiante(ctx, &estudiantesclient.ListFamiliaresEstudianteRequest{
		EstudianteId:     req.Id,
		IncluirHistorial: req.Historial,
	})
	if err != nil {
		return nil, err
	}
	familiares := make([]types.FamiliarDTO, 0, len(out.Familiares))
	for _, f := range out.Familiares {
		familiares = append(familiares, toFamiliarDTO(f))
	}
	return &types.ListFamiliaresEstudianteResponse{Familiares: familiares}, nil
}
