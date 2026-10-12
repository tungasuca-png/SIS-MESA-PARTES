package authorization

import "testing"

const (
	usuarioA = "550e8400-e29b-41d4-a716-446655440000"
	usuarioB = "660e8400-e29b-41d4-a716-446655440001"
)

func TestCanViewFullProfile_PersonalInterno(t *testing.T) {
	for _, role := range []string{"ADMIN", "DIRECTOR", "SUBDIRECTOR", "SECRETARIA", "DOCENTE", "AUXILIAR"} {
		if !CanViewFullProfile(role, usuarioA, usuarioB) {
			t.Errorf("expected %s to view another user's full profile", role)
		}
	}
}

func TestCanViewFullProfile_SolicitanteSoloElPropio(t *testing.T) {
	if CanViewFullProfile("SOLICITANTE", usuarioA, usuarioB) {
		t.Error("solicitante should not view another user's full profile")
	}
	if !CanViewFullProfile("SOLICITANTE", usuarioA, usuarioA) {
		t.Error("solicitante should view its own full profile")
	}
}

func TestCanViewBasic_CualquierAutenticado(t *testing.T) {
	if !CanViewBasic() {
		t.Error("any authenticated user should resolve basic user data")
	}
}
