package logic

import (
	"time"

	"derivaciones/derivaciones"
	"derivaciones/internal/repository"
)

func toProto(d *repository.Derivacion) *derivaciones.Derivacion {
	if d == nil {
		return nil
	}

	return &derivaciones.Derivacion{
		Id:             d.ID,
		ExpedienteId:   d.ExpedienteID,
		Tipo:           d.Tipo,
		Origen:         d.Origen,
		Destino:        d.Destino,
		Motivo:         d.Motivo,
		Condicion:      d.Condicion,
		RegistradoPor:  d.RegistradoPor,
		FechaRegistro:  d.FechaRegistro.Format(time.RFC3339),
		Clase:          d.Clase,
		Secuencia:      int32(d.Secuencia),
		EstadoAnterior: d.EstadoAnterior,
		EstadoNuevo:    d.EstadoNuevo,
		ActorRol:       d.ActorRol,
	}
}
