package integration

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"documentos/documentos"
	"documentos/internal/config"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/security"
	"documentos/internal/server"
	"documentos/internal/svc"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// grantsAuthClient decide SOLO por user_id (permisos que Auth le calcula a
// ese usuario), nunca por el nombre de rol del JWT.
type grantsAuthClient struct {
	*scriptedAuthClient
	grants map[string]map[string]bool
}

func (c *grantsAuthClient) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return &authclient.HasPermissionResponse{Allowed: c.grants[in.UserId][in.Permission]}, nil
}

// TestDeleteDocumentoPermisoEsLaAutoridad prueba que documentos.delete lo
// decide el permiso (Auth), no que el rol sea "interno": la regla de negocio
// independiente (estado PENDIENTE y alcance del expediente, delegado en
// Expedientes) sigue aplicando.
func TestDeleteDocumentoPermisoEsLaAutoridad(t *testing.T) {
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
	dueno := fixedUUID(base, 2)      // JWT SOLICITANTE (rol NO interno), con documentos.create+delete
	sinPermiso := fixedUUID(base, 3) // JWT SOLICITANTE, sin documentos.delete
	adminSin := fixedUUID(base, 4)   // JWT ADMIN, sin documentos.delete
	defer cleanupIntegrationData(t, pool, expedienteID)

	svcCtx := &svc.ServiceContext{
		Config:              config.Config{JWTSecret: testSecret},
		DB:                  pool,
		DocumentoRepository: repository.NewDocumentoRepository(pool),
		JWTValidator:        security.NewJWTValidator(testSecret),
		AuthClient: &grantsAuthClient{
			scriptedAuthClient: &scriptedAuthClient{},
			grants: map[string]map[string]bool{
				dueno:      {"documentos.create": true, "documentos.delete": true},
				sinPermiso: {"documentos.create": true},
			},
		},
		ExpedientesClient: &fakeExpedientesClient{expedienteID: expedienteID, solicitanteID: dueno},
	}
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.AuthenticationInterceptor(svcCtx.JWTValidator)))
	documentos.RegisterDocumentosServer(grpcServer, server.NewDocumentosServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := documentos.NewDocumentosClient(conn)

	upload := func(t *testing.T) string {
		t.Helper()
		resp, err := client.UploadDocumento(authContext(dueno, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID, Nombre: "d.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("upload: %v", err)
		}
		return resp.Documento.Id
	}
	estado := func(id string) string {
		var e string
		if err := pool.QueryRow(ctx, "SELECT estado FROM documentos WHERE id = $1", id).Scan(&e); err != nil {
			t.Fatalf("estado: %v", err)
		}
		return e
	}

	t.Run("CasoA_ConPermisoSinRolInterno_Puede", func(t *testing.T) {
		id := upload(t)
		if _, err := client.DeleteDocumento(authContext(dueno, "SOLICITANTE"), &documentos.DeleteDocumentoRequest{Id: id}); err != nil {
			t.Fatalf("con documentos.delete debía poder aunque el rol no sea interno: %v", err)
		}
		if estado(id) == "ACTIVO" {
			t.Fatal("se esperaba la baja lógica")
		}
	})

	t.Run("CasoB_SinPermiso_403_SinMutacion", func(t *testing.T) {
		id := upload(t)
		for _, c := range []struct{ user, role string }{{sinPermiso, "SOLICITANTE"}, {adminSin, "ADMIN"}} {
			_, err := client.DeleteDocumento(authContext(c.user, c.role), &documentos.DeleteDocumentoRequest{Id: id})
			if status.Code(err) != codes.PermissionDenied {
				t.Fatalf("%s sin documentos.delete: se esperaba PermissionDenied (ni siquiera ADMIN por ser ADMIN), got %v", c.role, err)
			}
		}
		if estado(id) != "ACTIVO" {
			t.Fatal("el documento no debía eliminarse")
		}
	})
}
