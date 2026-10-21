package authorization

import "testing"

func TestCanUpload(t *testing.T) {
	if !CanUpload("SOLICITANTE", "ADJUNTO") {
		t.Error("solicitante debe poder subir un ADJUNTO")
	}
	if CanUpload("SOLICITANTE", "PROVEIDO") {
		t.Error("solicitante no debe poder subir un PROVEIDO")
	}
	if !CanUpload("SECRETARIA", "PROVEIDO") {
		t.Error("personal interno debe poder subir cualquier tipo")
	}
	if !CanUpload("SECRETARIA", "ADJUNTO") {
		t.Error("personal interno debe poder subir ADJUNTO tambien")
	}
}

func TestCanDelete(t *testing.T) {
	if CanDelete("SOLICITANTE") {
		t.Error("solicitante no debe poder eliminar documentos")
	}
	if !CanDelete("ADMIN") {
		t.Error("admin debe poder eliminar documentos")
	}
	if !CanDelete("DOCENTE") {
		t.Error("personal interno debe poder eliminar documentos")
	}
}

func TestCanViewAll(t *testing.T) {
	if CanViewAll("SOLICITANTE") {
		t.Error("solicitante no debe poder ver todos los documentos sin filtro")
	}
	if !CanViewAll("ADMIN") {
		t.Error("admin debe poder ver todos los documentos")
	}
	if !CanViewAll("AUXILIAR") {
		t.Error("personal interno debe poder ver todos los documentos")
	}
}

// Seguridad (BOLA): "documentos.view" no da acceso a documentos de
// expedientes fuera del alcance del usuario. Solo ADMIN y SECRETARIA (alcance
// global en Expedientes) omiten la verificación contra Expedientes.
func TestDebeVerificarExpediente(t *testing.T) {
	for _, c := range []struct {
		hasViewAll bool
		role       string
		verificar  bool
	}{
		{true, "ADMIN", false},
		{true, "SECRETARIA", false},
		{true, "DIRECTOR", true},
		{true, "SUBDIRECTOR", true},
		{true, "DOCENTE", true},
		{true, "AUXILIAR", true},
		{true, "PSICOLOGO", true},
		{true, "SOLICITANTE", true},
		{true, "", true},
		{true, "admin", true}, // el rol es exacto: no se normaliza un claim alterado
		{false, "ADMIN", true},
		{false, "SOLICITANTE", true},
	} {
		if got := DebeVerificarExpediente(c.hasViewAll, c.role); got != c.verificar {
			t.Errorf("DebeVerificarExpediente(%v, %q) = %v, se esperaba %v", c.hasViewAll, c.role, got, c.verificar)
		}
	}
}
