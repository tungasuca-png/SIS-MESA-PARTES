package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"expedientes/expedientes"
	"expedientes/internal/config"
	"expedientes/internal/interceptor"
	"expedientes/internal/repository"
	"expedientes/internal/security"
	"expedientes/internal/server"
	"expedientes/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testSecret = "expedientes-integration-test-secret"

func TestExpedientesIntegration(t *testing.T) {
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

	testID := time.Now().UnixNano()
	solicitanteID := fixedUUID(testID, 1)
	otroSolicitanteID := fixedUUID(testID, 2)
	defer cleanupIntegrationData(t, pool, solicitanteID, otroSolicitanteID)

	svcCtx := &svc.ServiceContext{
		Config:               config.Config{JWTSecret: testSecret},
		DB:                   pool,
		ExpedienteRepository: repository.NewExpedienteRepository(pool),
		JWTValidator:         security.NewJWTValidator(testSecret),
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(svcCtx.JWTValidator),
	))
	expedientes.RegisterExpedientesServer(grpcServer, server.NewExpedientesServer(svcCtx))
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
	client := expedientes.NewExpedientesClient(conn)

	ctxSolicitante := authContext(solicitanteID, "SOLICITANTE")
	ctxOtroSolicitante := authContext(otroSolicitanteID, "SOLICITANTE")
	ctxAdmin := authContext(fixedUUID(testID, 3), "ADMIN")

	// --- Crear expediente ---
	createResp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Asunto: "Solicitud de certificado de estudios",
	})
	if err != nil {
		t.Fatalf("create expediente: %v", err)
	}
	if createResp.Expediente.Estado != "PENDIENTE" {
		t.Fatalf("expected initial estado PENDIENTE, got %q", createResp.Expediente.Estado)
	}
	if createResp.Expediente.Tipo != "SOLICITUD" || createResp.Expediente.Prioridad != "NORMAL" {
		t.Fatalf("unexpected defaults: tipo=%q prioridad=%q", createResp.Expediente.Tipo, createResp.Expediente.Prioridad)
	}
	if !strings.HasPrefix(createResp.Expediente.Codigo, "EXP-") {
		t.Fatalf("unexpected codigo format: %s", createResp.Expediente.Codigo)
	}

	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for missing asunto, got %v", err)
	}
	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Asunto: "x", Tipo: "INVALIDO"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for tipo inválido, got %v", err)
	}
	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Asunto: "x", Prioridad: "ALTA"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for prioridad inválida, got %v", err)
	}
	if _, err := client.CreateExpediente(context.Background(), &expedientes.CreateExpedienteRequest{Asunto: "x"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without token, got %v", err)
	}

	// --- Código único ---
	createResp2, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Asunto: "Otro expediente"})
	if err != nil {
		t.Fatalf("create second expediente: %v", err)
	}
	if createResp2.Expediente.Codigo == createResp.Expediente.Codigo {
		t.Fatalf("expected unique codigo, got duplicate %s", createResp.Expediente.Codigo)
	}

	// --- Consultar ---
	getResp, err := client.GetExpediente(ctxSolicitante, &expedientes.GetExpedienteRequest{Id: createResp.Expediente.Codigo})
	if err != nil {
		t.Fatalf("get own expediente by codigo: %v", err)
	}
	if getResp.Expediente.Id != createResp.Expediente.Id {
		t.Fatalf("expected same expediente by codigo lookup")
	}

	if _, err := client.GetExpediente(ctxOtroSolicitante, &expedientes.GetExpedienteRequest{Id: createResp.Expediente.Id}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for foreign expediente, got %v", err)
	}
	if _, err := client.GetExpediente(ctxAdmin, &expedientes.GetExpedienteRequest{Id: "no-existe-" + time.Now().String()}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound for missing expediente, got %v", err)
	}

	// --- Autorización de actualización ---
	if _, err := client.UpdateExpediente(ctxSolicitante, &expedientes.UpdateExpedienteRequest{Id: createResp.Expediente.Id, Asunto: "otro asunto"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for solicitante update, got %v", err)
	}
	updateResp, err := client.UpdateExpediente(ctxAdmin, &expedientes.UpdateExpedienteRequest{
		Id:        createResp.Expediente.Id,
		Asunto:    "Solicitud de certificado (actualizado)",
		Prioridad: "URGENTE",
	})
	if err != nil {
		t.Fatalf("update as admin: %v", err)
	}
	if updateResp.Expediente.Prioridad != "URGENTE" {
		t.Fatalf("expected priority updated to URGENTE, got %q", updateResp.Expediente.Prioridad)
	}

	// --- Autorización y transición de estados ---
	if _, err := client.ChangeEstado(ctxSolicitante, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "EN_PROCESO"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for solicitante change estado, got %v", err)
	}
	if _, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "ATENDIDO"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition for PENDIENTE->ATENDIDO, got %v", err)
	}
	changeResp, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "EN_PROCESO"})
	if err != nil {
		t.Fatalf("change estado to EN_PROCESO: %v", err)
	}
	if changeResp.Expediente.Estado != "EN_PROCESO" {
		t.Fatalf("expected EN_PROCESO, got %q", changeResp.Expediente.Estado)
	}
	if _, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "ATENDIDO"}); err != nil {
		t.Fatalf("change estado to ATENDIDO: %v", err)
	}
	if _, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "PENDIENTE"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition for ATENDIDO->PENDIENTE, got %v", err)
	}

	// --- Listado con alcance por rol ---
	listAdmin, err := client.ListExpedientes(ctxAdmin, &expedientes.ListExpedientesRequest{})
	if err != nil {
		t.Fatalf("list as admin: %v", err)
	}
	if listAdmin.Total < 2 {
		t.Fatalf("expected at least 2 expedientes visible to admin, got %d", listAdmin.Total)
	}

	listSolicitante, err := client.ListExpedientes(ctxSolicitante, &expedientes.ListExpedientesRequest{})
	if err != nil {
		t.Fatalf("list as solicitante: %v", err)
	}
	for _, e := range listSolicitante.Expedientes {
		if e.SolicitanteId != solicitanteID {
			t.Fatalf("solicitante should only see its own expedientes, saw solicitante_id=%s", e.SolicitanteId)
		}
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

// fixedUUID produce un UUID v4-like determinístico a partir del testID y un
// índice, únicamente para tener identificadores estables y DISTINTOS dentro
// de una misma corrida de test (no representa usuarios reales de Auth
// Service). Se acota explícitamente para no desbordar el campo de 12 dígitos
// hex del último grupo del UUID.
func fixedUUID(testID int64, index int) string {
	value := (testID%10_000_000_000)*10 + int64(index)
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", value)
}

func integrationDatabaseURL() string {
	if value := os.Getenv("EXPEDIENTES_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://expedientes_user:expedientes_password@127.0.0.1:5433/expedientes_db?sslmode=disable"
}

func cleanupIntegrationData(t *testing.T, pool *pgxpool.Pool, solicitanteIDs ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, id := range solicitanteIDs {
		if _, err := pool.Exec(ctx, "DELETE FROM expedientes WHERE solicitante_id = $1", id); err != nil {
			t.Logf("cleanup expedientes for %s: %v", id, err)
		}
	}
}
