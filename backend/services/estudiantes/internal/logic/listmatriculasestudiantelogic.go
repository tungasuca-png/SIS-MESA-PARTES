package logic

import (
	"context"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"
	"estudiantes/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMatriculasEstudianteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMatriculasEstudianteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMatriculasEstudianteLogic {
	return &ListMatriculasEstudianteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListMatriculasEstudiante (modelo maestro, etapa 3): historial académico
// del estudiante (una matrícula por periodo, con sus movimientos). Es
// información académica de nivel básico (E-02 §6): "estudiantes.view". No
// devuelve el motivo de retiro.
func (l *ListMatriculasEstudianteLogic) ListMatriculasEstudiante(in *estudiantes.ListMatriculasEstudianteRequest) (*estudiantes.ListMatriculasEstudianteResponse, error) {
	if _, err := autorizar(l.ctx, l.svcCtx, "estudiantes.view"); err != nil {
		return nil, err
	}
	if in == nil {
		in = &estudiantes.ListMatriculasEstudianteRequest{}
	}
	id, err := validarID(in.EstudianteId)
	if err != nil {
		return nil, escrituraError(l.Logger, "listar matrículas", err)
	}
	matriculas, err := l.svcCtx.EstudianteRepository.ListMatriculas(l.ctx, id)
	if err != nil {
		return nil, errorSeguro(l.Logger, "listar matrículas", "no se pudieron consultar las matrículas", err)
	}
	out := make([]*estudiantes.Matricula, 0, len(matriculas))
	for i := range matriculas {
		out = append(out, matriculaToProto(&matriculas[i]))
	}
	return &estudiantes.ListMatriculasEstudianteResponse{Matriculas: out}, nil
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func matriculaToProto(m *repository.Matricula) *estudiantes.Matricula {
	out := &estudiantes.Matricula{
		Id: m.ID,
		Periodo: &estudiantes.PeriodoAcademico{
			Id: m.Periodo.ID, Anio: int32(m.Periodo.Anio), Nombre: m.Periodo.Nombre, Estado: m.Periodo.Estado,
		},
		EstadoMatricula: m.EstadoMatricula,
		FechaMatricula:  strOrEmpty(m.FechaMatricula),
		FechaRetiro:     strOrEmpty(m.FechaRetiro),
		Origen:          m.Origen,
		Version:         int32(m.Version),
	}
	if m.Seccion != nil {
		out.Seccion = &estudiantes.SeccionAcademica{
			Id: m.Seccion.ID, Nivel: m.Seccion.Nivel, Grado: m.Seccion.Grado, Seccion: m.Seccion.Seccion,
			Turno: strOrEmpty(m.Seccion.Turno), Aula: strOrEmpty(m.Seccion.Aula),
		}
	}
	for _, mv := range m.Movimientos {
		out.Movimientos = append(out.Movimientos, &estudiantes.MovimientoMatricula{
			Tipo: mv.Tipo, Fecha: mv.Fecha,
			SeccionAnteriorId: strOrEmpty(mv.SeccionAnteriorID), SeccionNuevaId: strOrEmpty(mv.SeccionNuevaID),
		})
	}
	return out
}
