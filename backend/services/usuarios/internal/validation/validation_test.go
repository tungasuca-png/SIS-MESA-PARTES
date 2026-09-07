package validation

import "testing"

func TestIsValidUUID(t *testing.T) {
	cases := map[string]bool{
		"550e8400-e29b-41d4-a716-446655440000": true,
		"42578A85-C958-4316-8E5C-A1751657554D": true,
		" 550e8400-e29b-41d4-a716-446655440000 ": true,
		"550e8400-e29b-41d4-a716-44665544000":  false,
		"no-es-un-uuid":                        false,
		"":                                     false,
		"550e8400e29b41d4a716446655440000":     false,
		"zzze8400-e29b-41d4-a716-446655440000": false,
	}

	for value, want := range cases {
		if got := IsValidUUID(value); got != want {
			t.Errorf("IsValidUUID(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestIsValidTipoUsuario(t *testing.T) {
	cases := map[string]bool{
		"ADMIN":       true,
		"DIRECTOR":    true,
		"SUBDIRECTOR": true,
		"SECRETARIA":  true,
		"DOCENTE":     true,
		"AUXILIAR":    true,
		"SOLICITANTE": true,
		"INVENTADO":   false,
		"admin":       false,
		"":            false,
	}

	for value, want := range cases {
		if got := IsValidTipoUsuario(value); got != want {
			t.Errorf("IsValidTipoUsuario(%q) = %v, want %v", value, got, want)
		}
	}
}
