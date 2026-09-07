package logic

import (
	"documentos/documentosclient"
	"gateway/internal/types"
)

func toDocumentoDTO(d *documentosclient.Documento) types.DocumentoDTO {
	if d == nil {
		return types.DocumentoDTO{}
	}

	return types.DocumentoDTO{
		Id:            d.Id,
		ExpedienteId:  d.ExpedienteId,
		Nombre:        d.Nombre,
		TipoDocumento: d.TipoDocumento,
		Extension:     d.Extension,
		TamanoBytes:   d.TamanoBytes,
		SubidoPor:     d.SubidoPor,
		Estado:        d.Estado,
		FechaRegistro: d.FechaRegistro,
	}
}
