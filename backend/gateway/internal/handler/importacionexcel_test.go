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

func multipartCon(t *testing.T, campo, nombre string, contenido []byte, huella string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if campo != "" {
		fw, err := w.CreateFormFile(campo, nombre)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(contenido)
	}
	if huella != "" {
		_ = w.WriteField("huella", huella)
	}
	_ = w.Close()
	r := httptest.NewRequest(http.MethodPost, "/api/estudiantes/importacion/preview", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	return r
}

func TestLeerArchivoExcel(t *testing.T) {
	r := multipartCon(t, "archivo", `..\..\C:\temp\estudiantes.xlsx`, []byte("PK\x03\x04datos"), "abc123")
	a, err := leerArchivoExcel(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(a.Contenido) != "PK\x03\x04datos" || a.Huella != "abc123" {
		t.Fatalf("contenido/huella: %+v", a)
	}
	if strings.ContainsAny(a.Nombre, `/`) || !strings.HasSuffix(a.Nombre, ".xlsx") {
		t.Fatalf("el nombre del cliente nunca es una ruta: %q", a.Nombre)
	}
}

// Seguridad/robustez: sin archivo, con otro campo, sin multipart o demasiado
// grande responde 400 sin llegar al servicio (svcCtx nil).
func TestImportacionExcel_SolicitudesInvalidas400(t *testing.T) {
	httpx.SetErrorHandlerCtx(errorsafe.Handler)
	t.Cleanup(func() { httpx.SetErrorHandlerCtx(nil) })

	grande := multipartCon(t, "archivo", "e.xlsx", bytes.Repeat([]byte("x"), maxArchivoExcel+1), "")
	json := httptest.NewRequest(http.MethodPost, "/api/estudiantes/importacion/preview", strings.NewReader(`{"archivo":"x"}`))
	json.Header.Set("Content-Type", "application/json")
	for nombre, c := range map[string]struct {
		r    *http.Request
		code string
	}{
		"sin archivo":              {multipartCon(t, "", "", nil, "h"), "archivo_invalido"},
		"otro campo":               {multipartCon(t, "documento", "e.xlsx", []byte("PK"), ""), "archivo_invalido"},
		"JSON en vez de multipart": {json, "archivo_invalido"},
		"demasiado grande":         {grande, "archivo_grande"},
	} {
		for _, h := range []http.HandlerFunc{PreviewImportacionEstudiantesHandler(nil), ConfirmarImportacionEstudiantesHandler(nil)} {
			w := httptest.NewRecorder()
			h(w, c.r)
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), c.code) {
				t.Errorf("%s: se esperaba 400 %s, got %d %q", nombre, c.code, w.Code, w.Body.String())
			}
		}
	}
}

// Las rutas fijas nuevas no deben quedar capturadas por /api/estudiantes/:id.
func TestRutasExcelNoChocanConId(t *testing.T) {
	rt := router.NewRouter()
	marca := func(nombre string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(nombre)) }
	}
	for _, r := range []struct{ metodo, ruta, nombre string }{
		{http.MethodGet, "/api/estudiantes/:id", "get-id"},
		{http.MethodGet, "/api/estudiantes/:id/personal", "personal"},
		{http.MethodGet, "/api/estudiantes/duplicados", "duplicados"},
		{http.MethodGet, "/api/estudiantes/exportar", "exportar"},
		{http.MethodGet, "/api/estudiantes/importacion/plantilla", "plantilla"},
		{http.MethodPost, "/api/estudiantes/importacion/preview", "preview"},
		{http.MethodPost, "/api/estudiantes/importacion/confirmar", "confirmar"},
	} {
		if err := rt.Handle(r.metodo, r.ruta, marca(r.nombre)); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ metodo, ruta, want string }{
		{http.MethodGet, "/api/estudiantes/exportar", "exportar"},
		{http.MethodGet, "/api/estudiantes/importacion/plantilla", "plantilla"},
		{http.MethodPost, "/api/estudiantes/importacion/preview", "preview"},
		{http.MethodPost, "/api/estudiantes/importacion/confirmar", "confirmar"},
		{http.MethodGet, "/api/estudiantes/00000000-0000-4000-8000-000000000001", "get-id"},
	} {
		w := httptest.NewRecorder()
		rt.ServeHTTP(w, httptest.NewRequest(c.metodo, c.ruta, nil))
		if w.Body.String() != c.want {
			t.Errorf("%s %s -> %q, se esperaba %q", c.metodo, c.ruta, w.Body.String(), c.want)
		}
	}
}
