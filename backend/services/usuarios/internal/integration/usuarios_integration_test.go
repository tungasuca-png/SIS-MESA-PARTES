package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"auth/authclient"
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

// rolePermissionsReplica es una copia de la MISMA matriz rol->permiso ya
// sembrada en auth_db por la migración
// 005_seed_permissions_and_role_permissions.sql (Paso 9) — no se inventa
// ninguna regla nueva acá, solo se refleja la existente para poder probar
// esta suite (identidades sintéticas que no existen en auth_db) sin
// depender de un Auth Service real corriendo. Mismo patrón ya usado en
// Expedientes/Documentos/Derivaciones. Solo se listan los permisos que
// esta suite necesita distinguir (usuarios.view: ADMIN sí, SOLICITANTE no).
var rolePermissionsReplica = map[string]map[string]bool{
	"ADMIN":       {"usuarios.view": true},
	"SOLICITANTE": {},
	// DIRECTOR es personal interno (CanViewFullProfile/IsInternal lo
	// permitiría solo), pero el catálogo real (migración 005) NO le da
	// usuarios.view — solo ADMIN lo tiene. Se usa para probar que, tras
	// este paso, un interno no-ADMIN queda DENEGADO en GetUsuario sobre
	// el perfil de otro (cambio de comportamiento real, ver informe).
	"DIRECTOR": {},
}

// failingAuthClient simula "Auth no disponible": HasPermission siempre
// devuelve un error de comunicación (nunca allowed=true ni
// PermissionDenied) — para probar que el fallo se propaga (fail-closed)
// en vez de permitir la operación.
type failingAuthClient struct{ *replicaAuthClient }

func (c *failingAuthClient) HasPermission(context.Context, *authclient.HasPermissionRequest, ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return nil, status.Error(codes.Unavailable, "auth service no disponible (simulado)")
}

// replicaAuthClient es un doble de prueba SOLO para TestUsuariosIntegration:
// esa suite usa identidades sintéticas (testUUID) que nunca existen en
// auth_db, así que un AuthClient real respondería siempre allowed=false
// por "usuario inexistente" — impidiendo probar las reglas de negocio
// (CanViewFullProfile/CanListOrSearch/CanUpsert) que esta suite en
// realidad verifica. En vez de "permitir siempre", este doble lee el
// MISMO JWT que ya autenticó la llamada (reenviado como metadata saliente
// por authorization.HasPermission/RequirePermission) y aplica
// rolePermissionsReplica — la misma matriz real, no una regla inventada.
type replicaAuthClient struct {
	validator *security.JWTValidator
}

func (c *replicaAuthClient) HasPermission(ctx context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	values, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	authValues := values.Get("authorization")
	if len(authValues) == 0 {
		return nil, status.Error(codes.Unauthenticated, "falta el token de acceso")
	}
	parts := strings.Fields(authValues[0])
	if len(parts) != 2 {
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	claims, err := c.validator.Validate(parts[1])
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "token de acceso inválido")
	}
	return &authclient.HasPermissionResponse{Allowed: rolePermissionsReplica[claims.Role][in.Permission]}, nil
}

func (c *replicaAuthClient) Ping(context.Context, *authclient.Request, ...grpc.CallOption) (*authclient.Response, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *replicaAuthClient) Register(context.Context, *authclient.RegisterRequest, ...grpc.CallOption) (*authclient.RegisterResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *replicaAuthClient) Login(context.Context, *authclient.LoginRequest, ...grpc.CallOption) (*authclient.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *replicaAuthClient) RefreshToken(context.Context, *authclient.RefreshTokenRequest, ...grpc.CallOption) (*authclient.RefreshTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *replicaAuthClient) Logout(context.Context, *authclient.LogoutRequest, ...grpc.CallOption) (*authclient.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *replicaAuthClient) ValidateToken(context.Context, *authclient.ValidateTokenRequest, ...grpc.CallOption) (*authclient.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// ListRoles/ListPermissions/GetRolePermissions (Paso 21B): añadidos a
// authclient.Auth -- no usados por esta suite, solo necesarios para que
// este doble siga satisfaciendo la interfaz completa.
func (c *replicaAuthClient) ListRoles(context.Context, *authclient.ListRolesRequest, ...grpc.CallOption) (*authclient.ListRolesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) ListPermissions(context.Context, *authclient.ListPermissionsRequest, ...grpc.CallOption) (*authclient.ListPermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *replicaAuthClient) GetRolePermissions(context.Context, *authclient.GetRolePermissionsRequest, ...grpc.CallOption) (*authclient.GetRolePermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// UpdateRolePermissions (Paso 21C): añadido a authclient.Auth -- no usado
// por esta suite, solo necesario para que este doble siga satisfaciendo la
// interfaz completa.
func (c *replicaAuthClient) UpdateRolePermissions(context.Context, *authclient.UpdateRolePermissionsRequest, ...grpc.CallOption) (*authclient.UpdateRolePermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

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
		// Esta suite usa identidades sintéticas que no existen en auth_db
		// — ver replicaAuthClient más arriba. El flujo real contra Auth
		// Service se prueba por separado (ver informe del Paso 16B).
		AuthClient: &replicaAuthClient{validator: security.NewJWTValidator(testSecret)},
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

	// --- GetUsuario: interno no-ADMIN (DIRECTOR) queda denegado sobre el
	// perfil de OTRO usuario — cambio de comportamiento real de este paso:
	// antes CanViewFullProfile (IsInternal) alcanzaba; ahora también se
	// exige usuarios.view, y el catálogo real solo se lo da a ADMIN. ---
	directorID := testUUID(testID, 4)
	defer cleanup(t, pool, directorID)
	if _, err := client.UpsertUsuario(ctxAdmin, &usuarios.UpsertUsuarioRequest{
		Id: directorID, Nombres: "TEST USER Director", Apellidos: "TEST Director", TipoUsuario: "DIRECTOR",
	}); err != nil {
		t.Fatalf("upsert director profile: %v", err)
	}
	ctxDirector := authContext(directorID, "DIRECTOR")
	if _, err := client.GetUsuario(ctxDirector, &usuarios.GetUsuarioRequest{Id: solicitanteID}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for DIRECTOR (interno sin usuarios.view) viewing another profile, got %v", err)
	}
	// GetUsuarioBasic/GetUsuariosBasic NO dependen de usuarios.view: DIRECTOR
	// (sin ese permiso) sigue pudiendo resolverlos igual que cualquiera.
	if _, err := client.GetUsuarioBasic(ctxDirector, &usuarios.GetUsuarioBasicRequest{Id: solicitanteID}); err != nil {
		t.Fatalf("GetUsuarioBasic no debe depender de usuarios.view, got err=%v", err)
	}
	if _, err := client.GetUsuariosBasic(ctxDirector, &usuarios.GetUsuariosBasicRequest{Ids: []string{solicitanteID, adminID}}); err != nil {
		t.Fatalf("GetUsuariosBasic no debe depender de usuarios.view, got err=%v", err)
	}
	// ListUsuarios/SearchUsuarios: DIRECTOR tampoco tiene usuarios.view ni
	// es ADMIN — sigue denegado por ambas reglas, igual que antes.
	if _, err := client.ListUsuarios(ctxDirector, &usuarios.ListUsuariosRequest{}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied listing as DIRECTOR, got %v", err)
	}

	// --- Auth no disponible: fail-closed, nunca se permite la operación
	// protegida por un error de comunicación con Auth. ---
	downSvcCtx := &svc.ServiceContext{
		Config:            config.Config{JWTSecret: testSecret},
		DB:                pool,
		UsuarioRepository: repository.NewUsuarioRepository(pool),
		JWTValidator:      security.NewJWTValidator(testSecret),
		AuthClient:        &failingAuthClient{&replicaAuthClient{validator: security.NewJWTValidator(testSecret)}},
	}
	downListener := bufconn.Listen(1024 * 1024)
	downServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(downSvcCtx.JWTValidator),
	))
	usuarios.RegisterUsuariosServer(downServer, server.NewUsuariosServer(downSvcCtx))
	go func() { _ = downServer.Serve(downListener) }()
	defer downServer.Stop()

	downConn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return downListener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC test server (Auth caído): %v", err)
	}
	defer downConn.Close()
	downClient := usuarios.NewUsuariosClient(downConn)

	if _, err := downClient.ListUsuarios(ctxAdmin, &usuarios.ListUsuariosRequest{}); err == nil {
		t.Fatal("se esperaba un error (fail-closed) cuando Auth Service no está disponible, la operación se permitió")
	} else if status.Code(err) == codes.OK || status.Code(err) == codes.PermissionDenied {
		t.Fatalf("se esperaba un error de comunicación distinto de PermissionDenied/OK, got %v", err)
	}
	if _, err := downClient.GetUsuario(ctxAdmin, &usuarios.GetUsuarioRequest{Id: solicitanteID}); err == nil {
		t.Fatal("se esperaba un error (fail-closed) en GetUsuario cuando Auth Service no está disponible, la operación se permitió")
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
