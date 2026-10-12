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

type ListMatriculasEstudianteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListMatriculasEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMatriculasEstudianteLogic {
	return &ListMatriculasEstudianteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListMatriculasEstudiante (modelo maestro): historial académico; permiso
// (estudiantes.view) en Estudiantes Service.
func (l *ListMatriculasEstudianteLogic) ListMatriculasEstudiante(req *types.ListMatriculasEstudianteRequest) (resp *types.ListMatriculasEstudianteResponse, err error) {
	ctx := withAuthorization(l.ctx, req.Authorization)
	out, err := l.svcCtx.EstudiantesClient.ListMatriculasEstudiante(ctx, &estudiantesclient.ListMatriculasEstudianteRequest{EstudianteId: req.Id})
	if err != nil {
		return nil, err
	}
	matriculas := make([]types.MatriculaDTO, 0, len(out.Matriculas))
	for _, m := range out.Matriculas {
		dto := types.MatriculaDTO{
			Id: m.GetId(),
			Periodo: types.PeriodoAcademicoDTO{
				Id: m.GetPeriodo().GetId(), Anio: m.GetPeriodo().GetAnio(),
				Nombre: m.GetPeriodo().GetNombre(), Estado: m.GetPeriodo().GetEstado(),
			},
			EstadoMatricula: m.GetEstadoMatricula(),
			FechaMatricula:  m.GetFechaMatricula(),
			FechaRetiro:     m.GetFechaRetiro(),
			Origen:          m.GetOrigen(),
			Version:         m.GetVersion(),
			Movimientos:     make([]types.MovimientoMatriculaDTO, 0, len(m.GetMovimientos())),
		}
		if s := m.GetSeccion(); s != nil {
			dto.Seccion = &types.SeccionAcademicaDTO{
				Id: s.GetId(), Nivel: s.GetNivel(), Grado: s.GetGrado(), Seccion: s.GetSeccion(), Turno: s.GetTurno(), Aula: s.GetAula(),
			}
		}
		for _, mv := range m.GetMovimientos() {
			dto.Movimientos = append(dto.Movimientos, types.MovimientoMatriculaDTO{
				Tipo: mv.GetTipo(), Fecha: mv.GetFecha(), SeccionAnteriorId: mv.GetSeccionAnteriorId(), SeccionNuevaId: mv.GetSeccionNuevaId(),
			})
		}
		matriculas = append(matriculas, dto)
	}
	return &types.ListMatriculasEstudianteResponse{Matriculas: matriculas}, nil
}
