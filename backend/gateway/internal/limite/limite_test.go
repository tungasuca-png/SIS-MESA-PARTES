package limite

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gateway/internal/errorsafe"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func init() { httpx.SetErrorHandlerCtx(errorsafe.Handler) }

type reloj struct{ t time.Time }

func nuevoPrueba(rutas map[string]Ruta, proxy bool) (*Limitador, *reloj) {
	r := &reloj{t: time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)}
	l := Nuevo(rutas, proxy)
	l.ahora = func() time.Time { return r.t }
	return l, r
}

func pedir(l *Limitador, metodo, ruta, ip, authz, xff string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, ruta, strings.NewReader(`{"q":"45127823"}`))
	req.RemoteAddr = ip + ":51000"
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	l.Middleware(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })(rec, req)
	return rec
}

func TestTokenLimitadoPorIP(t *testing.T) {
	l, reloj := nuevoPrueba(RutasPorDefecto(), false)
	for i := 0; i < 5; i++ {
		if c := pedir(l, "POST", "/api/auth/token", "203.0.113.7", "", "").Code; c != http.StatusOK {
			t.Fatalf("intento %d dentro de la ráfaga: %d", i, c)
		}
	}
	rec := pedir(l, "POST", "/api/auth/token", "203.0.113.7", "", "")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("se esperaba 429 con Retry-After: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ResourceExhausted") || !strings.Contains(rec.Body.String(), MensajeLimite) {
		t.Fatalf("formato de error inesperado: %s", rec.Body.String())
	}
	// Otra IP no se ve afectada.
	if c := pedir(l, "POST", "/api/auth/token", "203.0.113.8", "", "").Code; c != http.StatusOK {
		t.Fatalf("otra IP: %d", c)
	}
	// Se recupera con el tiempo (10/min → 1 cada 6 s).
	reloj.t = reloj.t.Add(7 * time.Second)
	if c := pedir(l, "POST", "/api/auth/token", "203.0.113.7", "", "").Code; c != http.StatusOK {
		t.Fatalf("tras esperar: %d", c)
	}
}

func TestBusquedaLimitadaPorCredencialEIP(t *testing.T) {
	l, _ := nuevoPrueba(RutasPorDefecto(), false)
	const ruta = "/api/estudiantes/seleccion/buscar"
	for i := 0; i < 20; i++ {
		if c := pedir(l, "POST", ruta, "203.0.113.7", "Bearer cliente-a", "").Code; c != http.StatusOK {
			t.Fatalf("ráfaga del cliente A, %d: %d", i, c)
		}
	}
	if c := pedir(l, "POST", ruta, "203.0.113.7", "Bearer cliente-a", "").Code; c != http.StatusTooManyRequests {
		t.Fatalf("cliente A debió limitarse: %d", c)
	}
	// Otro cliente desde la misma IP tiene su propia cubeta...
	if c := pedir(l, "POST", ruta, "203.0.113.7", "Bearer cliente-b", "").Code; c != http.StatusOK {
		t.Fatalf("cliente B: %d", c)
	}
	// ...pero rotar tokens desde una IP no evade el límite por IP (ráfaga 30).
	rechazos := 0
	for i := 0; i < 40; i++ {
		if pedir(l, "POST", ruta, "203.0.113.9", "Bearer rotado-"+string(rune('a'+i)), "").Code == http.StatusTooManyRequests {
			rechazos++
		}
	}
	if rechazos == 0 {
		t.Fatal("rotar tokens no debe evadir el límite por IP")
	}
}

func TestRutasNoListadasYMetodosDistintosNoSeLimitan(t *testing.T) {
	l, _ := nuevoPrueba(map[string]Ruta{"POST /api/auth/token": {PorIP: Regla{PorMinuto: 1, Rafaga: 1}}}, false)
	for i := 0; i < 50; i++ {
		if c := pedir(l, "GET", "/api/expedientes", "203.0.113.7", "", "").Code; c != http.StatusOK {
			t.Fatalf("ruta no listada limitada: %d", c)
		}
		if c := pedir(l, "GET", "/api/auth/token", "203.0.113.7", "", "").Code; c != http.StatusOK {
			t.Fatalf("otro método limitado: %d", c)
		}
	}
}

func TestIPDelProxyLocal(t *testing.T) {
	regla := map[string]Ruta{"POST /api/auth/token": {PorIP: Regla{PorMinuto: 1, Rafaga: 1}}}
	// Sin confiar en el proxy, todos los clientes detrás de él comparten la IP 127.0.0.1.
	l, _ := nuevoPrueba(regla, false)
	pedir(l, "POST", "/api/auth/token", "127.0.0.1", "", "198.51.100.1")
	if c := pedir(l, "POST", "/api/auth/token", "127.0.0.1", "", "198.51.100.2").Code; c != http.StatusTooManyRequests {
		t.Fatalf("sin confiar en el proxy se usa la IP de conexión: %d", c)
	}
	// Confiando en el proxy local: la última IP de X-Forwarded-For.
	l, _ = nuevoPrueba(regla, true)
	pedir(l, "POST", "/api/auth/token", "127.0.0.1", "", "10.0.0.5, 198.51.100.1")
	if c := pedir(l, "POST", "/api/auth/token", "127.0.0.1", "", "198.51.100.2").Code; c != http.StatusOK {
		t.Fatalf("otro cliente tras el proxy: %d", c)
	}
	// Un cliente no puede falsificar su IP anteponiendo valores.
	if c := pedir(l, "POST", "/api/auth/token", "127.0.0.1", "", "1.2.3.4, 198.51.100.1").Code; c != http.StatusTooManyRequests {
		t.Fatalf("XFF falsificado al inicio no debe cambiar la clave: %d", c)
	}
	// X-Forwarded-For desde una IP que no es el proxy local se ignora.
	pedir(l, "POST", "/api/auth/token", "203.0.113.7", "", "198.51.100.3")
	if c := pedir(l, "POST", "/api/auth/token", "203.0.113.7", "", "198.51.100.4").Code; c != http.StatusTooManyRequests {
		t.Fatalf("XFF de un cliente directo no debe usarse: %d", c)
	}
}

func TestCubetasInactivasSeDescartan(t *testing.T) {
	l, reloj := nuevoPrueba(RutasPorDefecto(), false)
	pedir(l, "POST", "/api/auth/token", "203.0.113.7", "", "")
	reloj.t = reloj.t.Add(inactividadMaxima + intervaloLimpieza + time.Second)
	pedir(l, "POST", "/api/auth/token", "203.0.113.8", "", "")
	if len(l.cubetas) != 1 {
		t.Fatalf("se esperaba descartar la cubeta inactiva: %d", len(l.cubetas))
	}
}

func TestNoRegistraTokenNiConsulta(t *testing.T) {
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	defer logx.Reset()
	l, _ := nuevoPrueba(map[string]Ruta{"POST /api/estudiantes/seleccion/buscar": {PorCredencial: Regla{PorMinuto: 1, Rafaga: 1}}}, false)
	for i := 0; i < 3; i++ {
		pedir(l, "POST", "/api/estudiantes/seleccion/buscar", "203.0.113.7", "Bearer secreto.token.ficticio", "")
	}
	salida := buf.String()
	if !strings.Contains(salida, "límite de solicitudes excedido") {
		t.Fatalf("debe registrarse el rechazo: %s", salida)
	}
	for _, prohibido := range []string{"secreto.token.ficticio", "45127823", "203.0.113.7"} {
		if strings.Contains(salida, prohibido) {
			t.Fatalf("el registro contiene %q", prohibido)
		}
	}
	for k := range l.cubetas {
		if strings.Contains(k, "secreto") {
			t.Fatal("la clave interna no debe contener el token en claro")
		}
	}
}
