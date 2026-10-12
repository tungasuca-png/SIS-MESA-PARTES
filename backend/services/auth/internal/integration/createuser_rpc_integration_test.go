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
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// TestCreateUserRPC prueba CreateUser de punta a punta (JWT real ->
// AuthenticationInterceptor -> AuthorizationInterceptor
// ("usuarios.change_role", el mismo permiso que ya protege UpdateUserRole
// — no existe "usuarios.create" en el catálogo real y esta fase no agrega
// uno nuevo) -> UserRepository.CreateUserWithRoleID transaccional ->
// PostgreSQL), usando exactamente las mismas DefaultMethodPolicies() de
// producción. Distinto de Register (público, self-service, solo
// SOLICITANTE): acá el actor elige cualquier rol real ACTIVO por role_id.
func TestCreateUserRPC(t *testing.T) {
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
	jwtManager := security.NewJWTManager("create-user-rpc-test-secret", time.Hour)
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

	// Mismas policies que en producción: CreateUser queda protegido por
	// ProtectedMethod("usuarios.change_role"), ya en el policy map.
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
	usernamePrefix := "cu" + testID + "_"
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

	// Actor administrativo real: crea un usuario ADMIN de prueba (ADMIN ya
	// tiene "usuarios.change_role" desde la migración 006).
	var adminID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO usuarios (username, email, password_hash, estado)
		VALUES ($1, $2, $3, TRUE)
		RETURNING id
	`, usernamePrefix+"admin", usernamePrefix+"admin@test.local", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy").Scan(&adminID); err != nil {
		t.Fatalf("crear usuario admin de prueba: %v", err)
	}
	createdUserIDs = append(createdUserIDs, adminID)
	if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)", adminID, roleID(t, "ADMIN")); err != nil {
		t.Fatalf("asignar rol ADMIN: %v", err)
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

	trackCreated := func(id string) {
		if id != "" {
			createdUserIDs = append(createdUserIDs, id)
		}
	}

	// --- Test 1: Creación válida ---
	t.Run("CreacionValida", func(t *testing.T) {
		username := usernamePrefix + "valido"
		resp, err := client.CreateUser(authCtx(adminID), &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "@test.local",
			Password: "Password123!",
			RoleId:   roleID(t, "DOCENTE"),
		})
		if err != nil {
			t.Fatalf("CreateUser devolvió error inesperado: %v", err)
		}
		trackCreated(resp.Id)
		if resp.Username != username {
			t.Fatalf("username inesperado: %q", resp.Username)
		}
		if resp.RoleNombre != "DOCENTE" {
			t.Fatalf("se esperaba RoleNombre=DOCENTE, got %q", resp.RoleNombre)
		}

		var storedHash string
		if err := pool.QueryRow(ctx, "SELECT password_hash FROM usuarios WHERE id = $1", resp.Id).Scan(&storedHash); err != nil {
			t.Fatalf("consultar password_hash: %v", err)
		}
		if storedHash == "Password123!" {
			t.Fatal("la contraseña se guardó en texto plano")
		}
		if bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("Password123!")) != nil {
			t.Fatal("el hash almacenado no corresponde a la contraseña enviada")
		}
	})

	// --- Test 7 (verificado junto al anterior): rol único ---
	t.Run("RolUnico", func(t *testing.T) {
		username := usernamePrefix + "rolunico"
		resp, err := client.CreateUser(authCtx(adminID), &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "@test.local",
			Password: "Password123!",
			RoleId:   roleID(t, "AUXILIAR"),
		})
		if err != nil {
			t.Fatalf("CreateUser devolvió error inesperado: %v", err)
		}
		trackCreated(resp.Id)
		roles := rolesDeUsuario(t, resp.Id)
		if len(roles) != 1 || roles[0] != "AUXILIAR" {
			t.Fatalf("se esperaba exactamente [AUXILIAR], got %v", roles)
		}
	})

	// --- Test 2: Rol inexistente ---
	t.Run("RolInexistente", func(t *testing.T) {
		username := usernamePrefix + "rolinexistente"
		_, err := client.CreateUser(authCtx(adminID), &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "@test.local",
			Password: "Password123!",
			RoleId:   nonExistentUUID,
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound, got %v", err)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM usuarios WHERE username = $1", username).Scan(&count); err != nil {
			t.Fatalf("verificar no-creación: %v", err)
		}
		if count != 0 {
			t.Fatal("no debía crearse ningún usuario con rol inexistente")
		}
	})

	// --- Test 3: Rol inactivo ---
	t.Run("RolInactivo", func(t *testing.T) {
		var inactiveRoleID string
		inactiveName := "TEMP_CREATEUSER_INACTIVO_" + testID
		if err := pool.QueryRow(ctx, `
			INSERT INTO roles (nombre, estado) VALUES ($1, FALSE) RETURNING id
		`, inactiveName).Scan(&inactiveRoleID); err != nil {
			t.Fatalf("crear rol inactivo de prueba: %v", err)
		}
		createdRoleIDs = append(createdRoleIDs, inactiveRoleID)

		username := usernamePrefix + "rolinactivo"
		_, err := client.CreateUser(authCtx(adminID), &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "@test.local",
			Password: "Password123!",
			RoleId:   inactiveRoleID,
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("se esperaba FailedPrecondition, got %v", err)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM usuarios WHERE username = $1", username).Scan(&count); err != nil {
			t.Fatalf("verificar no-creación: %v", err)
		}
		if count != 0 {
			t.Fatal("no debía crearse ningún usuario con rol inactivo")
		}
	})

	// --- Test 4: Username duplicado ---
	t.Run("UsernameDuplicado", func(t *testing.T) {
		username := usernamePrefix + "duplicado"
		first, err := client.CreateUser(authCtx(adminID), &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "@test.local",
			Password: "Password123!",
			RoleId:   roleID(t, "DOCENTE"),
		})
		if err != nil {
			t.Fatalf("primera creación debía tener éxito: %v", err)
		}
		trackCreated(first.Id)

		_, err = client.CreateUser(authCtx(adminID), &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "b@test.local",
			Password: "Password123!",
			RoleId:   roleID(t, "DOCENTE"),
		})
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("se esperaba AlreadyExists para username duplicado, got %v", err)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM usuarios WHERE username = $1", username).Scan(&count); err != nil {
			t.Fatalf("verificar no-duplicación: %v", err)
		}
		if count != 1 {
			t.Fatalf("se esperaba exactamente 1 fila para el username, got %d", count)
		}
	})

	// --- Test 5: Sin permiso ---
	t.Run("SinPermiso", func(t *testing.T) {
		solicitanteUsername := usernamePrefix + "solicitante"
		var solicitanteID string
		if err := pool.QueryRow(ctx, `
			INSERT INTO usuarios (username, email, password_hash, estado)
			VALUES ($1, $2, $3, TRUE)
			RETURNING id
		`, solicitanteUsername, solicitanteUsername+"@test.local", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy").Scan(&solicitanteID); err != nil {
			t.Fatalf("crear usuario solicitante de prueba: %v", err)
		}
		createdUserIDs = append(createdUserIDs, solicitanteID)
		if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)", solicitanteID, roleID(t, "SOLICITANTE")); err != nil {
			t.Fatalf("asignar rol SOLICITANTE: %v", err)
		}

		username := usernamePrefix + "sinpermiso"
		_, err := client.CreateUser(authCtx(solicitanteID), &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "@test.local",
			Password: "Password123!",
			RoleId:   roleID(t, "DOCENTE"),
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied, got %v", err)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM usuarios WHERE username = $1", username).Scan(&count); err != nil {
			t.Fatalf("verificar no-creación: %v", err)
		}
		if count != 0 {
			t.Fatal("no debía crearse ningún usuario sin permiso")
		}
	})

	// --- Test 6: Sin autenticación ---
	t.Run("SinAutenticacion", func(t *testing.T) {
		username := usernamePrefix + "sinauth"
		_, err := client.CreateUser(ctx, &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "@test.local",
			Password: "Password123!",
			RoleId:   roleID(t, "DOCENTE"),
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})

	// --- Test adicional: role_id vacío (1 usuario = 1 rol, nunca 0) ---
	t.Run("RoleIdVacio", func(t *testing.T) {
		username := usernamePrefix + "sinrol"
		_, err := client.CreateUser(authCtx(adminID), &auth.CreateUserRequest{Dni: nextTestDNI(),
			Username: username,
			Email:    username + "@test.local",
			Password: "Password123!",
			RoleId:   "",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument para role_id vacío, got %v", err)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM usuarios WHERE username = $1", username).Scan(&count); err != nil {
			t.Fatalf("verificar no-creación: %v", err)
		}
		if count != 0 {
			t.Fatal("no debía crearse ningún usuario sin role_id")
		}
	})
}
