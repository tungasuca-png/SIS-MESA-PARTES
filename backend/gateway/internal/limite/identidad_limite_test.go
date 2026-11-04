package limite

import (
	"net/http"
	"testing"
	"time"
)

// Límites de la bandeja de verificación de identidad: rutas con :id.

const (
	rutaVerificarA = "/api/solicitantes/identidades/11111111-1111-4111-8111-111111111111/verificar"
	rutaVerificarB = "/api/solicitantes/identidades/22222222-2222-4222-8222-222222222222/verificar"
	rutaRechazarA  = "/api/solicitantes/identidades/11111111-1111-4111-8111-111111111111/rechazar"
)

func TestIdentidad_VerificarLimitadoPorPatronYCredencial(t *testing.T) {
	l, reloj := nuevoPrueba(RutasPorDefecto(), false)
	const ip, sec1, sec2 = "203.0.113.20", "Bearer token-secretaria-1", "Bearer token-secretaria-2"

	// Ráfaga de 5 por credencial: dentro del límite, permitido.
	for i := 0; i < 5; i++ {
		if c := pedir(l, "POST", rutaVerificarA, ip, sec1, "").Code; c != http.StatusOK {
			t.Fatalf("intento %d dentro del límite: %d", i, c)
		}
	}
	// Excede: 429 con Retry-After.
	rec := pedir(l, "POST", rutaVerificarA, ip, sec1, "")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("se esperaba 429: %d", rec.Code)
	}
	// Cambiar el id de la URL no evade el límite (contador por patrón).
	if c := pedir(l, "POST", rutaVerificarB, ip, sec1, "").Code; c != http.StatusTooManyRequests {
		t.Fatalf("otro id con la misma credencial: %d", c)
	}
	// Otro usuario (otra credencial) no hereda el contador.
	if c := pedir(l, "POST", rutaVerificarA, ip, sec2, "").Code; c != http.StatusOK {
		t.Fatalf("otra credencial: %d", c)
	}
	// Rechazar tiene su propio contador; otras rutas no se afectan.
	if c := pedir(l, "POST", rutaRechazarA, ip, sec1, "").Code; c != http.StatusOK {
		t.Fatalf("rechazar es independiente de verificar: %d", c)
	}
	if c := pedir(l, "GET", "/api/solicitantes/identidades", ip, sec1, "").Code; c != http.StatusOK {
		t.Fatalf("la bandeja es independiente: %d", c)
	}
	for _, ruta := range []string{"/api/expedientes", "/api/usuarios/basic-batch", "/api/solicitantes/identidades/x/y/verificar"} {
		if c := pedir(l, "POST", ruta, ip, sec1, "").Code; c != http.StatusOK {
			t.Fatalf("ruta no limitada %s: %d", ruta, c)
		}
	}
	// Se recupera con el tiempo (10/min por credencial → 1 cada 6 s).
	reloj.t = reloj.t.Add(7 * time.Second)
	if c := pedir(l, "POST", rutaVerificarB, ip, sec1, "").Code; c != http.StatusOK {
		t.Fatalf("tras la espera: %d", c)
	}
}

func TestIdentidad_RotarTokensNoEvadeElLimitePorIP(t *testing.T) {
	l, _ := nuevoPrueba(RutasPorDefecto(), false)
	const ip = "203.0.113.30"
	// Un token nuevo por solicitud: el límite por IP (ráfaga 10) se aplica igual.
	for i := 0; i < 10; i++ {
		if c := pedir(l, "POST", rutaVerificarA, ip, "Bearer rotado-"+string(rune('a'+i)), "").Code; c != http.StatusOK {
			t.Fatalf("intento %d: %d", i, c)
		}
	}
	if c := pedir(l, "POST", rutaVerificarB, ip, "Bearer rotado-z", "").Code; c != http.StatusTooManyRequests {
		t.Fatalf("rotar tokens desde la misma IP: %d", c)
	}
	// Otra IP con otra credencial no se ve afectada.
	if c := pedir(l, "POST", rutaVerificarA, "203.0.113.31", "Bearer otro", "").Code; c != http.StatusOK {
		t.Fatalf("otra IP: %d", c)
	}
}

func TestIdentidad_ReconciliarYPatronesNoAlteranRutasExactas(t *testing.T) {
	l, _ := nuevoPrueba(RutasPorDefecto(), false)
	const ip, cred = "203.0.113.40", "Bearer token-reconciliar"
	for i := 0; i < 5; i++ {
		if c := pedir(l, "POST", "/api/solicitantes/identidades/reconciliar", ip, cred, "").Code; c != http.StatusOK {
			t.Fatalf("reconciliar %d: %d", i, c)
		}
	}
	if c := pedir(l, "POST", "/api/solicitantes/identidades/reconciliar", ip, cred, "").Code; c != http.StatusTooManyRequests {
		t.Fatalf("reconciliar excedido: %d", c)
	}
	// "reconciliar" no se confunde con el patrón /:id/verificar.
	if c := pedir(l, "POST", rutaVerificarA, ip, cred, "").Code; c != http.StatusOK {
		t.Fatalf("verificar tras reconciliar: %d", c)
	}
	// Un GET a la ruta de verificar no se limita (otro método).
	if c := pedir(l, "GET", rutaVerificarA, ip, cred, "").Code; c != http.StatusOK {
		t.Fatalf("otro método: %d", c)
	}
}
