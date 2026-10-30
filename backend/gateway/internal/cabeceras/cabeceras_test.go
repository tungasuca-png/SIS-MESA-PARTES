package cabeceras

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSeguridad_AgregaCabeceras(t *testing.T) {
	h := Seguridad(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
	})
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest(http.MethodGet, "/api/expedientes", nil))

	for k, v := range map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Cache-Control":           "no-store",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "frame-ancestors 'none'",
		"Referrer-Policy":         "no-referrer",
		"Content-Type":            "application/json", // el handler conserva su tipo
	} {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, se esperaba %q", k, got, v)
		}
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("el middleware no debe alterar el código: %d", w.Code)
	}
}
