package logic

import (
	"context"
	"time"

	"estudiantes/estudiantes"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListHistorialEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListHistorialEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHistorialEstudianteLogic {
	return &ListHistorialEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListHistorialEstudiante (etapa 6): eventos de CAMBIO del estudiante (alta,
// edición, estado, datos personales, familia, matrícula), más reciente
// primero — "estudiantes.view". Solo nombres de campos, nunca valores; las
// lecturas auditadas no se incluyen.
func (l *ListHistorialEstudianteLogic) ListHistorialEstudiante(in *estudiantes.ListHistorialEstudianteRequest) (*estudiantes.ListHistorialEstudianteResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, permisoView); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ListHistorialEstudianteRequest{}
	}
	id, err := validarID(in.EstudianteId)
	if err != nil {
		return nil, errorSeguro(l.Logger, "historial", "no se pudo consultar el historial", err)
	}
	page, size := paginar(in.Page, in.PageSize)
	eventos, total, err := l.svcCtx.EstudianteRepository.ListHistorial(l.ctx, id, page, size)
	if err != nil {
		return nil, errorSeguro(l.Logger, "historial", "no se pudo consultar el historial", err)
	}
	out := make([]*estudiantes.EventoHistorial, 0, len(eventos))
	for _, e := range eventos {
		out = append(out, &estudiantes.EventoHistorial{
			Id: e.ID, Accion: e.Accion, FechaHora: e.FechaHora.UTC().Format(time.RFC3339), UsuarioId: e.UsuarioID,
			Campos: e.Campos, EstadoAnterior: strOrEmpty(e.EstadoAnterior), EstadoNuevo: strOrEmpty(e.EstadoNuevo),
			VersionResultante: int32(e.VersionResultante),
		})
	}
	return &estudiantes.ListHistorialEstudianteResponse{Eventos: out, Total: int32(total)}, nil
}
