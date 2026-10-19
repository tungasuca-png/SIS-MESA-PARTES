package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"github.com/zeromicro/go-zero/rest/router"
)

// Etapa 5: las rutas estáticas nuevas (periodos, personas, matriculas)
// comparten nivel con /api/estudiantes/:id. Se verifica con el árbol de
// go-zero que cada URL llega a la ruta prevista y no a otra.
func TestEtapa5_RutasResuelvenAlHandlerCorrecto(t *testing.T) {
	logx.Disable()
	// Las rutas reales de routes.go (el servidor no se inicia).
	server := rest.MustNewServer(rest.RestConf{ServiceConf: service.ServiceConf{Name: "test"}, Host: "127.0.0.1", Port: 1})
	RegisterHandlers(server, &svc.ServiceContext{})
	routes := server.Routes()

	real := router.NewRouter()
	for _, rt := range routes {
		pattern := rt.Method + " " + rt.Path
		if err := real.Handle(rt.Method, rt.Path, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(pattern))
		})); err != nil {
			t.Fatalf("ruta duplicada o inválida %s: %v", pattern, err)
		}
	}

	const id = "00000000-0000-4000-8000-000000000001"
	for _, c := range []struct{ method, url, want string }{
		{"GET", "/api/estudiantes/periodos", "GET /api/estudiantes/periodos"},
		{"POST", "/api/estudiantes/periodos", "POST /api/estudiantes/periodos"},
		{"PATCH", "/api/estudiantes/periodos/" + id + "/estado", "PATCH /api/estudiantes/periodos/:periodo_id/estado"},
		{"GET", "/api/estudiantes/periodos/" + id + "/secciones", "GET /api/estudiantes/periodos/:periodo_id/secciones"},
		{"POST", "/api/estudiantes/periodos/" + id + "/secciones", "POST /api/estudiantes/periodos/:periodo_id/secciones"},
		{"GET", "/api/estudiantes/periodos/" + id + "/matriculas", "GET /api/estudiantes/periodos/:periodo_id/matriculas"},
		{"GET", "/api/estudiantes/" + id + "/matriculas", "GET /api/estudiantes/:id/matriculas"},
		{"POST", "/api/estudiantes/" + id + "/matriculas", "POST /api/estudiantes/:id/matriculas"},
		{"PATCH", "/api/estudiantes/matriculas/" + id + "/seccion", "PATCH /api/estudiantes/matriculas/:matricula_id/seccion"},
		{"PATCH", "/api/estudiantes/matriculas/" + id + "/retiro", "PATCH /api/estudiantes/matriculas/:matricula_id/retiro"},
		{"PATCH", "/api/estudiantes/matriculas/" + id + "/reingreso", "PATCH /api/estudiantes/matriculas/:matricula_id/reingreso"},
		{"PATCH", "/api/estudiantes/matriculas/" + id + "/conclusion", "PATCH /api/estudiantes/matriculas/:matricula_id/conclusion"},
		{"GET", "/api/estudiantes/personas", "GET /api/estudiantes/personas"},
		{"POST", "/api/estudiantes/personas", "POST /api/estudiantes/personas"},
		{"GET", "/api/estudiantes/personas/" + id, "GET /api/estudiantes/personas/:persona_id"},
		{"PATCH", "/api/estudiantes/personas/" + id, "PATCH /api/estudiantes/personas/:persona_id"},
		{"PATCH", "/api/estudiantes/" + id + "/familiares/" + id, "PATCH /api/estudiantes/:id/familiares/:relacion_id"},
		{"PATCH", "/api/estudiantes/" + id + "/familiares/" + id + "/desvincular", "PATCH /api/estudiantes/:id/familiares/:relacion_id/desvincular"},
		// Las rutas anteriores siguen resolviendo igual.
		{"GET", "/api/estudiantes/" + id, "GET /api/estudiantes/:id"},
		{"PATCH", "/api/estudiantes/" + id, "PATCH /api/estudiantes/:id"},
		{"PATCH", "/api/estudiantes/" + id + "/estado", "PATCH /api/estudiantes/:id/estado"},
		{"GET", "/api/estudiantes/duplicados", "GET /api/estudiantes/duplicados"},
		{"POST", "/api/estudiantes/resolver", "POST /api/estudiantes/resolver"},
	} {
		w := httptest.NewRecorder()
		real.ServeHTTP(w, httptest.NewRequest(c.method, c.url, nil))
		if got := w.Body.String(); got != c.want {
			t.Errorf("%s %s -> %q (código %d), se esperaba %q", c.method, c.url, got, w.Code, c.want)
		}
	}
	// Sin DELETE: nada del dominio Estudiantes se borra por la API.
	for _, rt := range routes {
		if rt.Method == http.MethodDelete && strings.HasPrefix(rt.Path, "/api/estudiantes") {
			t.Errorf("no debe existir DELETE en Estudiantes: %s", rt.Path)
		}
	}
}

// PATCH /api/estudiantes/personas/:persona_id: ausente (nil) frente a vacío ("").
func TestUpdatePersonaRequest_AusenteVsVacio(t *testing.T) {
	r := httptest.NewRequest(http.MethodPatch, "/api/estudiantes/personas/x", strings.NewReader(`{"expected_version": 2, "apellido_materno": ""}`))
	r.Header.Set("Content-Type", "application/json")
	r = pathvar.WithVars(r, map[string]string{"persona_id": "00000000-0000-4000-8000-000000000002"})
	var req types.UpdatePersonaRequest
	if err := httpx.Parse(r, &req); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.PersonaId == "" || req.ExpectedVersion != 2 || req.ApellidoMaterno == nil || *req.ApellidoMaterno != "" {
		t.Fatalf("presente vacío: %+v", req)
	}
	if req.Nombres != nil || req.NumeroDocumento != nil || req.TipoDocumento != nil || req.ApellidoPaterno != nil {
		t.Fatalf("los ausentes deben llegar como nil: %+v", req)
	}
}

// es_apoderado es obligatorio: un cuerpo sin él no debe interpretarse como
// "quitar apoderado".
func TestActualizarFamiliarRequest_EsApoderadoObligatorio(t *testing.T) {
	r := httptest.NewRequest(http.MethodPatch, "/x", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r = pathvar.WithVars(r, map[string]string{"id": "a", "relacion_id": "b"})
	var req types.ActualizarFamiliarRequest
	if err := httpx.Parse(r, &req); err == nil {
		t.Fatal("sin es_apoderado debe rechazarse en el Gateway")
	}
}

// vacantes ausente (sin límite) frente a 0 (sección sin vacantes).
func TestCreateSeccionRequest_VacantesAusenteVsCero(t *testing.T) {
	parse := func(body string) types.CreateSeccionRequest {
		r := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r = pathvar.WithVars(r, map[string]string{"periodo_id": "p"})
		var req types.CreateSeccionRequest
		if err := httpx.Parse(r, &req); err != nil {
			t.Fatalf("parse: %v", err)
		}
		return req
	}
	if req := parse(`{"nivel": "PRIMARIA", "grado": "1", "seccion": "A"}`); req.Vacantes != nil {
		t.Fatalf("vacantes ausente debe ser nil: %v", *req.Vacantes)
	}
	if req := parse(`{"nivel": "PRIMARIA", "grado": "1", "seccion": "A", "vacantes": 0}`); req.Vacantes == nil || *req.Vacantes != 0 {
		t.Fatal("vacantes 0 debe conservarse")
	}
}
