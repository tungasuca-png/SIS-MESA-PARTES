package integration

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"estudiantes/estudiantesclient"
	"psicologia/internal/clinicalcrypto"
	"psicologia/internal/config"
	"psicologia/internal/interceptor"
	"psicologia/internal/repository"
	"psicologia/internal/security"
	"psicologia/internal/server"
	"psicologia/internal/svc"
	"psicologia/psicologia"
	"usuarios/usuariosclient"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
)

// Estas pruebas usan la base REAL psicologia_db (PostgreSQL) y el servicio
// real (interceptor JWT, lógica, repositorio). Auth y Usuarios se sustituyen
// por dobles porque las identidades de prueba (UUID sintéticos) no existen en
// auth_db/usuarios_db; los dobles NO conocen ningún catálogo rol -> permiso:
// responden exactamente lo que cada prueba concede a cada userID.

const testJWTSecret = "psicologia-integration-test-secret"

func fixedUUID(base int64, index int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", (base%1_000_000_000)*1000+int64(index))
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}
}

// appPassword: contraseña de psicologia_user. Sale de PSICOLOGIA_DB_PASSWORD o
// de backend/.env (mismo origen que usa el servicio). Nunca se escribe en el repo.
func appPassword(t *testing.T) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv("PSICOLOGIA_DB_PASSWORD")); v != "" {
		return v
	}
	raw, err := os.ReadFile("../../../../.env")
	if err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "PSICOLOGIA_DB_PASSWORD=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "PSICOLOGIA_DB_PASSWORD="))
			}
		}
	}
	t.Fatal("falta PSICOLOGIA_DB_PASSWORD (variable de entorno o backend/.env)")
	return ""
}

func appDatabaseURL(t *testing.T) string {
	return fmt.Sprintf("postgres://psicologia_user:%s@127.0.0.1:5433/psicologia_db?sslmode=disable", appPassword(t))
}

// adminDatabaseURL: dueño de las tablas (auth_user, mismo patrón que las demás
// suites de integración). Solo se usa para verificar y limpiar datos de prueba.
func adminDatabaseURL(t *testing.T) string {
	t.Helper()
	// Credenciales de PRUEBA desde el entorno (backend/.env.test, fuera de
	// Git). Sin la variable la prueba se omite: nunca hay credenciales
	// escritas en el código ni un superusuario de respaldo.
	url := os.Getenv("TEST_PSICOLOGIA_DB_URL")
	if url == "" {
		t.Skip("TEST_PSICOLOGIA_DB_URL no definido: cargar backend/.env.test para ejecutar esta prueba")
	}
	return url
}

// grantsAuthClient responde HasPermission según lo que la prueba concede a cada
// userID (nunca según el rol del JWT). failing simula Auth caído.
type grantsAuthClient struct {
	authclient.Auth
	grants  map[string]map[string]bool
	failing bool
	calls   int
}

func (c *grantsAuthClient) HasPermission(_ context.Context, in *authclient.HasPermissionRequest, _ ...grpc.CallOption) (*authclient.HasPermissionResponse, error) {
	c.calls++
	if c.failing {
		return nil, fmt.Errorf("auth no disponible")
	}
	return &authclient.HasPermissionResponse{Allowed: c.grants[in.UserId][in.Permission]}, nil
}

// fakeUsuarios: usuarios que "existen" en Usuarios Service, con su
// tipo_usuario real (id -> tipo_usuario). Un id ausente del mapa no existe.
type fakeUsuarios struct {
	usuariosclient.Usuarios
	known map[string]string
}

func (c *fakeUsuarios) GetUsuariosBasic(_ context.Context, in *usuariosclient.GetUsuariosBasicRequest, _ ...grpc.CallOption) (*usuariosclient.GetUsuariosBasicResponse, error) {
	out := &usuariosclient.GetUsuariosBasicResponse{}
	for _, id := range in.Ids {
		if tipo, ok := c.known[id]; ok {
			out.Usuarios = append(out.Usuarios, &usuariosclient.UsuarioBasic{Id: id, TipoUsuario: tipo})
		}
	}
	return out, nil
}

// fakeEstudiantes implementa svc.EstudianteVerifier directamente (sin pasar
// por estudianteServiceVerifier): sirve solo para probar el PUNTO DE
// EXTENSIÓN en sí (que OpenExpediente lo llame y respete su resultado), no la
// traducción real de códigos gRPC.
type fakeEstudiantes struct{ exist map[string]bool }

func (f *fakeEstudiantes) Exists(_ context.Context, id string) (bool, error) { return f.exist[id], nil }

// fakeDirectorio: doble de svc.EstudianteDirectory para el PDF. failing
// simula Estudiantes caído o sin permiso (el PDF no debe generarse).
type fakeDirectorio struct {
	nombres map[string]string
	failing error
}

func (f *fakeDirectorio) NombreCompleto(_ context.Context, id string) (string, error) {
	if f.failing != nil {
		return "", f.failing
	}
	return f.nombres[id], nil
}

// scriptedEstudiantesClient es un doble del cliente gRPC REAL de Estudiantes
// Service (estudiantesclient.Estudiantes) — a diferencia de fakeEstudiantes,
// este SÍ se envuelve con svc.NewEstudianteVerifier (la implementación real,
// FASE 5C), así que ejercita de verdad el reenvío del JWT
// (interceptor.TokenFromContext) y la traducción de códigos gRPC
// (NotFound -> false,nil; cualquier otro código o error de transporte -> se
// propaga, fail-closed). Nunca conoce ningún catálogo rol->permiso: solo
// responde lo que cada prueba configura.
type scriptedEstudiantesClient struct {
	estudiantesclient.Estudiantes
	exist   map[string]bool // ids que SÍ existen
	down    bool            // simula Estudiantes Service inaccesible
	denied  bool            // simula que Estudiantes rechazó el permiso ("estudiantes.view")
	lastAuz string          // último header "authorization" recibido (para comprobar el reenvío del JWT)
}

func (c *scriptedEstudiantesClient) GetEstudiante(ctx context.Context, in *estudiantesclient.GetEstudianteRequest, _ ...grpc.CallOption) (*estudiantesclient.GetEstudianteResponse, error) {
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if v := md.Get("authorization"); len(v) > 0 {
			c.lastAuz = v[0]
		}
	}
	if c.down {
		return nil, status.Error(codes.Unavailable, "estudiantes no disponible (simulado)")
	}
	if c.denied {
		return nil, status.Error(codes.PermissionDenied, "el usuario no tiene el permiso requerido")
	}
	if !c.exist[in.Id] {
		return nil, status.Error(codes.NotFound, "estudiante no encontrado")
	}
	return &estudiantesclient.GetEstudianteResponse{Estudiante: &estudiantesclient.Estudiante{Id: in.Id}}, nil
}

type harness struct {
	t       *testing.T
	ctx     context.Context
	app     *pgxpool.Pool // psicologia_user (privilegios mínimos)
	admin   *pgxpool.Pool // dueño (solo verificación y limpieza)
	auth    *grantsAuthClient
	users   *fakeUsuarios
	svcCtx  *svc.ServiceContext
	client  psicologia.PsicologiaClient
	created []string // ids de expedientes creados (limpieza)
	actors  []string // usuarios de auditoría (limpieza)
	// cifrador: clave ALEATORIA por corrida (nunca una clave fija ni la real).
	cifrador *clinicalcrypto.Cipher
}

// nuevoCifradorDePrueba genera una clave aleatoria de 32 bytes con un key_id
// "test-..." propio de la corrida.
func nuevoCifradorDePrueba(t *testing.T) *clinicalcrypto.Cipher {
	t.Helper()
	clave := make([]byte, 32)
	sufijo := make([]byte, 6)
	if _, err := rand.Read(clave); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(sufijo); err != nil {
		t.Fatal(err)
	}
	c, err := clinicalcrypto.New(base64.StdEncoding.EncodeToString(clave), fmt.Sprintf("test-%x", sufijo), "")
	if err != nil {
		t.Fatalf("cifrador de prueba: %v", err)
	}
	return c
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	requireIntegration(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	app, err := pgxpool.New(ctx, appDatabaseURL(t))
	if err != nil {
		t.Fatalf("pool app: %v", err)
	}
	admin, err := pgxpool.New(ctx, adminDatabaseURL(t))
	if err != nil {
		t.Fatalf("pool admin: %v", err)
	}

	h := &harness{
		t: t, ctx: ctx, app: app, admin: admin,
		auth:     &grantsAuthClient{grants: map[string]map[string]bool{}},
		users:    &fakeUsuarios{known: map[string]string{}},
		cifrador: nuevoCifradorDePrueba(t),
	}
	h.svcCtx = &svc.ServiceContext{
		Config:         config.Config{JWTSecret: testJWTSecret},
		DB:             app,
		ExpedienteRepo: repository.NewExpedienteRepository(app, h.cifrador),
		AtencionRepo:   repository.NewAtencionRepository(app, h.cifrador),
		AuditoriaRepo:  repository.NewAuditoriaRepository(app),
		JWTValidator:   security.NewJWTValidator(testJWTSecret),
		AuthClient:     h.auth,
		UsuariosClient: h.users,
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.AuthenticationInterceptor(h.svcCtx.JWTValidator)))
	psicologia.RegisterPsicologiaServer(grpcServer, server.NewPsicologiaServer(h.svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	h.client = psicologia.NewPsicologiaClient(conn)

	t.Cleanup(func() {
		conn.Close()
		grpcServer.Stop()
		h.cleanup()
		app.Close()
		admin.Close()
	})
	return h
}

// grant concede permisos a un userID (y lo registra para la limpieza).
func (h *harness) grant(userID string, perms ...string) {
	if h.auth.grants[userID] == nil {
		h.auth.grants[userID] = map[string]bool{}
	}
	for _, p := range perms {
		h.auth.grants[userID][p] = true
	}
	h.actors = append(h.actors, userID)
}

func (h *harness) tokenFor(userID, role string) string {
	claims := security.Claims{Username: "t", Role: role, RegisteredClaims: jwt.RegisteredClaims{
		Subject: userID, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		h.t.Fatalf("firmar jwt: %v", err)
	}
	return tok
}

func (h *harness) as(userID, role string) context.Context {
	return metadata.AppendToOutgoingContext(h.ctx, "authorization", "Bearer "+h.tokenFor(userID, role))
}

// cleanup elimina lo creado por la prueba. La bitácora es append-only (ni
// psicologia_user ni el dueño pueden borrarla por SQL normal: el trigger lo
// impide), así que la limpieza —solo de datos de PRUEBA— la hace un superusuario
// con session_replication_role = replica, que desactiva los triggers.
func (h *harness) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := h.admin.Acquire(ctx)
	if err != nil {
		return
	}
	defer conn.Release()
	_, _ = conn.Exec(ctx, "SET session_replication_role = replica")
	_, _ = conn.Exec(ctx, "DELETE FROM auditoria_psicologica WHERE usuario_id::text = ANY($1) OR recurso_id::text = ANY($2)", h.actors, h.created)
	// Atenciones de prueba (versiones -> atenciones -> contador) antes que sus
	// expedientes: las FK son RESTRICT a propósito.
	_, _ = conn.Exec(ctx, "DELETE FROM atencion_versiones WHERE atencion_id IN (SELECT id FROM atenciones_psicologicas WHERE expediente_id::text = ANY($1))", h.created)
	_, _ = conn.Exec(ctx, "DELETE FROM atenciones_psicologicas WHERE expediente_id::text = ANY($1)", h.created)
	_, _ = conn.Exec(ctx, "DELETE FROM atencion_numero_counters WHERE expediente_id::text = ANY($1)", h.created)
	_, _ = conn.Exec(ctx, "DELETE FROM expedientes_psicologicos WHERE id::text = ANY($1)", h.created)
	// El contador vuelve al máximo REAL existente: las pruebas no dejan huecos.
	_, _ = conn.Exec(ctx, `
		UPDATE expediente_codigo_counters c SET ultimo = COALESCE((
			SELECT MAX(SUBSTRING(codigo FROM 10)::int) FROM expedientes_psicologicos e
			WHERE e.codigo LIKE 'PSI-' || LPAD(c.anio::text, 4, '0') || '-%'), 0)
	`)
	// Canarios de las claves de prueba (key_id "test-...").
	_, _ = conn.Exec(ctx, "DELETE FROM cifrado_canario WHERE key_id LIKE 'test-%'")
	_, _ = conn.Exec(ctx, "RESET session_replication_role")
}

func (h *harness) countAudit(usuarioID, accion, recursoID string) int {
	var n int
	err := h.admin.QueryRow(h.ctx, `
		SELECT COUNT(*) FROM auditoria_psicologica
		WHERE usuario_id = $1::uuid AND accion = $2 AND ($3::text = '' OR recurso_id = NULLIF($3::text, '')::uuid)
	`, usuarioID, accion, recursoID).Scan(&n)
	if err != nil {
		h.t.Fatalf("contar auditoría: %v", err)
	}
	return n
}

func (h *harness) track(exp *psicologia.ExpedientePsicologico) *psicologia.ExpedientePsicologico {
	if exp != nil {
		h.created = append(h.created, exp.Id)
	}
	return exp
}
