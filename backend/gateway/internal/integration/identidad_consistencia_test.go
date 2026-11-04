package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

// Consistencia Auth ↔ Usuarios en la verificación de identidad, de punta a
// punta contra el entorno DESECHABLE (scratchpad/aislamiento.sh). Las fases
// las ordena el script: "sincronizada" (todo arriba), "usuarios_caido" (el
// script detuvo Usuarios), "reconciliacion" (Usuarios de vuelta) y "limite".

func fase(t *testing.T, nombre string) {
	t.Helper()
	if os.Getenv("AISLAMIENTO_FASE") != nombre {
		t.Skipf("fase %q (la ejecuta scratchpad/aislamiento.sh)", nombre)
	}
}

func idDeLogin(t *testing.T, base string, cuerpo map[string]string) (*cliente, string) {
	t.Helper()
	anon := &cliente{t: t, base: base}
	r := anon.pedir(http.MethodPost, "/api/auth/login", cuerpo)
	var ses struct {
		AccessToken string `json:"access_token"`
		UserId      string `json:"user_id"`
	}
	if r.codigo != http.StatusOK || json.Unmarshal([]byte(r.cuerpo), &ses) != nil {
		t.Fatalf("login: %d", r.codigo)
	}
	return &cliente{t: t, base: base, token: ses.AccessToken}, ses.UserId
}

func verificar(c *cliente, usuarioID string) (respuesta, string) {
	r := c.pedir(http.MethodPost, "/api/solicitantes/identidades/"+usuarioID+"/verificar", map[string]string{"metodo": "PRESENCIAL"})
	var v struct {
		Sincronizacion string `json:"sincronizacion"`
	}
	_ = json.Unmarshal([]byte(r.cuerpo), &v)
	return r, v.Sincronizacion
}

func perfilDNI(c *cliente, usuarioID string) string {
	c.t.Helper()
	r := c.pedir(http.MethodGet, "/api/usuarios/"+usuarioID, nil)
	var v struct {
		Usuario struct {
			Dni string `json:"dni"`
		} `json:"usuario"`
	}
	if r.codigo != http.StatusOK || json.Unmarshal([]byte(r.cuerpo), &v) != nil {
		c.t.Fatalf("perfil %s: %d", usuarioID, r.codigo)
	}
	return v.Usuario.Dni
}

// administrador: lee perfiles ajenos (usuarios.view) para comprobar Usuarios.
func administrador(t *testing.T, base, clave string) *cliente {
	c, _ := idDeLogin(t, base, map[string]string{"dni": entorno(t, "AISLAMIENTO_DNI_ADM"), "password": clave})
	return c
}

func secretaria(t *testing.T, base, clave string) *cliente {
	// Sesión nueva por fase: el límite por credencial de cada fase es propio.
	c, _ := idDeLogin(t, base, map[string]string{"dni": entorno(t, "AISLAMIENTO_DNI_S"), "password": clave})
	return c
}

func TestIdentidad_ConsistenciaConServiciosReales(t *testing.T) {
	fase(t, "sincronizada")
	base, clave := entorno(t, "AISLAMIENTO_GATEWAY_URL"), entorno(t, "AISLAMIENTO_CLAVE")
	s := secretaria(t, base, clave)
	adm := administrador(t, base, clave)
	_, pID := idDeLogin(t, base, map[string]string{"correo": entorno(t, "AISLAMIENTO_CORREO_P"), "password": clave})

	// Ambos servicios completan; el solicitante ya entra con su DNI y el perfil lo tiene.
	if r, sinc := verificar(s, pID); r.codigo != http.StatusOK || sinc != "COMPLETA" {
		t.Fatalf("verificación completa: %d %s", r.codigo, sinc)
	}
	if _, id := idDeLogin(t, base, map[string]string{"dni": "70000003", "password": clave}); id != pID {
		t.Fatal("tras verificar, el solicitante entra con su DNI")
	}
	if dni := perfilDNI(adm, pID); dni != "70000003" {
		t.Fatal("el perfil (Usuarios) tiene el DNI verificado")
	}
	// Repetir una verificación completa: idempotente.
	if r, sinc := verificar(s, pID); r.codigo != http.StatusOK || sinc != "COMPLETA" {
		t.Fatalf("verificación repetida: %d %s", r.codigo, sinc)
	}

	// DNI que solo existe en un perfil institucional de Usuarios: Auth verifica,
	// Usuarios rechaza y Auth COMPENSA; nada queda a medias.
	p2ID := entorno(t, "AISLAMIENTO_ID_P2")
	if r, _ := verificar(s, p2ID); r.codigo != http.StatusConflict || !strings.Contains(r.cuerpo, "identidad_conflicto") {
		t.Fatalf("conflicto con un perfil institucional: %d", r.codigo)
	}
	anon := &cliente{t: t, base: base}
	if r := anon.pedir(http.MethodPost, "/api/auth/login", map[string]string{"dni": "70000004", "password": clave}); r.codigo != http.StatusUnauthorized {
		t.Fatalf("tras la compensación, el DNI ajeno no sirve para entrar: %d", r.codigo)
	}
	if dni := perfilDNI(adm, entorno(t, "AISLAMIENTO_ID_INST")); dni != "70000004" {
		t.Fatal("el perfil institucional conserva su DNI")
	}
	if dni := perfilDNI(adm, p2ID); dni != "" {
		t.Fatal("el perfil del solicitante no recibió el DNI ajeno")
	}
	bandeja := s.pedir(http.MethodGet, "/api/solicitantes/identidades?estado=PENDIENTE", nil)
	var lista struct {
		Items []struct {
			UsuarioId string `json:"usuario_id"`
			Conflicto bool   `json:"conflicto"`
		} `json:"items"`
	}
	_ = json.Unmarshal([]byte(bandeja.cuerpo), &lista)
	visto := false
	for _, i := range lista.Items {
		if i.UsuarioId == p2ID {
			visto = i.Conflicto
		}
	}
	if !visto {
		t.Fatal("la bandeja muestra el conflicto (y no se reintenta a ciegas)")
	}
	if r, _ := verificar(s, p2ID); r.codigo != http.StatusConflict {
		t.Fatalf("reintentar un conflicto conocido: %d", r.codigo)
	}

	// Dos verificaciones simultáneas del mismo DNI: gana exactamente una.
	// (Sesión nueva: esta fase ya usó la ráfaga de 5 por credencial del
	// limitador; el límite por IP, 10, sigue cubriendo ambas sesiones.)
	s = secretaria(t, base, clave)
	ids := []string{entorno(t, "AISLAMIENTO_ID_P4"), entorno(t, "AISLAMIENTO_ID_P5")}
	codigos := make([]int, 2)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			r, _ := verificar(s, id)
			codigos[i] = r.codigo
		}(i, id)
	}
	wg.Wait()
	if !((codigos[0] == 200 && codigos[1] == 409) || (codigos[0] == 409 && codigos[1] == 200)) {
		t.Fatalf("verificaciones simultáneas: %v", codigos)
	}
	con := 0
	for _, id := range ids {
		if perfilDNI(adm, id) == "70000006" {
			con++
		}
	}
	if con != 1 {
		t.Fatalf("un solo perfil con el DNI: %d", con)
	}
}

func TestIdentidad_UsuariosCaidoQuedaPendiente(t *testing.T) {
	fase(t, "usuarios_caido")
	base, clave := entorno(t, "AISLAMIENTO_GATEWAY_URL"), entorno(t, "AISLAMIENTO_CLAVE")
	s := secretaria(t, base, clave)
	p3ID := entorno(t, "AISLAMIENTO_ID_P3")
	// Auth verifica; Usuarios no responde: queda PERFIL_PENDIENTE, sin error
	// para el personal (el DNI ya no puede usarlo otra cuenta).
	if r, sinc := verificar(s, p3ID); r.codigo != http.StatusOK || sinc != "PERFIL_PENDIENTE" {
		t.Fatalf("Usuarios caído: %d %s %s", r.codigo, sinc, r.cuerpo)
	}
	r := s.pedir(http.MethodGet, "/api/solicitantes/identidades?estado=VERIFICADA&sincronizacion_pendiente=true", nil)
	if r.codigo != http.StatusOK || !strings.Contains(r.cuerpo, p3ID) || !strings.Contains(r.cuerpo, "usuarios_no_disponible") {
		t.Fatalf("la operación pendiente queda trazada: %d", r.codigo)
	}
}

func TestIdentidad_ReconciliacionCompletaLoPendiente(t *testing.T) {
	fase(t, "reconciliacion")
	base, clave := entorno(t, "AISLAMIENTO_GATEWAY_URL"), entorno(t, "AISLAMIENTO_CLAVE")
	s := secretaria(t, base, clave)
	adm := administrador(t, base, clave)
	p3ID := entorno(t, "AISLAMIENTO_ID_P3")
	r := s.pedir(http.MethodPost, "/api/solicitantes/identidades/reconciliar", map[string]string{})
	var v struct {
		Completadas int `json:"completadas"`
		Pendientes  int `json:"pendientes"`
	}
	if r.codigo != http.StatusOK || json.Unmarshal([]byte(r.cuerpo), &v) != nil || v.Completadas < 1 || v.Pendientes != 0 {
		t.Fatalf("reconciliación: %d %s", r.codigo, r.cuerpo)
	}
	if dni := perfilDNI(adm, p3ID); dni != "70000005" {
		t.Fatal("tras reconciliar, el perfil tiene el DNI")
	}
	if r, sinc := verificar(s, p3ID); r.codigo != http.StatusOK || sinc != "COMPLETA" {
		t.Fatalf("tras reconciliar, la verificación figura completa: %d %s", r.codigo, sinc)
	}
	// Un solicitante no puede reconciliar ni verificar (403).
	sol, _ := idDeLogin(t, base, map[string]string{"dni": entorno(t, "AISLAMIENTO_DNI_A"), "password": clave})
	if r := sol.pedir(http.MethodPost, "/api/solicitantes/identidades/reconciliar", map[string]string{}); r.codigo != http.StatusForbidden {
		t.Fatalf("solicitante reconcilia: %d", r.codigo)
	}
	if r, _ := verificar(sol, p3ID); r.codigo != http.StatusForbidden {
		t.Fatalf("solicitante verifica: %d", r.codigo)
	}
}

func TestIdentidad_LimiteHTTPNoSeEvadeCambiandoElID(t *testing.T) {
	fase(t, "limite")
	base, clave := entorno(t, "AISLAMIENTO_GATEWAY_URL"), entorno(t, "AISLAMIENTO_CLAVE")
	s := secretaria(t, base, clave)
	ids := []string{"11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"}
	primero, _ := verificar(s, ids[0])
	if primero.codigo == http.StatusTooManyRequests {
		t.Fatal("la primera solicitud está dentro del límite")
	}
	limitado := false
	for i := 0; i < 8 && !limitado; i++ {
		r, _ := verificar(s, ids[i%len(ids)]) // ids distintos en cada intento
		limitado = r.codigo == http.StatusTooManyRequests
	}
	if !limitado {
		t.Fatal("cambiando el id también se alcanza el 429")
	}
	// Otras rutas legítimas siguen funcionando.
	if r := s.pedir(http.MethodGet, "/api/expedientes?page_size=5", nil); r.codigo != http.StatusOK {
		t.Fatalf("otra ruta: %d", r.codigo)
	}
}
