package logsafe

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/logx"
)

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	t.Cleanup(func() { logx.Reset() })
	return &buf
}

const (
	secretPassword = "MARKPASSWORD-123"
	secretJWT      = "eyJMARK.JWT.SECRET"
)

func doRequest(t *testing.T, h http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+secretJWT)
	req.Header.Set("Cookie", "session="+secretJWT)
	rec := httptest.NewRecorder()
	AccessLog(h)(rec, req)
	return rec
}

func TestAccessLog_SoloMetodoRutaCodigoDuracion(t *testing.T) {
	logs := captureLogs(t)
	h := func(w http.ResponseWriter, r *http.Request) {
		// El handler SÍ puede leer el cuerpo: el middleware no debe consumirlo ni registrarlo.
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), secretPassword) {
			t.Error("el middleware no debe consumir el cuerpo del request")
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"credenciales inválidas"}`))
	}
	rec := doRequest(t, h, http.MethodPost, "/api/auth/login?token="+secretJWT, `{"dni":"12345678","password":"`+secretPassword+`"}`)

	out := logs.String()
	if !strings.Contains(out, "[HTTP] POST /api/auth/login 401 ") {
		t.Fatalf("falta la línea con método, ruta, código y duración: %q", out)
	}
	for _, prohibido := range []string{secretPassword, secretJWT, "Authorization", "Bearer", "Cookie", "dni", "password", "token="} {
		if strings.Contains(out, prohibido) {
			t.Fatalf("el log no debe contener %q: %q", prohibido, out)
		}
	}
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "credenciales inválidas") {
		t.Fatalf("la respuesta debe conservarse intacta: %d %q", rec.Code, rec.Body.String())
	}
}

func TestAccessLog_5xxRegistraComoErrorSinVolcarElRequest(t *testing.T) {
	logs := captureLogs(t)
	h := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }
	doRequest(t, h, http.MethodPost, "/api/auth/refresh", `{"refresh_token":"`+secretPassword+`"}`)

	out := logs.String()
	if !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, "[HTTP] POST /api/auth/refresh 503 ") {
		t.Fatalf("un 5xx debe registrarse como error: %q", out)
	}
	for _, prohibido := range []string{secretPassword, secretJWT, "Authorization", "Bearer", "refresh_token", "Host:", "\\r\\n"} {
		if strings.Contains(out, prohibido) {
			t.Fatalf("el 5xx no debe volcar el request (%q): %q", prohibido, out)
		}
	}
}

func TestAccessLog_PreservaRespuestaIntacta(t *testing.T) {
	captureLogs(t)
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Prueba", "valor")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false}`))
	}
	rec := doRequest(t, h, http.MethodPut, "/api/x", "")
	if rec.Code != http.StatusConflict || rec.Body.String() != `{"ok":false}` ||
		rec.Header().Get("Content-Type") != "application/json" || rec.Header().Get("X-Prueba") != "valor" {
		t.Fatalf("el middleware alteró la respuesta: %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
}

func TestAccessLog_SinWriteHeaderEsOK(t *testing.T) {
	logs := captureLogs(t)
	h := func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hola")) }
	rec := doRequest(t, h, http.MethodGet, "/api/ok", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "hola" {
		t.Fatalf("respuesta alterada: %d %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "[HTTP] GET /api/ok 200 ") {
		t.Fatalf("se esperaba 200 por defecto: %q", logs.String())
	}
}

func TestAccessLog_PanicSeRelanzaYSeRegistra500(t *testing.T) {
	logs := captureLogs(t)
	defer func() {
		if recover() == nil {
			t.Fatal("el panic debe relanzarse para que lo maneje el Recover de go-zero")
		}
		out := logs.String()
		if !strings.Contains(out, "[HTTP] GET /api/boom 500 ") || strings.Contains(out, "valor-del-panic") {
			t.Fatalf("debe registrar 500 sin el valor del panic: %q", out)
		}
	}()
	h := func(w http.ResponseWriter, r *http.Request) { panic("valor-del-panic") }
	doRequest(t, h, http.MethodGet, "/api/boom", "")
}

func TestAccessLog_RutaConSaltosDeLineaNoInyectaLog(t *testing.T) {
	logs := captureLogs(t)
	h := func(w http.ResponseWriter, r *http.Request) {}
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.URL.Path = "/api/x\nFAKE-LINE"
	AccessLog(h)(httptest.NewRecorder(), req)
	if strings.Contains(logs.String(), "\nFAKE-LINE") {
		t.Fatalf("la ruta no debe inyectar líneas: %q", logs.String())
	}
}

func TestSensitiveRPCMethods_NombresVerificadosContraElCodigoGenerado(t *testing.T) {
	// Cantidad esperada por servicio (los nombres salen de las constantes
	// generadas por protoc en rpc.go, así que un nombre mal escrito ni
	// compila): Auth 6, Psicología 18 (7 de expedientes + 11 de atenciones)
	// y Estudiantes 8 (3 de lectura, FASE 5B, + 3 de escritura, FASE E-05,
	// + 2 de datos personales, FASE E-08, + 6 del modelo maestro).
	esperados := map[string]int{
		"/auth.Auth/":               6,
		"/psicologia.Psicologia/":   18,
		"/estudiantes.Estudiantes/": 32, // etapas 5 y 6: todos los RPC de Estudiantes
	}
	encontrados := map[string]int{}
	for _, m := range sensitiveRPCMethods {
		prefijo := ""
		for p := range esperados {
			if strings.HasPrefix(m, p) {
				prefijo = p
			}
		}
		if prefijo == "" {
			t.Fatalf("método inesperado %q", m)
		}
		encontrados[prefijo]++
	}
	for p, n := range esperados {
		if encontrados[p] != n {
			t.Fatalf("se esperaban %d métodos con prefijo %s, hay %d", n, p, encontrados[p])
		}
	}
	if len(sensitiveRPCMethods) != 56 {
		t.Fatalf("se esperaban 6 métodos de Auth + 18 de Psicología + 32 de Estudiantes, hay %d", len(sensitiveRPCMethods))
	}
}
