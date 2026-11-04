package logic

import (
	"gateway/internal/types"
	"psicologia/psicologiaclient"
)

func toExpedientePsicologicoDTO(e *psicologiaclient.ExpedientePsicologico) types.ExpedientePsicologicoDTO {
	if e == nil {
		return types.ExpedientePsicologicoDTO{}
	}
	var estudiante *types.EstudianteResumenDTO
	if r := e.Estudiante; r != nil {
		estudiante = &types.EstudianteResumenDTO{NombreCompleto: r.NombreCompleto, Resolucion: r.Resolucion}
	}
	return types.ExpedientePsicologicoDTO{
		Id:            e.Id,
		EstudianteId:  e.EstudianteId,
		Codigo:        e.Codigo,
		Estado:        e.Estado,
		ResponsableId: e.ResponsableId,
		AbiertoPor:    e.AbiertoPor,
		FechaApertura: e.FechaApertura,
		FechaCierre:   e.FechaCierre,
		MotivoCierre:  e.MotivoCierre,
		ResumenCaso:   e.ResumenCaso,
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
		Estudiante:    estudiante,
	}
}
