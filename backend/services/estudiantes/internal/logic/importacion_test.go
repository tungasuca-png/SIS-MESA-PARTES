package logic

import (
	"errors"
	"testing"

	"estudiantes/internal/excel"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Un archivo mal formado debe llegar al usuario como 400 con su motivo, no
// como 500 genérico; un error inesperado (BD) sí se oculta.
func TestErrorImportacion(t *testing.T) {
	log := logx.WithContext(t.Context())
	err := errorImportacion(log, "previsualizar", errorArchivo(&excel.ErrorArchivo{Code: "encabezados_invalidos", Msg: "faltan columnas"}))
	if status.Code(err) != codes.InvalidArgument || status.Convert(err).Message() != "encabezados_invalidos: faltan columnas" {
		t.Fatalf("archivo inválido: se esperaba 400 con el motivo, got %v", err)
	}
	err = errorImportacion(log, "previsualizar", errors.New("pq: conexión rechazada en 127.0.0.1:5433"))
	if status.Code(err) != codes.Internal || status.Convert(err).Message() != "no se pudo analizar el archivo" {
		t.Fatalf("error de BD: se esperaba 500 genérico, got %v", err)
	}
}
