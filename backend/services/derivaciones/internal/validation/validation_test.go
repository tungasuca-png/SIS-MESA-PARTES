package validation

import "testing"

func TestIsValidArea(t *testing.T) {
	valid := []string{"SECRETARIA", "DIRECTOR", "SUBDIRECTOR", "DOCENTE", "AUXILIAR", "ADMIN"}
	for _, area := range valid {
		if !IsValidArea(area) {
			t.Fatalf("se esperaba que %q fuera un área válida", area)
		}
	}

	invalid := []string{"", "AREA_INVENTADA", "secretaria", "DIRECTORIO"}
	for _, area := range invalid {
		if IsValidArea(area) {
			t.Fatalf("se esperaba que %q NO fuera un área válida", area)
		}
	}
}
