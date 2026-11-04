// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"

	"estudiantes/estudiantesclient"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BuscarEstudiantesSeleccionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBuscarEstudiantesSeleccionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuscarEstudiantesSeleccionLogic {
	return &BuscarEstudiantesSeleccionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BuscarEstudiantesSeleccionLogic) BuscarEstudiantesSeleccion(req *types.BuscarEstudiantesSeleccionRequest) (resp *types.BuscarEstudiantesSeleccionResponse, err error) {
	// Transporte puro: Estudiantes valida el token de servicio (firma, iss,
	// aud, exp, tipo), exige el scope estudiantes.search.basic y rechaza un
	// JWT humano. La consulta viaja en el cuerpo y no se registra.
	ctx := withAuthorization(l.ctx, req.Authorization)
	found, err := l.svcCtx.EstudiantesClient.BuscarEstudiantesSeleccion(ctx, &estudiantesclient.BuscarEstudiantesSeleccionRequest{
		Q: req.Q, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	// Solo los cuatro campos mínimos, aunque el servicio enviara más.
	out := make([]types.EstudianteBusquedaDTO, 0, len(found.Estudiantes))
	for _, e := range found.Estudiantes {
		out = append(out, types.EstudianteBusquedaDTO{Id: e.Id, NombreCompleto: e.NombreCompleto, Grado: e.Grado, Seccion: e.Seccion})
	}
	return &types.BuscarEstudiantesSeleccionResponse{Estudiantes: out, Total: found.Total}, nil
}
