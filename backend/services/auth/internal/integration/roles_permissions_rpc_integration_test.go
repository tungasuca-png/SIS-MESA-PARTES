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

// catalogoRolPermisos (Paso 21B) replica el conteo REAL de relaciones ya
// sembradas por la migración 005_seed_permissions_and_role_permissions —
// no es una regla nueva, es el mismo catálogo ya verificado en vivo
// (Paso 21A) contra auth_db. Se usa para probar que GetRolePermissions
// devuelve EXACTAMENTE lo que ya existe, no una lista inventada.
var catalogoRolPermisos = map[string]int{
	"ADMIN":       16,
	"DIRECTOR":    12,
	"SUBDIRECTOR": 12,
	"SECRETARIA":  12,
	"DOCENTE":     11,
	"AUXILIAR":    11,
	"SOLICITANTE": 6,
}

// catalogoPermisosReales (Paso 21B) son los 20 códigos reales ya sembrados
// (mismo catálogo verificado en el Paso 21A) — ListPermissions debe
// devolver exactamente estos 20, ni uno más ni uno menos.
var catalogoPermisosReales = []string{
	"dashboard.view", "expedientes.view", "expedientes.view_own", "expedientes.create",
	"expedientes.update", "expedientes.change_estado", "expedientes.delete",
	"documentos.view", "documentos.view_own", "documentos.create", "documentos.delete",
	"derivaciones.view", "derivaciones.create",
	"seguimiento.view", "seguimiento.view_own",
	"reportes.view", "solicitudes.create", "usuarios.view", "roles.view", "configuracion.view",
}

// TestRolesPermissionsRPC prueba ListRoles/ListPermissions/GetRolePermissions
// de punta a punta (JWT real -> AuthenticationInterceptor ->
// AuthorizationInterceptor -> RoleRepository/PermissionRepository/
// RolePermissionRepository -> PostgreSQL), usando exactamente las mismas
// DefaultMethodPolicies() de producción (ProtectedMethod("roles.view") ya
// agregado ahí, no un policy especial para el test) y el catálogo real
// sembrado por la migración 005.
func TestRolesPermissionsRPC(t *testing.T) {
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
	jwtManager := security.NewJWTManager("roles-permissions-rpc-test-secret", time.Hour)
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

	// Mismas policies que en producción: ListRoles/ListPermissions/
	// GetRolePermissions ya son ProtectedMethod("roles.view") en
	// DefaultMethodPolicies() -- no se agrega ningún policy especial para
	// el test.
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
	prefix := "hprp" + testID + "_"
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

	roleIDByName := func(t *testing.T, roleName string) string {
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
		`, prefix+suffix, prefix+suffix+"@test.local", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy").Scan(&userID)
		if err != nil {
			t.Fatalf("crear usuario de prueba: %v", err)
		}
		createdUserIDs = append(createdUserIDs, userID)
		for _, roleName := range roleNames {
			if _, err := pool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)", userID, roleIDByName(t, roleName)); err != nil {
				t.Fatalf("asignar rol %s: %v", roleName, err)
			}
		}
		return userID
	}

	authCtx := func(userID string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+mustToken(t, jwtManager, userID))
	}

	// countAll (Paso 21B, sección 20 — prueba de no mutación).
	countAll := func(t *testing.T) (roles, permissions, rolePermissions int) {
		t.Helper()
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM roles").Scan(&roles); err != nil {
			t.Fatalf("contar roles: %v", err)
		}
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM permissions").Scan(&permissions); err != nil {
			t.Fatalf("contar permissions: %v", err)
		}
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM role_permissions").Scan(&rolePermissions); err != nil {
			t.Fatalf("contar role_permissions: %v", err)
		}
		return
	}

	rolesAntes, permissionsAntes, rolePermissionsAntes := countAll(t)
	if rolesAntes != 7 || permissionsAntes != 20 || rolePermissionsAntes != 80 {
		t.Fatalf("precondición inesperada: roles=%d permissions=%d role_permissions=%d (se esperaba 7/20/80)", rolesAntes, permissionsAntes, rolePermissionsAntes)
	}

	// ---- ListRoles ----

	t.Run("ListRoles_Autorizado", func(t *testing.T) {
		adminID := createUser(t, "admin1", "ADMIN")
		resp, err := client.ListRoles(authCtx(adminID), &auth.ListRolesRequest{})
		if err != nil {
			t.Fatalf("se esperaba éxito para ADMIN/roles.view, got err=%v", err)
		}
		if len(resp.Roles) != 7 {
			t.Fatalf("se esperaban 7 roles, got %d", len(resp.Roles))
		}
		found := map[string]bool{}
		for _, r := range resp.Roles {
			found[r.Nombre] = true
			if r.Id == "" {
				t.Fatalf("rol sin id: %+v", r)
			}
		}
		for _, expected := range []string{"ADMIN", "AUXILIAR", "DIRECTOR", "DOCENTE", "SECRETARIA", "SOLICITANTE", "SUBDIRECTOR"} {
			if !found[expected] {
				t.Fatalf("falta el rol real %q en la respuesta", expected)
			}
		}
	})

	t.Run("ListRoles_SinPermiso_Denegado", func(t *testing.T) {
		docenteID := createUser(t, "docente1", "DOCENTE")
		_, err := client.ListRoles(authCtx(docenteID), &auth.ListRolesRequest{})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para DOCENTE (sin roles.view), got %v", err)
		}
	})

	t.Run("ListRoles_SinJWT_Unauthenticated", func(t *testing.T) {
		_, err := client.ListRoles(ctx, &auth.ListRolesRequest{})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})

	// ---- ListPermissions ----

	t.Run("ListPermissions_Autorizado_CatalogoCompleto", func(t *testing.T) {
		adminID := createUser(t, "admin2", "ADMIN")
		resp, err := client.ListPermissions(authCtx(adminID), &auth.ListPermissionsRequest{})
		if err != nil {
			t.Fatalf("se esperaba éxito para ADMIN/roles.view, got err=%v", err)
		}
		if len(resp.Permissions) != 20 {
			t.Fatalf("se esperaban 20 permisos, got %d", len(resp.Permissions))
		}
		found := map[string]bool{}
		for _, p := range resp.Permissions {
			found[p.Codigo] = true
			if p.Id == "" || p.Modulo == "" {
				t.Fatalf("permiso incompleto: %+v", p)
			}
		}
		for _, code := range catalogoPermisosReales {
			if !found[code] {
				t.Fatalf("falta el permiso real %q en la respuesta", code)
			}
		}
	})

	t.Run("ListPermissions_SinPermiso_Denegado", func(t *testing.T) {
		docenteID := createUser(t, "docente2", "DOCENTE")
		_, err := client.ListPermissions(authCtx(docenteID), &auth.ListPermissionsRequest{})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para DOCENTE (sin roles.view), got %v", err)
		}
	})

	t.Run("ListPermissions_SinJWT_Unauthenticated", func(t *testing.T) {
		_, err := client.ListPermissions(ctx, &auth.ListPermissionsRequest{})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})

	// ---- GetRolePermissions ----

	// Caso obligatorio (sección 18): los 7 roles reales, cada uno con el
	// conteo EXACTO ya sembrado por la migración 005.
	adminID := createUser(t, "admin3", "ADMIN")
	for roleName, expectedCount := range catalogoRolPermisos {
		roleName, expectedCount := roleName, expectedCount
		t.Run("GetRolePermissions_"+roleName, func(t *testing.T) {
			resp, err := client.GetRolePermissions(authCtx(adminID), &auth.GetRolePermissionsRequest{RoleId: roleIDByName(t, roleName)})
			if err != nil {
				t.Fatalf("se esperaba éxito consultando permisos de %s, got err=%v", roleName, err)
			}
			if len(resp.Permissions) != expectedCount {
				t.Fatalf("%s: se esperaban %d permisos, got %d (%+v)", roleName, expectedCount, len(resp.Permissions), resp.Permissions)
			}
		})
	}

	// Caso obligatorio (sección 19): aislamiento explícito por role_id —
	// "solicitudes.create" es EXCLUSIVO de SOLICITANTE en el catálogo real
	// (ver migración 005): ningún otro rol debe traerlo mezclado.
	t.Run("GetRolePermissions_Aislamiento", func(t *testing.T) {
		adminResp, err := client.GetRolePermissions(authCtx(adminID), &auth.GetRolePermissionsRequest{RoleId: roleIDByName(t, "ADMIN")})
		if err != nil {
			t.Fatalf("no se pudo consultar permisos de ADMIN: %v", err)
		}
		for _, p := range adminResp.Permissions {
			if p.Codigo == "solicitudes.create" {
				t.Fatal("ADMIN no debía traer 'solicitudes.create' (exclusivo de SOLICITANTE) — posible mezcla de relaciones por role_id")
			}
		}

		solicitanteResp, err := client.GetRolePermissions(authCtx(adminID), &auth.GetRolePermissionsRequest{RoleId: roleIDByName(t, "SOLICITANTE")})
		if err != nil {
			t.Fatalf("no se pudo consultar permisos de SOLICITANTE: %v", err)
		}
		hasSolicitudesCreate := false
		for _, p := range solicitanteResp.Permissions {
			if p.Codigo == "expedientes.view" {
				t.Fatal("SOLICITANTE no debía traer 'expedientes.view' (exclusivo de roles internos) — posible mezcla de relaciones por role_id")
			}
			if p.Codigo == "solicitudes.create" {
				hasSolicitudesCreate = true
			}
		}
		if !hasSolicitudesCreate {
			t.Fatal("SOLICITANTE debía traer 'solicitudes.create'")
		}
	})

	t.Run("GetRolePermissions_RolInexistente_NotFound", func(t *testing.T) {
		_, err := client.GetRolePermissions(authCtx(adminID), &auth.GetRolePermissionsRequest{RoleId: "00000000-0000-4000-8000-000000000000"})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un role_id inexistente, got %v", err)
		}
	})

	t.Run("GetRolePermissions_RoleIdInvalido_InvalidArgument", func(t *testing.T) {
		_, err := client.GetRolePermissions(authCtx(adminID), &auth.GetRolePermissionsRequest{RoleId: "no-es-un-uuid"})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument para un role_id malformado, got %v", err)
		}
	})

	t.Run("GetRolePermissions_SinPermiso_Denegado", func(t *testing.T) {
		docenteID := createUser(t, "docente3", "DOCENTE")
		_, err := client.GetRolePermissions(authCtx(docenteID), &auth.GetRolePermissionsRequest{RoleId: roleIDByName(t, "ADMIN")})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para DOCENTE (sin roles.view), got %v", err)
		}
	})

	t.Run("GetRolePermissions_SinJWT_Unauthenticated", func(t *testing.T) {
		_, err := client.GetRolePermissions(ctx, &auth.GetRolePermissionsRequest{RoleId: roleIDByName(t, "ADMIN")})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})

	// Prueba de no mutación (sección 20): después de TODAS las consultas de
	// solo lectura anteriores, los 3 conteos deben seguir exactamente
	// iguales.
	t.Run("NoMutacion", func(t *testing.T) {
		rolesDespues, permissionsDespues, rolePermissionsDespues := countAll(t)
		if rolesDespues != rolesAntes {
			t.Fatalf("roles cambió: antes=%d despues=%d", rolesAntes, rolesDespues)
		}
		if permissionsDespues != permissionsAntes {
			t.Fatalf("permissions cambió: antes=%d despues=%d", permissionsAntes, permissionsDespues)
		}
		if rolePermissionsDespues != rolePermissionsAntes {
			t.Fatalf("role_permissions cambió: antes=%d despues=%d", rolePermissionsAntes, rolePermissionsDespues)
		}
	})
}
