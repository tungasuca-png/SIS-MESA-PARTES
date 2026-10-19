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

type ListHistorialEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListHistorialEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHistorialEstudianteLogic {
	return &ListHistorialEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListHistorialEstudiante (etapa 6): eventos de cambio del estudiante, sin valores (estudiantes.view). Permiso, validación y
// auditoría en Estudiantes Service.
func (l *ListHistorialEstudianteLogic) ListHistorialEstudiante(req *types.ListHistorialEstudianteRequest) (resp *types.ListHistorialEstudianteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ListHistorialEstudiante(ctx, &estudiantesclient.ListHistorialEstudianteRequest{
		EstudianteId: req.Id, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	eventos := make([]types.EventoHistorialDTO, 0, len(out.Eventos))
	for _, e := range out.Eventos {
		campos := e.GetCampos()
		if campos == nil {
			campos = []string{}
		}
		eventos = append(eventos, types.EventoHistorialDTO{
			Id: e.GetId(), Accion: e.GetAccion(), FechaHora: e.GetFechaHora(), UsuarioId: e.GetUsuarioId(), Campos: campos,
			EstadoAnterior: e.GetEstadoAnterior(), EstadoNuevo: e.GetEstadoNuevo(), VersionResultante: e.GetVersionResultante(),
		})
	}
	return &types.ListHistorialEstudianteResponse{Eventos: eventos, Total: out.Total}, nil
}
