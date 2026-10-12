package integration

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"auth/auth"
	"auth/internal/authorization"
	"auth/internal/config"
	"auth/internal/interceptor"
	"auth/internal/repository"
	"auth/internal/security"
	"auth/internal/server"
	"auth/internal/svc"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// nonExistentUUID: formato válido, garantizado a no coincidir con ningún
// usuario/rol real sembrado en la BD de pruebas.
const nonExistentUUID = "00000000-0000-4000-8000-000000000000"

// TestUpdateUserRoleRPC prueba UpdateUserRole de punta a punta (JWT real ->
// AuthenticationInterceptor -> AuthorizationInterceptor
// ("usuarios.change_role", ya sembrado en la migración 006 y asignado
// solo a ADMIN) -> UserRepository.ReplaceUserRole transaccional ->
// PostgreSQL), usando exactamente las mismas DefaultMethodPolicies() de
// producción. Usuario -> Rol es un reemplazo (1 usuario = 1 rol): nunca
// deja dos filas en usuario_roles.
func TestUpdateUserRoleRPC(t *testing.T) {
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

	userRepo := repository.NewUserRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	rolePermissionRepo := repository.NewRolePermissionRepository(pool)
	authorizationService := authorization.NewAuthorizationService(userRepo, roleRepo, rolePermissionRepo)
	jwtManager := security.NewJWTManager("update-user-role-rpc-test-secret", time.Hour)
	svcCtx := &svc.ServiceContext{
		Config:                   config.Config{RefreshTokenDuration: 24 * time.Hour},
		DB:                       pool,
		UserRepository:           userRepo,
		RoleRepository:           roleRepo,
		PermissionRepository:     repository.NewPermissionRepository(pool),
		RolePermissionRepository: rolePermissionRepo,
		RefreshTokenRepository:   repository.NewRefreshTokenRepository(pool),
		AuthorizationService:     authorizationService,
		JWTManager:               jwtManager,
	}

	// Mismas policies que en producción: UpdateUserRole queda protegido
	// por ProtectedMethod("usuarios.change_role"), ya en el policy map.
	policies := interceptor.DefaultMethodPolicies()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(jwtManager, policies),
		interceptor.AuthorizationInterceptor(authorizationService, policies),
	))
	auth.RegisterAuthServer(grpcServer, server.NewAuthServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC test server: %v", err)
	}
	defer conn.Close()
	client := auth.NewAuthClient(conn)

	testID := strconv.FormatInt(time.Now().UnixNano()%1e8, 10)
	usernamePrefix := "uur" + testID + "_"
	var createdUserIDs []string
	var createdRoleIDs []string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, id := range createdUserIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM usuario_roles WHERE usuario_id = $1", id); err != nil {
				t.Logf("cleanup usuario_roles %s: %v", id, err)
			}
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM usuarios WHERE id = $1", id); err != nil {
				t.Logf("cleanup usuario %s: %v", id, err)
			}
		}
		for _, id := range createdRoleIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM roles WHERE id = $1", id); err != nil {
				t.Logf("cleanup rol temporal %s: %v", id, err)
			}
		}
	}()

	roleID := func(t *testing.T, roleName string) string {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, "SELECT id FROM roles WHERE nombre = $1 AND estado = TRUE", roleName).Scan(&id); err != nil {
			t.Fatalf("buscar rol %s: %v", roleName, err)
		}
		return id
	}

	createUser := func(t *testing.T, suffix string, roleNames ...string) string {
		t.Helper()
		var userID string
		err := pool.QueryRow(ctx, `
			INSERT INTO usuarios (username, email, password_hash, estado)
			VALUES ($1, $2, $3, TRUE)
			RETURNING id
		`, usernamePrefix+suffix, usernamePrefix+suffix+"@test.local", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy").Scan(&userID)
		if err != nil {
			t.Fatalf("crear usuario de prueba: %v", err)
		}
		createdUserIDs = append(createdUserIDs, userID)
		for _, roleName := range roleNames {
			if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)", userID, roleID(t, roleName)); err != nil {
				t.Fatalf("asignar rol %s: %v", roleName, err)
			}
		}
		return userID
	}

	authCtx := func(userID string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+mustToken(t, jwtManager, userID))
	}

	rolesDeUsuario := func(t *testing.T, userID string) []string {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT r.nombre FROM usuario_roles ur
			JOIN roles r ON r.id = ur.rol_id
			WHERE ur.usuario_id = $1
			ORDER BY r.nombre
		`, userID)
		if err != nil {
			t.Fatalf("consultar roles de %s: %v", userID, err)
		}
		defer rows.Close()
		names := make([]string, 0)
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				t.Fatalf("escanear rol: %v", err)
			}
			names = append(names, name)
		}
		return names
	}

	// Actor administrativo real: ADMIN ya tiene "usuarios.change_role"
	// desde la migración 006 (sembrado, no inventado en este test).
	adminID := createUser(t, "admin", "ADMIN")

	// --- Test 1: Cambio normal (SOLICITANTE -> DOCENTE) ---
	t.Run("CambioNormal", func(t *testing.T) {
		userID := createUser(t, "cambionormal", "SOLICITANTE")
		resp, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: userID,
			RoleId: roleID(t, "DOCENTE"),
		})
		if err != nil {
			t.Fatalf("UpdateUserRole devolvió error inesperado: %v", err)
		}
		if resp.RoleNombre != "DOCENTE" {
			t.Fatalf("se esperaba RoleNombre=DOCENTE, got %q", resp.RoleNombre)
		}
		roles := rolesDeUsuario(t, userID)
		if len(roles) != 1 || roles[0] != "DOCENTE" {
			t.Fatalf("se esperaba exactamente [DOCENTE] en BD, got %v", roles)
		}
	})

	// --- Test 2: Cambio a otro rol (DOCENTE -> DIRECTOR) ---
	t.Run("CambioAOtroRol", func(t *testing.T) {
		userID := createUser(t, "otrorol", "DOCENTE")
		if _, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: userID,
			RoleId: roleID(t, "DIRECTOR"),
		}); err != nil {
			t.Fatalf("UpdateUserRole devolvió error inesperado: %v", err)
		}
		roles := rolesDeUsuario(t, userID)
		if len(roles) != 1 || roles[0] != "DIRECTOR" {
			t.Fatalf("se esperaba exactamente [DIRECTOR] en BD, got %v", roles)
		}
	})

	// --- Test 3: Mismo rol (idempotente) ---
	t.Run("MismoRol", func(t *testing.T) {
		userID := createUser(t, "mismorol", "DIRECTOR")
		directorID := roleID(t, "DIRECTOR")
		if _, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: userID,
			RoleId: directorID,
		}); err != nil {
			t.Fatalf("UpdateUserRole (mismo rol) devolvió error inesperado: %v", err)
		}
		roles := rolesDeUsuario(t, userID)
		if len(roles) != 1 || roles[0] != "DIRECTOR" {
			t.Fatalf("se esperaba exactamente [DIRECTOR] (sin duplicar), got %v", roles)
		}
	})

	// --- Test 4: Usuario inexistente ---
	t.Run("UsuarioInexistente", func(t *testing.T) {
		_, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: nonExistentUUID,
			RoleId: roleID(t, "DOCENTE"),
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound, got %v", err)
		}
	})

	// --- Test 5: Rol inexistente ---
	t.Run("RolInexistente", func(t *testing.T) {
		userID := createUser(t, "rolinexistente", "SOLICITANTE")
		_, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: userID,
			RoleId: nonExistentUUID,
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound, got %v", err)
		}
		roles := rolesDeUsuario(t, userID)
		if len(roles) != 1 || roles[0] != "SOLICITANTE" {
			t.Fatalf("el rol original no debía cambiar, got %v", roles)
		}
	})

	// --- Test 6: Rol inactivo ---
	t.Run("RolInactivo", func(t *testing.T) {
		var inactiveRoleID string
		inactiveName := "TEMP_INACTIVO_" + testID
		if err := pool.QueryRow(ctx, `
			INSERT INTO roles (nombre, estado) VALUES ($1, FALSE) RETURNING id
		`, inactiveName).Scan(&inactiveRoleID); err != nil {
			t.Fatalf("crear rol inactivo de prueba: %v", err)
		}
		createdRoleIDs = append(createdRoleIDs, inactiveRoleID)

		userID := createUser(t, "rolinactivo", "SOLICITANTE")
		_, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: userID,
			RoleId: inactiveRoleID,
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("se esperaba FailedPrecondition, got %v", err)
		}
		roles := rolesDeUsuario(t, userID)
		if len(roles) != 1 || roles[0] != "SOLICITANTE" {
			t.Fatalf("el rol original no debía cambiar, got %v", roles)
		}
	})

	// --- Test 7: Usuario inactivo ---
	t.Run("UsuarioInactivo", func(t *testing.T) {
		userID := createUser(t, "usuarioinactivo", "SOLICITANTE")
		if _, err := pool.Exec(ctx, "UPDATE usuarios SET estado = FALSE WHERE id = $1", userID); err != nil {
			t.Fatalf("desactivar usuario de prueba: %v", err)
		}
		_, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: userID,
			RoleId: roleID(t, "DOCENTE"),
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("se esperaba FailedPrecondition, got %v", err)
		}
		roles := rolesDeUsuario(t, userID)
		if len(roles) != 1 || roles[0] != "SOLICITANTE" {
			t.Fatalf("el rol original no debía cambiar, got %v", roles)
		}
	})

	// --- Test 8: Atomicidad ---
	t.Run("Atomicidad", func(t *testing.T) {
		userID := createUser(t, "atomicidad", "AUXILIAR")
		_, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: userID,
			RoleId: nonExistentUUID,
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound, got %v", err)
		}
		roles := rolesDeUsuario(t, userID)
		if len(roles) != 1 || roles[0] != "AUXILIAR" {
			t.Fatalf("se esperaba que el rol original (AUXILIAR) permaneciera intacto tras el fallo, got %v", roles)
		}
	})

	// --- Test 9: Sin permiso ---
	t.Run("SinPermiso", func(t *testing.T) {
		solicitanteID := createUser(t, "sinpermiso", "SOLICITANTE")
		victimaID := createUser(t, "victima", "AUXILIAR")
		_, err := client.UpdateUserRole(authCtx(solicitanteID), &auth.UpdateUserRoleRequest{
			UserId: victimaID,
			RoleId: roleID(t, "DOCENTE"),
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied, got %v", err)
		}
		roles := rolesDeUsuario(t, victimaID)
		if len(roles) != 1 || roles[0] != "AUXILIAR" {
			t.Fatalf("la BD no debía cambiar tras un intento sin permiso, got %v", roles)
		}
	})

	// --- Test 10: ADMIN puede cambiar el rol ---
	t.Run("AdminPuedeCambiar", func(t *testing.T) {
		userID := createUser(t, "adminpuede", "SOLICITANTE")
		if _, err := client.UpdateUserRole(authCtx(adminID), &auth.UpdateUserRoleRequest{
			UserId: userID,
			RoleId: roleID(t, "SUBDIRECTOR"),
		}); err != nil {
			t.Fatalf("ADMIN con usuarios.change_role debería poder cambiar el rol, got %v", err)
		}
		roles := rolesDeUsuario(t, userID)
		if len(roles) != 1 || roles[0] != "SUBDIRECTOR" {
			t.Fatalf("se esperaba exactamente [SUBDIRECTOR], got %v", roles)
		}
	})

	// --- Sin JWT ---
	t.Run("JWTAusente", func(t *testing.T) {
		_, err := client.UpdateUserRole(ctx, &auth.UpdateUserRoleRequest{
			UserId: adminID,
			RoleId: roleID(t, "ADMIN"),
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})
}
