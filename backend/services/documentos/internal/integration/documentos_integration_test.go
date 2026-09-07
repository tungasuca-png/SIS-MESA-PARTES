package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"documentos/documentos"
	"documentos/internal/config"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/security"
	"documentos/internal/server"
	"documentos/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testSecret = "documentos-integration-test-secret"

func TestDocumentosIntegration(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, integrationDatabaseURL())
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}

	expedienteID := fixedUUID(time.Now().UnixNano(), 1)
	defer cleanupIntegrationData(t, pool, expedienteID)

	svcCtx := &svc.ServiceContext{
		Config:              config.Config{JWTSecret: testSecret},
		DB:                  pool,
		DocumentoRepository: repository.NewDocumentoRepository(pool),
		JWTValidator:        security.NewJWTValidator(testSecret),
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(svcCtx.JWTValidator),
	))
	documentos.RegisterDocumentosServer(grpcServer, server.NewDocumentosServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()

	clientCtx := context.Background()
	conn, err := grpc.DialContext(clientCtx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC test server: %v", err)
	}
	defer conn.Close()
	client := documentos.NewDocumentosClient(conn)

	ctxSolicitante := authContext(fixedUUID(time.Now().UnixNano(), 2), "SOLICITANTE")
	ctxAdmin := authContext(fixedUUID(time.Now().UnixNano(), 3), "ADMIN")
	contenido := []byte("%PDF-1.4 contenido de prueba")

	// --- Subir documento como SOLICITANTE (ADJUNTO permitido) ---
	uploadResp, err := client.UploadDocumento(ctxSolicitante, &documentos.UploadDocumentoRequest{
		ExpedienteId:  expedienteID,
		Nombre:        "dni-solicitante.pdf",
		TipoDocumento: "ADJUNTO",
		Extension:     "pdf",
		Contenido:     contenido,
	})
	if err != nil {
		t.Fatalf("upload as solicitante: %v", err)
	}
	if uploadResp.Documento.TamanoBytes != int64(len(contenido)) {
		t.Fatalf("expected tamano_bytes=%d, got %d", len(contenido), uploadResp.Documento.TamanoBytes)
	}

	// --- SOLICITANTE no puede subir PROVEIDO ---
	if _, err := client.UploadDocumento(ctxSolicitante, &documentos.UploadDocumentoRequest{
		ExpedienteId: expedienteID, Nombre: "x.pdf", TipoDocumento: "PROVEIDO", Extension: "pdf", Contenido: contenido,
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}

	// --- Validaciones ---
	if _, err := client.UploadDocumento(ctxSolicitante, &documentos.UploadDocumentoRequest{
		ExpedienteId: expedienteID, Nombre: "", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: contenido,
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for empty nombre, got %v", err)
	}
	if _, err := client.UploadDocumento(ctxSolicitante, &documentos.UploadDocumentoRequest{
		ExpedienteId: expedienteID, Nombre: "x.exe", TipoDocumento: "ADJUNTO", Extension: "exe", Contenido: contenido,
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for invalid extension, got %v", err)
	}
	if _, err := client.UploadDocumento(context.Background(), &documentos.UploadDocumentoRequest{
		ExpedienteId: expedienteID, Nombre: "x.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: contenido,
	}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without token, got %v", err)
	}

	// --- Personal interno sube un PROVEIDO ---
	proveidoResp, err := client.UploadDocumento(ctxAdmin, &documentos.UploadDocumentoRequest{
		ExpedienteId: expedienteID, Nombre: "proveido.pdf", TipoDocumento: "PROVEIDO", Extension: "pdf", Contenido: contenido,
	})
	if err != nil {
		t.Fatalf("upload proveido as admin: %v", err)
	}

	// --- Consultar metadata ---
	getResp, err := client.GetDocumento(ctxAdmin, &documentos.GetDocumentoRequest{Id: uploadResp.Documento.Id})
	if err != nil {
		t.Fatalf("get documento: %v", err)
	}
	if getResp.Documento.Nombre != "dni-solicitante.pdf" {
		t.Fatalf("unexpected nombre: %s", getResp.Documento.Nombre)
	}
	if _, err := client.GetDocumento(ctxAdmin, &documentos.GetDocumentoRequest{Id: fixedUUID(1, 9)}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound for missing documento, got %v", err)
	}

	// --- Descargar contenido ---
	downloadResp, err := client.DownloadDocumento(ctxAdmin, &documentos.DownloadDocumentoRequest{Id: uploadResp.Documento.Id})
	if err != nil {
		t.Fatalf("download documento: %v", err)
	}
	if string(downloadResp.Contenido) != string(contenido) {
		t.Fatalf("downloaded content does not match uploaded content")
	}

	// --- Listar por expediente ---
	listResp, err := client.ListDocumentos(ctxAdmin, &documentos.ListDocumentosRequest{ExpedienteId: expedienteID})
	if err != nil {
		t.Fatalf("list documentos: %v", err)
	}
	if len(listResp.Documentos) != 2 {
		t.Fatalf("expected 2 documentos, got %d", len(listResp.Documentos))
	}

	// --- Eliminar: SOLICITANTE no puede, ADMIN si ---
	if _, err := client.DeleteDocumento(ctxSolicitante, &documentos.DeleteDocumentoRequest{Id: proveidoResp.Documento.Id}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for solicitante delete, got %v", err)
	}
	if _, err := client.DeleteDocumento(ctxAdmin, &documentos.DeleteDocumentoRequest{Id: proveidoResp.Documento.Id}); err != nil {
		t.Fatalf("delete as admin: %v", err)
	}

	listAfterDelete, err := client.ListDocumentos(ctxAdmin, &documentos.ListDocumentosRequest{ExpedienteId: expedienteID})
	if err != nil {
		t.Fatalf("list documentos after delete: %v", err)
	}
	if len(listAfterDelete.Documentos) != 1 {
		t.Fatalf("expected 1 documento activo tras baja logica, got %d", len(listAfterDelete.Documentos))
	}
	if _, err := client.GetDocumento(ctxAdmin, &documentos.GetDocumentoRequest{Id: proveidoResp.Documento.Id}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound for soft-deleted documento, got %v", err)
	}
}

func authContext(userID, role string) context.Context {
	claims := security.Claims{
		Username: "integration-test",
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

func fixedUUID(testID int64, index int) string {
	value := (testID%10_000_000_000)*10 + int64(index)
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", value)
}

func integrationDatabaseURL() string {
	if value := os.Getenv("DOCUMENTOS_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://documentos_user:documentos_password@127.0.0.1:5433/documentos_db?sslmode=disable"
}

func cleanupIntegrationData(t *testing.T, pool *pgxpool.Pool, expedienteID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "DELETE FROM documentos WHERE expediente_id = $1", expedienteID); err != nil {
		t.Logf("cleanup documentos: %v", err)
	}
}
