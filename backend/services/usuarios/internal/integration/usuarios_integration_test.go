package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"usuarios/internal/config"
	"usuarios/internal/interceptor"
	"usuarios/internal/repository"
	"usuarios/internal/security"
	"usuarios/internal/server"
	"usuarios/internal/svc"
	"usuarios/usuarios"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const testSecret = "usuarios-integration-test-secret"

func TestUsuariosIntegration(t *testing.T) {
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
	adminID := testUUID(testID, 1)
	solicitanteID := testUUID(testID, 2)
	otroSolicitanteID := testUUID(testID, 3)
	defer cleanup(t, pool, adminID, solicitanteID, otroSolicitanteID)

	svcCtx := &svc.ServiceContext{
		Config:            config.Config{JWTSecret: testSecret},
		DB:                pool,
		UsuarioRepository: repository.NewUsuarioRepository(pool),
		JWTValidator:      security.NewJWTValidator(testSecret),
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(svcCtx.JWTValidator),
	))
	usuarios.RegisterUsuariosServer(grpcServer, server.NewUsuariosServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC test server: %v", err)
	}
	defer conn.Close()
	client := usuarios.NewUsuariosClient(conn)

	ctxAdmin := authContext(adminID, "ADMIN")
	ctxSolicitante := authContext(solicitanteID, "SOLICITANTE")

	// --- Sin JWT ---
	if _, err := client.GetUsuarioBasic(context.Background(), &usuarios.GetUsuarioBasicRequest{Id: adminID}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without JWT, got %v", err)
	}

	// --- Upsert: solo ADMIN ---
	if _, err := client.UpsertUsuario(ctxSolicitante, &usuarios.UpsertUsuarioRequest{
		Id: solicitanteID, Nombres: "X", Apellidos: "Y", TipoUsuario: "SOLICITANTE",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for solicitante upsert, got %v", err)
	}

	created, err := client.UpsertUsuario(ctxAdmin, &usuarios.UpsertUsuarioRequest{
		Id:          solicitanteID,
		Nombres:     "TEST USER Juan",
		Apellidos:   "TEST Pérez",
		Dni:         fmt.Sprintf("%08d", testID%100000000),
		Telefono:    "999000111",
		Correo:      "test.integration@test.local",
		Direccion:   "Dirección de prueba",
		TipoUsuario: "SOLICITANTE",
	})
	if err != nil {
		t.Fatalf("upsert usuario: %v", err)
	}
	if created.Usuario.Id != solicitanteID {
		t.Fatalf("expected the Auth UUID to be preserved, got %s", created.Usuario.Id)
	}

	// --- UUID inválido ---
	if _, err := client.GetUsuario(ctxAdmin, &usuarios.GetUsuarioRequest{Id: "no-es-uuid"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for malformed UUID, got %v", err)
	}

	// --- Usuario inexistente ---
	if _, err := client.GetUsuario(ctxAdmin, &usuarios.GetUsuarioRequest{Id: otroSolicitanteID}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound for missing usuario, got %v", err)
	}

	// --- GetUsuario (perfil completo) ---
	full, err := client.GetUsuario(ctxAdmin, &usuarios.GetUsuarioRequest{Id: solicitanteID})
	if err != nil {
		t.Fatalf("get usuario as admin: %v", err)
	}
	if full.Usuario.Telefono != "999000111" || full.Usuario.Direccion == "" {
		t.Fatalf("expected full profile for admin, got %+v", full.Usuario)
	}

	// El solicitante puede ver su propio perfil completo...
	if _, err := client.GetUsuario(ctxSolicitante, &usuarios.GetUsuarioRequest{Id: solicitanteID}); err != nil {
		t.Fatalf("solicitante should read its own full profile: %v", err)
	}
	// ...pero no el de otro usuario.
	if _, err := client.GetUsuario(ctxSolicitante, &usuarios.GetUsuarioRequest{Id: adminID}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for foreign full profile, got %v", err)
	}

	// --- GetUsuarioBasic: cualquiera autenticado resuelve el nombre ---
	basic, err := client.GetUsuarioBasic(ctxSolicitante, &usuarios.GetUsuarioBasicRequest{Id: solicitanteID})
	if err != nil {
		t.Fatalf("get usuario basic: %v", err)
	}
	if basic.Usuario.NombreCompleto != "TEST USER Juan TEST Pérez" {
		t.Fatalf("unexpected nombre_completo: %q", basic.Usuario.NombreCompleto)
	}

	// Un solicitante que consulta a OTRO usuario no debe recibir el DNI.
	if _, err := client.UpsertUsuario(ctxAdmin, &usuarios.UpsertUsuarioRequest{
		Id: adminID, Nombres: "TEST USER Admin", Apellidos: "TEST Admin", TipoUsuario: "ADMIN",
		Dni: fmt.Sprintf("%08d", (testID+1)%100000000),
	}); err != nil {
		t.Fatalf("upsert admin profile: %v", err)
	}
	foreignBasic, err := client.GetUsuarioBasic(ctxSolicitante, &usuarios.GetUsuarioBasicRequest{Id: adminID})
	if err != nil {
		t.Fatalf("solicitante should resolve another user's basic data: %v", err)
	}
	if foreignBasic.Usuario.Dni != "" {
		t.Fatal("basic projection leaked DNI to an unauthorized caller")
	}
	if foreignBasic.Usuario.NombreCompleto == "" {
		t.Fatal("expected nombre_completo to be resolved")
	}

	// --- GetUsuariosBasic (lote) ---
	batch, err := client.GetUsuariosBasic(ctxSolicitante, &usuarios.GetUsuariosBasicRequest{
		Ids: []string{solicitanteID, adminID, otroSolicitanteID, solicitanteID},
	})
	if err != nil {
		t.Fatalf("batch lookup: %v", err)
	}
	if len(batch.Usuarios) != 2 {
		t.Fatalf("expected 2 resolved users (duplicates collapsed, missing omitted), got %d", len(batch.Usuarios))
	}
	if _, err := client.GetUsuariosBasic(ctxSolicitante, &usuarios.GetUsuariosBasicRequest{Ids: []string{"no-es-uuid"}}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for malformed UUID in batch, got %v", err)
	}

	// --- ListUsuarios / SearchUsuarios: solo ADMIN ---
	if _, err := client.ListUsuarios(ctxSolicitante, &usuarios.ListUsuariosRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied listing as solicitante, got %v", err)
	}
	list, err := client.ListUsuarios(ctxAdmin, &usuarios.ListUsuariosRequest{TipoUsuario: "SOLICITANTE"})
	if err != nil {
		t.Fatalf("list usuarios as admin: %v", err)
	}
	if list.Total < 1 {
		t.Fatalf("expected at least one SOLICITANTE, got %d", list.Total)
	}

	if _, err := client.SearchUsuarios(ctxAdmin, &usuarios.SearchUsuariosRequest{Query: "ab"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for too-short query, got %v", err)
	}
	search, err := client.SearchUsuarios(ctxAdmin, &usuarios.SearchUsuariosRequest{Query: "TEST USER Juan"})
	if err != nil {
		t.Fatalf("search usuarios: %v", err)
	}
	if search.Total < 1 {
		t.Fatalf("expected search to find the test user, got %d", search.Total)
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

// testUUID genera identificadores estables y distintos por corrida; simula
// los UUID que Auth Service asignaría, sin representar usuarios reales.
func testUUID(testID int64, index int) string {
	value := (testID%10_000_000_000)*10 + int64(index)
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", value)
}

func integrationDatabaseURL() string {
	if value := os.Getenv("USUARIOS_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://usuarios_user:usuarios_password@127.0.0.1:5433/usuarios_db?sslmode=disable"
}

func cleanup(t *testing.T, pool *pgxpool.Pool, ids ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, id := range ids {
		if _, err := pool.Exec(ctx, "DELETE FROM usuarios WHERE id = $1", id); err != nil {
			t.Logf("cleanup usuario %s: %v", id, err)
		}
	}
}
