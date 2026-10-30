package authorization

import "testing"

func TestAlcanceGlobalExpedientes(t *testing.T) {
	for role, want := range map[string]bool{
		"ADMIN": true, "SECRETARIA": true,
		"DIRECTOR": false, "SUBDIRECTOR": false, "DOCENTE": false, "AUXILIAR": false,
		"PSICOLOGO": false, "SOLICITANTE": false, "": false, "admin": false,
	} {
		if got := AlcanceGlobalExpedientes(role); got != want {
			t.Errorf("AlcanceGlobalExpedientes(%q) = %v, se esperaba %v", role, got, want)
		}
	}
}
