package logic

import (
	"gateway/internal/types"
	"psicologia/psicologiaclient"
)

// Conversión 1:1 entre el contrato HTTP y el proto de Psicología Service. Sin
// lógica de negocio: validación, permisos y alcance se resuelven en el servicio.

func toAtencionContenidoProto(c types.AtencionContenidoDTO) *psicologiaclient.AtencionContenido {
	return &psicologiaclient.AtencionContenido{
		FechaAtencion:         c.FechaAtencion,
		HoraAtencion:          c.HoraAtencion,
		RealizadaPor:          c.RealizadaPor,
		RealizadaPorOtros:     c.RealizadaPorOtros,
		Motivos:               c.Motivos,
		MotivoOtros:           c.MotivoOtros,
		DescripcionMotivo:     c.DescripcionMotivo,
		AnalisisIntegral:      c.AnalisisIntegral,
		Estrategias:           c.Estrategias,
		Derivaciones:          c.Derivaciones,
		DerivacionOtro:        c.DerivacionOtro,
		SeguimientoResultados: c.SeguimientoResultados,
		Acuerdos:              c.Acuerdos,
		CompromisoEstudiante:  c.CompromisoEstudiante,
		CompromisoFamilia:     c.CompromisoFamilia,
	}
}

func toAtencionContenidoDTO(c *psicologiaclient.AtencionContenido) types.AtencionContenidoDTO {
	if c == nil {
		return types.AtencionContenidoDTO{RealizadaPor: []string{}, Motivos: []string{}, Derivaciones: []string{}}
	}
	return types.AtencionContenidoDTO{
		FechaAtencion:         c.FechaAtencion,
		HoraAtencion:          c.HoraAtencion,
		RealizadaPor:          nonNilStrings(c.RealizadaPor),
		RealizadaPorOtros:     c.RealizadaPorOtros,
		Motivos:               nonNilStrings(c.Motivos),
		MotivoOtros:           c.MotivoOtros,
		DescripcionMotivo:     c.DescripcionMotivo,
		AnalisisIntegral:      c.AnalisisIntegral,
		Estrategias:           c.Estrategias,
		Derivaciones:          nonNilStrings(c.Derivaciones),
		DerivacionOtro:        c.DerivacionOtro,
		SeguimientoResultados: c.SeguimientoResultados,
		Acuerdos:              c.Acuerdos,
		CompromisoEstudiante:  c.CompromisoEstudiante,
		CompromisoFamilia:     c.CompromisoFamilia,
	}
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func toOpcionesDTO(items []*psicologiaclient.OpcionCatalogo) []types.OpcionCatalogoDTO {
	out := make([]types.OpcionCatalogoDTO, 0, len(items))
	for _, o := range items {
		out = append(out, types.OpcionCatalogoDTO{Codigo: o.Codigo, Etiqueta: o.Etiqueta})
	}
	return out
}

func toAtencionVersionDTO(v *psicologiaclient.AtencionVersion) types.AtencionVersionDTO {
	if v == nil {
		return types.AtencionVersionDTO{Contenido: toAtencionContenidoDTO(nil), Motivos: []types.OpcionCatalogoDTO{}, Derivaciones: []types.OpcionCatalogoDTO{}}
	}
	return types.AtencionVersionDTO{
		NumeroVersion:    v.NumeroVersion,
		Contenido:        toAtencionContenidoDTO(v.Contenido),
		Motivos:          toOpcionesDTO(v.Motivos),
		Derivaciones:     toOpcionesDTO(v.Derivaciones),
		MotivoCorreccion: v.MotivoCorreccion,
		CreadaPor:        v.CreadaPor,
		CreadaEn:         v.CreadaEn,
	}
}

func toAtencionResponse(a *psicologiaclient.AtencionPsicologica) *types.PsicologiaAtencionResponse {
	if a == nil {
		return &types.PsicologiaAtencionResponse{}
	}
	return &types.PsicologiaAtencionResponse{Atencion: types.AtencionPsicologicaDTO{
		Id:              a.Id,
		ExpedienteId:    a.ExpedienteId,
		Numero:          a.Numero,
		Estado:          a.Estado,
		AutorId:         a.AutorId,
		RegistradaPor:   a.RegistradaPor,
		RegistradaEn:    a.RegistradaEn,
		VersionVigente:  a.VersionVigente,
		Revision:        a.Revision,
		AnuladaPor:      a.AnuladaPor,
		AnuladaEn:       a.AnuladaEn,
		MotivoAnulacion: a.MotivoAnulacion,
		CreatedAt:       a.CreatedAt,
		UpdatedAt:       a.UpdatedAt,
		Version:         toAtencionVersionDTO(a.Version),
	}}
}

func toAtencionResumenDTO(r *psicologiaclient.AtencionResumen) types.AtencionResumenDTO {
	return types.AtencionResumenDTO{
		Id:             r.Id,
		Numero:         r.Numero,
		Estado:         r.Estado,
		FechaAtencion:  r.FechaAtencion,
		HoraAtencion:   r.HoraAtencion,
		Motivos:        toOpcionesDTO(r.Motivos),
		AutorId:        r.AutorId,
		RegistradaPor:  r.RegistradaPor,
		RegistradaEn:   r.RegistradaEn,
		VersionVigente: r.VersionVigente,
	}
}
