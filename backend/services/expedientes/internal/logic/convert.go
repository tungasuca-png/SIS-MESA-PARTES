package logic

import (
	"time"

	"expedientes/expedientes"
	"expedientes/internal/repository"
)

func toProto(e *repository.Expediente) *expedientes.Expediente {
	if e == nil {
		return nil
	}

	return &expedientes.Expediente{
		Id:                 e.ID,
		Codigo:             e.Codigo,
		Tipo:               e.Tipo,
		Asunto:             e.Asunto,
		Descripcion:        e.Descripcion,
		SolicitanteId:      e.SolicitanteID,
		Estado:             e.Estado,
		Prioridad:          e.Prioridad,
		FechaRegistro:      e.FechaRegistro.Format(time.RFC3339),
		FechaActualizacion: e.FechaActualizacion.Format(time.RFC3339),
		Activo:             e.Activo,
		AreaActual:         e.AreaActual,
	}
}
