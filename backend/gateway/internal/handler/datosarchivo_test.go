package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/errorsafe"

	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/router"
)

func multipartDatos(t *testing.T, archivo []byte, nombre, solicitud string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if archivo != nil {
		fw, _ := w.CreateFormFile("archivo", nombre)
		_, _ = fw.Write(archivo)
	}
	if solicitud != "" {
		_ = w.WriteField("solicitud", solicitud)
	}
	_ = w.Close()
	r := httptest.NewRequest(http.MethodPost, "/api/estudiantes/datos/analizar", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}

func TestLeerArchivoDatos_SolicitudCompleta(t *testing.T) {
	r := multipartDatos(t, []byte("dni;nombre\n01234567;Ana\n"), `C:\x\..\alumnos.csv`,
		`{"opciones":{"separador":";","fila_encabezado":2},"mapeo":[{"columna":0,"campo":"numero_documento"}],"mapeo_confirmado":true,"huella":"h","excluir_invalidos":true,"aceptadas":[5],"omitidas":[7]}`)
	s, err := leerArchivoDatos(r)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Proto()
	if p.Opciones.Separador != ";" || p.Opciones.FilaEncabezado != 2 || len(p.Mapeo) != 1 || !p.MapeoConfirmado ||
		p.Huella != "h" || !p.ExcluirInvalidos || p.Aceptadas[0] != 5 || p.Omitidas[0] != 7 {
		t.Fatalf("solicitud: %+v", p)
	}
	if strings.ContainsAny(p.NombreArchivo, `\/`) || string(p.Contenido) != "dni;nombre\n01234567;Ana\n" {
		t.Fatalf("archivo: %q", p.NombreArchivo)
	}
	sinOpciones, _ := leerArchivoDatos(multipartDatos(t, []byte("x"), "a.csv", ""))
	if sinOpciones.Proto().Opciones != nil {
		t.Fatal("sin opciones, el servicio detecta todo")
	}
}

func TestDatos_SolicitudesInvalidas400(t *testing.T) {
	httpx.SetErrorHandlerCtx(errorsafe.Handler)
	t.Cleanup(func() { httpx.SetErrorHandlerCtx(nil) })
	for nombre, c := range map[string]struct {
		r    func() *http.Request
		code string
	}{
		"sin archivo":       {func() *http.Request { return multipartDatos(t, nil, "", `{}`) }, "archivo_invalido"},
		"solicitud no JSON": {func() *http.Request { return multipartDatos(t, []byte("x"), "a.csv", `no-json`) }, "opcion_invalida"},
		"campo desconocido": {func() *http.Request { return multipartDatos(t, []byte("x"), "a.csv", `{"sql":"DROP TABLE"}`) }, "opcion_invalida"},
		"archivo demasiado grande": {func() *http.Request {
			return multipartDatos(t, bytes.Repeat([]byte("a"), maxArchivoDatos+1), "a.csv", "")
		}, "archivo_grande"},
	} {
		for _, h := range []http.HandlerFunc{AnalizarDatosEstudiantesHandler(nil), ConfirmarDatosEstudiantesHandler(nil)} {
			w := httptest.NewRecorder()
			h(w, c.r())
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), c.code) {
				t.Errorf("%s: se esperaba 400 %s, got %d %q", nombre, c.code, w.Code, w.Body.String())
			}
		}
	}
}

func TestRutasDatosNoChocanConId(t *testing.T) {
	rt := router.NewRouter()
	marca := func(n string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(n)) }
	}
	for _, r := range []struct{ m, p, n string }{
		{http.MethodGet, "/api/estudiantes/:id", "get-id"},
		{http.MethodGet, "/api/estudiantes/datos/campos", "campos"},
		{http.MethodPost, "/api/estudiantes/datos/analizar", "analizar"},
		{http.MethodGet, "/api/estudiantes/datos/plantillas", "plantillas"},
		{http.MethodPost, "/api/estudiantes/datos/plantillas/:plantilla_id/eliminar", "eliminar"},
	} {
		if err := rt.Handle(r.m, r.p, marca(r.n)); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ m, p, want string }{
		{http.MethodGet, "/api/estudiantes/datos/campos", "campos"},
		{http.MethodPost, "/api/estudiantes/datos/analizar", "analizar"},
		{http.MethodPost, "/api/estudiantes/datos/plantillas/00000000-0000-4000-8000-000000000001/eliminar", "eliminar"},
		{http.MethodGet, "/api/estudiantes/00000000-0000-4000-8000-000000000001", "get-id"},
	} {
		w := httptest.NewRecorder()
		rt.ServeHTTP(w, httptest.NewRequest(c.m, c.p, nil))
		if w.Body.String() != c.want {
			t.Errorf("%s %s -> %q, se esperaba %q", c.m, c.p, w.Body.String(), c.want)
		}
	}
}
