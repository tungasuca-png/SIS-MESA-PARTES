package logic

import (
	"context"
	"sync"
	"testing"
	"time"

	"auth/authclient"
	"psicologia/internal/interceptor"
	"psicologia/internal/repository"
	"psicologia/internal/security"
	"psicologia/internal/svc"
	"psicologia/psicologia"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// La credencial de servicio de Psicología para Estudiantes NO reemplaza los
// permisos del usuario: OpenExpediente comprueba psicologia.expedientes.create
// ANTES de consultar al estudiante. Sin base de datos: la bitácora usa un
// pool perezoso hacia un puerto cerrado (la escritura falla y se registra, y
// la denegación se mantiene, como exige auditDenied).

type authPermisos struct {
	authclient.Auth
	permitidos map[string]bool
	err        error
	mu         sync.Mutex
	consultas  []string
}

func (a *authPermisos) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.consultas = append(a.consultas, in.Permission)
	if a.err != nil {
		return nil, a.err
	}
	return &authclient.HasPermissionResponse{Allowed: a.permitidos[in.Permission]}, nil
}

type verificadorEspia struct{ llamadas int }

func (v *verificadorEspia) Exists(context.Context, string) (bool, error) {
	v.llamadas++
	return true, nil
}

func ctxPsicologo(t *testing.T) context.Context {
	t.Helper()
	const clave = "clave-de-usuarios-solo-para-pruebas-logic"
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{Role: "PSICOLOGO", RegisteredClaims: jwt.RegisteredClaims{
		Subject: "3f2b1c4d-0000-4000-8000-000000000001", ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
	}}).SignedString([]byte(clave))
	if err != nil {
		t.Fatal(err)
	}
	in := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	var ctx context.Context
	_, err = interceptor.AuthenticationInterceptor(security.NewJWTValidator(clave))(in, nil, &grpc.UnaryServerInfo{},
		func(c context.Context, _ any) (any, error) { ctx = c; return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func bitacoraSinBase(t *testing.T) *repository.AuditoriaRepository {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "postgres://sin_base@127.0.0.1:1/sin_base?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return repository.NewAuditoriaRepository(pool)
}

func TestOpenExpediente_PermisosHumanosAntesDeConsultarEstudiantes(t *testing.T) {
	estudiante := "11111111-1111-4111-8111-111111111111"

	t.Run("sin psicologia.expedientes.create", func(t *testing.T) {
		espia := &verificadorEspia{}
		auth := &authPermisos{permitidos: map[string]bool{}}
		sc := &svc.ServiceContext{AuthClient: auth, Estudiantes: espia, AuditoriaRepo: bitacoraSinBase(t)}
		_, err := NewOpenExpedienteLogic(ctxPsicologo(t), sc).OpenExpediente(&psicologia.OpenExpedienteRequest{EstudianteId: estudiante})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: %v", err)
		}
		if espia.llamadas != 0 {
			t.Fatal("sin permiso del usuario no debe consultarse Estudiantes (ni usarse la credencial del servicio)")
		}
	})

	t.Run("Auth no disponible", func(t *testing.T) {
		espia := &verificadorEspia{}
		auth := &authPermisos{err: status.Error(codes.Unavailable, "auth caído")}
		sc := &svc.ServiceContext{AuthClient: auth, Estudiantes: espia, AuditoriaRepo: bitacoraSinBase(t)}
		_, err := NewOpenExpedienteLogic(ctxPsicologo(t), sc).OpenExpediente(&psicologia.OpenExpedienteRequest{EstudianteId: estudiante})
		if err == nil || espia.llamadas != 0 {
			t.Fatalf("fail-closed: sin decisión de Auth no se consulta Estudiantes: %v", err)
		}
	})

	t.Run("con permiso: la verificación del estudiante sí se ejecuta", func(t *testing.T) {
		espia := &verificadorEspia{}
		auth := &authPermisos{permitidos: map[string]bool{permExpedientesCreate: true}}
		sc := &svc.ServiceContext{AuthClient: auth, Estudiantes: espia, AuditoriaRepo: bitacoraSinBase(t)}
		// Sin ExpedienteRepo la apertura no llega a escribir: se detiene en
		// el panic controlado del repositorio nulo, después de verificar.
		func() {
			defer func() { _ = recover() }()
			_, _ = NewOpenExpedienteLogic(ctxPsicologo(t), sc).OpenExpediente(&psicologia.OpenExpedienteRequest{EstudianteId: estudiante})
		}()
		if espia.llamadas != 1 {
			t.Fatalf("con permiso debe verificarse el estudiante una vez, hubo %d", espia.llamadas)
		}
		if auth.consultas[0] != permExpedientesCreate {
			t.Fatalf("el primer control debe ser el permiso del usuario: %v", auth.consultas)
		}
	})
}
