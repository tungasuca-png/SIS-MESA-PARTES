// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"psicologia/psicologiaclient"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BuscarPsicologiaEstudiantesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBuscarPsicologiaEstudiantesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuscarPsicologiaEstudiantesLogic {
	return &BuscarPsicologiaEstudiantesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BuscarPsicologiaEstudiantesLogic) BuscarPsicologiaEstudiantes(req *types.BuscarPsicologiaEstudiantesRequest) (resp *types.BuscarPsicologiaEstudiantesResponse, err error) {
	// Transporte puro: la autorización (permisos psicologia.* del usuario) y
	// la consulta a Estudiantes con la credencial de servicio las resuelve
	// Psicología. La consulta viaja en el cuerpo, nunca en la URL.
	ctx := withAuthorization(l.ctx, req.Authorization)
	found, err := l.svcCtx.PsicologiaClient.BuscarEstudiantes(ctx, &psicologiaclient.BuscarEstudiantesRequest{
		Q: req.Q, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	out := make([]types.EstudianteBusquedaDTO, 0, len(found.Estudiantes))
	for _, e := range found.Estudiantes {
		out = append(out, types.EstudianteBusquedaDTO{Id: e.Id, NombreCompleto: e.NombreCompleto, Grado: e.Grado, Seccion: e.Seccion})
	}
	return &types.BuscarPsicologiaEstudiantesResponse{Estudiantes: out, Total: found.Total}, nil
}
