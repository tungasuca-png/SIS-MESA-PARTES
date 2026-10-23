package logic

import (
	"encoding/base64"

	"gateway/internal/types"

	"estudiantes/estudiantesclient"
)

// Excel de estudiantes: el Gateway reenvía el archivo (o la solicitud) a
// Estudiantes Service con el JWT del usuario y traduce la respuesta. Toda la
// validación, la autorización y la escritura ocurren en el servicio.

// ArchivoExcel es el archivo subido por multipart (ver handler).
type ArchivoExcel struct {
	Nombre    string
	Contenido []byte
	Huella    string
}

func toArchivoExcelDTO(in *estudiantesclient.ArchivoExcelResponse) *types.ArchivoExcelResponse {
	if in == nil {
		return &types.ArchivoExcelResponse{}
	}
	return &types.ArchivoExcelResponse{
		Nombre:          in.Nombre,
		ContenidoBase64: base64.StdEncoding.EncodeToString(in.Contenido),
		Filas:           in.Filas,
	}
}

func toImportacionDTO(in *estudiantesclient.ImportarEstudiantesResponse) *types.ImportarEstudiantesExcelResponse {
	out := &types.ImportarEstudiantesExcelResponse{Filas: []types.FilaImportacionDTO{}}
	if in == nil {
		return out
	}
	out.Huella, out.Confirmada = in.Huella, in.Confirmada
	if r := in.Resumen; r != nil {
		out.Resumen = types.ResumenImportacionDTO{
			Total: r.Total, Validas: r.Validas, ConErrores: r.ConErrores, Duplicadas: r.Duplicadas,
			YaExisten: r.YaExisten, Insertadas: r.Insertadas, Omitidas: r.Omitidas, Rechazadas: r.Rechazadas,
			ConAvisos: r.ConAvisos,
		}
	}
	for _, f := range in.Filas {
		out.Filas = append(out.Filas, types.FilaImportacionDTO{
			Fila: f.Fila, TipoDocumento: f.TipoDocumento, NumeroDocumento: f.NumeroDocumento,
			ApellidoPaterno: f.ApellidoPaterno, ApellidoMaterno: f.ApellidoMaterno, Nombres: f.Nombres,
			Estado: f.Estado, Nivel: f.Nivel, Resultado: f.Resultado,
			Errores: append([]string{}, f.Errores...), Avisos: append([]string{}, f.Avisos...),
		})
	}
	return out
}
