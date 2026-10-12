package integration

import (
	"context"
	"net"
	"os"
	"sort"
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

// TestGetMyPermissionsRPC prueba GetMyPermissions de punta a punta (JWT real
// -> AuthenticationInterceptor -> GetMyPermissionsLogic ->
// AuthorizationService.GetPermissionsByUserID -> PostgreSQL), reutilizando
// las mismas DefaultMethodPolicies() de producción (AuthenticatedMethod,
// sin ningún permiso administrativo) y el catálogo real sembrado por la
// migración 005.
func TestGetMyPermissionsRPC(t *testing.T) {
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
	jwtManager := security.NewJWTManager("getmypermissions-rpc-test-secret", time.Hour)
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

	// Mismas policies que en producción: GetMyPermissions queda como
	// AuthenticatedMethod(), sin ningún ProtectedMethod agregado.
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
	usernamePrefix := "gmp" + testID + "_"
	var createdUserIDs []string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, id := range createdUserIDs {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM usuarios WHERE id = $1", id); err != nil {
				t.Logf("cleanup usuario %s: %v", id, err)
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

	permissionsOf := func(t *testing.T, roleName string) []string {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT p.codigo FROM role_permissions rp
			JOIN permissions p ON p.id = rp.permission_id
			WHERE rp.role_id = $1 AND p.estado = TRUE
		`, roleID(t, roleName))
		if err != nil {
			t.Fatalf("consultar permisos reales de %s: %v", roleName, err)
		}
		defer rows.Close()
		codes := make([]string, 0)
		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err != nil {
				t.Fatalf("escanear permiso: %v", err)
			}
			codes = append(codes, code)
		}
		sort.Strings(codes)
		return codes
	}

	// Caso 1 — usuario con un solo rol: la lista debe coincidir EXACTAMENTE
	// con los permisos reales de ese rol en la BD (nunca inventados).
	t.Run("UnRol", func(t *testing.T) {
		userID := createUser(t, "secretaria", "SECRETARIA")
		resp, err := client.GetMyPermissions(authCtx(userID), &auth.GetMyPermissionsRequest{})
		if err != nil {
			t.Fatalf("GetMyPermissions devolvió error inesperado: %v", err)
		}
		got := append([]string{}, resp.Permissions...)
		sort.Strings(got)
		want := permissionsOf(t, "SECRETARIA")
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("permisos incorrectos para SECRETARIA.\n got=%v\nwant=%v", got, want)
		}
	})

	// Caso 2 — 1 usuario = 1 rol garantizado por la BD (migración 007): un
	// segundo rol se rechaza y GetMyPermissions sigue devolviendo SOLO los
	// permisos del rol asignado.
	t.Run("MultiplesRoles", func(t *testing.T) {
		userID := createUser(t, "multirol", "DIRECTOR")
		if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)", userID, roleID(t, "SOLICITANTE")); err == nil {
			t.Fatal("la BD debía rechazar un segundo rol para el mismo usuario")
		}
		resp, err := client.GetMyPermissions(authCtx(userID), &auth.GetMyPermissionsRequest{})
		if err != nil {
			t.Fatalf("GetMyPermissions devolvió error inesperado: %v", err)
		}
		got := append([]string(nil), resp.Permissions...)
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(permissionsOf(t, "DIRECTOR"), ",") {
			t.Fatalf("se esperaban solo los permisos de DIRECTOR, got %v", got)
		}
	})

	// Caso 3 — usuario sin roles: lista vacía, sin error.
	t.Run("SinRoles", func(t *testing.T) {
		userID := createUser(t, "sinroles")
		resp, err := client.GetMyPermissions(authCtx(userID), &auth.GetMyPermissionsRequest{})
		if err != nil {
			t.Fatalf("GetMyPermissions devolvió error inesperado: %v", err)
		}
		if len(resp.Permissions) != 0 {
			t.Fatalf("se esperaba lista vacia para un usuario sin roles, got %v", resp.Permissions)
		}
	})

	// Caso 4 — sin JWT.
	t.Run("JWTAusente", func(t *testing.T) {
		_, err := client.GetMyPermissions(ctx, &auth.GetMyPermissionsRequest{})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})

	// Caso 5 — no existe ningun campo user_id en el request: se verifica a
	// nivel de compilación (auth.GetMyPermissionsRequest{} no tiene ese
	// campo) y en tiempo de ejecución comprobando que dos usuarios distintos
	// siempre reciben SU PROPIA lista, nunca la del otro.
	t.Run("SelfOnly_NoHayUserIdParaSuplantar", func(t *testing.T) {
		userA := createUser(t, "usuarioa", "ADMIN")
		userB := createUser(t, "usuariob", "SOLICITANTE")

		respA, err := client.GetMyPermissions(authCtx(userA), &auth.GetMyPermissionsRequest{})
		if err != nil {
			t.Fatalf("GetMyPermissions (A) devolvió error inesperado: %v", err)
		}
		respB, err := client.GetMyPermissions(authCtx(userB), &auth.GetMyPermissionsRequest{})
		if err != nil {
			t.Fatalf("GetMyPermissions (B) devolvió error inesperado: %v", err)
		}

		wantA := permissionsOf(t, "ADMIN")
		wantB := permissionsOf(t, "SOLICITANTE")
		gotA := append([]string{}, respA.Permissions...)
		gotB := append([]string{}, respB.Permissions...)
		sort.Strings(gotA)
		sort.Strings(gotB)

		if strings.Join(gotA, ",") != strings.Join(wantA, ",") {
			t.Fatalf("usuario A (ADMIN) recibio permisos incorrectos: got=%v want=%v", gotA, wantA)
		}
		if strings.Join(gotB, ",") != strings.Join(wantB, ",") {
			t.Fatalf("usuario B (SOLICITANTE) recibio permisos incorrectos: got=%v want=%v", gotB, wantB)
		}
		if strings.Join(gotA, ",") == strings.Join(gotB, ",") {
			t.Fatalf("A y B no deberian tener la misma lista (roles distintos): %v", gotA)
		}
	})
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
