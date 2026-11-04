// Package integration: pruebas HTTP de punta a punta contra un entorno
// DESECHABLE (Gateway + Auth + Usuarios + Expedientes + Documentos en puertos
// propios y PostgreSQL temporal; ver scratchpad/aislamiento.sh). Nunca contra
// los servicios reales: sin AISLAMIENTO_GATEWAY_URL la prueba se omite.
package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type cliente struct {
	t     *testing.T
	base  string
	token string
}

type respuesta struct {
	codigo int
	cuerpo string
}

func (c *cliente) pedir(metodo, ruta string, cuerpo any) respuesta {
	c.t.Helper()
	var r io.Reader
	if cuerpo != nil {
		b, _ := json.Marshal(cuerpo)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(metodo, c.base+ruta, r)
	if err != nil {
		c.t.Fatal(err)
	}
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", metodo, ruta, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return respuesta{codigo: res.StatusCode, cuerpo: string(b)}
}

func entorno(t *testing.T, clave string) string {
	t.Helper()
	v := os.Getenv(clave)
	if v == "" {
		t.Skipf("%s no definida: solo corre contra el entorno desechable (scratchpad/aislamiento.sh)", clave)
	}
	return v
}

func login(t *testing.T, base string, cuerpo map[string]string) *cliente {
	t.Helper()
	anon := &cliente{t: t, base: base}
	r := anon.pedir(http.MethodPost, "/api/auth/login", cuerpo)
	var ses struct {
		AccessToken string `json:"access_token"`
	}
	if r.codigo != http.StatusOK || json.Unmarshal([]byte(r.cuerpo), &ses) != nil || ses.AccessToken == "" {
		t.Fatalf("login: %d", r.codigo)
	}
	return &cliente{t: t, base: base, token: ses.AccessToken}
}

func crearExpediente(c *cliente, asunto string) string {
	c.t.Helper()
	r := c.pedir(http.MethodPost, "/api/expedientes", map[string]string{"tipo": "CERTIFICADO", "asunto": asunto, "prioridad": "NORMAL"})
	var v struct {
		Expediente struct {
			Id string `json:"id"`
		} `json:"expediente"`
	}
	if r.codigo != http.StatusOK || json.Unmarshal([]byte(r.cuerpo), &v) != nil || v.Expediente.Id == "" {
		c.t.Fatalf("crear expediente: %d %s", r.codigo, r.cuerpo)
	}
	return v.Expediente.Id
}

func subirDocumento(c *cliente, expedienteID, nombre string) (string, respuesta) {
	c.t.Helper()
	r := c.pedir(http.MethodPost, "/api/documentos", map[string]string{
		"expediente_id": expedienteID, "nombre": nombre, "tipo_documento": "ADJUNTO", "extension": "pdf",
		"contenido": base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 contenido de prueba " + nombre)),
	})
	var v struct {
		Documento struct {
			Id string `json:"id"`
		} `json:"documento"`
	}
	_ = json.Unmarshal([]byte(r.cuerpo), &v)
	return v.Documento.Id, r
}

// denegado: 403 o 404, nunca 2xx, y sin datos del recurso ajeno.
func denegado(t *testing.T, nombre string, r respuesta, secretos ...string) {
	t.Helper()
	if r.codigo != http.StatusForbidden && r.codigo != http.StatusNotFound {
		t.Errorf("%s: se esperaba 403/404, fue %d", nombre, r.codigo)
	}
	for _, s := range secretos {
		if s != "" && strings.Contains(r.cuerpo, s) {
			t.Errorf("%s: la respuesta revela datos del recurso ajeno", nombre)
		}
	}
}

func TestAislamientoEntreSolicitantes(t *testing.T) {
	base := entorno(t, "AISLAMIENTO_GATEWAY_URL")
	clave := entorno(t, "AISLAMIENTO_CLAVE")
	a := login(t, base, map[string]string{"dni": entorno(t, "AISLAMIENTO_DNI_A"), "password": clave})
	b := login(t, base, map[string]string{"dni": entorno(t, "AISLAMIENTO_DNI_B"), "password": clave})
	// P: registro público con identidad PENDIENTE (sin DNI definitivo): entra con correo.
	p := login(t, base, map[string]string{"correo": entorno(t, "AISLAMIENTO_CORREO_P"), "password": clave})
	s := login(t, base, map[string]string{"dni": entorno(t, "AISLAMIENTO_DNI_S"), "password": clave})
	anon := &cliente{t: t, base: base}

	eA := crearExpediente(a, "Trámite privado de A")
	eB := crearExpediente(b, "Trámite privado de B")
	eP := crearExpediente(p, "Trámite de P (identidad pendiente)")
	dA, r := subirDocumento(a, eA, "documento-de-A")
	if r.codigo != http.StatusOK || dA == "" {
		t.Fatalf("A sube a su expediente: %d %s", r.codigo, r.cuerpo)
	}
	dB, r := subirDocumento(b, eB, "documento-de-B")
	if r.codigo != http.StatusOK || dB == "" {
		t.Fatalf("B sube a su expediente: %d %s", r.codigo, r.cuerpo)
	}

	// 1. A consulta lo suyo: permitido.
	for nombre, ruta := range map[string]string{
		"detalle de su documento":   "/api/documentos/" + dA,
		"contenido de su documento": "/api/documentos/" + dA + "/contenido",
		"documentos de su trámite":  "/api/expedientes/" + eA + "/documentos",
		"su expediente":             "/api/expedientes/" + eA,
	} {
		if r := a.pedir(http.MethodGet, ruta, nil); r.codigo != http.StatusOK {
			t.Errorf("A, %s: %d", nombre, r.codigo)
		}
	}

	// 2-5. A sobre recursos de B (cambiando ids en la URL o en el cuerpo): denegado.
	denegado(t, "A lee el documento de B", a.pedir(http.MethodGet, "/api/documentos/"+dB, nil), "documento-de-B")
	denegado(t, "A descarga el documento de B", a.pedir(http.MethodGet, "/api/documentos/"+dB+"/contenido", nil), "contenido de prueba")
	denegado(t, "A lee el expediente de B", a.pedir(http.MethodGet, "/api/expedientes/"+eB, nil), "Trámite privado de B")
	denegado(t, "A lista los documentos del expediente de B", a.pedir(http.MethodGet, "/api/expedientes/"+eB+"/documentos", nil), dB)
	denegado(t, "A genera el FUT del expediente de B", a.pedir(http.MethodGet, "/api/expedientes/"+eB+"/fut-pdf", nil), "Trámite privado de B")
	denegado(t, "A modifica el expediente de B", a.pedir(http.MethodPut, "/api/expedientes/"+eB, map[string]string{"asunto": "alterado por A"}))
	denegado(t, "A elimina el expediente de B", a.pedir(http.MethodDelete, "/api/expedientes/"+eB, nil))
	denegado(t, "A elimina el documento de B", a.pedir(http.MethodDelete, "/api/documentos/"+dB, nil))
	if _, r := subirDocumento(a, eB, "intruso"); r.codigo != http.StatusForbidden && r.codigo != http.StatusNotFound {
		t.Errorf("A sube a un expediente de B: %d", r.codigo)
	}
	// Listados y filtros: solo lo propio, aunque se pase el id de B o un
	// solicitante_id (el backend usa la identidad autenticada).
	if r := a.pedir(http.MethodGet, "/api/expedientes?page_size=100&solicitante_id=x", nil); r.codigo != http.StatusOK ||
		!strings.Contains(r.cuerpo, eA) || strings.Contains(r.cuerpo, eB) || strings.Contains(r.cuerpo, eP) {
		t.Errorf("A lista expedientes: %d (debe ver solo los suyos)", r.codigo)
	}
	if r := a.pedir(http.MethodGet, "/api/documentos?page_size=100", nil); r.codigo == http.StatusOK && (strings.Contains(r.cuerpo, dB) || !strings.Contains(r.cuerpo, dA)) {
		t.Errorf("A lista documentos: debe ver solo los suyos")
	} else if r.codigo != http.StatusOK && r.codigo != http.StatusForbidden {
		t.Errorf("A lista documentos: %d", r.codigo)
	}
	if r := a.pedir(http.MethodGet, "/api/documentos?expediente_id="+eB, nil); r.codigo == http.StatusOK && strings.Contains(r.cuerpo, dB) {
		t.Errorf("A filtra documentos por el expediente de B")
	}
	// Exportaciones y reportes: no son para solicitantes.
	for _, ruta := range []string{"/api/reportes/expedientes", "/api/reportes/expedientes/exportar?formato=csv", "/api/reportes/documentos"} {
		if r := a.pedir(http.MethodGet, ruta, nil); r.codigo != http.StatusForbidden {
			t.Errorf("A en %s: %d", ruta, r.codigo)
		}
	}
	// Lo de B quedó intacto.
	if r := b.pedir(http.MethodGet, "/api/expedientes/"+eB, nil); r.codigo != http.StatusOK || !strings.Contains(r.cuerpo, "Trámite privado de B") {
		t.Errorf("el expediente de B no debe cambiar: %d", r.codigo)
	}
	if r := b.pedir(http.MethodGet, "/api/documentos/"+dB+"/contenido", nil); r.codigo != http.StatusOK {
		t.Errorf("el documento de B no debe cambiar: %d", r.codigo)
	}

	// 6-7. Identidad pendiente: lo suyo sí, lo ajeno no.
	if r := p.pedir(http.MethodGet, "/api/expedientes/"+eP, nil); r.codigo != http.StatusOK {
		t.Errorf("P consulta su trámite: %d", r.codigo)
	}
	if r := p.pedir(http.MethodGet, "/api/expedientes?page_size=100", nil); r.codigo != http.StatusOK || !strings.Contains(r.cuerpo, eP) || strings.Contains(r.cuerpo, eA) {
		t.Errorf("P lista: solo los suyos (%d)", r.codigo)
	}
	denegado(t, "P lee el expediente de A", p.pedir(http.MethodGet, "/api/expedientes/"+eA, nil), "Trámite privado de A")
	denegado(t, "P lee el documento de A", p.pedir(http.MethodGet, "/api/documentos/"+dA, nil), "documento-de-A")
	denegado(t, "P descarga el documento de A", p.pedir(http.MethodGet, "/api/documentos/"+dA+"/contenido", nil), "contenido de prueba")

	// 8. Sin autenticación: 401.
	for _, ruta := range []string{"/api/documentos/" + dA, "/api/documentos/" + dA + "/contenido", "/api/expedientes/" + eA, "/api/expedientes"} {
		if r := anon.pedir(http.MethodGet, ruta, nil); r.codigo != http.StatusUnauthorized {
			t.Errorf("sin sesión %s: %d", ruta, r.codigo)
		}
	}

	// 9. Personal autorizado (SECRETARIA: expedientes.view, documentos.view).
	for _, ruta := range []string{"/api/expedientes/" + eA, "/api/expedientes/" + eB, "/api/documentos/" + dA, "/api/documentos/" + dB + "/contenido"} {
		if r := s.pedir(http.MethodGet, ruta, nil); r.codigo != http.StatusOK {
			t.Errorf("SECRETARIA %s: %d", ruta, r.codigo)
		}
	}

	// 10. La identidad pendiente no concede permisos adicionales.
	permisos := func(c *cliente) string {
		r := c.pedir(http.MethodGet, "/api/auth/me/permissions", nil)
		if r.codigo != http.StatusOK {
			t.Fatalf("permisos: %d", r.codigo)
		}
		var v struct {
			Permissions []string `json:"permissions"`
		}
		_ = json.Unmarshal([]byte(r.cuerpo), &v)
		return strings.Join(v.Permissions, ",")
	}
	if pa, pp := permisos(a), permisos(p); pa == "" || pa != pp {
		t.Errorf("P debe tener exactamente los permisos de un SOLICITANTE")
	}
	for _, ruta := range []string{"/api/solicitantes/identidades", "/api/reportes/expedientes"} {
		if r := p.pedir(http.MethodGet, ruta, nil); r.codigo != http.StatusForbidden {
			t.Errorf("P en %s: %d", ruta, r.codigo)
		}
	}
}

// Un recurso ajeno no debe distinguirse de uno inexistente cuando la
// respuesta correcta es 404 (no revelar existencia).
func TestAislamiento_RecursoAjenoVsInexistente(t *testing.T) {
	base := entorno(t, "AISLAMIENTO_GATEWAY_URL")
	clave := entorno(t, "AISLAMIENTO_CLAVE")
	a := login(t, base, map[string]string{"dni": entorno(t, "AISLAMIENTO_DNI_A"), "password": clave})
	b := login(t, base, map[string]string{"dni": entorno(t, "AISLAMIENTO_DNI_B"), "password": clave})
	eB := crearExpediente(b, "Otro trámite de B")
	dB, _ := subirDocumento(b, eB, "otro-documento-de-B")
	const inexistente = "00000000-0000-4000-8000-0000000000ff"
	for _, par := range [][2]string{
		{"/api/expedientes/" + eB, "/api/expedientes/" + inexistente},
		{"/api/documentos/" + dB, "/api/documentos/" + inexistente},
		{"/api/documentos/" + dB + "/contenido", "/api/documentos/" + inexistente + "/contenido"},
		{"/api/expedientes/" + eB + "/documentos", "/api/expedientes/" + inexistente + "/documentos"},
	} {
		ajeno, nada := a.pedir(http.MethodGet, par[0], nil), a.pedir(http.MethodGet, par[1], nil)
		if ajeno.codigo != nada.codigo || ajeno.cuerpo != nada.cuerpo {
			t.Errorf("%s: ajeno %d vs inexistente %d (revela que el recurso existe)", par[0], ajeno.codigo, nada.codigo)
		}
	}
}
