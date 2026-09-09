package logic

import (
	"derivaciones/derivacionesclient"
	"gateway/internal/types"
)

func toDerivacionDTO(d *derivacionesclient.Derivacion) types.DerivacionDTO {
	if d == nil {
		return types.DerivacionDTO{}
	}

	return types.DerivacionDTO{
		Id:            d.Id,
		ExpedienteId:  d.ExpedienteId,
		Tipo:          d.Tipo,
		Origen:        d.Origen,
		Destino:       d.Destino,
		Motivo:        d.Motivo,
		Condicion:     d.Condicion,
		RegistradoPor: d.RegistradoPor,
		FechaRegistro: d.FechaRegistro,
	}
}
