package logic

import (
	"os"
	"testing"

	"auth/auth"
	"auth/internal/config"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Base con las migraciones 000-023 y SIN 024/025 (estado de una instalación
// que todavía no aplicó el registro manual ni la verificación de identidad).
// El servicio está cableado como en producción (repositorios de registro e
// identidad presentes). Solo corre con AUTH_DB_SIN_024=1 contra un PostgreSQL
// desechable (scratchpad/auth-sin-024.sh).
func TestMigracionesPendientes_LoginSigueFuncionando(t *testing.T) {
	if os.Getenv("AUTH_DB_SIN_024") != "1" {
		t.Skip("solo con AUTH_DB_SIN_024=1 (base sin las migraciones 024 y 025)")
	}
	e := nuevoEntornoRegistro(t, config.RegistroCorreoConfig{})
	dniAdmin, dniSol := e.dniNuevo(), e.dniNuevo()
	correoSol := e.unico("sol") + "@correo.example"
	admin := e.crearUsuario(dniAdmin, e.unico("adm")+"@colegio.example", "ClaveAdmin123", "ADMIN", true)
	sol := e.crearUsuario(dniSol, correoSol, "ClaveSol12345", "SOLICITANTE", true)

	// Login institucional y de solicitante por DNI: funcionan (antes, 500).
	if ses, err := e.login(dniAdmin, "ClaveAdmin123"); err != nil || ses.UserId != admin || ses.Role != "ADMIN" {
		t.Fatalf("login ADMIN por DNI: %v", err)
	}
	if ses, err := e.login(dniSol, "ClaveSol12345"); err != nil || ses.UserId != sol {
		t.Fatalf("login SOLICITANTE por DNI: %v", err)
	}
	// Errores esperados, nunca Internal.
	if _, err := e.login(dniAdmin, "clave-equivocada"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("clave incorrecta: %v", err)
	}
	if _, err := e.login(e.dniNuevo(), "x"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("DNI inexistente: %v", err)
	}
	if ses, err := e.loginCorreo(correoSol, "ClaveSol12345"); err != nil || ses.UserId != sol {
		t.Fatalf("login por correo de un solicitante: %v", err)
	}
	if _, err := e.loginCorreo(e.unico("nadie")+"@correo.example", "x"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("correo inexistente: %v", err)
	}

	// Registro público: "no disponible", sin crear nada (el DNI nunca se reserva).
	if _, err := e.registrar(e.datos()); status.Code(err) != codes.Unavailable {
		t.Fatalf("registro manual sin 024: %v", err)
	}
	dni := e.dniNuevo()
	email := e.unico("g") + "@gmail.example"
	cb, err := e.callback(e.unico("sub-"), email, true, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewGoogleRegisterLogic(e.ctx, e.sc).GoogleRegister(&auth.GoogleRegisterRequest{LoginCode: cb.LoginCode, Dni: dni})
	if status.Code(err) != codes.Unavailable || clave(err) != "registro_no_disponible" {
		t.Fatalf("registro con Google sin 025: %v", err)
	}
	if e.contar(`SELECT count(*) FROM usuarios WHERE email = $1 OR dni = $2`, email, dni) != 0 {
		t.Fatal("sin la 025 no se crea ninguna cuenta ni se reserva el DNI")
	}
}
