package validation

import "testing"

func TestIsValidUUID(t *testing.T) {
	cases := map[string]bool{
		"550e8400-e29b-41d4-a716-446655440000": true,
		"not-a-uuid":                           false,
		"":                                     false,
	}
	for value, want := range cases {
		if got := IsValidUUID(value); got != want {
			t.Errorf("IsValidUUID(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestIsValidTipoDocumento(t *testing.T) {
	cases := map[string]bool{
		"ADJUNTO":  true,
		"PROVEIDO": true,
		"ACTA":     true,
		"INFORME":  true,
		"FORMATO":  true,
		"OTRO":     true,
		"INVALIDO": false,
		"":         false,
	}
	for value, want := range cases {
		if got := IsValidTipoDocumento(value); got != want {
			t.Errorf("IsValidTipoDocumento(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestIsValidExtension(t *testing.T) {
	cases := map[string]bool{
		"pdf":  true,
		"PDF":  true,
		"jpg":  true,
		"png":  true,
		"docx": true,
		"exe":  false,
		"sh":   false,
		"":     false,
	}
	for value, want := range cases {
		if got := IsValidExtension(value); got != want {
			t.Errorf("IsValidExtension(%q) = %v, want %v", value, got, want)
		}
	}
}
