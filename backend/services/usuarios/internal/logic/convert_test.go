package logic

import (
	"testing"
	"time"

	"usuarios/internal/repository"
)

func testUsuario() *repository.Usuario {
	return &repository.Usuario{
		ID:          "550e8400-e29b-41d4-a716-446655440000",
		Nombres:     "Juan Carlos",
		Apellidos:   "Pérez Ramos",
		DNI:         "12345678",
		Telefono:    "999888777",
		Correo:      "juan@test.local",
		Direccion:   "Av. Siempre Viva 123",
		TipoUsuario: "SOLICITANTE",
		Activo:      true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func TestToProto_IncluyeDatosDePerfil(t *testing.T) {
	proto := toProto(testUsuario())
	if proto.Nombres != "Juan Carlos" || proto.Apellidos != "Pérez Ramos" {
		t.Fatalf("unexpected names: %+v", proto)
	}
	if proto.Dni != "12345678" || proto.Telefono != "999888777" {
		t.Fatalf("expected full profile data, got %+v", proto)
	}
	if proto.CreatedAt == "" || proto.UpdatedAt == "" {
		t.Fatal("expected timestamps to be formatted")
	}
}

func TestToProtoBasic_ArmaNombreCompleto(t *testing.T) {
	basic := toProtoBasic(testUsuario(), false)
	if basic.NombreCompleto != "Juan Carlos Pérez Ramos" {
		t.Fatalf("unexpected nombre_completo: %q", basic.NombreCompleto)
	}
	if basic.TipoUsuario != "SOLICITANTE" {
		t.Fatalf("unexpected tipo_usuario: %q", basic.TipoUsuario)
	}
}

func TestToProtoBasic_OcultaDniSinAutorizacion(t *testing.T) {
	basic := toProtoBasic(testUsuario(), false)
	if basic.Dni != "" {
		t.Fatalf("expected dni to be hidden, got %q", basic.Dni)
	}
}

func TestToProtoBasic_IncluyeDniConAutorizacion(t *testing.T) {
	basic := toProtoBasic(testUsuario(), true)
	if basic.Dni != "12345678" {
		t.Fatalf("expected dni to be included, got %q", basic.Dni)
	}
}

func TestToProtoBasic_NoExponeDatosDeContacto(t *testing.T) {
	// UsuarioBasic no debe tener forma de transportar teléfono/correo/dirección.
	basic := toProtoBasic(testUsuario(), true)
	if basic.String() == "" {
		t.Fatal("unexpected empty message")
	}
	for _, sensible := range []string{"999888777", "juan@test.local", "Av. Siempre Viva 123"} {
		if contains(basic.String(), sensible) {
			t.Fatalf("basic projection leaked sensitive field: %s", sensible)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && stringIndex(haystack, needle) >= 0
}

func stringIndex(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestToProto_NilSeguro(t *testing.T) {
	if toProto(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
	if toProtoBasic(nil, true) != nil {
		t.Fatal("expected nil for nil input")
	}
}
