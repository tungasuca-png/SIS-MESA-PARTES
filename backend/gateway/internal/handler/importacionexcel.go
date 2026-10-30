package handler

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"

	"gateway/internal/logic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Carga del Excel de estudiantes (multipart/form-data). El Gateway solo
// transporta el archivo: no lo interpreta ni lo guarda. Estudiantes Service
// valida extensión, estructura, tamaño y contenido.

// maxArchivoExcel: tope del archivo (mismo predeterminado que el servicio).
// El cuerpo HTTP completo además está limitado por el MaxBytes de go-zero.
const maxArchivoExcel = 1 << 20

// margenMultipart: encabezados y separadores del multipart.
const margenMultipart = 64 << 10

var errSinArchivo = status.Error(codes.InvalidArgument,
	"archivo_invalido: envíe el Excel como multipart/form-data en el campo «archivo» (máximo 1 MB)")

// leerArchivoExcel lee el campo "archivo" (y "huella" en la confirmación)
// sin escribir nada en disco: todo el formulario cabe en memoria, y aun así
// se eliminan los temporales que pudiera crear el parser. El nombre del
// archivo del cliente solo se usa para verificar la extensión, nunca como ruta.
func leerArchivoExcel(r *http.Request) (logic.ArchivoExcel, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxArchivoExcel+margenMultipart)
	if err := r.ParseMultipartForm(maxArchivoExcel + margenMultipart); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return logic.ArchivoExcel{}, status.Error(codes.InvalidArgument, "archivo_grande: el archivo supera el máximo de 1 MB")
		}
		return logic.ArchivoExcel{}, errSinArchivo
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	f, encabezado, err := r.FormFile("archivo")
	if err != nil {
		return logic.ArchivoExcel{}, errSinArchivo
	}
	defer func() { _ = f.Close() }()
	contenido, err := io.ReadAll(io.LimitReader(f, maxArchivoExcel+1))
	if err != nil {
		return logic.ArchivoExcel{}, errSinArchivo
	}
	if len(contenido) > maxArchivoExcel {
		return logic.ArchivoExcel{}, status.Error(codes.InvalidArgument, "archivo_grande: el archivo supera el máximo de 1 MB")
	}
	return logic.ArchivoExcel{
		Nombre:    filepath.Base(encabezado.Filename),
		Contenido: contenido,
		Huella:    r.FormValue("huella"),
	}, nil
}
