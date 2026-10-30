package logic

import (
	"context"
	"testing"
	"time"

	"auth/authclient"
	"expedientes/expedientes"
	"expedientes/internal/interceptor"
	"expedientes/internal/security"
	"expedientes/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Pruebas de seguridad de GET /api/expedientes que NO necesitan PostgreSQL:
// todo lo que se comprueba aquí ocurre antes de consultar la base (el
// repositorio queda nil; si se llegara a usar, la prueba fallaría con panic).

const secretoListado = "secreto-de-prueba-listado-expedientes"

// authFalso responde HasPermission según `permitido`, o falla si `caido`.
type authFalso struct {
	authclient.Auth
	permitido map[string]bool
	caido     bool
}

func (a *authFalso) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	if a.caido {
		return nil, status.Error(codes.Unavailable, "auth no disponible")
	}
	return &authclient.HasPermissionResponse{Allowed: a.permitido[in.Permission]}, nil
}

// listarComo ejecuta ListExpedientes autenticado (JWT real validado por el
// interceptor del servicio) con el rol indicado.
func listarComo(t *testing.T, auth *authFalso, role string, in *expedientes.ListExpedientesRequest) error {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, security.Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "00000000-0000-4000-8000-000000000001",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}).SignedString([]byte(secretoListado))
	if err != nil {
		t.Fatal(err)
	}
	sc := &svc.ServiceContext{AuthClient: auth}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tok))
	_, err = interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoListado))(ctx, in, &grpc.UnaryServerInfo{},
		func(ctx context.Context, req any) (any, error) {
			return NewListExpedientesLogic(ctx, sc).ListExpedientes(req.(*expedientes.ListExpedientesRequest))
		})
	return err
}

func TestListExpedientes_FiltrosConInyeccionSonRechazados(t *testing.T) {
	auth := &authFalso{permitido: map[string]bool{"expedientes.view": true}}
	for nombre, in := range map[string]*expedientes.ListExpedientesRequest{
		"tipo con OR":           {Tipo: "CONSTANCIA' OR 1=1--"},
		"tipo con UNION":        {Tipo: "X' UNION SELECT password_hash FROM usuarios--"},
		"estado con comentario": {Estado: "PENDIENTE'/*"},
		"estado con ;":          {Estado: "PENDIENTE; DROP TABLE expedientes"},
		"prioridad con comilla": {Prioridad: "URGENTE'"},
		"tipo con NUL":          {Tipo: "CONSTANCIA\x00"},
		"tipo con LIKE %":       {Tipo: "%"},
	} {
		if err := listarComo(t, auth, "SECRETARIA", in); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: se esperaba InvalidArgument (sin llegar a SQL), got %v", nombre, err)
		}
	}
}

func TestListExpedientes_PermisosYAutenticacion(t *testing.T) {
	sinPermiso := &authFalso{permitido: map[string]bool{}}
	if err := listarComo(t, sinPermiso, "DOCENTE", &expedientes.ListExpedientesRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("personal sin expedientes.view: se esperaba PermissionDenied, got %v", err)
	}
	// Un SOLICITANTE necesita view_own; tener solo "expedientes.view" no le
	// sirve para el listado (el permiso exigido depende del rol del JWT).
	soloView := &authFalso{permitido: map[string]bool{"expedientes.view": true}}
	if err := listarComo(t, soloView, "SOLICITANTE", &expedientes.ListExpedientesRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Errorf("SOLICITANTE sin view_own: se esperaba PermissionDenied, got %v", err)
	}
	// Auth caído: fail-closed, nunca se lista sin confirmar el permiso.
	if err := listarComo(t, &authFalso{caido: true}, "ADMIN", &expedientes.ListExpedientesRequest{}); status.Code(err) != codes.Unavailable {
		t.Errorf("Auth caído: se esperaba Unavailable (fail-closed), got %v", err)
	}
	// Sin JWT: Unauthenticated antes de cualquier otra cosa.
	sc := &svc.ServiceContext{AuthClient: soloView}
	_, err := interceptor.AuthenticationInterceptor(security.NewJWTValidator(secretoListado))(context.Background(), &expedientes.ListExpedientesRequest{},
		&grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
			return NewListExpedientesLogic(ctx, sc).ListExpedientes(req.(*expedientes.ListExpedientesRequest))
		})
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("sin token: se esperaba Unauthenticated, got %v", err)
	}
}
