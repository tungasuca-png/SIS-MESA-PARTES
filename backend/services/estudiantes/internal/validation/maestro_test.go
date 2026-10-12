package validation

import (
	"testing"
	"time"
)

// Pruebas unitarias (sin base de datos) de las reglas del modelo maestro.

func fecha(t *testing.T, s string) time.Time {
	t.Helper()
	f, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func ptr(s string) *string { return &s }

func TestEdad_CalculadaAFechaDeReferencia(t *testing.T) {
	casos := []struct {
		nac, ref string
		edad     int
		ok       bool
	}{
		{"2013-07-21", "2026-07-20", 12, true}, // un día antes del cumpleaños
		{"2013-07-21", "2026-07-21", 13, true}, // el día del cumpleaños
		{"2013-07-21", "2013-07-21", 0, true},
		{"2012-02-29", "2013-02-28", 0, true}, // nacido un 29 de febrero
		{"2012-02-29", "2013-03-01", 1, true},
		{"2012-02-29", "2016-02-29", 4, true},
		{"2013-07-21", "2013-07-20", 0, false}, // referencia anterior: sin edad
	}
	for _, c := range casos {
		edad, ok := Edad(fecha(t, c.nac), fecha(t, c.ref))
		if edad != c.edad || ok != c.ok {
			t.Errorf("Edad(%s, %s) = %d,%v; se esperaba %d,%v", c.nac, c.ref, edad, ok, c.edad, c.ok)
		}
	}
}

func TestDocumento_PorTipo(t *testing.T) {
	validos := []struct{ tipo, num string }{{TipoDNI, "12345678"}, {TipoCE, "AB1234"}, {TipoPasaporte, "X12345678901"}}
	for _, c := range validos {
		if err := Documento(c.tipo, ptr(c.num)); err != nil {
			t.Errorf("%s %s debía ser válido: %v", c.tipo, c.num, err)
		}
	}
	if err := Documento(TipoSinDocumento, nil); err != nil {
		t.Errorf("SIN_DOCUMENTO sin número debía ser válido: %v", err)
	}
	invalidos := []struct {
		tipo string
		num  *string
	}{
		{TipoDNI, ptr("1234567")}, {TipoDNI, ptr("1234567A")}, {TipoDNI, nil},
		{TipoCE, ptr("ab")}, {TipoCE, nil}, {TipoPasaporte, ptr("AB-123")},
		{TipoSinDocumento, ptr("12345678")}, {"OTRO", ptr("12345678")},
	}
	for _, c := range invalidos {
		err := Documento(c.tipo, c.num)
		if e, ok := err.(*Error); !ok || e.Code != CodeDocumento {
			t.Errorf("%s %v debía rechazarse con invalid_document, se obtuvo %v", c.tipo, c.num, err)
		}
	}
}

func TestDomicilioYContacto(t *testing.T) {
	if err := Domicilio(ptr("Jr. Los Andes 123"), ptr("Cusco"), ptr("Canchis"), ptr("Tinta"), ptr("080810")); err != nil {
		t.Fatalf("domicilio válido: %v", err)
	}
	if err := Domicilio(nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("domicilio ausente es válido: %v", err)
	}
	if Domicilio(nil, nil, nil, nil, ptr("8081")) == nil {
		t.Error("ubigeo de 4 dígitos debía rechazarse")
	}
	if Domicilio(ptr(string(make([]byte, MaxDireccion+1))), nil, nil, nil, nil) == nil {
		t.Error("dirección demasiado larga debía rechazarse")
	}
	for _, ok := range []string{"familia@correo.pe", "a.b+c@dominio.edu.pe"} {
		if err := Contacto(ptr(ok), nil); err != nil {
			t.Errorf("correo %q debía ser válido: %v", ok, err)
		}
	}
	for _, malo := range []string{"sin-arroba", "a@b", "a b@c.pe"} {
		if Contacto(ptr(malo), nil) == nil {
			t.Errorf("correo %q debía rechazarse", malo)
		}
	}
	for _, ok := range []string{"984123456", "+51 984 123 456", "084-123456"} {
		if err := Contacto(nil, ptr(ok)); err != nil {
			t.Errorf("teléfono %q debía ser válido: %v", ok, err)
		}
	}
	for _, malo := range []string{"12345", "abc123456", "+51 984 123 456 789 123"} {
		if Contacto(nil, ptr(malo)) == nil {
			t.Errorf("teléfono %q debía rechazarse", malo)
		}
	}
}

func TestNormalizacionYParentesco(t *testing.T) {
	if got := CleanText("  DE   LA CRUZ  "); got != "DE LA CRUZ" {
		t.Errorf("CleanText conserva mayúsculas y apellidos compuestos: %q", got)
	}
	if Optional("") != nil {
		t.Error("vacío debe ser NULL")
	}
	for _, ok := range []string{"PADRE", "TUTOR_LEGAL"} {
		if err := Parentesco(ok); err != nil {
			t.Errorf("parentesco %q: %v", ok, err)
		}
	}
	for _, malo := range []string{"", "padre", "P1"} {
		if Parentesco(malo) == nil {
			t.Errorf("parentesco %q debía rechazarse", malo)
		}
	}
}
