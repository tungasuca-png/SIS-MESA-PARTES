package authorization

import "testing"

func TestCanCreate(t *testing.T) {
	for _, role := range []string{"SOLICITANTE", "ADMIN", "DOCENTE", "AUXILIAR"} {
		if !CanCreate(role) {
			t.Errorf("expected %s to be able to create", role)
		}
	}
	if CanCreate("DESCONOCIDO") {
		t.Error("expected unknown role to be denied")
	}
}

func TestCanUpdate_OnlyInternal(t *testing.T) {
	if CanUpdate(RoleSolicitante) {
		t.Error("solicitante should not be able to update")
	}
	if !CanUpdate("SECRETARIA") {
		t.Error("internal role should be able to update")
	}
}

func TestCanChangeEstado_OnlyInternal(t *testing.T) {
	if CanChangeEstado(RoleSolicitante) {
		t.Error("solicitante should not be able to change estado")
	}
	if !CanChangeEstado("DIRECTOR") {
		t.Error("internal role should be able to change estado")
	}
}

func TestCanDelete_OnlyInternal(t *testing.T) {
	if CanDelete(RoleSolicitante) {
		t.Error("solicitante should not be able to delete")
	}
	if !CanDelete("SECRETARIA") {
		t.Error("internal role should be able to delete")
	}
}

func TestCanViewAll_OnlyInternal(t *testing.T) {
	if CanViewAll(RoleSolicitante) {
		t.Error("solicitante should not view all")
	}
	if !CanViewAll("ADMIN") {
		t.Error("admin should view all")
	}
}
