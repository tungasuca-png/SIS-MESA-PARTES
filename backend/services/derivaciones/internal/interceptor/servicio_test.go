package interceptor

import (
	"context"
	"testing"
	"time"

	"derivaciones/internal/security"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	secretoUsuario      = "secreto-de-usuarios-para-pruebas-0123456789"
	secretoDerivaciones = "secreto-exclusivo-derivaciones-pruebas-0123"
	secretoEstudiantes  = "secreto-exclusivo-estudiantes-pruebas-01234"
	sujetoServicio      = "6f1c2b9e-3d4a-4b5c-8d7e-9f0a1b2c3d4e"
)

type opcionesToken struct {
	secreto, aud, tipo, scope, sujeto string
	vida                              time.Duration
	iat                               time.Time
}

func tokenServicio(t *testing.T, o opcionesToken) string {
	t.Helper()
	if o.iat.IsZero() {
		o.iat = time.Now()
	}
	claims := security.ServiceClaims{
		TokenType: o.tipo, ClientID: "cli_prueba", Scope: o.scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: "tungasuca-auth", Subject: o.sujeto, Audience: jwt.ClaimStrings{o.aud},
			IssuedAt: jwt.NewNumericDate(o.iat), NotBefore: jwt.NewNumericDate(o.iat), ExpiresAt: jwt.NewNumericDate(o.iat.Add(o.vida)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(o.secreto))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func tokenUsuario(t *testing.T, rol string) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": sujetoServicio, "role": rol, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix()}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secretoUsuario))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func llamar(t *testing.T, metodo, token string) (codes.Code, bool) {
	t.Helper()
	sv, err := security.NewServiceTokenValidator(secretoDerivaciones, secretoUsuario, "", "")
	if err != nil {
		t.Fatal(err)
	}
	icp := AuthenticationInterceptor(security.NewJWTValidator(secretoUsuario), sv)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	llego := false
	_, err = icp(ctx, nil, &grpc.UnaryServerInfo{FullMethod: metodo}, func(ctx context.Context, req any) (any, error) {
		llego = true
		return nil, nil
	})
	return status.Code(err), llego
}

func TestAplicarMovimiento_SoloTokenDeServicioValido(t *testing.T) {
	base := opcionesToken{secreto: secretoDerivaciones, aud: "derivaciones", tipo: "service", scope: "derivaciones.proyectar", sujeto: sujetoServicio, vida: 5 * time.Minute}
	casos := []struct {
		nombre string
		mod    func(o *opcionesToken)
		token  string
		codigo codes.Code
	}{
		{"válido", func(o *opcionesToken) {}, "", codes.OK},
		{"audiencia estudiantes", func(o *opcionesToken) { o.aud = "estudiantes" }, "", codes.Unauthenticated},
		{"firmado con el secreto de estudiantes", func(o *opcionesToken) { o.secreto = secretoEstudiantes }, "", codes.Unauthenticated},
		{"firmado con el secreto de usuarios", func(o *opcionesToken) { o.secreto = secretoUsuario }, "", codes.Unauthenticated},
		{"sin el scope derivaciones.proyectar", func(o *opcionesToken) { o.scope = "estudiantes.basic.view" }, "", codes.PermissionDenied},
		{"vencido", func(o *opcionesToken) { o.iat = time.Now().Add(-10 * time.Minute) }, "", codes.Unauthenticated},
		{"vida mayor a 15 minutos", func(o *opcionesToken) { o.vida = time.Hour }, "", codes.Unauthenticated},
		{"token_type distinto de service", func(o *opcionesToken) { o.tipo = "access" }, "", codes.Unauthenticated},
		{"sujeto no UUID", func(o *opcionesToken) { o.sujeto = "admin" }, "", codes.Unauthenticated},
		{"JWT de usuario ADMIN", nil, tokenUsuario(t, "ADMIN"), codes.Unauthenticated},
		{"JWT de usuario SECRETARIA", nil, tokenUsuario(t, "SECRETARIA"), codes.Unauthenticated},
	}
	for _, c := range casos {
		tok := c.token
		if c.mod != nil {
			o := base
			c.mod(&o)
			tok = tokenServicio(t, o)
		}
		got, llego := llamar(t, MetodoAplicarMovimiento, tok)
		if got != c.codigo || llego != (c.codigo == codes.OK) {
			t.Errorf("%s: código %v (llegó al handler=%v), esperado %v", c.nombre, got, llego, c.codigo)
		}
	}
}

func TestTokenDeServicio_RechazadoEnLosDemasRPC(t *testing.T) {
	tok := tokenServicio(t, opcionesToken{secreto: secretoDerivaciones, aud: "derivaciones", tipo: "service", scope: "derivaciones.proyectar", sujeto: sujetoServicio, vida: 5 * time.Minute})
	for _, m := range []string{"/derivaciones.Derivaciones/CrearDerivacion", "/derivaciones.Derivaciones/ListDerivaciones",
		"/derivaciones.Derivaciones/GetDerivacion", "/derivaciones.Derivaciones/ReporteDerivaciones"} {
		if got, llego := llamar(t, m, tok); got != codes.Unauthenticated || llego {
			t.Errorf("%s con token de servicio: %v (llegó=%v), esperado Unauthenticated", m, got, llego)
		}
	}
	if got, llego := llamar(t, "/derivaciones.Derivaciones/ListDerivaciones", tokenUsuario(t, "SECRETARIA")); got != codes.OK || !llego {
		t.Errorf("JWT de usuario en ListDerivaciones debe seguir funcionando: %v", got)
	}
}

func TestSinSecretoDeServicio_AplicarDeshabilitado(t *testing.T) {
	sv, err := security.NewServiceTokenValidator("", secretoUsuario, "", "")
	if err != nil || sv != nil {
		t.Fatalf("sin secreto el validador debe ser nil: %v %v", sv, err)
	}
	if _, err := security.NewServiceTokenValidator(secretoUsuario, secretoUsuario, "", ""); err == nil {
		t.Fatal("el secreto de servicio no puede ser igual al de usuarios")
	}
	if _, err := security.NewServiceTokenValidator("corto", secretoUsuario, "", ""); err == nil {
		t.Fatal("el secreto de servicio debe tener al menos 32 caracteres")
	}
	icp := AuthenticationInterceptor(security.NewJWTValidator(secretoUsuario), nil)
	tok := tokenServicio(t, opcionesToken{secreto: secretoDerivaciones, aud: "derivaciones", tipo: "service", scope: "derivaciones.proyectar", sujeto: sujetoServicio, vida: time.Minute})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	_, err = icp(ctx, nil, &grpc.UnaryServerInfo{FullMethod: MetodoAplicarMovimiento}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("sin validador AplicarMovimiento debe rechazar: %v", err)
	}
}
