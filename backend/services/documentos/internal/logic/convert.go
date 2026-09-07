package logic

import (
	"time"

	"documentos/documentos"
	"documentos/internal/repository"
)

func toProto(d *repository.Documento) *documentos.Documento {
	if d == nil {
		return nil
	}
	return &documentos.Documento{
		Id:            d.ID,
		ExpedienteId:  d.ExpedienteID,
		Nombre:        d.Nombre,
		TipoDocumento: d.TipoDocumento,
		Extension:     d.Extension,
		TamanoBytes:   d.TamanoBytes,
		SubidoPor:     d.SubidoPor,
		Estado:        d.Estado,
		FechaRegistro: d.FechaRegistro.Format(time.RFC3339),
	}
}
