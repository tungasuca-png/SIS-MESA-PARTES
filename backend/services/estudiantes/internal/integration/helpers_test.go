package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"estudiantes/estudiantes"
	"estudiantes/internal/config"
	"estudiantes/internal/interceptor"
	"estudiantes/internal/repository"
	"estudiantes/internal/security"
	"estudiantes/internal/server"
	"estudiantes/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// scriptedAuthClient es un doble de Auth.HasPermission, mismo patrón ya usado
// en Psicología/Usuarios/Derivaciones: concede por userID+permission
// explícitamente (nunca por nombre de rol), y puede simular "Auth caído"
// para probar fail-closed. Embebe authclient.Auth (nil) para satisfacer la
// interfaz completa sin implementar cada método uno por uno — solo se llama
// HasPermission en esta suite.
type scriptedAuthClient struct {
	authclient.Auth
	grants  map[string]map[string]bool
	failing bool
}

func (c *scriptedAuthClient) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	if c.failing {
		return nil, status.Error(codes.Unavailable, "auth service no disponible (simulado)")
	}
	return &authclient.HasPermissionResponse{Allowed: c.grants[in.UserId][in.Permission]}, nil
}

// Estas pruebas usan la base REAL estudiantes_db y el servicio real
// (interceptor JWT, lógica, repositorio). No hay ningún RPC de alta en esta
// fase (FASE 5A): los fixtures se insertan por SQL directo contra el mismo
// pool que usa la aplicación (estudiantes_user, que ya tiene INSERT/UPDATE
// otorgado por la migración 001) y se eliminan al terminar cada test.

const testJWTSecret = "estudiantes-integration-test-secret"

func requireIntegration(t *testing.T) {
	t.Helper()
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
}

// appPassword: contraseña de estudiantes_user. Sale de ESTUDIANTES_DB_PASSWORD
// o de backend/.env (mismo origen que usa el servicio). Nunca se escribe en
// el repo.
func appPassword(t *testing.T) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv("ESTUDIANTES_DB_PASSWORD")); v != "" {
		return v
	}
	raw, err := os.ReadFile("../../../../.env")
	if err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ESTUDIANTES_DB_PASSWORD=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "ESTUDIANTES_DB_PASSWORD="))
			}
		}
	}
	t.Fatal("falta ESTUDIANTES_DB_PASSWORD (variable de entorno o backend/.env)")
	return ""
}

func appDatabaseURL(t *testing.T) string {
	return fmt.Sprintf("postgres://estudiantes_user:%s@127.0.0.1:5433/estudiantes_db?sslmode=disable", appPassword(t))
}

// adminDatabaseURL: dueño de la tabla (auth_user, mismo patrón que las demás
// suites de integración del proyecto). Solo se usa para verificar el estado
// final de la BD (que no quedaron datos de prueba).
func adminDatabaseURL(t *testing.T) string {
	t.Helper()
	// Credenciales de PRUEBA desde el entorno (backend/.env.test, fuera de
	// Git). Sin la variable la prueba se omite: nunca hay credenciales
	// escritas en el código ni un superusuario de respaldo.
	url := os.Getenv("TEST_ESTUDIANTES_DB_URL")
	if url == "" {
		t.Skip("TEST_ESTUDIANTES_DB_URL no definido: cargar backend/.env.test para ejecutar esta prueba")
	}
	return url
}

type harness struct {
	t       *testing.T
	ctx     context.Context
	app     *pgxpool.Pool // estudiantes_user (privilegios reales de la app)
	admin   *pgxpool.Pool // dueño (solo verificación)
	auth    *scriptedAuthClient
	svcCtx  *svc.ServiceContext
	client  estudiantes.EstudiantesClient
	created []string // ids insertados por los fixtures (limpieza)
	// Modelo maestro: fixtures de matrícula y familia (limpieza).
	periodos []string
	personas []string
}

// grant concede un permiso a un userID para esta prueba (nunca por rol).
func (h *harness) grant(userID, permission string) {
	if h.auth.grants[userID] == nil {
		h.auth.grants[userID] = map[string]bool{}
	}
	h.auth.grants[userID][permission] = true
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	requireIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	app, err := pgxpool.New(ctx, appDatabaseURL(t))
	if err != nil {
		t.Fatalf("pool app: %v", err)
	}
	admin, err := pgxpool.New(ctx, adminDatabaseURL(t))
	if err != nil {
		t.Fatalf("pool admin: %v", err)
	}

	h := &harness{t: t, ctx: ctx, app: app, admin: admin, auth: &scriptedAuthClient{grants: map[string]map[string]bool{}}}
	h.svcCtx = &svc.ServiceContext{
		Config:               config.Config{JWTSecret: testJWTSecret},
		DB:                   app,
		EstudianteRepository: repository.NewEstudianteRepository(app),
		JWTValidator:         security.NewJWTValidator(testJWTSecret),
		AuthClient:           h.auth,
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.AuthenticationInterceptor(h.svcCtx.JWTValidator)))
	estudiantes.RegisterEstudiantesServer(grpcServer, server.NewEstudiantesServer(h.svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	h.client = estudiantes.NewEstudiantesClient(conn)

	t.Cleanup(func() {
		conn.Close()
		grpcServer.Stop()
		h.cleanup()
		app.Close()
		admin.Close()
	})
	return h
}

func (h *harness) tokenFor(userID string) string {
	claims := security.Claims{Username: "t", Role: "ADMIN", RegisteredClaims: jwt.RegisteredClaims{
		Subject: userID, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		h.t.Fatalf("firmar jwt: %v", err)
	}
	return tok
}

func (h *harness) as(userID string) context.Context {
	return metadata.AppendToOutgoingContext(h.ctx, "authorization", "Bearer "+h.tokenFor(userID))
}

// seed inserta un estudiante por SQL directo (no hay RPC de alta en esta
// fase) y lo registra para limpieza automática. NumeroDocumento (FASE 5D)
// es opcional: las pruebas que no lo necesitan simplemente no lo pasan.
// FASE E-03: sin NumeroDocumento el estudiante es SIN_DOCUMENTO (número
// NULL); con número, TipoDocumento es DNI salvo que la prueba indique otro.
type seedInput struct {
	Codigo, Nombres, Apellidos, TipoDocumento, NumeroDocumento, Nivel, Grado, Seccion, Turno, EstadoMatricula string
	// FASE E-04 (opcionales; vacío = NULL, y Estado vacío = ACTIVO).
	CodigoSiagie, ApellidoPaterno, ApellidoMaterno, Estado string
}

func (h *harness) seed(in seedInput) string {
	h.t.Helper()
	estado := in.EstadoMatricula
	if estado == "" {
		estado = "ACTIVO"
	}
	tipo := in.TipoDocumento
	if tipo == "" {
		tipo = "SIN_DOCUMENTO"
		if in.NumeroDocumento != "" {
			tipo = "DNI"
		}
	}
	var id string
	err := h.app.QueryRow(h.ctx, `
		INSERT INTO estudiantes (codigo, nombres, apellidos, tipo_documento, numero_documento, nivel, grado, seccion, turno, estado_matricula,
			codigo_siagie, apellido_paterno, apellido_materno, estado)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9, $10,
			NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''), COALESCE(NULLIF($14, ''), 'ACTIVO'))
		RETURNING id
	`, in.Codigo, in.Nombres, in.Apellidos, tipo, in.NumeroDocumento, in.Nivel, in.Grado, in.Seccion, in.Turno, estado,
		in.CodigoSiagie, in.ApellidoPaterno, in.ApellidoMaterno, in.Estado).Scan(&id)
	if err != nil {
		h.t.Fatalf("seed estudiante: %v", err)
	}
	h.created = append(h.created, id)
	return id
}

// cleanup borra los fixtures de prueba con el pool ADMIN (dueño de la
// tabla): estudiantes_user, a propósito, no tiene DELETE otorgado (no hay
// baja física prevista para la aplicación en esta fase — ver migración
// 001), así que la limpieza no puede hacerse con el mismo pool que usa el
// servicio.
func (h *harness) cleanup() {
	if len(h.created) == 0 && len(h.periodos) == 0 && len(h.personas) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// FASE E-05: auditoria_estudiantes es append-only (trigger). Como en
	// Psicología, la limpieza —solo de datos de PRUEBA— usa
	// session_replication_role = replica (concedido solo a estudiantes_tester,
	// ver db/roles/01_roles.sql), que desactiva los triggers en esta sesión.
	conn, err := h.admin.Acquire(ctx)
	if err != nil {
		h.t.Errorf("no se pudo obtener conexión de limpieza: %v", err)
		return
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SET session_replication_role = replica"); err != nil {
		h.t.Errorf("no se pudo preparar la limpieza: %v", err)
		return
	}
	defer func() { _, _ = conn.Exec(context.Background(), "RESET session_replication_role") }()
	if _, err := conn.Exec(ctx, "DELETE FROM auditoria_estudiantes WHERE estudiante_id = ANY($1)", h.created); err != nil {
		h.t.Errorf("no se pudo limpiar la auditoría de prueba: %v", err)
	}
	// Modelo maestro: matrícula (movimientos -> matrículas -> secciones ->
	// periodos) y familia (relaciones -> personas), solo de PRUEBA.
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{"DELETE FROM matricula_movimientos WHERE matricula_id IN (SELECT id FROM matriculas WHERE estudiante_id = ANY($1) OR periodo_id = ANY($2))", []any{h.created, h.periodos}},
		{"DELETE FROM matriculas WHERE estudiante_id = ANY($1) OR periodo_id = ANY($2)", []any{h.created, h.periodos}},
		{"DELETE FROM secciones WHERE periodo_id = ANY($1)", []any{h.periodos}},
		{"DELETE FROM periodos_academicos WHERE id = ANY($1)", []any{h.periodos}},
		{"DELETE FROM estudiante_familiares WHERE estudiante_id = ANY($1) OR persona_id = ANY($2)", []any{h.created, h.personas}},
		{"DELETE FROM personas WHERE id = ANY($1)", []any{h.personas}},
	} {
		if _, err := conn.Exec(ctx, q.sql, q.args...); err != nil {
			h.t.Errorf("no se pudo limpiar fixtures del modelo maestro: %v", err)
		}
	}
	if _, err := conn.Exec(ctx, "DELETE FROM estudiantes WHERE id = ANY($1)", h.created); err != nil {
		h.t.Errorf("no se pudieron limpiar los estudiantes de prueba: %v", err)
	}
}
