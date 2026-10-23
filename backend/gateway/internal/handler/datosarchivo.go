package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"gateway/internal/logic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Carga del motor universal de datos (multipart/form-data):
//   - "archivo": el archivo (xlsx, ods, csv, json, xml, ...), hasta 3 MB;
//   - "solicitud": JSON con opciones de lectura, mapeo y decisiones.
// El Gateway no interpreta el archivo: lo reenvía a Estudiantes Service, que
// detecta el formato por su contenido y valida todo.

const (
	maxArchivoDatos   = 3 << 20
	maxSolicitudDatos = 256 << 10
)

var errSinArchivoDatos = status.Error(codes.InvalidArgument,
	"archivo_invalido: envíe el archivo como multipart/form-data en el campo «archivo» (máximo 3 MB)")

func leerArchivoDatos(r *http.Request) (logic.SolicitudDatos, error) {
	var out logic.SolicitudDatos
	r.Body = http.MaxBytesReader(nil, r.Body, maxArchivoDatos+maxSolicitudDatos+margenMultipart)
	if err := r.ParseMultipartForm(maxArchivoDatos + maxSolicitudDatos + margenMultipart); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return out, status.Error(codes.InvalidArgument, "archivo_grande: el archivo supera el máximo de 3 MB")
		}
		return out, errSinArchivoDatos
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	f, encabezado, err := r.FormFile("archivo")
	if err != nil {
		return out, errSinArchivoDatos
	}
	defer func() { _ = f.Close() }()
	contenido, err := io.ReadAll(io.LimitReader(f, maxArchivoDatos+1))
	if err != nil {
		return out, errSinArchivoDatos
	}
	if len(contenido) > maxArchivoDatos {
		return out, status.Error(codes.InvalidArgument, "archivo_grande: el archivo supera el máximo de 3 MB")
	}
	if raw := strings.TrimSpace(r.FormValue("solicitud")); raw != "" {
		if len(raw) > maxSolicitudDatos {
			return out, status.Error(codes.InvalidArgument, "opcion_invalida: la solicitud es demasiado grande")
		}
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&out); err != nil {
			return out, status.Error(codes.InvalidArgument, "opcion_invalida: el campo «solicitud» no es un JSON válido")
		}
	}
	out.NombreArchivo = filepath.Base(encabezado.Filename) // solo para la extensión, nunca como ruta
	out.Contenido = contenido
	return out, nil
}
