package errorsafe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func responder(t *testing.T, err error) (int, string) {
	t.Helper()
	httpx.SetErrorHandlerCtx(Handler)
	t.Cleanup(func() { httpx.SetErrorHandlerCtx(nil) })
	w := httptest.NewRecorder()
	httpx.ErrorCtx(context.Background(), w, err)
	return w.Code, w.Body.String()
}

func TestHandler_4xx_IgualQueGoZero(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
	}{
		{status.Error(codes.NotFound, "student_not_found: estudiante no encontrado"), 404},
		{status.Error(codes.PermissionDenied, "permission_denied: sin permiso"), 403},
		{status.Error(codes.Unauthenticated, "se requiere autenticación"), 401},
		{status.Error(codes.InvalidArgument, "validation_error: dato inválido"), 400},
		{status.Error(codes.FailedPrecondition, "periodo_cerrado: cerrado"), 400},
		{status.Error(codes.AlreadyExists, "duplicate_codigo: ya existe"), 409},
		{status.Error(codes.Aborted, "version_conflict: modificado"), 409},
	} {
		code, body := responder(t, c.err)
		if code != c.code || strings.TrimSpace(body) != c.err.Error() {
			t.Errorf("%v: se esperaba %d con el texto original, se obtuvo %d %q", c.err, c.code, code, body)
		}
	}
}

func TestHandler_5xx_SinDetallesInternos(t *testing.T) {
	interno := "last connection error: connection error: desc = \"transport: Error while dialing: dial tcp 127.0.0.1:8086: connectex\""
	for _, c := range []struct {
		code codes.Code
		http int
	}{
		{codes.Unavailable, 503},
		{codes.DeadlineExceeded, 504},
		{codes.Internal, 500},
		{codes.Unknown, 500},
		{codes.Unimplemented, 501},
	} {
		code, body := responder(t, status.Error(c.code, interno))
		if code != c.http {
			t.Errorf("%v: código %d, se esperaba %d", c.code, code, c.http)
		}
		if strings.Contains(body, "127.0.0.1") || strings.Contains(body, "dial") || strings.Contains(body, "transport") {
			t.Errorf("%v: el cuerpo expone detalles internos: %q", c.code, body)
		}
		if !strings.Contains(body, "code = "+c.code.String()) || !strings.Contains(body, MensajeNoDisponible) {
			t.Errorf("%v: formato inesperado: %q", c.code, body)
		}
	}
}

// Seguridad: los errores de parseo con detalles internos (strconv, json, tipos
// de Go) responden 400 con un mensaje genérico. Reproduce GET
// /api/expedientes?page=abc y ?page_size=99999999999 y un JSON mal tipado.
func TestHandler_ErrorDeParseo_SinDetallesInternos(t *testing.T) {
	for _, interno := range []string{
		`strconv.ParseInt: parsing "abc": invalid syntax`,
		`strconv.ParseInt: parsing "99999999999": value out of range`,
		`json: cannot unmarshal string into Go struct field LoginRequest.dni of type int`,
		`invalid character 'x' looking for beginning of value`,
		`field "x" is not set; DROP TABLE`,
	} {
		code, body := responder(t, errors.New(interno))
		if code != http.StatusBadRequest {
			t.Errorf("%q: código %d, se esperaba 400", interno, code)
		}
		if !strings.Contains(body, MensajeSolicitudInvalida) || strings.Contains(body, "strconv") ||
			strings.Contains(body, "Go struct") || strings.Contains(body, "DROP") {
			t.Errorf("%q: el cuerpo expone detalles internos: %q", interno, body)
		}
	}
}

func TestHandler_ErrorDeParseo_400(t *testing.T) {
	code, body := responder(t, errors.New("field \"es_apoderado\" is not set"))
	if code != http.StatusBadRequest || !strings.Contains(body, "es_apoderado") {
		t.Fatalf("se esperaba el mismo 400 de go-zero, se obtuvo %d %q", code, body)
	}
}
