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
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// TestSessionRoleFreshness prueba, con login/refresh/UpdateUserRole reales y
// el interceptor de producción (con el checker de vigencia de sesión), qué
// pasa con un access token emitido ANTES de un cambio de rol.
func TestSessionRoleFreshness(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	jwtManager := security.NewJWTManager("session-freshness-test-secret", time.Hour)
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
		interceptor.AuthenticationInterceptor(jwtManager, policies, userRepo), // igual que auth.go
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

	prefix := "sf" + strconv.FormatInt(time.Now().UnixNano()%1e8, 10) + "_"
	var userIDs []string
	defer func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		for _, id := range userIDs {
			_, _ = pool.Exec(c, "DELETE FROM refresh_tokens WHERE usuario_id = $1", id)
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
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.MinCost)
	dnis := map[string]string{} // username -> DNI (el login es por DNI)
	mkUser := func(suffix, role string) (id, username string) {
		username = prefix + suffix
		dnis[username] = nextTestDNI()
		if err := pool.QueryRow(ctx, `INSERT INTO usuarios (username, email, password_hash, estado, dni) VALUES ($1,$2,$3,TRUE,$4) RETURNING id`,
			username, username+"@test.local", string(hash), dnis[username]).Scan(&id); err != nil {
			t.Fatalf("crear usuario: %v", err)
		}
		userIDs = append(userIDs, id)
		if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1,$2)", id, roleID(role)); err != nil {
			t.Fatalf("asignar rol: %v", err)
		}
		return
	}
	bearer := func(token string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	dbRole := func(id string) string {
		var r string
		if err := pool.QueryRow(ctx, `SELECT r.nombre FROM usuario_roles ur JOIN roles r ON r.id=ur.rol_id WHERE ur.usuario_id=$1`, id).Scan(&r); err != nil {
			t.Fatalf("rol en BD: %v", err)
		}
		return r
	}
	permsFromDB := func(role string) []string {
		rows, err := pool.Query(ctx, `SELECT p.codigo FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id WHERE rp.role_id=$1 AND p.estado ORDER BY 1`, roleID(role))
		if err != nil {
			t.Fatalf("permisos: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var c string
			_ = rows.Scan(&c)
			out = append(out, c)
		}
		return out
	}

	_, adminUser := mkUser("admin", "ADMIN")
	adminLogin, err := client.Login(ctx, &auth.LoginRequest{Dni: dnis[adminUser], Password: "Password123!"})
	if err != nil {
		t.Fatalf("login admin: %v", err)
	}
	dirID, dirUser := mkUser("dir", "DIRECTOR")
	_, ctrlUser := mkUser("ctrl", "DIRECTOR") // control: nadie le cambia el rol

	// Caso 1 — rol inicial en el JWT.
	old, err := client.Login(ctx, &auth.LoginRequest{Dni: dnis[dirUser], Password: "Password123!"})
	if err != nil {
		t.Fatalf("login director: %v", err)
	}
	claims, err := jwtManager.ValidateAccessToken(old.AccessToken)
	if err != nil || claims.Role != "DIRECTOR" {
		t.Fatalf("Caso1: se esperaba JWT.role=DIRECTOR, got %+v err=%v", claims, err)
	}
	ctrl, err := client.Login(ctx, &auth.LoginRequest{Dni: dnis[ctrlUser], Password: "Password123!"})
	if err != nil {
		t.Fatalf("login control: %v", err)
	}
	if _, err := client.GetMyPermissions(bearer(old.AccessToken), &auth.GetMyPermissionsRequest{}); err != nil {
		t.Fatalf("antes del cambio el token debía servir: %v", err)
	}

	// Caso 2 — ADMIN cambia DIRECTOR -> DOCENTE (segundo siguiente: la
	// comparación es a resolución de segundos, igual que el claim iat).
	time.Sleep(1100 * time.Millisecond)
	if _, err := client.UpdateUserRole(bearer(adminLogin.AccessToken), &auth.UpdateUserRoleRequest{UserId: dirID, RoleId: roleID("DOCENTE")}); err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}
	if got := dbRole(dirID); got != "DOCENTE" {
		t.Fatalf("Caso2: rol en BD = %s, se esperaba DOCENTE", got)
	}

	// Caso 3 / 6 — el access token anterior deja de servir (401), incluso
	// para HasPermission, que es lo que llaman todos los microservicios.
	t.Run("Caso3_TokenAnteriorRechazado", func(t *testing.T) {
		if _, err := client.HasPermission(bearer(old.AccessToken), &auth.HasPermissionRequest{UserId: dirID, Permission: "dashboard.view"}); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("HasPermission con token viejo: se esperaba Unauthenticated, got %v", err)
		}
		if _, err := client.GetMyPermissions(bearer(old.AccessToken), &auth.GetMyPermissionsRequest{}); status.Code(err) != codes.Unauthenticated {
			t.Fatalf("GetMyPermissions con token viejo: se esperaba Unauthenticated, got %v", err)
		}
	})

	t.Run("ControlSinCambioDeRol_TokenSigueValido", func(t *testing.T) {
		if _, err := client.GetMyPermissions(bearer(ctrl.AccessToken), &auth.GetMyPermissionsRequest{}); err != nil {
			t.Fatalf("el token de un usuario sin cambio de rol debía seguir válido: %v", err)
		}
	})

	// Caso 4 — el refresh relee el rol real y emite un token con DOCENTE.
	fresh, err := client.RefreshToken(ctx, &auth.RefreshTokenRequest{RefreshToken: old.RefreshToken})
	if err != nil {
		t.Fatalf("Caso4: refresh: %v", err)
	}
	fc, err := jwtManager.ValidateAccessToken(fresh.AccessToken)
	if err != nil || fc.Role != "DOCENTE" {
		t.Fatalf("Caso4: se esperaba JWT.role=DOCENTE tras refresh, got %+v err=%v", fc, err)
	}
	// El refresh anterior queda rotado (no reutilizable).
	if _, err := client.RefreshToken(ctx, &auth.RefreshTokenRequest{RefreshToken: old.RefreshToken}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("el refresh token anterior no debía poder reutilizarse, got %v", err)
	}

	// Caso 5 — HasPermission/GetMyPermissions usan los permisos del rol ACTUAL.
	t.Run("Caso5_PermisosDelRolActual", func(t *testing.T) {
		resp, err := client.GetMyPermissions(bearer(fresh.AccessToken), &auth.GetMyPermissionsRequest{})
		if err != nil {
			t.Fatalf("GetMyPermissions con token nuevo: %v", err)
		}
		got := append([]string(nil), resp.Permissions...)
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(permsFromDB("DOCENTE"), ",") {
			t.Fatalf("permisos = %v, se esperaban los de DOCENTE %v", got, permsFromDB("DOCENTE"))
		}
		// reportes.view lo tiene DIRECTOR y no DOCENTE: no debe heredarse.
		hp, err := client.HasPermission(bearer(fresh.AccessToken), &auth.HasPermissionRequest{UserId: dirID, Permission: "reportes.view"})
		if err != nil || hp.Allowed {
			t.Fatalf("reportes.view (solo DIRECTOR) no debía estar permitido tras el cambio: err=%v", err)
		}
	})

	// Caso 6 — el token viejo sigue rechazado (no recupera privilegios).
	if _, err := client.GetMyPermissions(bearer(old.AccessToken), &auth.GetMyPermissionsRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Caso6: el token viejo debía seguir rechazado, got %v", err)
	}
}
