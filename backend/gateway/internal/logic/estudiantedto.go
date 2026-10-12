package logic

import (
	"gateway/internal/types"

	"estudiantes/estudiantesclient"
)

func toEstudianteDTO(e *estudiantesclient.Estudiante) types.EstudianteDTO {
	if e == nil {
		return types.EstudianteDTO{}
	}
	return types.EstudianteDTO{
		Id:              e.Id,
		Codigo:          e.Codigo,
		Nombres:         e.Nombres,
		Apellidos:       e.Apellidos,
		Nivel:           e.Nivel,
		Grado:           e.Grado,
		Seccion:         e.Seccion,
		Turno:           e.Turno,
		EstadoMatricula: e.EstadoMatricula,
		Version:         e.Version,
	}
}

func toEstudianteBasicoDTO(e *estudiantesclient.EstudianteBasico) types.EstudianteBasicoDTO {
	if e == nil {
		return types.EstudianteBasicoDTO{}
	}
	return types.EstudianteBasicoDTO{
		Id:              e.Id,
		Codigo:          e.Codigo,
		NombreCompleto:  e.NombreCompleto,
		Nivel:           e.Nivel,
		Grado:           e.Grado,
		Seccion:         e.Seccion,
		EstadoMatricula: e.EstadoMatricula,
		NumeroDocumento: e.NumeroDocumento,
		Nombres:         e.Nombres,
		Apellidos:       e.Apellidos,
		ApellidoPaterno: e.ApellidoPaterno,
		ApellidoMaterno: e.ApellidoMaterno,
		CodigoSiagie:    e.CodigoSiagie,
		TipoDocumento:   e.TipoDocumento,
		Estado:          e.Estado,
		Turno:           e.Turno,
		Version:         e.Version,
	}
}

// toFamiliarDTO (modelo maestro): relación familiar con la persona.
func toFamiliarDTO(f *estudiantesclient.Familiar) types.FamiliarDTO {
	if f == nil {
		return types.FamiliarDTO{}
	}
	p := f.GetPersona()
	return types.FamiliarDTO{
		RelacionId: f.GetRelacionId(),
		Persona: types.PersonaDTO{
			Id: p.GetId(), TipoDocumento: p.GetTipoDocumento(), NumeroDocumento: p.GetNumeroDocumento(),
			Nombres: p.GetNombres(), ApellidoPaterno: p.GetApellidoPaterno(), ApellidoMaterno: p.GetApellidoMaterno(),
		},
		Parentesco:  f.GetParentesco(),
		EsApoderado: f.GetEsApoderado(),
		Vigente:     f.GetVigente(),
		Desde:       f.GetDesde(),
		Hasta:       f.GetHasta(),
	}
}
