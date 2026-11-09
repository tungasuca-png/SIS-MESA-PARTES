package logic

import (
	"context"
	"testing"

	"auth/internal/repository"
	"auth/internal/security"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
)

const secretoDerivacionesPrueba = "secreto-exclusivo-derivaciones-prueba-auth-0123"

func emisoresPrueba(t *testing.T, conDerivaciones bool) security.EmisoresServicio {
	t.Helper()
	e := security.EmisoresServicio{security.AudienciaEstudiantes: nuevoEmisor(t)}
	if conDerivaciones {
		d, err := security.NewServiceTokenIssuer(secretoDerivacionesPrueba, secretoUsuariosPrueba, "", security.AudienciaDerivaciones, 0)
		if err != nil || d == nil {
			t.Fatalf("emisor derivaciones: %v", err)
		}
		e[security.AudienciaDerivaciones] = d
	}
	return e
}

func clienteConAudiencia(t *testing.T, audiencia string, permisos ...string) (*storeFalso, string, string) {
	store, clientID, secret := clientePrueba(t, repository.ServiceClientActivo, permisos...)
	store.clientes[clientID].Audiencia = audiencia
	return store, clientID, secret
}

func audienciaYFirma(t *testing.T, token, secreto string) (string, bool) {
	t.Helper()
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return []byte(secreto), nil })
	aud, _ := claims.GetAudience()
	if len(aud) != 1 {
		return "", err == nil
	}
	return aud[0], err == nil
}

// Un cliente de derivaciones recibe un token firmado con el secreto de
// derivaciones, audiencia derivaciones y SOLO derivaciones.proyectar.
func TestAudiencia_DerivacionesFirmaConSuSecreto(t *testing.T) {
	store, clientID, secret := clienteConAudiencia(t, security.AudienciaDerivaciones, "derivaciones.proyectar", "estudiantes.view")
	resp, err := emitirTokenServicio(context.Background(), store, emisoresPrueba(t, true), clientID, secret)
	if err != nil {
		t.Fatalf("se esperaba token: %v", err)
	}
	if resp.Scope != "derivaciones.proyectar" {
		t.Fatalf("scope de derivaciones debe ser solo derivaciones.proyectar: %q", resp.Scope)
	}
	if aud, ok := audienciaYFirma(t, resp.AccessToken, secretoDerivacionesPrueba); !ok || aud != "derivaciones" {
		t.Fatalf("token de derivaciones: aud=%q firma válida con su secreto=%v", aud, ok)
	}
	if _, ok := audienciaYFirma(t, resp.AccessToken, secretoServicioPrueba); ok {
		t.Fatal("el token de derivaciones NO debe validar con el secreto de estudiantes")
	}
}

// Los clientes existentes (sin audiencia o estudiantes) siguen igual que antes.
func TestAudiencia_EstudiantesSinCambios(t *testing.T) {
	for _, aud := range []string{"", security.AudienciaEstudiantes} {
		store, clientID, secret := clienteConAudiencia(t, aud, "estudiantes.basic.view", "derivaciones.proyectar")
		resp, err := emitirTokenServicio(context.Background(), store, emisoresPrueba(t, true), clientID, secret)
		if err != nil {
			t.Fatalf("audiencia %q: %v", aud, err)
		}
		if resp.Scope != "estudiantes.basic.view" {
			t.Fatalf("audiencia %q: un cliente de estudiantes nunca recibe derivaciones.proyectar: %q", aud, resp.Scope)
		}
		if a, ok := audienciaYFirma(t, resp.AccessToken, secretoServicioPrueba); !ok || a != "estudiantes" {
			t.Fatalf("audiencia %q: aud=%q firma=%v", aud, a, ok)
		}
	}
}

// Sin secreto de derivaciones, esa audiencia queda deshabilitada (Unavailable)
// y la de estudiantes sigue funcionando.
func TestAudiencia_DerivacionesNoHabilitada(t *testing.T) {
	store, clientID, secret := clienteConAudiencia(t, security.AudienciaDerivaciones, "derivaciones.proyectar")
	if _, err := emitirTokenServicio(context.Background(), store, emisoresPrueba(t, false), clientID, secret); codigo(err) != codes.Unavailable {
		t.Fatalf("audiencia no habilitada debe ser Unavailable: %v", err)
	}
}

func TestAudiencia_AltaYPermisos(t *testing.T) {
	if _, err := validarPermisosSistema(security.AudienciaDerivaciones, []string{"derivaciones.proyectar"}); err != nil {
		t.Fatalf("alta de derivaciones con derivaciones.proyectar: %v", err)
	}
	for _, malos := range [][]string{{"estudiantes.basic.view"}, {"derivaciones.proyectar", "estudiantes.basic.view"}, {"derivaciones.create"}} {
		if _, err := validarPermisosSistema(security.AudienciaDerivaciones, malos); codigo(err) != codes.InvalidArgument {
			t.Fatalf("alta de derivaciones con %v debe rechazarse: %v", malos, err)
		}
	}
	if _, err := validarPermisosSistema(security.AudienciaEstudiantes, []string{"derivaciones.proyectar"}); codigo(err) != codes.InvalidArgument {
		t.Fatalf("un sistema de estudiantes no puede recibir derivaciones.proyectar: %v", err)
	}
	if security.AudienciaValida("otra") || !security.AudienciaValida("derivaciones") {
		t.Fatal("solo estudiantes y derivaciones son audiencias válidas")
	}
}
