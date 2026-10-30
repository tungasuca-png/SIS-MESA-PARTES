package logic

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/security"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Pruebas sin base de datos: el repositorio se reemplaza por un doble en
// memoria. Credenciales ficticias generadas en cada prueba.

const (
	secretoUsuariosPrueba = "secreto-de-usuarios-de-prueba-0123456789"
	secretoServicioPrueba = "secreto-de-servicios-de-prueba-9876543210"
)

type storeFalso struct {
	clientes map[string]*repository.ServiceClient
	usados   []string
}

func (s *storeFalso) FindByClientID(_ context.Context, clientID string) (*repository.ServiceClient, error) {
	if c, ok := s.clientes[clientID]; ok {
		return c, nil
	}
	return nil, repository.ErrServiceClientNotFound
}

func (s *storeFalso) TouchLastUsed(_ context.Context, id string) error {
	s.usados = append(s.usados, id)
	return nil
}

func nuevoEmisor(t *testing.T) *security.ServiceTokenIssuer {
	t.Helper()
	issuer, err := security.NewServiceTokenIssuer(secretoServicioPrueba, secretoUsuariosPrueba, "", "", 0)
	if err != nil || issuer == nil {
		t.Fatalf("emisor: %v", err)
	}
	return issuer
}

func clientePrueba(t *testing.T, estado string, permisos ...string) (*storeFalso, string, string) {
	t.Helper()
	clientID, secret, hash, err := security.GenerateClientCredentials()
	if err != nil {
		t.Fatal(err)
	}
	store := &storeFalso{clientes: map[string]*repository.ServiceClient{
		clientID: {ID: "7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d", ClientID: clientID, SecretHash: hash, Estado: estado, Permisos: permisos},
	}}
	return store, clientID, secret
}

func codigo(err error) codes.Code { return status.Code(err) }

func TestEmitirTokenServicio_ClienteValidoObtieneToken(t *testing.T) {
	store, clientID, secret := clientePrueba(t, repository.ServiceClientActivo, "estudiantes.view", "estudiantes.basic.view")
	resp, err := emitirTokenServicio(context.Background(), store, nuevoEmisor(t), clientID, secret)
	if err != nil {
		t.Fatalf("se esperaba token: %v", err)
	}
	if resp.Scope != "estudiantes.basic.view estudiantes.view" || resp.ExpiresIn != 300 {
		t.Fatalf("scope/expiración inesperados: %q %d", resp.Scope, resp.ExpiresIn)
	}
	claims := &security.ServiceClaims{}
	_, err = jwt.ParseWithClaims(resp.AccessToken, claims, func(*jwt.Token) (any, error) { return []byte(secretoServicioPrueba), nil },
		jwt.WithIssuer(security.DefaultServiceTokenIssuer), jwt.WithAudience(security.DefaultServiceTokenAudience), jwt.WithExpirationRequired())
	if err != nil {
		t.Fatalf("token inválido: %v", err)
	}
	if claims.TokenType != "service" || claims.ClientID != clientID || claims.Subject == "" {
		t.Fatalf("claims inesperados: %+v", claims)
	}
	if len(store.usados) != 1 {
		t.Fatal("no se registró last_used_at")
	}
}

func TestEmitirTokenServicio_Rechazos(t *testing.T) {
	store, clientID, secret := clientePrueba(t, repository.ServiceClientActivo, "estudiantes.view")
	revocado, revID, revSecret := clientePrueba(t, repository.ServiceClientRevocado, "estudiantes.view")
	sinPermisos, sinID, sinSecret := clientePrueba(t, repository.ServiceClientActivo)
	// Un permiso que no es asignable a sistemas nunca llega al token.
	admin, adminID, adminSecret := clientePrueba(t, repository.ServiceClientActivo, "roles.view")
	otroID, _, _, _ := security.GenerateClientCredentials()

	casos := []struct {
		nombre        string
		store         *storeFalso
		clientID, sec string
		esperado      codes.Code
	}{
		{"client_id incorrecto", store, otroID, secret, codes.Unauthenticated},
		{"client_secret incorrecto", store, clientID, secret + "x", codes.Unauthenticated},
		{"cliente revocado", revocado, revID, revSecret, codes.Unauthenticated},
		{"sin permisos", sinPermisos, sinID, sinSecret, codes.PermissionDenied},
		{"permiso no asignable", admin, adminID, adminSecret, codes.PermissionDenied},
		{"campos vacíos", store, "", "", codes.InvalidArgument},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			resp, err := emitirTokenServicio(context.Background(), c.store, nuevoEmisor(t), c.clientID, c.sec)
			if codigo(err) != c.esperado || resp != nil {
				t.Fatalf("se esperaba %v, se obtuvo %v", c.esperado, err)
			}
		})
	}
}

func TestEmitirTokenServicio_DeshabilitadoSinClave(t *testing.T) {
	store, clientID, secret := clientePrueba(t, repository.ServiceClientActivo, "estudiantes.view")
	if _, err := emitirTokenServicio(context.Background(), store, nil, clientID, secret); codigo(err) != codes.Unavailable {
		t.Fatalf("se esperaba Unavailable: %v", err)
	}
}

func TestEmitirTokenServicio_NoRegistraSecretosNiTokens(t *testing.T) {
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	defer logx.Reset()

	store, clientID, secret := clientePrueba(t, repository.ServiceClientActivo, "estudiantes.view")
	resp, err := emitirTokenServicio(context.Background(), store, nuevoEmisor(t), clientID, secret)
	if err != nil {
		t.Fatal(err)
	}
	// Un secreto escrito por error en el campo client_id tampoco se registra.
	_, _ = emitirTokenServicio(context.Background(), store, nuevoEmisor(t), secret, secret)
	_, _ = emitirTokenServicio(context.Background(), store, nuevoEmisor(t), clientID, "otro-secreto")

	salida := buf.String()
	if salida == "" {
		t.Fatal("se esperaban registros de emisión y rechazo")
	}
	for _, sensible := range []string{secret, resp.AccessToken, "otro-secreto"} {
		if strings.Contains(salida, sensible) {
			t.Fatalf("el log contiene un secreto o token: %s", salida)
		}
	}
}

func TestValidarPermisosSistema(t *testing.T) {
	ok, err := validarPermisosSistema([]string{" estudiantes.view", "estudiantes.view", "estudiantes.matricula.update"})
	if err != nil || strings.Join(ok, ",") != "estudiantes.matricula.update,estudiantes.view" {
		t.Fatalf("normalización inesperada: %v %v", ok, err)
	}
	for _, malos := range [][]string{nil, {""}, {"roles.view"}, {"usuarios.change_role"}, {"integraciones.manage"}, {"estudiantes.view", "expedientes.view"}} {
		if _, err := validarPermisosSistema(malos); codigo(err) != codes.InvalidArgument {
			t.Fatalf("%v debió rechazarse: %v", malos, err)
		}
	}
}

func TestServiceTokenIssuer_Configuracion(t *testing.T) {
	if i, err := security.NewServiceTokenIssuer("", secretoUsuariosPrueba, "", "", 0); i != nil || err != nil {
		t.Fatal("sin clave debe quedar deshabilitado sin error")
	}
	if _, err := security.NewServiceTokenIssuer("corta", secretoUsuariosPrueba, "", "", 0); err == nil {
		t.Fatal("una clave corta debe rechazarse")
	}
	if _, err := security.NewServiceTokenIssuer(secretoUsuariosPrueba, secretoUsuariosPrueba, "", "", 0); err == nil {
		t.Fatal("la clave de servicios no puede ser la de usuarios")
	}
	i, _ := security.NewServiceTokenIssuer(secretoServicioPrueba, secretoUsuariosPrueba, "", "", 2*time.Hour)
	_, _, dur, err := i.Issue("7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d", "cli_x", []string{"estudiantes.view"})
	if err != nil || dur != security.MaxServiceTokenDuration {
		t.Fatalf("la duración debe limitarse a 15 min: %v %v", dur, err)
	}
}

// Un token de servicio nunca autentica a un usuario en Auth: ni con su
// propia clave (firma distinta) ni aunque alguien reutilizara la clave de
// usuarios (token_type presente).
func TestTokenDeServicioNoEsTokenDeUsuario(t *testing.T) {
	usuarios := security.NewJWTManager(secretoUsuariosPrueba, 15*time.Minute)
	tok, _, _, err := nuevoEmisor(t).Issue("7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d", "cli_x", []string{"estudiantes.view"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := usuarios.ValidateAccessToken(tok); err == nil {
		t.Fatal("un token de servicio validó como token de usuario")
	}
	// Misma clave que usuarios (configuración errónea simulada a mano).
	falsificado := jwt.NewWithClaims(jwt.SigningMethodHS256, security.ServiceClaims{
		TokenType: "service", ClientID: "cli_x", Scope: "estudiantes.view",
		RegisteredClaims: jwt.RegisteredClaims{Subject: "7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))},
	})
	firmado, _ := falsificado.SignedString([]byte(secretoUsuariosPrueba))
	if _, err := usuarios.ValidateAccessToken(firmado); err == nil {
		t.Fatal("un token con token_type validó como token de usuario")
	}
	// Los tokens de usuario siguen funcionando igual.
	userTok, _, err := usuarios.GenerateAccessToken("3f2b1c4d-0000-4000-8000-000000000001", "secretaria.demo", "SECRETARIA")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := usuarios.ValidateAccessToken(userTok)
	if err != nil || claims.Role != "SECRETARIA" || claims.TokenType != "" {
		t.Fatalf("el token de usuario dejó de validar: %v", err)
	}
}

// La emisión es pública (como Login) y la administración exige
// integraciones.manage; ningún método nuevo queda sin política.
func TestPoliticasClientCredentials(t *testing.T) {
	p := interceptor.DefaultMethodPolicies()
	if !p["/auth.Auth/IssueServiceToken"].Public {
		t.Fatal("IssueServiceToken debe ser público")
	}
	for _, m := range []string{"/auth.Auth/CreateServiceClient", "/auth.Auth/RevokeServiceClient", "/auth.Auth/ListServiceClients"} {
		if p[m].Public || p[m].Permission != "integraciones.manage" {
			t.Fatalf("%s debe exigir integraciones.manage: %+v", m, p[m])
		}
	}
}
