package interceptor

import (
	"context"
	"net"
	"sync"
	"testing"

	"auth/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// M-01: invocación DIRECTA de la RPC Register por gRPC (sin el Gateway), con
// la cadena de interceptores de producción (DefaultMethodPolicies). El
// servidor solo cuenta las llamadas que llegan a la lógica: una invocación
// rechazada no debe ejecutar Register (no crea ninguna cuenta).

type authContador struct {
	auth.UnimplementedAuthServer
	mu            sync.Mutex
	registros     int
	emailRegistro int
}

func (a *authContador) Register(context.Context, *auth.RegisterRequest) (*auth.RegisterResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.registros++
	return &auth.RegisterResponse{Id: "nuevo"}, nil
}

func (a *authContador) EmailRegister(context.Context, *auth.EmailRegisterRequest) (*auth.EmailRegisterResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.emailRegistro++
	return &auth.EmailRegisterResponse{RegistrationId: "r1"}, nil
}

// checkerPorUsuario: concede el permiso solo a los usuarios indicados y
// guarda qué permiso se consultó.
type checkerPorUsuario struct {
	mu         sync.Mutex
	conPermiso map[string]bool
	consultado []string
}

func (c *checkerPorUsuario) HasPermission(_ context.Context, userID, permiso string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consultado = append(c.consultado, permiso)
	return c.conPermiso[userID], nil
}

func authPorGRPC(t *testing.T, checker PermissionChecker) (auth.AuthClient, *authContador) {
	t.Helper()
	policies := DefaultMethodPolicies()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(
		AuthenticationInterceptor(testJWTManager(), policies),
		AuthorizationInterceptor(checker, policies),
	))
	contador := &authContador{}
	auth.RegisterAuthServer(srv, contador)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return auth.NewAuthClient(conn), contador
}

func conToken(t *testing.T, userID, role string) context.Context {
	t.Helper()
	token, _, err := testJWTManager().GenerateAccessToken(userID, "u-"+userID, role)
	if err != nil {
		t.Fatal(err)
	}
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

func altaSolicitante() *auth.RegisterRequest {
	return &auth.RegisterRequest{Username: "suplantador", Dni: "45678912", Email: "x@correo.example", Password: "ClaveSegura123"}
}

func TestRegister_InvocacionDirectaSinSesionRechazada(t *testing.T) {
	cliente, contador := authPorGRPC(t, &checkerPorUsuario{})
	_, err := cliente.Register(context.Background(), altaSolicitante())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Register sin sesión: %v, se esperaba Unauthenticated", err)
	}
	if contador.registros != 0 {
		t.Fatal("la lógica de Register no debía ejecutarse (se habría creado una cuenta)")
	}
}

func TestRegister_SinPermisoRechazadaAntesDeLaLogica(t *testing.T) {
	checker := &checkerPorUsuario{conPermiso: map[string]bool{}}
	cliente, contador := authPorGRPC(t, checker)
	// Un solicitante (o cualquier cuenta sin usuarios.change_role).
	_, err := cliente.Register(conToken(t, "sol-1", "SOLICITANTE"), altaSolicitante())
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Register sin permiso: %v, se esperaba PermissionDenied", err)
	}
	if contador.registros != 0 {
		t.Fatal("la lógica de Register no debía ejecutarse")
	}
	if len(checker.consultado) != 1 || checker.consultado[0] != "usuarios.change_role" {
		t.Fatalf("permiso consultado: %v", checker.consultado)
	}
	// Un token con firma alterada tampoco pasa.
	_, err = cliente.Register(metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer abc.def.ghi"), altaSolicitante())
	if status.Code(err) != codes.Unauthenticated || contador.registros != 0 {
		t.Fatalf("token inválido: %v", err)
	}
}

func TestRegister_SoloConPermisoDeAltaDeCuentas(t *testing.T) {
	cliente, contador := authPorGRPC(t, &checkerPorUsuario{conPermiso: map[string]bool{"adm-1": true}})
	if _, err := cliente.Register(conToken(t, "adm-1", "ADMIN"), altaSolicitante()); err != nil {
		t.Fatalf("con usuarios.change_role: %v", err)
	}
	if contador.registros != 1 {
		t.Fatalf("llamadas a Register: %d", contador.registros)
	}
}

func TestRegistroVerificadoSigueSiendoPublico(t *testing.T) {
	cliente, contador := authPorGRPC(t, &checkerPorUsuario{})
	if _, err := cliente.EmailRegister(context.Background(), &auth.EmailRegisterRequest{}); err != nil {
		t.Fatalf("EmailRegister sin sesión debe seguir permitido: %v", err)
	}
	if contador.emailRegistro != 1 {
		t.Fatal("EmailRegister no llegó a la lógica")
	}
	p := DefaultMethodPolicies()
	for _, m := range []string{"EmailRegister", "EmailResendCode", "EmailVerify", "EmailCompleteRegistration", "EmailCancelRegistration", "GoogleRegister", "Login", "RefreshToken"} {
		if !p["/auth.Auth/"+m].Public {
			t.Errorf("%s debe seguir siendo pública", m)
		}
	}
	if pol := p["/auth.Auth/Register"]; pol.Public || pol.Permission != "usuarios.change_role" || !pol.RequireAuthentication {
		t.Fatalf("política de Register: %+v", pol)
	}
}
