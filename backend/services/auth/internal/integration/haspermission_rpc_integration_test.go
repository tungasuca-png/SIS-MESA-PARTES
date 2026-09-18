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

// TestHasPermissionRPC prueba el RPC HasPermission de punta a punta (JWT
// real -> AuthenticationInterceptor -> HasPermissionLogic ->
// AuthorizationService.HasPermission -> PostgreSQL), usando exactamente las
// mismas DefaultMethodPolicies() de producción (sin agregar ningún
// ProtectedMethod) y el catálogo real sembrado por la migración
// 005_seed_permissions_and_role_permissions.
func TestHasPermissionRPC(t *testing.T) {
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

	userRepo := repository.NewUserRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	rolePermissionRepo := repository.NewRolePermissionRepository(pool)
	authorizationService := authorization.NewAuthorizationService(userRepo, roleRepo, rolePermissionRepo)
	jwtManager := security.NewJWTManager("haspermission-rpc-test-secret", time.Hour)
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

	// Mismas policies que en producción: no se agrega ningún
	// ProtectedMethod, HasPermission queda como AuthenticatedMethod().
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
	usernamePrefix := "hp" + testID + "_"
	var createdUserIDs []string
	var tempPermissionCodes []string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, id := range createdUserIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM usuarios WHERE id = $1", id); err != nil {
				t.Logf("cleanup usuario %s: %v", id, err)
			}
		}
		if len(tempPermissionCodes) > 0 {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo = ANY($1))", tempPermissionCodes); err != nil {
				t.Logf("cleanup role_permissions temporales: %v", err)
			}
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM permissions WHERE codigo = ANY($1)", tempPermissionCodes); err != nil {
				t.Logf("cleanup permissions temporales: %v", err)
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

	// Test 1 — permiso permitido: SECRETARIA tiene expedientes.create.
	t.Run("PermisoPermitido", func(t *testing.T) {
		userID := createUser(t, "secretaria", "SECRETARIA")
		resp, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: userID, Permission: "expedientes.create"})
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado: %v", err)
		}
		if !resp.Allowed {
			t.Fatalf("se esperaba allowed=true para SECRETARIA/expedientes.create")
		}
	})

	// Test 2 — permiso denegado: DOCENTE no tiene expedientes.create.
	t.Run("PermisoDenegado", func(t *testing.T) {
		userID := createUser(t, "docente", "DOCENTE")
		resp, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: userID, Permission: "expedientes.create"})
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado: %v", err)
		}
		if resp.Allowed {
			t.Fatalf("se esperaba allowed=false para DOCENTE/expedientes.create")
		}
	})

	// Test 3 — usuario sin roles.
	t.Run("UsuarioSinRoles", func(t *testing.T) {
		userID := createUser(t, "sinroles")
		resp, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: userID, Permission: "expedientes.view"})
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado: %v", err)
		}
		if resp.Allowed {
			t.Fatalf("se esperaba allowed=false para un usuario sin roles")
		}
	})

	// Test 4 — permiso inexistente.
	t.Run("PermisoInexistente", func(t *testing.T) {
		userID := createUser(t, "adminnoperm", "ADMIN")
		resp, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: userID, Permission: "permiso.que.no.existe." + testID})
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado: %v", err)
		}
		if resp.Allowed {
			t.Fatalf("se esperaba allowed=false para un permiso inexistente")
		}
	})

	// Test 5 — permiso inactivo (creado desactivado a propósito, asignado a
	// un rol real, para probar que un permiso con estado=FALSE no autoriza).
	t.Run("PermisoInactivo", func(t *testing.T) {
		inactiveCode := "integration.hp.inactive." + testID
		tempPermissionCodes = append(tempPermissionCodes, inactiveCode)
		var permissionID string
		if err := pool.QueryRow(ctx, `
			INSERT INTO permissions (codigo, nombre, modulo, estado)
			VALUES ($1, $1, 'integration', FALSE)
			RETURNING id
		`, inactiveCode).Scan(&permissionID); err != nil {
			t.Fatalf("crear permiso inactivo: %v", err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)", roleID(t, "ADMIN"), permissionID); err != nil {
			t.Fatalf("asignar permiso inactivo: %v", err)
		}
		userID := createUser(t, "permisoinactivo", "ADMIN")
		resp, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: userID, Permission: inactiveCode})
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado: %v", err)
		}
		if resp.Allowed {
			t.Fatalf("se esperaba allowed=false para un permiso inactivo")
		}
	})

	// Test 6 — usuario con múltiples roles (DOCENTE + AUXILIAR): si
	// cualquiera de los dos tiene expedientes.view, debe ser true. Prueba
	// que el RPC reutiliza el soporte multi-rol ya existente en
	// AuthorizationService (no depende del único rol que llevaría un JWT).
	t.Run("MultiRol", func(t *testing.T) {
		userID := createUser(t, "multirol", "DOCENTE", "AUXILIAR")
		resp, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: userID, Permission: "expedientes.view"})
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado: %v", err)
		}
		if !resp.Allowed {
			t.Fatalf("se esperaba allowed=true para un usuario DOCENTE+AUXILIAR consultando expedientes.view")
		}
	})

	// Test 7 — user_id vacío.
	t.Run("UserIdVacio", func(t *testing.T) {
		userID := createUser(t, "useridvacio", "ADMIN")
		_, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: "", Permission: "expedientes.view"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument con user_id vacío, got %v", err)
		}
	})

	// Test 8 — permission vacío.
	t.Run("PermissionVacio", func(t *testing.T) {
		userID := createUser(t, "permvacio", "ADMIN")
		_, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: userID, Permission: ""})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument con permission vacío, got %v", err)
		}
	})

	// Test 9 — UUID inválido.
	t.Run("UUIDInvalido", func(t *testing.T) {
		userID := createUser(t, "uuidinvalido", "ADMIN")
		_, err := client.HasPermission(authCtx(userID), &auth.HasPermissionRequest{UserId: "no-es-un-uuid", Permission: "expedientes.view"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument con user_id no-UUID, got %v", err)
		}
	})

	// Test 10 — JWT ausente.
	t.Run("JWTAusente", func(t *testing.T) {
		_, err := client.HasPermission(ctx, &auth.HasPermissionRequest{UserId: "550e8400-e29b-41d4-a716-446655440000", Permission: "expedientes.view"})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})

	// Test 11 — user_id distinto al sub autenticado.
	t.Run("UserIdDistintoDelJWT", func(t *testing.T) {
		userA := createUser(t, "usuarioa", "ADMIN")
		userB := createUser(t, "usuariob", "ADMIN")
		_, err := client.HasPermission(authCtx(userA), &auth.HasPermissionRequest{UserId: userB, Permission: "expedientes.view"})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied consultando el permiso de otro usuario, got %v", err)
		}
	})
}
