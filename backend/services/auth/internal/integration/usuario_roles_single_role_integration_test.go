package integration

import (
	"context"
	"errors"
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

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// TestUsuarioRolesUnRolPorUsuarioBD prueba que PostgreSQL (UNIQUE
// usuario_id, migración 007) garantiza 1 usuario = 1 rol, y que el flujo
// oficial UpdateUserRole (DELETE + INSERT en una transacción) sigue
// funcionando y es atómico.
func TestUsuarioRolesUnRolPorUsuarioBD(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, integrationDatabaseURL(t))
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	rpRepo := repository.NewRolePermissionRepository(pool)
	authz := authorization.NewAuthorizationService(userRepo, roleRepo, rpRepo)
	jwtManager := security.NewJWTManager("single-role-test-secret", time.Hour)
	svcCtx := &svc.ServiceContext{
		Config:                   config.Config{RefreshTokenDuration: 24 * time.Hour},
		DB:                       pool,
		UserRepository:           userRepo,
		RoleRepository:           roleRepo,
		PermissionRepository:     repository.NewPermissionRepository(pool),
		RolePermissionRepository: rpRepo,
		RefreshTokenRepository:   repository.NewRefreshTokenRepository(pool),
		AuthorizationService:     authz,
		JWTManager:               jwtManager,
	}
	policies := interceptor.DefaultMethodPolicies()
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(jwtManager, policies),
		interceptor.AuthorizationInterceptor(authz, policies),
	))
	auth.RegisterAuthServer(grpcServer, server.NewAuthServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := auth.NewAuthClient(conn)

	prefix := "sr" + strconv.FormatInt(time.Now().UnixNano()%1e8, 10) + "_"
	var userIDs []string
	defer func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		for _, id := range userIDs {
			_, _ = pool.Exec(c, "DELETE FROM usuario_roles WHERE usuario_id = $1", id)
			_, _ = pool.Exec(c, "DELETE FROM usuarios WHERE id = $1", id)
		}
	}()
	roleID := func(name string) string {
		var id string
		if err := pool.QueryRow(ctx, "SELECT id FROM roles WHERE nombre = $1", name).Scan(&id); err != nil {
			t.Fatalf("rol %s: %v", name, err)
		}
		return id
	}
	mkUser := func(suffix, role string) string {
		var id string
		u := prefix + suffix
		if err := pool.QueryRow(ctx, `INSERT INTO usuarios (username, email, password_hash, estado) VALUES ($1,$2,'x',TRUE) RETURNING id`, u, u+"@test.local").Scan(&id); err != nil {
			t.Fatalf("crear usuario: %v", err)
		}
		userIDs = append(userIDs, id)
		if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1,$2)", id, roleID(role)); err != nil {
			t.Fatalf("asignar rol: %v", err)
		}
		return id
	}
	rolesOf := func(id string) []string {
		rows, err := pool.Query(ctx, `SELECT r.nombre FROM usuario_roles ur JOIN roles r ON r.id=ur.rol_id WHERE ur.usuario_id=$1 ORDER BY 1`, id)
		if err != nil {
			t.Fatalf("roles: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var n string
			_ = rows.Scan(&n)
			out = append(out, n)
		}
		return out
	}
	bearer := func(id string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+mustToken(t, jwtManager, id))
	}
	expectRoles := func(t *testing.T, id string, want ...string) {
		t.Helper()
		if got := rolesOf(id); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("roles = %v, se esperaba %v", got, want)
		}
	}

	adminID := mkUser("admin", "ADMIN")
	dirID := mkUser("dir", "DIRECTOR")
	otherID := mkUser("otro", "AUXILIAR")

	t.Run("Caso1_UsuarioConUnRol", func(t *testing.T) {
		expectRoles(t, dirID, "DIRECTOR")
	})

	t.Run("Caso2_SegundoRolRechazadoPorPostgreSQL", func(t *testing.T) {
		_, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1,$2)", dirID, roleID("DOCENTE"))
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "uq_usuario_roles_usuario" {
			t.Fatalf("se esperaba violación de uq_usuario_roles_usuario (23505), got %v", err)
		}
		expectRoles(t, dirID, "DIRECTOR")
	})

	t.Run("Caso3_CambioOficialDeRol", func(t *testing.T) {
		if _, err := client.UpdateUserRole(bearer(adminID), &auth.UpdateUserRoleRequest{UserId: dirID, RoleId: roleID("DOCENTE")}); err != nil {
			t.Fatalf("UpdateUserRole: %v", err)
		}
		expectRoles(t, dirID, "DOCENTE")
		// mismo rol: idempotente, sigue 1 fila
		if _, err := client.UpdateUserRole(bearer(adminID), &auth.UpdateUserRoleRequest{UserId: dirID, RoleId: roleID("DOCENTE")}); err != nil {
			t.Fatalf("UpdateUserRole idempotente: %v", err)
		}
		expectRoles(t, dirID, "DOCENTE")
	})

	t.Run("Caso4_CambioFallidoNoDejaNiCeroNiDosRoles", func(t *testing.T) {
		var inactive string
		name := "TEMP_SR_INACTIVO_" + prefix
		if err := pool.QueryRow(ctx, `INSERT INTO roles (nombre, estado) VALUES ($1, FALSE) RETURNING id`, name).Scan(&inactive); err != nil {
			t.Fatalf("rol inactivo: %v", err)
		}
		defer pool.Exec(context.Background(), "DELETE FROM roles WHERE id = $1", inactive)

		if _, err := client.UpdateUserRole(bearer(adminID), &auth.UpdateUserRoleRequest{UserId: dirID, RoleId: inactive}); status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("rol inactivo: se esperaba FailedPrecondition, got %v", err)
		}
		expectRoles(t, dirID, "DOCENTE")
		if _, err := client.UpdateUserRole(bearer(adminID), &auth.UpdateUserRoleRequest{UserId: dirID, RoleId: nonExistentUUID}); status.Code(err) != codes.NotFound {
			t.Fatalf("rol inexistente: se esperaba NotFound, got %v", err)
		}
		expectRoles(t, dirID, "DOCENTE")
	})

	t.Run("Caso5_Aislamiento", func(t *testing.T) {
		expectRoles(t, otherID, "AUXILIAR")
		expectRoles(t, adminID, "ADMIN")
	})

	t.Run("Caso6_PermisosDelRolNuevo", func(t *testing.T) {
		// dirID quedó como DOCENTE: reportes.view (DIRECTOR) no debe heredarse;
		// dashboard.view sí lo tiene DOCENTE. No se tocan permisos.
		var docenteHasReportes bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=$1 AND p.codigo='reportes.view')`, roleID("DOCENTE")).Scan(&docenteHasReportes); err != nil {
			t.Fatalf("consulta: %v", err)
		}
		resp, err := client.HasPermission(bearer(dirID), &auth.HasPermissionRequest{UserId: dirID, Permission: "reportes.view"})
		if err != nil || resp.Allowed != docenteHasReportes {
			t.Fatalf("HasPermission(reportes.view)=%v (err=%v), el rol DOCENTE actual lo tiene=%v", resp != nil && resp.Allowed, err, docenteHasReportes)
		}
		resp, err = client.HasPermission(bearer(dirID), &auth.HasPermissionRequest{UserId: dirID, Permission: "dashboard.view"})
		if err != nil || !resp.Allowed {
			t.Fatalf("dashboard.view debía estar permitido para DOCENTE, err=%v", err)
		}
	})

	t.Run("CreateUser_AsignaExactamenteUnRol", func(t *testing.T) {
		u := prefix + "creado"
		resp, err := client.CreateUser(bearer(adminID), &auth.CreateUserRequest{Dni: nextTestDNI(),Username: u, Email: u + "@test.local", Password: "Password123!", RoleId: roleID("SECRETARIA")})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		userIDs = append(userIDs, resp.Id)
		expectRoles(t, resp.Id, "SECRETARIA")
	})
}
