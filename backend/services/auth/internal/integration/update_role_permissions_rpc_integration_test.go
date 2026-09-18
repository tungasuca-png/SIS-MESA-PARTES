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

// catalogoRolPermisos21B / catalogoPermisosReales viven en
// roles_permissions_rpc_integration_test.go (mismo paquete) y se reutilizan
// acá para la verificación final de no-mutación (sección 13: los conteos
// por rol deben seguir exactamente como en el Paso 21B).

// TestUpdateRolePermissionsRPC prueba UpdateRolePermissions de punta a
// punta (JWT real -> AuthenticationInterceptor -> AuthorizationInterceptor
// -> RolePermissionRepository.ReplacePermissionsByRoleID transaccional ->
// PostgreSQL), usando exactamente las mismas DefaultMethodPolicies() de
// producción (ProtectedMethod("roles.view"), ya agregado ahí) y el
// catálogo real sembrado por la migración 005.
//
// Todas las mutaciones se hacen exclusivamente sobre el rol SOLICITANTE
// (el más pequeño: 6 permisos reales) y cada subtest que lo modifica lo
// restaura a su estado real original antes de terminar (defer), para que
// al final de la suite los conteos globales (7 roles / 20 permissions / 80
// role_permissions) y por rol quedn exactamente iguales a los del Paso 21B.
func TestUpdateRolePermissionsRPC(t *testing.T) {
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
	jwtManager := security.NewJWTManager("update-role-permissions-rpc-test-secret", time.Hour)
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

	prefix := "hpurp" + strconv.FormatInt(time.Now().UnixNano()%1e8, 10) + "_"
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

	permissionIDByCode := func(t *testing.T, code string) string {
		t.Helper()
		var id string
		if err := pool.QueryRow(ctx, "SELECT id FROM permissions WHERE codigo = $1", code).Scan(&id); err != nil {
			t.Fatalf("buscar permiso %s: %v", code, err)
		}
		return id
	}

	currentPermissionIDs := func(t *testing.T, roleID string) []string {
		t.Helper()
		rows, err := pool.Query(ctx, "SELECT permission_id FROM role_permissions WHERE role_id = $1", roleID)
		if err != nil {
			t.Fatalf("consultar role_permissions de %s: %v", roleID, err)
		}
		defer rows.Close()
		ids := make([]string, 0)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatalf("leer permission_id: %v", err)
			}
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return ids
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

	adminID := createUser(t, "admin", "ADMIN")
	docenteID := createUser(t, "docente", "DOCENTE")
	solicitanteRoleID := roleIDByName(t, "SOLICITANTE")
	solicitanteOriginalIDs := currentPermissionIDs(t, solicitanteRoleID)
	if len(solicitanteOriginalIDs) != 6 {
		t.Fatalf("precondición inesperada: SOLICITANTE debía tener 6 permisos reales, got %d", len(solicitanteOriginalIDs))
	}

	restoreSolicitante := func(t *testing.T) {
		t.Helper()
		if _, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: solicitanteOriginalIDs,
		}); err != nil {
			t.Fatalf("no se pudo restaurar SOLICITANTE a su estado original: %v", err)
		}
	}
	// Red de seguridad final, además de las restauraciones explícitas de
	// cada subtest -- si algo dejara el estado a medias, esto lo corrige
	// antes de que termine la suite.
	defer restoreSolicitante(t)

	// ---- Autorización ----

	t.Run("Autorizado_ADMIN_MismoConjunto", func(t *testing.T) {
		resp, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: solicitanteOriginalIDs,
		})
		if err != nil {
			t.Fatalf("se esperaba éxito para ADMIN/roles.view, got err=%v", err)
		}
		got := append([]string{}, resp.PermissionIds...)
		sort.Strings(got)
		if !equalStringSlices(got, solicitanteOriginalIDs) {
			t.Fatalf("se esperaba el mismo conjunto original, got %v", got)
		}
	})

	t.Run("SinPermiso_Denegado", func(t *testing.T) {
		_, err := client.UpdateRolePermissions(authCtx(docenteID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: solicitanteOriginalIDs,
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para DOCENTE (sin roles.view), got %v", err)
		}
	})

	t.Run("SinJWT_Unauthenticated", func(t *testing.T) {
		_, err := client.UpdateRolePermissions(ctx, &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: solicitanteOriginalIDs,
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})

	// ---- role_id ----

	t.Run("RoleIdMalformado_InvalidArgument", func(t *testing.T) {
		_, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: "no-es-un-uuid", PermissionIds: solicitanteOriginalIDs,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument para role_id malformado, got %v", err)
		}
	})

	t.Run("RolInexistente_NotFound", func(t *testing.T) {
		_, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: "00000000-0000-4000-8000-000000000000", PermissionIds: solicitanteOriginalIDs,
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un role_id inexistente, got %v", err)
		}
	})

	// ---- permission_ids ----

	t.Run("PermissionIdMalformado_InvalidArgument", func(t *testing.T) {
		_, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: []string{"no-es-un-uuid"},
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument para un permission_id malformado, got %v", err)
		}
	})

	// Caso obligatorio también de ATOMICIDAD: un permission_id real +
	// uno con formato válido pero inexistente -> NotFound, y el estado
	// anterior de SOLICITANTE debe permanecer INTACTO (la validación ocurre
	// DENTRO de la transacción, antes de cualquier DELETE).
	t.Run("PermisoInexistente_NotFound_Atomicidad", func(t *testing.T) {
		antes := currentPermissionIDs(t, solicitanteRoleID)
		_, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID,
			PermissionIds: []string{
				solicitanteOriginalIDs[0],
				"00000000-0000-4000-8000-000000000099", // formato válido, no existe
			},
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un permission_id inexistente, got %v", err)
		}
		despues := currentPermissionIDs(t, solicitanteRoleID)
		if !equalStringSlices(antes, despues) {
			t.Fatalf("atomicidad rota: SOLICITANTE cambió tras una operación inválida — antes=%v despues=%v", antes, despues)
		}
	})

	// ---- Persistencia / reemplazo / eliminación completa / duplicados ----

	t.Run("Persistencia_GetTrasUpdate", func(t *testing.T) {
		defer restoreSolicitante(t)

		nuevoConjunto := []string{solicitanteOriginalIDs[0], solicitanteOriginalIDs[1]}
		if _, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: nuevoConjunto,
		}); err != nil {
			t.Fatalf("no se pudo actualizar: %v", err)
		}

		getResp, err := client.GetRolePermissions(authCtx(adminID), &auth.GetRolePermissionsRequest{RoleId: solicitanteRoleID})
		if err != nil {
			t.Fatalf("no se pudo consultar GetRolePermissions: %v", err)
		}
		got := make([]string, 0, len(getResp.Permissions))
		for _, p := range getResp.Permissions {
			got = append(got, p.Id)
		}
		sort.Strings(got)
		sortedNuevo := append([]string{}, nuevoConjunto...)
		sort.Strings(sortedNuevo)
		if !equalStringSlices(got, sortedNuevo) {
			t.Fatalf("GetRolePermissions no coincide con lo actualizado: esperado=%v got=%v", sortedNuevo, got)
		}
	})

	t.Run("Reemplazo_ABC_a_AD", func(t *testing.T) {
		defer restoreSolicitante(t)

		a := solicitanteOriginalIDs[0]
		b := solicitanteOriginalIDs[1]
		c := solicitanteOriginalIDs[2]
		d := permissionIDByCode(t, "usuarios.view") // real, NO asignado hoy a SOLICITANTE

		if _, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: []string{a, b, c},
		}); err != nil {
			t.Fatalf("no se pudo preparar el estado [A,B,C]: %v", err)
		}

		resp, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: []string{a, d},
		})
		if err != nil {
			t.Fatalf("no se pudo reemplazar a [A,D]: %v", err)
		}
		got := append([]string{}, resp.PermissionIds...)
		sort.Strings(got)
		esperado := []string{a, d}
		sort.Strings(esperado)
		if !equalStringSlices(got, esperado) {
			t.Fatalf("se esperaba exactamente [A,D]=%v, got %v (B y C debían desaparecer)", esperado, got)
		}
	})

	t.Run("EliminacionCompleta_a_Vacio", func(t *testing.T) {
		defer restoreSolicitante(t)

		resp, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: []string{},
		})
		if err != nil {
			t.Fatalf("se esperaba éxito con permission_ids vacío, got err=%v", err)
		}
		if len(resp.PermissionIds) != 0 {
			t.Fatalf("se esperaban 0 permisos, got %v", resp.PermissionIds)
		}
		getResp, err := client.GetRolePermissions(authCtx(adminID), &auth.GetRolePermissionsRequest{RoleId: solicitanteRoleID})
		if err != nil {
			t.Fatalf("no se pudo consultar GetRolePermissions: %v", err)
		}
		if len(getResp.Permissions) != 0 {
			t.Fatalf("se esperaba el rol sin ningún permiso, got %d", len(getResp.Permissions))
		}
	})

	t.Run("Duplicados_Normalizados", func(t *testing.T) {
		defer restoreSolicitante(t)

		a := solicitanteOriginalIDs[0]
		b := solicitanteOriginalIDs[1]
		resp, err := client.UpdateRolePermissions(authCtx(adminID), &auth.UpdateRolePermissionsRequest{
			RoleId: solicitanteRoleID, PermissionIds: []string{a, a, b},
		})
		if err != nil {
			t.Fatalf("se esperaba éxito normalizando duplicados, got err=%v", err)
		}
		if len(resp.PermissionIds) != 2 {
			t.Fatalf("se esperaban 2 permisos únicos tras deduplicar [A,A,B], got %d (%v)", len(resp.PermissionIds), resp.PermissionIds)
		}
		// Confirma en BD que no quedó ninguna fila duplicada (la PK
		// compuesta ya lo garantizaría, pero se verifica explícitamente).
		dbIDs := currentPermissionIDs(t, solicitanteRoleID)
		if len(dbIDs) != 2 {
			t.Fatalf("se esperaban 2 filas en role_permissions, got %d", len(dbIDs))
		}
	})

	// ---- No mutación global (sección 13) ----

	t.Run("NoMutacionGlobal", func(t *testing.T) {
		// Restaura explícitamente ANTES de verificar (además del defer
		// final) para que esta verificación sea significativa incluso si
		// se ejecuta con -run apuntando solo a este subtest en el futuro.
		restoreSolicitante(t)

		var roles, permissions, rolePermissions int
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM roles").Scan(&roles); err != nil {
			t.Fatalf("contar roles: %v", err)
		}
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM permissions").Scan(&permissions); err != nil {
			t.Fatalf("contar permissions: %v", err)
		}
		if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM role_permissions").Scan(&rolePermissions); err != nil {
			t.Fatalf("contar role_permissions: %v", err)
		}
		if roles != 7 {
			t.Fatalf("roles cambió: got %d, se esperaban 7", roles)
		}
		if permissions != 20 {
			t.Fatalf("permissions cambió: got %d, se esperaban 20", permissions)
		}
		if rolePermissions != 80 {
			t.Fatalf("role_permissions cambió: got %d, se esperaban 80", rolePermissions)
		}
		for roleName, expectedCount := range catalogoRolPermisos {
			got := currentPermissionIDs(t, roleIDByName(t, roleName))
			if len(got) != expectedCount {
				t.Fatalf("%s: se esperaban %d permisos tras restaurar, got %d", roleName, expectedCount, len(got))
			}
		}
	})
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
