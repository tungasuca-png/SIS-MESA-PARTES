package interceptor_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"estudiantes/internal/authorization"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/security"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Pruebas sin base de datos ni red: tokens ficticios firmados aquí con
// claves de prueba; Auth se reemplaza por un doble.

const (
	claveUsuarios = "clave-de-usuarios-solo-para-pruebas-0001"
	claveServicio = "clave-de-servicios-solo-para-pruebas-0002"
	sistemaID     = "7b0f6d2e-1c1a-4c43-9d55-0a6a8f1b2c3d"
	usuarioID     = "3f2b1c4d-0000-4000-8000-000000000001"
	metodoGet     = "/estudiantes.Estudiantes/GetEstudiante"
	metodoBasic   = "/estudiantes.Estudiantes/GetEstudiantesBasic"
)

type tokenServicio struct {
	clave, iss, aud, tipo, sub, scope string
	vida                              time.Duration
	emitido                           time.Time
}

func base() tokenServicio {
	return tokenServicio{clave: claveServicio, iss: "tungasuca-auth", aud: "estudiantes", tipo: "service",
		sub: sistemaID, scope: "estudiantes.view", vida: 5 * time.Minute, emitido: time.Now()}
}

func (t tokenServicio) firmar(tb testing.TB) string {
	tb.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, security.ServiceClaims{
		TokenType: t.tipo, ClientID: "cli_0123456789abcdef0123456789abcdef", Scope: t.scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: t.iss, Subject: t.sub, Audience: jwt.ClaimStrings{t.aud},
			IssuedAt: jwt.NewNumericDate(t.emitido), ExpiresAt: jwt.NewNumericDate(t.emitido.Add(t.vida)),
		},
	})
	s, err := tok.SignedString([]byte(t.clave))
	if err != nil {
		tb.Fatal(err)
	}
	return s
}

func tokenUsuario(tb testing.TB, clave string, extra func(*security.Claims)) string {
	tb.Helper()
	c := security.Claims{Username: "secretaria.demo", Role: "SECRETARIA", RegisteredClaims: jwt.RegisteredClaims{
		Subject: usuarioID, IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
	}}
	if extra != nil {
		extra(&c)
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(clave))
	if err != nil {
		tb.Fatal(err)
	}
	return s
}

func validadorServicio(t *testing.T) *security.ServiceTokenValidator {
	t.Helper()
	v, err := security.NewServiceTokenValidator(claveServicio, claveUsuarios, "", "")
	if err != nil || v == nil {
		t.Fatalf("validador: %v", err)
	}
	return v
}

// authFalso: registra si se consultó Auth; para usuarios responde "permitido".
type authFalso struct{ llamadas int }

func (a *authFalso) HasPermission(context.Context, *authclient.HasPermissionRequest, ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	a.llamadas++
	return &authclient.HasPermissionResponse{Allowed: true}, nil
}

type resultado struct {
	err     error
	llamado bool
	auth    *authFalso
}

// invocar pasa el token por el interceptor real y, dentro del handler, exige
// el permiso con el mismo authorization.RequirePermission que usa cada logic.
func invocar(t *testing.T, token, metodo, permiso string) resultado {
	t.Helper()
	icpt := interceptor.AuthenticationInterceptor(security.NewJWTValidator(claveUsuarios), validadorServicio(t))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	r := resultado{auth: &authFalso{}}
	_, r.err = icpt(ctx, nil, &grpc.UnaryServerInfo{FullMethod: metodo}, func(ctx context.Context, _ any) (any, error) {
		r.llamado = true
		userID, _, ok := interceptor.UserFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "sin identidad")
		}
		return nil, authorization.RequirePermission(ctx, r.auth, userID, permiso)
	})
	return r
}

func TestTokenDeSistemaValidoConPermiso(t *testing.T) {
	r := invocar(t, base().firmar(t), metodoGet, "estudiantes.view")
	if r.err != nil || !r.llamado {
		t.Fatalf("se esperaba acceso: %v", r.err)
	}
	if r.auth.llamadas != 0 {
		t.Fatal("un token de sistema no debe consultar Auth.HasPermission")
	}
}

func TestTokenDeSistemaInvalido(t *testing.T) {
	casos := map[string]func(*tokenServicio){
		"expirado":                      func(s *tokenServicio) { s.emitido = time.Now().Add(-10 * time.Minute) },
		"firma inválida":                func(s *tokenServicio) { s.clave = "otra-clave-cualquiera-de-32-caracteres!!" },
		"firmado con clave de usuarios": func(s *tokenServicio) { s.clave = claveUsuarios },
		"audiencia incorrecta":          func(s *tokenServicio) { s.aud = "expedientes" },
		"emisor incorrecto":             func(s *tokenServicio) { s.iss = "otro-emisor" },
		"sin token_type":                func(s *tokenServicio) { s.tipo = "" },
		"token_type de usuario":         func(s *tokenServicio) { s.tipo = "user" },
		"sub no es UUID":                func(s *tokenServicio) { s.sub = "admin" },
		"sin scope":                     func(s *tokenServicio) { s.scope = "" },
		"vida mayor a 15 min":           func(s *tokenServicio) { s.vida = 2 * time.Hour },
	}
	for nombre, mutar := range casos {
		t.Run(nombre, func(t *testing.T) {
			tok := base()
			mutar(&tok)
			r := invocar(t, tok.firmar(t), metodoGet, "estudiantes.view")
			if status.Code(r.err) != codes.Unauthenticated || r.llamado {
				t.Fatalf("se esperaba Unauthenticated sin ejecutar la operación: %v", r.err)
			}
		})
	}
}

func TestTokenDeUsuarioSigueFuncionando(t *testing.T) {
	r := invocar(t, tokenUsuario(t, claveUsuarios, nil), metodoGet, "estudiantes.view")
	if r.err != nil || !r.llamado || r.auth.llamadas != 1 {
		t.Fatalf("el flujo de usuario cambió: err=%v llamadas a Auth=%d", r.err, r.auth.llamadas)
	}
	// Los usuarios no tienen la lista de métodos de sistemas.
	r = invocar(t, tokenUsuario(t, claveUsuarios, nil), "/estudiantes.Estudiantes/VincularFamiliar", "estudiantes.familia.update")
	if r.err != nil || !r.llamado {
		t.Fatalf("un usuario debe poder invocar cualquier RPC según sus permisos: %v", r.err)
	}
}

func TestTokenDeSistemaNoSeHacePasarPorUsuario(t *testing.T) {
	// Un token de usuario que diga token_type=service no es un usuario.
	falso := tokenUsuario(t, claveUsuarios, func(c *security.Claims) { c.TokenType = "service" })
	if r := invocar(t, falso, metodoGet, "estudiantes.view"); status.Code(r.err) != codes.Unauthenticated {
		t.Fatalf("se esperaba Unauthenticated: %v", r.err)
	}
	// Dentro de la operación, el sistema no tiene rol ni token reenviable.
	icpt := interceptor.AuthenticationInterceptor(security.NewJWTValidator(claveUsuarios), validadorServicio(t))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+base().firmar(t)))
	_, err := icpt(ctx, nil, &grpc.UnaryServerInfo{FullMethod: metodoGet}, func(ctx context.Context, _ any) (any, error) {
		if _, role, _ := interceptor.UserFromContext(ctx); role != "" {
			t.Fatalf("un sistema obtuvo rol %q", role)
		}
		if _, ok := interceptor.TokenFromContext(ctx); ok {
			t.Fatal("el token de un sistema no debe reenviarse a Auth")
		}
		if s, ok := interceptor.ServiceFromContext(ctx); !ok || s.ID != sistemaID {
			t.Fatal("falta la identidad del sistema")
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSistemaSinPermiso(t *testing.T) {
	// Permiso ausente del scope.
	if r := invocar(t, base().firmar(t), "/estudiantes.Estudiantes/CreateEstudiante", "estudiantes.create"); status.Code(r.err) != codes.PermissionDenied {
		t.Fatalf("se esperaba PermissionDenied: %v", r.err)
	}
	// Operación exclusiva de usuarios, aunque el scope tuviera el permiso.
	tok := base()
	tok.scope = "estudiantes.view estudiantes.update"
	r := invocar(t, tok.firmar(t), "/estudiantes.Estudiantes/ListPosiblesDuplicados", "estudiantes.update")
	if status.Code(r.err) != codes.PermissionDenied || r.llamado {
		t.Fatalf("se esperaba PermissionDenied sin ejecutar: %v", r.err)
	}
	// basic.view no alcanza para el registro completo.
	tok.scope = "estudiantes.basic.view"
	if r := invocar(t, tok.firmar(t), metodoGet, "estudiantes.view"); status.Code(r.err) != codes.PermissionDenied {
		t.Fatalf("basic.view no debe permitir GetEstudiante: %v", r.err)
	}
}

func TestSistemaConPermisoAutorizado(t *testing.T) {
	tok := base()
	tok.scope = "estudiantes.basic.view"
	if r := invocar(t, tok.firmar(t), metodoBasic, "estudiantes.view"); r.err != nil {
		t.Fatalf("basic.view debe permitir GetEstudiantesBasic: %v", r.err)
	}
	tok.scope = "estudiantes.create estudiantes.view"
	if r := invocar(t, tok.firmar(t), "/estudiantes.Estudiantes/CreateEstudiante", "estudiantes.create"); r.err != nil {
		t.Fatalf("se esperaba acceso a CreateEstudiante: %v", r.err)
	}
}

func TestAccesoDeSistemaSeRegistraSinToken(t *testing.T) {
	var buf bytes.Buffer
	logx.SetWriter(logx.NewWriter(&buf))
	defer logx.Reset()

	tok := base().firmar(t)
	invocar(t, tok, metodoGet, "estudiantes.view")
	invocar(t, tok, "/estudiantes.Estudiantes/ExportarEstudiantes", "estudiantes.view")

	salida := buf.String()
	for _, esperado := range []string{"acceso de sistema consumidor", sistemaID, "cli_0123456789abcdef0123456789abcdef", metodoGet, "OK", "PermissionDenied"} {
		if !strings.Contains(salida, esperado) {
			t.Fatalf("falta %q en el registro: %s", esperado, salida)
		}
	}
	if strings.Contains(salida, tok) || strings.Contains(salida, strings.Split(tok, ".")[2]) {
		t.Fatal("el registro contiene el token")
	}
}

func TestValidadorDeServicioConfiguracion(t *testing.T) {
	if v, err := security.NewServiceTokenValidator("", claveUsuarios, "", ""); v != nil || err != nil {
		t.Fatal("sin clave debe quedar deshabilitado")
	}
	if _, err := security.NewServiceTokenValidator("corta", claveUsuarios, "", ""); err == nil {
		t.Fatal("clave corta aceptada")
	}
	if _, err := security.NewServiceTokenValidator(claveUsuarios, claveUsuarios, "", ""); err == nil {
		t.Fatal("la clave de servicio no puede ser la de usuarios")
	}
	// Sin validador de servicio, un token de sistema se rechaza.
	icpt := interceptor.AuthenticationInterceptor(security.NewJWTValidator(claveUsuarios), nil)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+base().firmar(t)))
	if _, err := icpt(ctx, nil, &grpc.UnaryServerInfo{FullMethod: metodoGet}, func(context.Context, any) (any, error) { return nil, nil }); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("se esperaba Unauthenticated: %v", err)
	}
}
