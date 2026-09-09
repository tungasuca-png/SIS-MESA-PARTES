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

func TestCanViewAll_OnlySecretariaAndAdmin(t *testing.T) {
	if CanViewAll(RoleSolicitante) {
		t.Error("solicitante should not view all")
	}
	for _, role := range []string{"ADMIN", "SECRETARIA"} {
		if !CanViewAll(role) {
			t.Errorf("%s should view all", role)
		}
	}
	// El resto del personal interno solo ve lo que se le derivo a su area
	// (ver AreaDelRol) — no todo, aunque siga siendo IsInternal.
	for _, role := range []string{"DIRECTOR", "SUBDIRECTOR", "DOCENTE", "AUXILIAR"} {
		if CanViewAll(role) {
			t.Errorf("%s should NOT view all, only its own area", role)
		}
	}
}

func TestAreaDelRol(t *testing.T) {
	for _, role := range []string{"DIRECTOR", "SUBDIRECTOR", "DOCENTE", "AUXILIAR"} {
		if AreaDelRol(role) != role {
			t.Errorf("expected AreaDelRol(%s) == %s, got %q", role, role, AreaDelRol(role))
		}
	}
	for _, role := range []string{"ADMIN", "SECRETARIA", RoleSolicitante} {
		if AreaDelRol(role) != "" {
			t.Errorf("expected AreaDelRol(%s) == \"\", got %q", role, AreaDelRol(role))
		}
	}
}

func TestCanUpdateArea_OnlyInternal(t *testing.T) {
	if CanUpdateArea(RoleSolicitante) {
		t.Error("solicitante should not be able to update area")
	}
	if !CanUpdateArea("SECRETARIA") {
		t.Error("internal role should be able to update area")
	}
}
