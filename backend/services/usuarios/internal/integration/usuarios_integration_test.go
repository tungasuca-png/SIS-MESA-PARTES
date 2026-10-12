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

// failingAuthClient simula "Auth no disponible": HasPermission siempre
// devuelve un error de comunicación (nunca allowed=true ni
// PermissionDenied) — para probar que el fallo se propaga (fail-closed)
// en vez de permitir la operación.
type failingAuthClient struct{ *scriptedAuthClient }

func (c *failingAuthClient) HasPermission(context.Context, *authclient.HasPermissionRequest, ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return nil, status.Error(codes.Unavailable, "auth service no disponible (simulado)")
}

// scriptedAuthClient es un doble de Auth SOLO para TestUsuariosIntegration:
// esa suite usa identidades sintéticas (testUUID) inexistentes en auth_db,
// así que un Auth real respondería siempre allowed=false. NO conoce ningún
// catálogo rol->permiso: responde allowed=true salvo que el propio test
// declare una negación puntual (userID -> permiso). Qué permisos tiene
// realmente cada rol se prueba contra el Auth Service real.
type scriptedAuthClient struct {
	denied map[string]map[string]bool
}

func (c *scriptedAuthClient) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	return &authclient.HasPermissionResponse{Allowed: !c.denied[in.UserId][in.Permission]}, nil
}

func (c *scriptedAuthClient) Ping(context.Context, *authclient.Request, ...grpc.CallOption) (*authclient.Response, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *scriptedAuthClient) Register(context.Context, *authclient.RegisterRequest, ...grpc.CallOption) (*authclient.RegisterResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *scriptedAuthClient) Login(context.Context, *authclient.LoginRequest, ...grpc.CallOption) (*authclient.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *scriptedAuthClient) RefreshToken(context.Context, *authclient.RefreshTokenRequest, ...grpc.CallOption) (*authclient.RefreshTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *scriptedAuthClient) Logout(context.Context, *authclient.LogoutRequest, ...grpc.CallOption) (*authclient.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}
func (c *scriptedAuthClient) ValidateToken(context.Context, *authclient.ValidateTokenRequest, ...grpc.CallOption) (*authclient.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// ListRoles/ListPermissions/GetRolePermissions (Paso 21B): añadidos a
// authclient.Auth -- no usados por esta suite, solo necesarios para que
// este doble siga satisfaciendo la interfaz completa.
func (c *scriptedAuthClient) ListRoles(context.Context, *authclient.ListRolesRequest, ...grpc.CallOption) (*authclient.ListRolesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) ListPermissions(context.Context, *authclient.ListPermissionsRequest, ...grpc.CallOption) (*authclient.ListPermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) GetRolePermissions(context.Context, *authclient.GetRolePermissionsRequest, ...grpc.CallOption) (*authclient.GetRolePermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// UpdateRolePermissions (Paso 21C): añadido a authclient.Auth -- no usado
// por esta suite, solo necesario para que este doble siga satisfaciendo la
// interfaz completa.
func (c *scriptedAuthClient) UpdateRolePermissions(context.Context, *authclient.UpdateRolePermissionsRequest, ...grpc.CallOption) (*authclient.UpdateRolePermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

// GetMyPermissions: añadido a authclient.Auth -- no usado por esta suite,
// solo necesario para que este doble siga satisfaciendo la interfaz
// completa.
func (c *scriptedAuthClient) GetMyPermissions(context.Context, *authclient.GetMyPermissionsRequest, ...grpc.CallOption) (*authclient.GetMyPermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) UpdateUserRole(context.Context, *authclient.UpdateUserRoleRequest, ...grpc.CallOption) (*authclient.UpdateUserRoleResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func (c *scriptedAuthClient) CreateUser(context.Context, *authclient.CreateUserRequest, ...grpc.CallOption) (*authclient.CreateUserResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no usado en esta prueba")
}

func TestUsuariosIntegration(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, integrationDatabaseURL(t))
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

	// Auth simulado sin catálogo: permite todo salvo lo que este escenario
	// declara explícitamente — el solicitante no tiene permisos de usuarios.
	denied := map[string]map[string]bool{
		solicitanteID: {"usuarios.view": true, "usuarios.change_role": true},
	}

	svcCtx := &svc.ServiceContext{
		Config:            config.Config{JWTSecret: testSecret},
		DB:                pool,
		UsuarioRepository: repository.NewUsuarioRepository(pool),
		JWTValidator:      security.NewJWTValidator(testSecret),
		// Esta suite usa identidades sintéticas que no existen en auth_db
		// — ver scriptedAuthClient más arriba. El flujo real contra Auth
		// Service se prueba por separado (ver informe del Paso 16B).
		AuthClient: &scriptedAuthClient{denied: denied},
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
	// El DIRECTOR de este escenario no tiene permisos de usuarios (solo lo tendría un rol al que ADMIN se los asigne).
	denied[directorID] = map[string]bool{"usuarios.view": true, "usuarios.change_role": true}
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

	// --- UpdateUsuarioRole (sincronización Usuario -> Rol): solo refleja
	// tipo_usuario, nunca decide el rol ni toca el resto del perfil. ---
	if _, err := client.UpdateUsuarioRole(ctxAdmin, &usuarios.UpdateUsuarioRoleRequest{
		Id: solicitanteID, TipoUsuario: "DOCENTE",
	}); err != nil {
		t.Fatalf("sync tipo_usuario como ADMIN: %v", err)
	}
	afterSync, err := client.GetUsuario(ctxAdmin, &usuarios.GetUsuarioRequest{Id: solicitanteID})
	if err != nil {
		t.Fatalf("get usuario tras sincronizar: %v", err)
	}
	if afterSync.Usuario.TipoUsuario != "DOCENTE" {
		t.Fatalf("esperado tipo_usuario=DOCENTE tras sync, got %q", afterSync.Usuario.TipoUsuario)
	}
	if afterSync.Usuario.Telefono != "999000111" {
		t.Fatalf("UpdateUsuarioRole no debe tocar otros campos del perfil, telefono cambió a %q", afterSync.Usuario.Telefono)
	}

	// Sin "usuarios.change_role" (SOLICITANTE): PermissionDenied, sin tocar
	// el perfil.
	if _, err := client.UpdateUsuarioRole(ctxSolicitante, &usuarios.UpdateUsuarioRoleRequest{
		Id: solicitanteID, TipoUsuario: "AUXILIAR",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied sincronizando sin usuarios.change_role, got %v", err)
	}
	unchanged, err := client.GetUsuario(ctxAdmin, &usuarios.GetUsuarioRequest{Id: solicitanteID})
	if err != nil {
		t.Fatalf("get usuario tras intento sin permiso: %v", err)
	}
	if unchanged.Usuario.TipoUsuario != "DOCENTE" {
		t.Fatalf("el perfil no debía cambiar sin permiso, got tipo_usuario=%q", unchanged.Usuario.TipoUsuario)
	}

	// Usuario sin perfil registrado en Usuarios Service: NotFound (no hay
	// nada que sincronizar, no es un fallo de este RPC).
	if _, err := client.UpdateUsuarioRole(ctxAdmin, &usuarios.UpdateUsuarioRoleRequest{
		Id: otroSolicitanteID, TipoUsuario: "DOCENTE",
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound sincronizando un usuario sin perfil, got %v", err)
	}

	// tipo_usuario inválido: InvalidArgument, sin tocar el perfil.
	if _, err := client.UpdateUsuarioRole(ctxAdmin, &usuarios.UpdateUsuarioRoleRequest{
		Id: solicitanteID, TipoUsuario: "NO_EXISTE",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument para tipo_usuario inválido, got %v", err)
	}

	// --- Auth no disponible: fail-closed, nunca se permite la operación
	// protegida por un error de comunicación con Auth. ---
	downSvcCtx := &svc.ServiceContext{
		Config:            config.Config{JWTSecret: testSecret},
		DB:                pool,
		UsuarioRepository: repository.NewUsuarioRepository(pool),
		JWTValidator:      security.NewJWTValidator(testSecret),
		AuthClient:        &failingAuthClient{&scriptedAuthClient{denied: denied}},
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
	if _, err := downClient.UpdateUsuarioRole(ctxAdmin, &usuarios.UpdateUsuarioRoleRequest{Id: solicitanteID, TipoUsuario: "DOCENTE"}); err == nil {
		t.Fatal("se esperaba un error (fail-closed) en UpdateUsuarioRole cuando Auth Service no está disponible, la operación se permitió")
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

func integrationDatabaseURL(t *testing.T) string {
	t.Helper()
	// Credenciales de PRUEBA desde el entorno (backend/.env.test, fuera de
	// Git). Sin la variable la prueba se omite: nunca hay credenciales
	// escritas en el código ni un superusuario de respaldo.
	url := os.Getenv("TEST_USUARIOS_DB_URL")
	if url == "" {
		t.Skip("TEST_USUARIOS_DB_URL no definido: cargar backend/.env.test para ejecutar esta prueba")
	}
	return url
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
