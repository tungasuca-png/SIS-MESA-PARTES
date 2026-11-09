package limite

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A-04: límites por IP de POST /api/auth/login y POST /api/auth/refresh con
// las reglas REALES (RutasPorDefecto).

func pedirDesde(l *Limitador, metodo, ruta, remoteAddr, xff string) int {
	req := httptest.NewRequest(metodo, ruta, strings.NewReader(`{}`))
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	l.Middleware(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })(rec, req)
	if rec.Code == http.StatusTooManyRequests && rec.Header().Get("Retry-After") == "" {
		return -1 // 429 sin Retry-After: formato incorrecto
	}
	return rec.Code
}

func TestLogin_PermiteLaRafagaYBloqueaElExceso(t *testing.T) {
	l, reloj := nuevoPrueba(RutasPorDefecto(), false)
	for i := 0; i < 10; i++ {
		if c := pedir(l, "POST", "/api/auth/login", "203.0.113.20", "", "").Code; c != http.StatusOK {
			t.Fatalf("login %d dentro de la ráfaga: %d", i+1, c)
		}
	}
	rec := pedir(l, "POST", "/api/auth/login", "203.0.113.20", "", "")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("el login 11 debía responder 429 con Retry-After: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), MensajeLimite) || strings.Contains(rec.Body.String(), "203.0.113.20") {
		t.Fatalf("429 sin el mensaje genérico o con datos del cliente: %s", rec.Body.String())
	}
	// 30/min: se recupera 1 cada 2 s.
	reloj.t = reloj.t.Add(2100 * time.Millisecond)
	if c := pedir(l, "POST", "/api/auth/login", "203.0.113.20", "", "").Code; c != http.StatusOK {
		t.Fatalf("tras esperar: %d", c)
	}
}

func TestRefresh_PermiteLaRafagaYBloqueaElExceso(t *testing.T) {
	l, _ := nuevoPrueba(RutasPorDefecto(), false)
	for i := 0; i < 20; i++ {
		if c := pedir(l, "POST", "/api/auth/refresh", "203.0.113.21", "", "").Code; c != http.StatusOK {
			t.Fatalf("refresh %d dentro de la ráfaga: %d", i+1, c)
		}
	}
	if c := pedirDesde(l, "POST", "/api/auth/refresh", "203.0.113.21:51000", ""); c != http.StatusTooManyRequests {
		t.Fatalf("el refresh 21 debía responder 429 con Retry-After: %d", c)
	}
}

func TestLoginYRefresh_AisladosPorIPYPorRuta(t *testing.T) {
	l, _ := nuevoPrueba(RutasPorDefecto(), false)
	for i := 0; i < 11; i++ {
		pedir(l, "POST", "/api/auth/login", "203.0.113.30", "", "")
	}
	if c := pedir(l, "POST", "/api/auth/login", "203.0.113.30", "", "").Code; c != http.StatusTooManyRequests {
		t.Fatalf("la IP agotada debía quedar limitada: %d", c)
	}
	// Otra IP no se ve afectada.
	if c := pedir(l, "POST", "/api/auth/login", "203.0.113.31", "", "").Code; c != http.StatusOK {
		t.Fatalf("otra IP: %d", c)
	}
	// Agotar el login no bloquea la renovación de sesiones de esa IP.
	if c := pedir(l, "POST", "/api/auth/refresh", "203.0.113.30", "", "").Code; c != http.StatusOK {
		t.Fatalf("refresh de la IP con el login agotado: %d", c)
	}
	// Un token de otra persona no evade el límite por IP del login.
	if c := pedir(l, "POST", "/api/auth/login", "203.0.113.30", "Bearer otro-token", "").Code; c != http.StatusTooManyRequests {
		t.Fatalf("cambiar Authorization no debe evadir el límite por IP: %d", c)
	}
	// Los límites existentes siguen intactos.
	for _, ruta := range []string{"POST /api/auth/token", "POST /api/auth/registro", "GET /api/auth/google/iniciar"} {
		if _, ok := RutasPorDefecto()[ruta]; !ok {
			t.Fatalf("se perdió la regla %s", ruta)
		}
	}
	// GET no se limita (otro método, ruta inexistente en el Gateway).
	for i := 0; i < 30; i++ {
		if c := pedir(l, "GET", "/api/auth/login", "203.0.113.30", "", "").Code; c != http.StatusOK {
			t.Fatalf("GET /api/auth/login no debía limitarse: %d", c)
		}
	}
}

func TestLogin_IPNoIdentificableSigueLimitada(t *testing.T) {
	l, _ := nuevoPrueba(RutasPorDefecto(), false)
	// RemoteAddr sin formato host:puerto ni IP válida: se usa el texto tal
	// cual como clave; NO queda exento del límite.
	for _, origen := range []string{"desconocido", ""} {
		for i := 0; i < 10; i++ {
			if c := pedirDesde(l, "POST", "/api/auth/login", origen, ""); c != http.StatusOK {
				t.Fatalf("origen %q, intento %d: %d", origen, i+1, c)
			}
		}
		if c := pedirDesde(l, "POST", "/api/auth/login", origen, ""); c != http.StatusTooManyRequests {
			t.Fatalf("origen no identificable %q debía limitarse: %d", origen, c)
		}
	}

	// Detrás del proxy local, un X-Forwarded-For inválido no da una clave
	// nueva: se usa la IP del proxy (más restrictivo, nunca exento).
	l, _ = nuevoPrueba(RutasPorDefecto(), true)
	for i := 0; i < 10; i++ {
		pedirDesde(l, "POST", "/api/auth/login", "127.0.0.1:51000", "no-es-una-ip")
	}
	if c := pedirDesde(l, "POST", "/api/auth/login", "127.0.0.1:51000", "tampoco-ip"); c != http.StatusTooManyRequests {
		t.Fatalf("XFF inválido no debe evadir el límite: %d", c)
	}
	// Un cliente real detrás del proxy conserva su propia cubeta.
	if c := pedirDesde(l, "POST", "/api/auth/login", "127.0.0.1:51000", "198.51.100.40"); c != http.StatusOK {
		t.Fatalf("cliente identificable tras el proxy: %d", c)
	}
}
