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
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// TestPermissionsSeedIntegration verifica, contra PostgreSQL real, que los
// datos sembrados por la migración 005_seed_permissions_and_role_permissions
// (permissions/role_permissions ya no están vacíos) hacen que
// AuthorizationService.HasPermission() — que ya existía y ya estaba probado
// con stubs/datos sintéticos en authorization_test.go y en
// auth_integration_test.go — devuelva lo correcto con el CATÁLOGO REAL de
// permisos, sin modificar esa lógica. No se toca AuthorizationInterceptor,
// DefaultMethodPolicies, los authorization.go locales de los microservicios,
// el frontend ni el Gateway.
func TestPermissionsSeedIntegration(t *testing.T) {
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

	// usuarios.username es VARCHAR(50) (000_create_users_and_roles.sql): el
	// identificador único se acota a 8 dígitos para dejar espacio de sobra
	// a los sufijos de cada sub-test.
	testID := strconv.FormatInt(time.Now().UnixNano()%1e8, 10)
	usernamePrefix := "ps" + testID + "_"
	var createdUserIDs []string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, id := range createdUserIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM usuarios WHERE id = $1", id); err != nil {
				t.Logf("cleanup usuario %s: %v", id, err)
			}
		}
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM usuarios WHERE username LIKE $1", usernamePrefix+"%"); err != nil {
			t.Logf("cleanup por prefijo: %v", err)
		}
	}()

	// createTestUser inserta un usuario desechable directo por SQL (mismo
	// patrón ya usado en auth_integration_test.go para asignar roles) y,
	// si se indica un rol, lo vincula en usuario_roles resolviendo el
	// role_id por nombre — nunca por UUID fijo.
	createTestUser := func(t *testing.T, suffix string, roleName string) string {
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

		if roleName != "" {
			var roleID string
			if err := pool.QueryRow(ctx, "SELECT id FROM roles WHERE nombre = $1 AND estado = TRUE", roleName).Scan(&roleID); err != nil {
				t.Fatalf("buscar rol %s: %v", roleName, err)
			}
			if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)", userID, roleID); err != nil {
				t.Fatalf("asignar rol %s: %v", roleName, err)
			}
		}
		return userID
	}

	// Prueba 1: permiso concedido — SECRETARIA sí tiene expedientes.create
	// en el catálogo real recién sembrado.
	t.Run("PermisoConcedido_SecretariaExpedientesCreate", func(t *testing.T) {
		userID := createTestUser(t, "secretaria", "SECRETARIA")
		ok, err := authorizationService.HasPermission(ctx, userID, "expedientes.create")
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado: %v", err)
		}
		if !ok {
			t.Fatalf("se esperaba que SECRETARIA tuviera expedientes.create, HasPermission devolvió false")
		}
	})

	// Prueba 2: permiso denegado — DOCENTE no tiene expedientes.create en
	// el mock original (frontend/src/permissions/permissions.js), y la
	// migración respeta esa misma regla.
	t.Run("PermisoDenegado_DocenteExpedientesCreate", func(t *testing.T) {
		userID := createTestUser(t, "docente", "DOCENTE")
		ok, err := authorizationService.HasPermission(ctx, userID, "expedientes.create")
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado: %v", err)
		}
		if ok {
			t.Fatalf("se esperaba que DOCENTE NO tuviera expedientes.create, HasPermission devolvió true")
		}
	})

	// Prueba 3: permiso inexistente — un código que no está en la tabla
	// permissions debe denegar sin panic, no fallar con error.
	t.Run("PermisoInexistente", func(t *testing.T) {
		userID := createTestUser(t, "adminnoperm", "ADMIN")
		ok, err := authorizationService.HasPermission(ctx, userID, "permiso.que.no.existe."+testID)
		if err != nil {
			t.Fatalf("HasPermission devolvió error inesperado para un permiso inexistente: %v", err)
		}
		if ok {
			t.Fatalf("se esperaba false para un permiso inexistente, HasPermission devolvió true")
		}
	})

	// Prueba 4: usuario sin ningún rol asignado — GetPermissionsByUserID
	// debe devolver un conjunto vacío, no error.
	t.Run("UsuarioSinRoles", func(t *testing.T) {
		userID := createTestUser(t, "sin-roles", "")
		permissions, err := authorizationService.GetPermissionsByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("GetPermissionsByUserID devolvió error inesperado: %v", err)
		}
		if len(permissions) != 0 {
			t.Fatalf("se esperaba un conjunto vacío de permisos, se obtuvo: %v", permissions)
		}
	})

	// Pruebas 5 y 6 comparten un servidor gRPC real (bufconn) igual al que
	// ya monta auth_integration_test.go, para confirmar contra el flujo
	// completo (no solo AuthorizationService en aislado) que Login y
	// Register siguen funcionando exactamente igual después de poblar
	// permissions/role_permissions. Las policies NO se tocan: se usan las
	// mismas DefaultMethodPolicies() sin agregar ningún ProtectedMethod.
	jwtManager := security.NewJWTManager("permissions-seed-test-secret", time.Hour)
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

	// Prueba 5: Login sigue funcionando después de la migración de datos.
	t.Run("LoginSigueFuncionando", func(t *testing.T) {
		username := usernamePrefix + "login"
		registerResp, err := client.Register(ctx, &auth.RegisterRequest{
			Username: username,
			Email:    username + "@test.local",
			Password: "permission-seed-password",
			Role:     "",
		})
		if err != nil {
			t.Fatalf("Register falló: %v", err)
		}
		createdUserIDs = append(createdUserIDs, registerResp.Id)

		loginResp, err := client.Login(ctx, &auth.LoginRequest{Username: username, Password: "permission-seed-password"})
		if err != nil {
			t.Fatalf("Login falló tras poblar permissions/role_permissions: %v", err)
		}
		if loginResp.AccessToken == "" || loginResp.RefreshToken == "" || loginResp.Role != "SOLICITANTE" {
			t.Fatalf("Login devolvió una respuesta inesperada: %+v", loginResp)
		}
	})

	// Prueba 6: Register sigue bloqueando el autoregistro de roles internos.
	t.Run("RegisterBloqueaRolesInternos", func(t *testing.T) {
		username := usernamePrefix + "register-director"
		resp, err := client.Register(ctx, &auth.RegisterRequest{
			Username: username,
			Email:    username + "@test.local",
			Password: "permission-seed-password",
			Role:     "DIRECTOR",
		})
		if status.Code(err) != codes.PermissionDenied || resp != nil {
			t.Fatalf("se esperaba PermissionDenied al registrar rol DIRECTOR públicamente, resp=%v err=%v", resp, err)
		}
	})
}
