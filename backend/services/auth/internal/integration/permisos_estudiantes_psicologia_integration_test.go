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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// permisosValidadosEnBackend son los códigos que Estudiantes Service y
// Psicología Service exigen con Auth.HasPermission (literales del código de
// producción). Deben existir y estar activos en el catálogo: un permiso que
// el backend exige pero no existe dejaría la operación inaccesible para todos.
var permisosValidadosEnBackend = []string{
	"estudiantes.view", "estudiantes.create", "estudiantes.update", "estudiantes.change_estado",
	"estudiantes.personal.view", "estudiantes.personal.update",
	"estudiantes.familia.view", "estudiantes.familia.update", "estudiantes.matricula.update",
	"psicologia.expedientes.view", "psicologia.expedientes.view_own",
	"psicologia.expedientes.create", "psicologia.expedientes.update",
	"psicologia.atenciones.view", "psicologia.atenciones.create",
	"psicologia.atenciones.correct", "psicologia.atenciones.annul",
}

// rolesClinicosPermitidos: únicos roles que pueden tener permisos de
// expedientes o atenciones psicológicas. ADMIN figura porque hoy los tiene
// asignados; si la institución decide retirárselos (pendiente), se quita de
// esta lista y la prueba lo vigila.
var rolesClinicosPermitidos = map[string]bool{"PSICOLOGO": true, "ADMIN": true}

// TestPermisosEstudiantesPsicologiaPorRol valida contra la BD VIVA y por el
// RPC real (JWT -> interceptores de producción -> HasPermission ->
// PostgreSQL), con un usuario temporal por rol:
//   - que la respuesta de HasPermission coincida con las asignaciones reales
//     (role_permissions) para cada permiso de estudiantes y psicología;
//   - que todo permiso exigido por el backend exista y esté activo;
//   - invariantes que no dependen de decisiones pendientes: SOLICITANTE
//     (usuario externo) nunca tiene permisos de estudiantes ni de psicología,
//     y los permisos clínicos solo están en roles clínicos permitidos.
//
// No modifica asignaciones ni el catálogo; solo crea y borra sus usuarios.
func TestPermisosEstudiantesPsicologiaPorRol(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, integrationDatabaseURL(t))
	if err != nil {
		t.Fatalf("create PostgreSQL pool: %v", err)
	}
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	rolePermissionRepo := repository.NewRolePermissionRepository(pool)
	authorizationService := authorization.NewAuthorizationService(userRepo, roleRepo, rolePermissionRepo)
	jwtManager := security.NewJWTManager("permisos-por-rol-test-secret", time.Hour)
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

	prefix := "pr" + strconv.FormatInt(time.Now().UnixNano()%1e8, 10) + "_"
	var creados []string
	defer func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		for _, id := range creados {
			if _, err := pool.Exec(c, "DELETE FROM usuarios WHERE id = $1", id); err != nil {
				t.Logf("cleanup usuario %s: %v", id, err)
			}
		}
	}()

	// Catálogo y asignaciones vivas (solo lectura).
	activos := map[string]bool{}
	rows, err := pool.Query(ctx, `SELECT codigo, estado FROM permissions WHERE codigo LIKE 'estudiantes.%' OR codigo LIKE 'psicologia.%'`)
	if err != nil {
		t.Fatal(err)
	}
	var codigos []string
	for rows.Next() {
		var c string
		var estado bool
		if err := rows.Scan(&c, &estado); err != nil {
			t.Fatal(err)
		}
		activos[c] = estado
		codigos = append(codigos, c)
	}
	rows.Close()
	sort.Strings(codigos)

	asignado := map[string]map[string]bool{}
	rows, err = pool.Query(ctx, `
		SELECT r.nombre, p.codigo FROM role_permissions rp
		JOIN roles r ON r.id = rp.role_id JOIN permissions p ON p.id = rp.permission_id
		WHERE r.estado AND p.estado AND (p.codigo LIKE 'estudiantes.%' OR p.codigo LIKE 'psicologia.%')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var rol, c string
		if err := rows.Scan(&rol, &c); err != nil {
			t.Fatal(err)
		}
		if asignado[rol] == nil {
			asignado[rol] = map[string]bool{}
		}
		asignado[rol][c] = true
	}
	rows.Close()

	var roles []string
	rows, err = pool.Query(ctx, `SELECT nombre FROM roles WHERE estado ORDER BY nombre`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		roles = append(roles, r)
	}
	rows.Close()

	t.Run("PermisosDelBackend_ExistenYActivos", func(t *testing.T) {
		for _, c := range permisosValidadosEnBackend {
			estado, ok := activos[c]
			if !ok || !estado {
				t.Errorf("el backend exige %q pero no existe o está inactivo en el catálogo", c)
			}
		}
	})

	t.Run("HasPermission_CoincideConAsignaciones_PorRol", func(t *testing.T) {
		for _, rol := range roles {
			var userID string
			if err := pool.QueryRow(ctx, `
				INSERT INTO usuarios (username, email, password_hash, estado) VALUES ($1, $2, $3, TRUE) RETURNING id
			`, prefix+strings.ToLower(rol), prefix+strings.ToLower(rol)+"@test.local",
				"$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy").Scan(&userID); err != nil {
				t.Fatalf("crear usuario %s: %v", rol, err)
			}
			creados = append(creados, userID)
			if _, err := pool.Exec(ctx, `INSERT INTO usuario_roles (usuario_id, rol_id) SELECT $1, id FROM roles WHERE nombre = $2`, userID, rol); err != nil {
				t.Fatalf("asignar rol %s: %v", rol, err)
			}
			authCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+mustToken(t, jwtManager, userID))

			var concedidos []string
			for _, c := range codigos {
				resp, err := client.HasPermission(authCtx, &auth.HasPermissionRequest{UserId: userID, Permission: c})
				if err != nil {
					t.Fatalf("HasPermission %s/%s: %v", rol, c, err)
				}
				if resp.Allowed != (asignado[rol][c] && activos[c]) {
					t.Errorf("%s/%s: HasPermission=%v, asignación real=%v", rol, c, resp.Allowed, asignado[rol][c])
				}
				if resp.Allowed {
					concedidos = append(concedidos, c)
				}
				// Invariante 1: un usuario externo nunca accede a estudiantes ni a psicología.
				if rol == "SOLICITANTE" && resp.Allowed {
					t.Errorf("SOLICITANTE no debe tener %s", c)
				}
				// Invariante 2: permisos clínicos solo en roles clínicos permitidos.
				clinico := strings.HasPrefix(c, "psicologia.expedientes.") || strings.HasPrefix(c, "psicologia.atenciones.")
				if clinico && resp.Allowed && !rolesClinicosPermitidos[rol] {
					t.Errorf("permiso clínico %s asignado a %s (no es un rol clínico permitido)", c, rol)
				}
			}
			t.Logf("%-12s %d permisos de estudiantes/psicología: %s", rol, len(concedidos), strings.Join(concedidos, ", "))
		}
	})
}
