package logic

import (
	"context"
	"strings"
	"testing"

	"auth/auth"
	"auth/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidarPassword(t *testing.T) {
	for pass, want := range map[string]codes.Code{
		"corta":                 codes.InvalidArgument,
		"12345678":              codes.OK,
		strings.Repeat("a", 72): codes.OK,
		strings.Repeat("a", 73): codes.InvalidArgument,
		strings.Repeat("ñ", 37): codes.InvalidArgument, // 74 bytes: el límite es en bytes, no en runas
	} {
		if got := status.Code(validarPassword(pass)); got != want {
			t.Errorf("validarPassword(len=%d) = %v, se esperaba %v", len(pass), got, want)
		}
	}
}

// Las validaciones de entrada de Register y Login ocurren antes de tocar la
// base de datos: responden InvalidArgument (400), nunca Internal (500).
func TestRegisterYLogin_EntradasInvalidasSon400(t *testing.T) {
	sc := &svc.ServiceContext{}
	nuevo := func(cambiar func(*auth.RegisterRequest)) *auth.RegisterRequest {
		r := &auth.RegisterRequest{Username: "usuario", Email: "u@example.com", Password: "12345678", Dni: "12345678"}
		cambiar(r)
		return r
	}

	for nombre, c := range map[string]struct {
		req  *auth.RegisterRequest
		want codes.Code
	}{
		"password > 72 bytes": {nuevo(func(r *auth.RegisterRequest) { r.Password = strings.Repeat("x", 100) }), codes.InvalidArgument},
		"sin DNI":             {nuevo(func(r *auth.RegisterRequest) { r.Dni = "" }), codes.InvalidArgument},
		"DNI con SQL":         {nuevo(func(r *auth.RegisterRequest) { r.Dni = "1' OR '1'='1" }), codes.InvalidArgument},
		"rol ADMIN público":   {nuevo(func(r *auth.RegisterRequest) { r.Role = "ADMIN" }), codes.PermissionDenied},
		"solicitud nula":      {nil, codes.InvalidArgument},
	} {
		if _, err := NewRegisterLogic(context.Background(), sc).Register(c.req); status.Code(err) != c.want {
			t.Errorf("Register %s: se esperaba %v, got %v", nombre, c.want, err)
		}
	}

	for nombre, req := range map[string]*auth.LoginRequest{
		"nulo":            nil,
		"sin password":    {Dni: "12345678"},
		"DNI corto":       {Dni: "123", Password: "x"},
		"DNI con SQL":     {Dni: "1' OR 1=1--", Password: "x"},
		"DNI con letras":  {Dni: "1234567a", Password: "x"},
		"DNI con espacio": {Dni: "1234 5678", Password: "x"},
	} {
		if _, err := NewLoginLogic(context.Background(), sc).Login(req); status.Code(err) != codes.InvalidArgument {
			t.Errorf("Login %s: se esperaba InvalidArgument, got %v", nombre, err)
		}
	}
}
