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
