package integration

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"derivaciones/derivaciones"
	"derivaciones/internal/config"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/security"
	"derivaciones/internal/server"
	"derivaciones/internal/svc"
	"expedientes/expedientesclient"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// Dobles mínimos: embeben la interfaz (nil) y solo implementan lo que
// CrearDerivacion usa. No dependen del catálogo real de Auth.
type permAuthClient struct {
	authclient.Auth
	grants map[string]map[string]bool
}

func (c *permAuthClient) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return &authclient.HasPermissionResponse{Allowed: c.grants[in.UserId][in.Permission]}, nil
}

type fixedExpedientesClient struct {
	expedientesclient.Expedientes
	expedienteID string
	area         string
}

func (c *fixedExpedientesClient) GetExpediente(_ context.Context, in *expedientesclient.GetExpedienteRequest, _ ...grpc.CallOption) (*expedientesclient.GetExpedienteResponse, error) {
	if in.Id != c.expedienteID {
		return nil, status.Error(codes.NotFound, "expediente no encontrado")
	}
	return &expedientesclient.GetExpedienteResponse{Expediente: &expedientesclient.Expediente{Id: c.expedienteID, AreaActual: c.area}}, nil
}

func (c *fixedExpedientesClient) ValidateDerivacion(context.Context, *expedientesclient.ValidateDerivacionRequest, ...grpc.CallOption) (*expedientesclient.ValidateDerivacionResponse, error) {
	return &expedientesclient.ValidateDerivacionResponse{Valido: true}, nil
}

// TestCrearDerivacionPermisoEsLaAutoridad prueba que derivaciones.create lo
// decide el permiso (Auth), no que el rol sea "interno". Las reglas de
// negocio independientes (expediente real/alcance, origen/destino válidos)
// se conservan: se verifican también.
func TestCrearDerivacionPermisoEsLaAutoridad(t *testing.T) {
	requireRealJWTSecret(t)
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, integrationDatabaseURL(t))
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	base := time.Now().UnixNano()
	expedienteID := fixedUUID(base, 1)
	conPermiso := fixedUUID(base, 2) // JWT SOLICITANTE (no interno) con derivaciones.create
	sinPermiso := fixedUUID(base, 3) // JWT DIRECTOR (interno) sin derivaciones.create
	adminSin := fixedUUID(base, 4)   // JWT ADMIN sin derivaciones.create
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM derivaciones WHERE expediente_id = $1", expedienteID)
	}()

	svcCtx := &svc.ServiceContext{
		Config:               config.Config{JWTSecret: realSharedJWTSecret},
		DB:                   pool,
		DerivacionRepository: repository.NewDerivacionRepository(pool),
		JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
		AuthClient:           &permAuthClient{grants: map[string]map[string]bool{conPermiso: {"derivaciones.create": true}}},
		ExpedientesClient:    &fixedExpedientesClient{expedienteID: expedienteID, area: "SECRETARIA"},
	}
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.AuthenticationInterceptor(svcCtx.JWTValidator)))
	derivaciones.RegisterDerivacionesServer(grpcServer, server.NewDerivacionesServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := derivaciones.NewDerivacionesClient(conn)

	authFor := func(userID, role string) context.Context {
		claims := security.Claims{Username: "t", Role: role, RegisteredClaims: jwt.RegisteredClaims{
			Subject: userID, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		}}
		tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(realSharedJWTSecret))
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
	}
	count := func() int {
		var n int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM derivaciones WHERE expediente_id = $1", expedienteID).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}
	req := func(origen, destino string) *derivaciones.CrearDerivacionRequest {
		return &derivaciones.CrearDerivacionRequest{ExpedienteId: expedienteID, Tipo: "DERIVACION", Origen: origen, Destino: destino, Motivo: "test"}
	}

	t.Run("CasoA_ConPermisoSinRolInterno_Puede", func(t *testing.T) {
		if _, err := client.CrearDerivacion(authFor(conPermiso, "SOLICITANTE"), req("SECRETARIA", "DIRECTOR")); err != nil {
			t.Fatalf("con derivaciones.create debía poder aunque el rol no sea interno: %v", err)
		}
		if count() != 1 {
			t.Fatalf("se esperaba 1 derivación, got %d", count())
		}
	})

	t.Run("CasoB_SinPermiso_403_SinMutacion", func(t *testing.T) {
		antes := count()
		for _, c := range []struct{ user, role string }{{sinPermiso, "DIRECTOR"}, {adminSin, "ADMIN"}} {
			if _, err := client.CrearDerivacion(authFor(c.user, c.role), req("SECRETARIA", "DIRECTOR")); status.Code(err) != codes.PermissionDenied {
				t.Fatalf("%s sin derivaciones.create: se esperaba PermissionDenied, got %v", c.role, err)
			}
		}
		if count() != antes {
			t.Fatal("no debía insertarse ninguna derivación")
		}
	})

	t.Run("ReglasDeNegocioIndependientesSeConservan", func(t *testing.T) {
		antes := count()
		c := authFor(conPermiso, "SOLICITANTE")
		if _, err := client.CrearDerivacion(c, req("SECRETARIA", "AREA_INVENTADA")); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("destino inválido: se esperaba InvalidArgument, got %v", err)
		}
		if _, err := client.CrearDerivacion(c, req("DOCENTE", "AUXILIAR")); status.Code(err) != codes.PermissionDenied {
			t.Fatalf("origen/destino ajenos al área real: se esperaba PermissionDenied, got %v", err)
		}
		if count() != antes {
			t.Fatal("no debía insertarse ninguna derivación")
		}
	})
}
