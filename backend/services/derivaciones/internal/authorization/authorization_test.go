package authorization

import "testing"

func TestCanCreate_OnlyInternal(t *testing.T) {
	if CanCreate(RoleSolicitante) {
		t.Error("solicitante should not be able to create a derivacion")
	}
	if !CanCreate("SECRETARIA") {
		t.Error("internal role should be able to create a derivacion")
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
