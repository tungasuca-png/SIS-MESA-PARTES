package integration

import (
	"context"
	"fmt"
	"net"
	"os"
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

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const integrationUserPrefix = "integration_test_user_"

func TestAuthIntegration(t *testing.T) {
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
	jwtManager := security.NewJWTManager("integration-test-secret", time.Hour)
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

	testID := fmt.Sprintf("%d", time.Now().UnixNano())
	username := integrationUserPrefix + testID
	email := "integration_" + testID + "@test.local"
	permissionCodes := []string{"integration.docente." + testID, "integration.auxiliar." + testID}
	cleanupIntegrationData(t, pool, username, permissionCodes)
	defer cleanupIntegrationData(t, pool, username, permissionCodes)

	roles := map[string]string{}
	for _, roleName := range []string{"SOLICITANTE", "DOCENTE", "AUXILIAR", "SECRETARIA", "SUBDIRECTOR", "DIRECTOR", "ADMIN"} {
		var roleID string
		if err := pool.QueryRow(ctx, "SELECT id FROM roles WHERE nombre = $1 AND estado = TRUE", roleName).Scan(&roleID); err != nil {
			t.Fatalf("find role %s: %v", roleName, err)
		}
		roles[roleName] = roleID
	}

	permissionIDs := make([]string, 0, len(permissionCodes))
	for _, code := range permissionCodes {
		var permissionID string
		err := pool.QueryRow(ctx, `
			INSERT INTO permissions (codigo, nombre, modulo, estado)
			VALUES ($1, $2, 'integration', TRUE)
			RETURNING id
		`, code, code).Scan(&permissionID)
		if err != nil {
			t.Fatalf("create integration permission %s: %v", code, err)
		}
		permissionIDs = append(permissionIDs, permissionID)
	}
	for i, roleName := range []string{"DOCENTE", "AUXILIAR"} {
		if _, err := pool.Exec(ctx, "INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)", roles[roleName], permissionIDs[i]); err != nil {
			t.Fatalf("assign integration permission: %v", err)
		}
	}

	policies := interceptor.DefaultMethodPolicies()
	policies["/auth.Auth/Ping"] = interceptor.ProtectedMethod(permissionCodes[0])
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(jwtManager, policies),
		interceptor.AuthorizationInterceptor(authorizationService, policies),
	))
	auth.RegisterAuthServer(grpcServer, server.NewAuthServer(svcCtx))
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
	client := auth.NewAuthClient(conn)

	registerResponse, err := client.Register(clientCtx, &auth.RegisterRequest{Username: username, Email: email, Password: "integration-password", Role: ""})
	if err != nil {
		t.Fatalf("register integration user: %v", err)
	}
	if registerResponse.Role != "SOLICITANTE" {
		t.Fatalf("expected SOLICITANTE, got %q", registerResponse.Role)
	}
	var storedHash, storedRole string
	if err := pool.QueryRow(ctx, `
		SELECT u.password_hash, r.nombre
		FROM usuarios u
		JOIN usuario_roles ur ON ur.usuario_id = u.id
		JOIN roles r ON r.id = ur.rol_id
		WHERE u.id = $1
	`, registerResponse.Id).Scan(&storedHash, &storedRole); err != nil {
		t.Fatalf("verify registered user: %v", err)
	}
	if storedHash == "integration-password" || !strings.HasPrefix(storedHash, "$2") || storedRole != "SOLICITANTE" {
		t.Fatalf("registration did not persist bcrypt SOLICITANTE data")
	}

	for _, roleName := range []string{"ADMIN", "DIRECTOR", "SUBDIRECTOR", "SECRETARIA", "DOCENTE", "AUXILIAR"} {
		response, err := client.Register(clientCtx, &auth.RegisterRequest{Username: integrationUserPrefix + roleName + "_" + testID, Email: strings.ToLower(roleName) + "_" + testID + "@test.local", Password: "integration-password", Role: roleName})
		if status.Code(err) != codes.PermissionDenied || response != nil {
			t.Fatalf("expected public registration of %s to be denied, response=%v err=%v", roleName, response, err)
		}
	}

	if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2), ($1, $3)", registerResponse.Id, roles["DOCENTE"], roles["AUXILIAR"]); err != nil {
		t.Fatalf("assign multiple roles: %v", err)
	}
	loginResponse, err := client.Login(clientCtx, &auth.LoginRequest{Username: username, Password: "integration-password"})
	if err != nil || loginResponse.AccessToken == "" || loginResponse.RefreshToken == "" {
		t.Fatalf("expected real login tokens, response=%v err=%v", loginResponse, err)
	}
	validationResponse, err := client.ValidateToken(clientCtx, &auth.ValidateTokenRequest{AccessToken: loginResponse.AccessToken})
	if err != nil || validationResponse == nil || !validationResponse.Valid || validationResponse.UserId != registerResponse.Id {
		t.Fatalf("expected valid ValidateToken response, response=%v err=%v", validationResponse, err)
	}
	if _, err := client.ValidateToken(clientCtx, &auth.ValidateTokenRequest{AccessToken: loginResponse.AccessToken + "tampered"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected manipulated ValidateToken request to be Unauthenticated, got %v", err)
	}
	if _, err := client.ValidateToken(clientCtx, &auth.ValidateTokenRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected empty ValidateToken request to be Unauthenticated, got %v", err)
	}
	if _, err := client.Login(clientCtx, &auth.LoginRequest{Username: username, Password: "wrong-password"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected wrong password to be Unauthenticated, got %v", err)
	}
	if _, err := client.Login(clientCtx, &auth.LoginRequest{Username: "missing_" + testID, Password: "wrong-password"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected missing user to be Unauthenticated, got %v", err)
	}

	refreshResponse, err := client.RefreshToken(clientCtx, &auth.RefreshTokenRequest{RefreshToken: loginResponse.RefreshToken})
	if err != nil || refreshResponse.AccessToken == "" || refreshResponse.RefreshToken == "" || refreshResponse.RefreshToken == loginResponse.RefreshToken {
		t.Fatalf("expected refresh rotation, response=%v err=%v", refreshResponse, err)
	}
	if _, err := client.RefreshToken(clientCtx, &auth.RefreshTokenRequest{RefreshToken: loginResponse.RefreshToken}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected reused refresh token to be rejected, got %v", err)
	}

	authorizedCtx := metadata.AppendToOutgoingContext(clientCtx, "authorization", "Bearer "+loginResponse.AccessToken)
	logoutResponse, err := client.Logout(authorizedCtx, &auth.LogoutRequest{RefreshToken: refreshResponse.RefreshToken})
	if err != nil || logoutResponse == nil || !logoutResponse.Success {
		t.Fatalf("expected authenticated logout to succeed, response=%v err=%v", logoutResponse, err)
	}
	var revoked bool
	if err := pool.QueryRow(ctx, "SELECT revoked_at IS NOT NULL FROM refresh_tokens WHERE token_hash = $1", security.HashRefreshToken(refreshResponse.RefreshToken)).Scan(&revoked); err != nil {
		t.Fatalf("verify refresh token revocation: %v", err)
	}
	if !revoked {
		t.Fatal("expected refresh token to be revoked")
	}
	if _, err := client.RefreshToken(clientCtx, &auth.RefreshTokenRequest{RefreshToken: refreshResponse.RefreshToken}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected logged-out refresh token to be rejected, got %v", err)
	}
	if _, err := client.Logout(authorizedCtx, &auth.LogoutRequest{RefreshToken: refreshResponse.RefreshToken}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected already revoked refresh token to remain rejected, got %v", err)
	}
	if _, err := client.Logout(clientCtx, &auth.LogoutRequest{RefreshToken: loginResponse.RefreshToken}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected logout without JWT to be Unauthenticated, got %v", err)
	}
	if _, err := client.Logout(metadata.AppendToOutgoingContext(clientCtx, "authorization", "Bearer "+loginResponse.AccessToken[:len(loginResponse.AccessToken)-1]+"x"), &auth.LogoutRequest{RefreshToken: loginResponse.RefreshToken}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected logout with invalid JWT to be Unauthenticated, got %v", err)
	}

	if _, err := client.Ping(authorizedCtx, &auth.Request{Ping: "protected"}); err != nil {
		t.Fatalf("expected permission from DOCENTE role to allow protected RPC: %v", err)
	}
	if _, err := client.Ping(metadata.AppendToOutgoingContext(clientCtx, "authorization", "Bearer "+mustToken(t, jwtManager, registerResponse.Id)), &auth.Request{Ping: "protected"}); err != nil {
		t.Fatalf("expected multiple-role authorization: %v", err)
	}
	if _, err := client.Ping(clientCtx, &auth.Request{Ping: "protected"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected missing JWT to be Unauthenticated, got %v", err)
	}
	noPermissionToken := mustToken(t, jwtManager, registerResponse.Id)
	policies["/auth.Auth/Ping"] = interceptor.ProtectedMethod("integration.permission.missing." + testID)
	if _, err := client.Ping(metadata.AppendToOutgoingContext(clientCtx, "authorization", "Bearer "+noPermissionToken), &auth.Request{Ping: "protected"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected valid JWT without permission to be PermissionDenied, got %v", err)
	}
	policies["/auth.Auth/Ping"] = interceptor.ProtectedMethod(permissionCodes[0])
	if _, err := client.Ping(metadata.AppendToOutgoingContext(clientCtx, "authorization", "Bearer "+loginResponse.AccessToken[:len(loginResponse.AccessToken)-1]+"x"), &auth.Request{Ping: "protected"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected tampered JWT to be Unauthenticated, got %v", err)
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, security.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: registerResponse.Id, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))},
	})
	expiredRaw, err := expiredToken.SignedString([]byte("integration-test-secret"))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}
	if _, err := client.Ping(metadata.AppendToOutgoingContext(clientCtx, "authorization", "Bearer "+expiredRaw), &auth.Request{Ping: "protected"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected expired JWT to be Unauthenticated, got %v", err)
	}

	if _, err := pool.Exec(ctx, "UPDATE roles SET estado = FALSE WHERE id = $1", roles["DOCENTE"]); err != nil {
		t.Fatalf("deactivate DOCENTE role: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "UPDATE roles SET estado = TRUE WHERE id = $1", roles["DOCENTE"])
	}()
	if _, err := client.Ping(authorizedCtx, &auth.Request{Ping: "protected"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected inactive DOCENTE role to deny access, got %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE roles SET estado = TRUE WHERE id = $1", roles["DOCENTE"]); err != nil {
		t.Fatalf("restore DOCENTE role: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE permissions SET estado = FALSE WHERE id = $1", permissionIDs[0]); err != nil {
		t.Fatalf("deactivate integration permission: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "UPDATE permissions SET estado = TRUE WHERE id = $1", permissionIDs[0])
	}()
	if _, err := client.Ping(authorizedCtx, &auth.Request{Ping: "protected"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected inactive permission to deny access, got %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE permissions SET estado = TRUE WHERE id = $1", permissionIDs[0]); err != nil {
		t.Fatalf("restore integration permission: %v", err)
	}
}

func integrationDatabaseURL() string {
	if value := os.Getenv("AUTH_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://auth_user:auth_password@127.0.0.1:5433/auth_db?sslmode=disable"
}

func cleanupIntegrationData(t *testing.T, pool *pgxpool.Pool, username string, permissionCodes []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "DELETE FROM usuarios WHERE username LIKE $1", integrationUserPrefix+"%"+strings.TrimPrefix(username, integrationUserPrefix)); err != nil {
		t.Logf("cleanup users: %v", err)
	}
	if len(permissionCodes) > 0 {
		if _, err := pool.Exec(ctx, "DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE codigo = ANY($1))", permissionCodes); err != nil {
			t.Logf("cleanup role permissions: %v", err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM permissions WHERE codigo = ANY($1)", permissionCodes); err != nil {
			t.Logf("cleanup permissions: %v", err)
		}
	}
}

func mustToken(t *testing.T, manager *security.JWTManager, userID string) string {
	t.Helper()
	token, _, err := manager.GenerateAccessToken(userID, "integration", "SOLICITANTE")
	if err != nil {
		t.Fatal(fmt.Sprintf("generate test token: %v", err))
	}
	return token
}
