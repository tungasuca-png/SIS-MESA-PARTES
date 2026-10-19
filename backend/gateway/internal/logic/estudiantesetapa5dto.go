package logic

import (
	"gateway/internal/types"

	"estudiantes/estudiantesclient"
)

// Etapa 5 — conversiones de periodos, secciones, matrículas y personas.
// Transporte puro: el Gateway no decide nada sobre estos datos.

func toPersonaDTO(p *estudiantesclient.Persona) types.PersonaDTO {
	return types.PersonaDTO{
		Id: p.GetId(), TipoDocumento: p.GetTipoDocumento(), NumeroDocumento: p.GetNumeroDocumento(),
		Nombres: p.GetNombres(), ApellidoPaterno: p.GetApellidoPaterno(), ApellidoMaterno: p.GetApellidoMaterno(),
		Version: p.GetVersion(),
	}
}

func toPersonaInput(p types.PersonaInputDTO) *estudiantesclient.Persona {
	return &estudiantesclient.Persona{
		TipoDocumento: p.TipoDocumento, NumeroDocumento: p.NumeroDocumento, Nombres: p.Nombres,
		ApellidoPaterno: p.ApellidoPaterno, ApellidoMaterno: p.ApellidoMaterno,
	}
}

func toPeriodoDTO(p *estudiantesclient.PeriodoAcademico) types.PeriodoAcademicoDTO {
	return types.PeriodoAcademicoDTO{
		Id: p.GetId(), Anio: p.GetAnio(), Nombre: p.GetNombre(), Estado: p.GetEstado(),
		FechaInicio: p.GetFechaInicio(), FechaFin: p.GetFechaFin(),
	}
}

func toSeccionDTO(s *estudiantesclient.SeccionAcademica) types.SeccionAcademicaDTO {
	return types.SeccionAcademicaDTO{
		Id: s.GetId(), PeriodoId: s.GetPeriodoId(), Nivel: s.GetNivel(), Grado: s.GetGrado(), Seccion: s.GetSeccion(),
		Turno: s.GetTurno(), Aula: s.GetAula(), Vacantes: s.Vacantes, Matriculados: s.GetMatriculados(),
	}
}

func toMatriculaDTO(m *estudiantesclient.Matricula) types.MatriculaDTO {
	dto := types.MatriculaDTO{
		Id:              m.GetId(),
		EstudianteId:    m.GetEstudianteId(),
		Periodo:         toPeriodoDTO(m.GetPeriodo()),
		EstadoMatricula: m.GetEstadoMatricula(),
		FechaMatricula:  m.GetFechaMatricula(),
		FechaRetiro:     m.GetFechaRetiro(),
		Origen:          m.GetOrigen(),
		Version:         m.GetVersion(),
		Movimientos:     make([]types.MovimientoMatriculaDTO, 0, len(m.GetMovimientos())),
	}
	if s := m.GetSeccion(); s != nil {
		seccion := toSeccionDTO(s)
		dto.Seccion = &seccion
	}
	for _, mv := range m.GetMovimientos() {
		dto.Movimientos = append(dto.Movimientos, types.MovimientoMatriculaDTO{
			Tipo: mv.GetTipo(), Fecha: mv.GetFecha(), SeccionAnteriorId: mv.GetSeccionAnteriorId(), SeccionNuevaId: mv.GetSeccionNuevaId(),
		})
	}
	return dto
}

func toMatriculaResponse(out *estudiantesclient.MatriculaResponse) *types.MatriculaResponse {
	return &types.MatriculaResponse{Matricula: toMatriculaDTO(out.GetMatricula())}
}

// toEstudianteBasicoPtr: nil si el servicio no devolvió al estudiante.
func toEstudianteBasicoPtr(e *estudiantesclient.EstudianteBasico) *types.EstudianteBasicoDTO {
	if e == nil {
		return nil
	}
	dto := toEstudianteBasicoDTO(e)
	return &dto
}
