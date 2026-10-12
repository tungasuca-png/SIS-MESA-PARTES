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

type ResolveEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResolveEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveEstudianteLogic {
	return &ResolveEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ResolveEstudiante (modelo maestro): transporte puro; permiso
// (estudiantes.view) y validación en Estudiantes Service.
func (l *ResolveEstudianteLogic) ResolveEstudiante(req *types.ResolveEstudianteRequest) (resp *types.ResolveEstudianteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ResolveEstudiante(ctx, &estudiantesclient.ResolveEstudianteRequest{
		Codigo:          req.Codigo,
		TipoDocumento:   req.TipoDocumento,
		NumeroDocumento: req.NumeroDocumento,
	})
	if err != nil {
		return nil, err
	}
	return &types.ResolveEstudianteResponse{Estudiante: toEstudianteBasicoDTO(out.Estudiante)}, nil
}
