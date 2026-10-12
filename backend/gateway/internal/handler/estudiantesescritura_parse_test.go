package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gateway/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

// FASE E-05: PATCH /api/estudiantes/:id es una actualización PARCIAL. El
// Gateway debe distinguir un campo AUSENTE (nil: no cambia) de uno presente
// vacío ("" : se deja en NULL), porque Estudiantes Service aplica esa regla.
func TestUpdateEstudianteRequest_AusenteVsVacio(t *testing.T) {
	body := `{"expected_version": 3, "nombres": "Ana", "sexo": ""}`
	r := httptest.NewRequest(http.MethodPatch, "/api/estudiantes/x", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = pathvar.WithVars(r, map[string]string{"id": "00000000-0000-4000-8000-000000000001"})

	var req types.UpdateEstudianteRequest
	if err := httpx.Parse(r, &req); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Id != "00000000-0000-4000-8000-000000000001" || req.ExpectedVersion != 3 {
		t.Fatalf("id/version: %+v", req)
	}
	if req.Nombres == nil || *req.Nombres != "Ana" {
		t.Fatalf("nombres presente: %v", req.Nombres)
	}
	if req.Sexo == nil || *req.Sexo != "" {
		t.Fatalf("sexo presente vacío debe llegar como \"\", no nil: %v", req.Sexo)
	}
	for nombre, v := range map[string]*string{
		"apellidos": req.Apellidos, "numero_documento": req.NumeroDocumento,
		"tipo_documento": req.TipoDocumento, "fecha_nacimiento": req.FechaNacimiento, "codigo_siagie": req.CodigoSiagie,
	} {
		if v != nil {
			t.Fatalf("%s ausente debe llegar como nil", nombre)
		}
	}
}

func TestCreateEstudianteRequest_CamposOpcionales(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/estudiantes", strings.NewReader(`{"codigo": "C1", "nombres": "Ana", "apellidos": "Soto"}`))
	r.Header.Set("Content-Type", "application/json")
	var req types.CreateEstudianteRequest
	if err := httpx.Parse(r, &req); err != nil {
		t.Fatalf("un alta mínima no debe fallar en el Gateway (valida el servicio): %v", err)
	}
	if req.Codigo != "C1" || req.TipoDocumento != "" {
		t.Fatalf("alta mínima: %+v", req)
	}
}

// FASE E-08: PATCH /api/estudiantes/:id/personal — mismo criterio parcial.
func TestUpdateEstudiantePersonalRequest_AusenteVsVacio(t *testing.T) {
	r := httptest.NewRequest(http.MethodPatch, "/api/estudiantes/x/personal",
		strings.NewReader(`{"expected_version": 5, "sexo": "F", "segunda_lengua": ""}`))
	r.Header.Set("Content-Type", "application/json")
	r = pathvar.WithVars(r, map[string]string{"id": "00000000-0000-4000-8000-000000000002"})

	var req types.UpdateEstudiantePersonalRequest
	if err := httpx.Parse(r, &req); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.Id != "00000000-0000-4000-8000-000000000002" || req.ExpectedVersion != 5 {
		t.Fatalf("id/version: %+v", req)
	}
	if req.Sexo == nil || *req.Sexo != "F" || req.SegundaLengua == nil || *req.SegundaLengua != "" {
		t.Fatalf("presentes: sexo=%v segunda_lengua=%v", req.Sexo, req.SegundaLengua)
	}
	for nombre, v := range map[string]*string{"fecha_nacimiento": req.FechaNacimiento, "pais_nacimiento": req.PaisNacimiento,
		"ubigeo_nacimiento": req.UbigeoNacimiento, "lengua_materna": req.LenguaMaterna} {
		if v != nil {
			t.Fatalf("%s ausente debe llegar como nil", nombre)
		}
	}
}

// El DTO de lectura personal expone exactamente los campos del dominio:
// ni foto_ref, ni identidad, ni estado, ni datos académicos.
func TestEstudiantePersonalDTO_SoloCamposPersonales(t *testing.T) {
	b, err := json.Marshal(types.EstudiantePersonalDTO{})
	if err != nil {
		t.Fatal(err)
	}
	var campos map[string]any
	if err := json.Unmarshal(b, &campos); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"id": true, "version": true, "fecha_nacimiento": true, "sexo": true, "pais_nacimiento": true,
		"ubigeo_nacimiento": true, "lengua_materna": true, "segunda_lengua": true,
		// Modelo maestro: domicilio vigente, contacto y edad calculada.
		"domicilio_direccion": true, "domicilio_departamento": true, "domicilio_provincia": true,
		"domicilio_distrito": true, "domicilio_ubigeo": true, "correo": true, "telefono": true, "edad": true}
	if len(campos) != len(want) {
		t.Fatalf("campos del DTO personal: %v", campos)
	}
	for c := range campos {
		if !want[c] {
			t.Fatalf("campo no permitido en el DTO personal: %s", c)
		}
	}
}

// Modelo maestro: vinculación de familiar (persona opcional como objeto) y
// desvinculación (dos variables de ruta).
func TestVincularYDesvincularFamiliar_Parse(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/estudiantes/x/familiares",
		strings.NewReader(`{"parentesco": "PADRE", "es_apoderado": true, "persona": {"tipo_documento": "DNI", "numero_documento": "12345678", "nombres": "Juan"}}`))
	r.Header.Set("Content-Type", "application/json")
	r = pathvar.WithVars(r, map[string]string{"id": "e1"})
	var v types.VincularFamiliarRequest
	if err := httpx.Parse(r, &v); err != nil {
		t.Fatalf("parse vincular: %v", err)
	}
	if v.Id != "e1" || v.Parentesco != "PADRE" || !v.EsApoderado || v.Persona == nil || v.Persona.Nombres != "Juan" || v.PersonaId != "" {
		t.Fatalf("vincular: %+v", v)
	}

	r = httptest.NewRequest(http.MethodPatch, "/api/estudiantes/x/familiares/y/desvincular", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r = pathvar.WithVars(r, map[string]string{"id": "e1", "relacion_id": "r1"})
	var d types.DesvincularFamiliarRequest
	if err := httpx.Parse(r, &d); err != nil {
		t.Fatalf("parse desvincular: %v", err)
	}
	if d.Id != "e1" || d.RelacionId != "r1" || d.Hasta != "" {
		t.Fatalf("desvincular: %+v", d)
	}
}

// La resolución exacta recibe el documento en el CUERPO, nunca en la URL.
func TestResolveEstudiante_DocumentoEnCuerpo(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/estudiantes/resolver",
		strings.NewReader(`{"tipo_documento": "DNI", "numero_documento": "12345678"}`))
	r.Header.Set("Content-Type", "application/json")
	var req types.ResolveEstudianteRequest
	if err := httpx.Parse(r, &req); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.TipoDocumento != "DNI" || req.NumeroDocumento != "12345678" || req.Codigo != "" {
		t.Fatalf("resolver: %+v", req)
	}
}
