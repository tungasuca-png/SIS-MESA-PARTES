package integration

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"expedientes/expedientes"
	"expedientes/internal/config"
	"expedientes/internal/interceptor"
	"expedientes/internal/repository"
	"expedientes/internal/security"
	"expedientes/internal/server"
	"expedientes/internal/svc"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"github.com/zeromicro/go-zero/zrpc"
	"net"
)

// realSharedJWTSecret es el MISMO secreto que backend/.env (JWT_SECRET) —
// tiene que coincidir con el que usa el Auth Service real corriendo en
// 127.0.0.1:8080, porque este test valida el flujo end-to-end real:
// Expedientes -> Auth.HasPermission -> PostgreSQL, no un doble de prueba.
const realSharedJWTSecret = "41807ca75b3f05df6db6a17da41503e4b4fcb2521f6edf5e7651984e63db6d6c"

// TestHasPermissionIntegration prueba la integración REAL (Paso 13B):
// Expedientes -> Auth.HasPermission (RPC real, contra un Auth Service real
// corriendo en 127.0.0.1:8080) -> AuthorizationService -> PostgreSQL real
// (auth_db), combinada con las reglas de negocio existentes de Expedientes
// (ownership/área/estado), que NO cambian.
//
// Requiere, además de INTEGRATION_TEST=true, que el Auth Service real esté
// corriendo (go run auth.go -f etc/auth.yaml, con JWT_SECRET =
// realSharedJWTSecret) — si no está disponible, el propio Test G lo
// verifica como caso esperado (fail-closed), y los demás casos fallarían
// con un error de conexión claro, no con un falso "permitido".
func TestHasPermissionIntegration(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	expedientesPool, err := pgxpool.New(ctx, integrationDatabaseURL())
	if err != nil {
		t.Fatalf("create expedientes_db pool: %v", err)
	}
	defer expedientesPool.Close()
	if err := expedientesPool.Ping(ctx); err != nil {
		t.Fatalf("connect to expedientes_db: %v", err)
	}

	authPool, err := pgxpool.New(ctx, authDatabaseURL())
	if err != nil {
		t.Fatalf("create auth_db pool: %v", err)
	}
	defer authPool.Close()
	if err := authPool.Ping(ctx); err != nil {
		t.Fatalf("connect to auth_db: %v", err)
	}

	testID := strconv.FormatInt(time.Now().UnixNano()%1e8, 10)
	usernamePrefix := "hpexp" + testID + "_"
	var createdAuthUserIDs []string
	var createdExpedienteIDs []string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, id := range createdExpedienteIDs {
			if _, err := expedientesPool.Exec(cleanupCtx, "DELETE FROM expedientes WHERE id = $1", id); err != nil {
				t.Logf("cleanup expediente %s: %v", id, err)
			}
		}
		for _, id := range createdAuthUserIDs {
			if _, err := authPool.Exec(cleanupCtx, "DELETE FROM usuarios WHERE id = $1", id); err != nil {
				t.Logf("cleanup usuario %s: %v", id, err)
			}
		}
	}()

	roleID := func(t *testing.T, roleName string) string {
		t.Helper()
		var id string
		if err := authPool.QueryRow(ctx, "SELECT id FROM roles WHERE nombre = $1 AND estado = TRUE", roleName).Scan(&id); err != nil {
			t.Fatalf("buscar rol real %s en auth_db: %v", roleName, err)
		}
		return id
	}

	// createRealUser inserta un usuario REAL en auth_db (no un id
	// sintético) y le asigna, también en usuario_roles, el/los rol(es)
	// indicados — así Auth.HasPermission puede resolverlo de verdad.
	createRealUser := func(t *testing.T, suffix string, roleNames ...string) string {
		t.Helper()
		var userID string
		err := authPool.QueryRow(ctx, `
			INSERT INTO usuarios (username, email, password_hash, estado)
			VALUES ($1, $2, $3, TRUE)
			RETURNING id
		`, usernamePrefix+suffix, usernamePrefix+suffix+"@test.local", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy").Scan(&userID)
		if err != nil {
			t.Fatalf("crear usuario real en auth_db: %v", err)
		}
		createdAuthUserIDs = append(createdAuthUserIDs, userID)
		for _, roleName := range roleNames {
			if _, err := authPool.Exec(ctx, "INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)", userID, roleID(t, roleName)); err != nil {
				t.Fatalf("asignar rol real %s: %v", roleName, err)
			}
		}
		return userID
	}

	// token firma un JWT con el secreto real compartido — el claim "role"
	// es el que usan las reglas de negocio de ESTE servicio (área/
	// ownership), tal como ya hace authContext() en
	// expedientes_integration_test.go; el permiso real, en cambio, lo
	// resuelve Auth consultando los roles reales asignados arriba, no este
	// claim.
	token := func(userID, jwtRole string) string {
		claims := security.Claims{
			Username: "integration-test",
			Role:     jwtRole,
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   userID,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}
		signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(realSharedJWTSecret))
		return signed
	}
	authCtx := func(userID, jwtRole string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token(userID, jwtRole))
	}

	// ServiceContext real: JWTValidator con el secreto real (para que el
	// propio AuthenticationInterceptor de Expedientes acepte los tokens de
	// arriba) y AuthClient real, vía zrpc, contra el Auth Service real.
	svcCtx := &svc.ServiceContext{
		Config:               config.Config{JWTSecret: realSharedJWTSecret, AuthRpc: zrpc.RpcClientConf{Target: "127.0.0.1:8080"}},
		DB:                   expedientesPool,
		ExpedienteRepository: repository.NewExpedienteRepository(expedientesPool),
		JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
		AuthClient:           authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(svcCtx.JWTValidator),
	))
	expedientes.RegisterExpedientesServer(grpcServer, server.NewExpedientesServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC test server: %v", err)
	}
	defer conn.Close()
	client := expedientes.NewExpedientesClient(conn)

	trackExpediente := func(codigo string) {
		createdExpedienteIDs = append(createdExpedienteIDs, codigo)
	}

	// Test A — permiso permitido: SECRETARIA (rol REAL en auth_db) tiene
	// expedientes.create -> debe poder crear.
	t.Run("PermisoPermitido", func(t *testing.T) {
		userID := createRealUser(t, "secretaria", "SECRETARIA")
		resp, err := client.CreateExpediente(authCtx(userID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test permiso permitido", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito para SECRETARIA/expedientes.create, got err=%v", err)
		}
		trackExpediente(resp.Expediente.Id)
	})

	// Test B — permiso denegado: DOCENTE (rol REAL) NO tiene
	// expedientes.create -> PermissionDenied.
	t.Run("PermisoDenegado", func(t *testing.T) {
		userID := createRealUser(t, "docente", "DOCENTE")
		_, err := client.CreateExpediente(authCtx(userID, "DOCENTE"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test permiso denegado", Prioridad: "NORMAL",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para DOCENTE/expedientes.create, got %v", err)
		}
	})

	// Test C — multi-rol: usuario con DOCENTE (sin expedientes.create) +
	// SECRETARIA (con expedientes.create) asignados de verdad en
	// usuario_roles. El JWT lleva SOLO "DOCENTE" como claim (igual que un
	// login real solo llevaría un rol) — si el resultado depende del JWT,
	// esto fallaría; si depende de Auth (todos sus roles reales), debe
	// permitir.
	t.Run("MultiRol", func(t *testing.T) {
		userID := createRealUser(t, "multirol", "DOCENTE", "SECRETARIA")
		resp, err := client.CreateExpediente(authCtx(userID, "DOCENTE"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test multi-rol", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito (multi-rol incluye SECRETARIA con expedientes.create), got err=%v", err)
		}
		trackExpediente(resp.Expediente.Id)
	})

	// Test D — ownership: un SOLICITANTE con expedientes.view_own no debe
	// poder ver el expediente de OTRO solicitante, aunque el permiso en sí
	// esté concedido.
	t.Run("Ownership", func(t *testing.T) {
		userA := createRealUser(t, "solicitantea", "SOLICITANTE")
		userB := createRealUser(t, "solicitanteb", "SOLICITANTE")
		created, err := client.CreateExpediente(authCtx(userA, "SOLICITANTE"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test ownership", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		_, err = client.GetExpediente(authCtx(userB, "SOLICITANTE"), &expedientes.GetExpedienteRequest{Id: created.Expediente.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: otro solicitante no debe ver un expediente ajeno, got %v", err)
		}
	})

	// Test E — área: DOCENTE (rol REAL, con expedientes.change_estado
	// según el catálogo) intenta cambiar el estado de un expediente que
	// está en área SECRETARIA (default al crearse) — el permiso se
	// concede, pero el área debe seguir bloqueando.
	t.Run("Area", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria2", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test area", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		docenteID := createRealUser(t, "docente2", "DOCENTE")
		_, err = client.ChangeEstado(authCtx(docenteID, "DOCENTE"), &expedientes.ChangeEstadoRequest{
			Id: created.Expediente.Id, NuevoEstado: "EN_PROCESO",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por área (expediente en SECRETARIA, actor DOCENTE), got %v", err)
		}
	})

	// Test F — estado/transición: SECRETARIA (mismo área, permiso
	// concedido) no debe poder saltar directamente de PENDIENTE a
	// ATENDIDO — la tabla de transiciones sigue aplicando.
	t.Run("Estado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria3", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test estado", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		_, err = client.ChangeEstado(authCtx(secretariaID, "SECRETARIA"), &expedientes.ChangeEstadoRequest{
			Id: created.Expediente.Id, NuevoEstado: "ATENDIDO",
		})
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("se esperaba FailedPrecondition por transición inválida PENDIENTE->ATENDIDO, got %v", err)
		}
	})

	// Tests de PASO 18B — UpdateExpediente ahora también valida ÁREA,
	// reutilizando exactamente CanViewAll/AreaDelRol (mismas funciones que
	// UpdateArea/DerivarExpediente/RechazarExpediente/ResolverExpediente,
	// sin duplicar el mapeo rol->área). Un expediente recién creado por
	// SECRETARIA queda con area_actual = SECRETARIA (default real de
	// CreateExpediente, ver expediente_repository.go).

	// Caso 1 — ADMIN: acceso global, sin importar el área del expediente.
	t.Run("UpdateExpediente_Admin_CualquierArea", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria4", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test update area admin", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		adminID := createRealUser(t, "admin1", "ADMIN")
		_, err = client.UpdateExpediente(authCtx(adminID, "ADMIN"), &expedientes.UpdateExpedienteRequest{
			Id: created.Expediente.Id, Asunto: "Editado por ADMIN", Prioridad: "URGENTE",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito para ADMIN sobre un expediente de cualquier área, got err=%v", err)
		}
	})

	// Caso 2 — usuario del área correcta: SECRETARIA (CanViewAll, ve/edita
	// cualquier área) editando un expediente que además está, literalmente,
	// en su propia área (SECRETARIA es el área default al crear).
	t.Run("UpdateExpediente_SecretariaMismaArea", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria5", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test update area secretaria", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		_, err = client.UpdateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.UpdateExpedienteRequest{
			Id: created.Expediente.Id, Asunto: "Editado por SECRETARIA", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito para SECRETARIA sobre un expediente en su propia área, got err=%v", err)
		}
	})

	// Caso 3 — usuario de otra área: DIRECTOR (con expedientes.update, pero
	// SIN CanViewAll — ver AreaDelRol) intenta editar un expediente cuya
	// area_actual sigue siendo SECRETARIA (nunca se derivó a Dirección).
	t.Run("UpdateExpediente_DirectorOtraArea_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria6", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test update area director", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		directorID := createRealUser(t, "director1", "DIRECTOR")
		_, err = client.UpdateExpediente(authCtx(directorID, "DIRECTOR"), &expedientes.UpdateExpedienteRequest{
			Id: created.Expediente.Id, Asunto: "Intento de edición ajena", Prioridad: "NORMAL",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por área (expediente en SECRETARIA, actor DIRECTOR), got %v", err)
		}
	})

	// Caso 4 / Caso 6 — SOLICITANTE no tiene "expedientes.update" en el
	// catálogo real: Auth.HasPermission devuelve allowed=false (no un
	// error), y eso debe traducirse en PermissionDenied ANTES de llegar a
	// leer el expediente o comparar áreas — incluso sobre su PROPIO
	// expediente. En el catálogo actual ningún rol interno carece de
	// "expedientes.update", así que este es también el caso real de
	// "Auth responde false" pedido como Caso 6.
	t.Run("UpdateExpediente_Solicitante_Denegado", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitantec", "SOLICITANTE")
		created, err := client.CreateExpediente(authCtx(solicitanteID, "SOLICITANTE"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test update area solicitante", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		_, err = client.UpdateExpediente(authCtx(solicitanteID, "SOLICITANTE"), &expedientes.UpdateExpedienteRequest{
			Id: created.Expediente.Id, Asunto: "Intento propio sin permiso", Prioridad: "NORMAL",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: SOLICITANTE no tiene expedientes.update ni sobre su propio expediente, got %v", err)
		}
	})

	// Caso 5 — expediente inexistente: el permiso se concede (SECRETARIA sí
	// tiene expedientes.update), pero el id no existe -> debe conservar el
	// comportamiento previo (NotFound), no un error distinto.
	t.Run("UpdateExpediente_Inexistente_NotFound", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria7", "SECRETARIA")
		_, err := client.UpdateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.UpdateExpedienteRequest{
			Id: "00000000-0000-4000-8000-000000000000", Asunto: "No existe", Prioridad: "NORMAL",
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un expediente inexistente, got %v", err)
		}
	})

	// Caso 8 — campos: tras la validación de área, UpdateExpediente debe
	// seguir modificando ÚNICAMENTE asunto/descripcion/prioridad; area_actual,
	// estado y solicitante_id deben permanecer intactos.
	t.Run("UpdateExpediente_CamposPermitidos", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria8", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Original", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)
		original := created.Expediente

		updated, err := client.UpdateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.UpdateExpedienteRequest{
			Id: created.Expediente.Id, Asunto: "Modificado", Descripcion: "Nueva descripción", Prioridad: "URGENTE",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito, got err=%v", err)
		}
		got := updated.Expediente
		if got.Asunto != "Modificado" || got.Descripcion != "Nueva descripción" || got.Prioridad != "URGENTE" {
			t.Fatalf("los campos permitidos no se actualizaron correctamente: %+v", got)
		}
		if got.AreaActual != original.AreaActual {
			t.Fatalf("area_actual no debe cambiar por UpdateExpediente: antes=%s despues=%s", original.AreaActual, got.AreaActual)
		}
		if got.Estado != original.Estado {
			t.Fatalf("estado no debe cambiar por UpdateExpediente: antes=%s despues=%s", original.Estado, got.Estado)
		}
		if got.SolicitanteId != original.SolicitanteId {
			t.Fatalf("solicitante_id no debe cambiar por UpdateExpediente: antes=%s despues=%s", original.SolicitanteId, got.SolicitanteId)
		}
	})

	// activoDe consulta directamente expedientes_db (fuera del RPC) para
	// confirmar sin ambigüedad si la baja lógica ocurrió o no.
	activoDe := func(t *testing.T, id string) bool {
		t.Helper()
		var activo bool
		if err := expedientesPool.QueryRow(ctx, "SELECT activo FROM expedientes WHERE id = $1", id).Scan(&activo); err != nil {
			t.Fatalf("consultar activo de %s: %v", id, err)
		}
		return activo
	}

	// Tests de PASO 18C — DeleteExpediente ahora exige, además del permiso:
	// estado PENDIENTE, y la MISMA regla de área que UpdateExpediente
	// (Paso 18B) — reutiliza CanViewAll/AreaDelRol, sin duplicar el mapeo.
	// NOTA: el estado "RECHAZADO" mencionado en la especificación no existe
	// en este servicio (verificado: ni estados.go ni ningún repositorio lo
	// define — un F4 rechazado vuelve a OBSERVADO, ver RechazarYDevolver).
	// No se crea un estado nuevo solo para probarlo; los 4 estados reales
	// (PENDIENTE/EN_PROCESO/OBSERVADO/ATENDIDO) cubren la regla igual.

	// Caso 1 — ADMIN + PENDIENTE + cualquier área -> PERMITIDO, activo=FALSE.
	t.Run("DeleteExpediente_Admin_PendienteCualquierArea", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria9", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test delete admin", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		adminID := createRealUser(t, "admin2", "ADMIN")
		_, err = client.DeleteExpediente(authCtx(adminID, "ADMIN"), &expedientes.DeleteExpedienteRequest{Id: created.Expediente.Id})
		if err != nil {
			t.Fatalf("se esperaba éxito para ADMIN sobre PENDIENTE de cualquier área, got err=%v", err)
		}
		if activoDe(t, created.Expediente.Id) {
			t.Fatal("se esperaba activo=FALSE tras la baja lógica")
		}
	})

	// Caso 2 — interno + misma área + PENDIENTE -> PERMITIDO.
	t.Run("DeleteExpediente_MismaArea_Pendiente", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria10", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test delete misma area", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		_, err = client.DeleteExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.DeleteExpedienteRequest{Id: created.Expediente.Id})
		if err != nil {
			t.Fatalf("se esperaba éxito para SECRETARIA sobre un expediente en su propia área, got err=%v", err)
		}
		if activoDe(t, created.Expediente.Id) {
			t.Fatal("se esperaba activo=FALSE tras la baja lógica")
		}
	})

	// Caso 3 — interno + otra área + PENDIENTE -> PermissionDenied, activo
	// permanece TRUE.
	t.Run("DeleteExpediente_OtraArea_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria11", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test delete otra area", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		directorID := createRealUser(t, "director2", "DIRECTOR")
		_, err = client.DeleteExpediente(authCtx(directorID, "DIRECTOR"), &expedientes.DeleteExpedienteRequest{Id: created.Expediente.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por área (expediente en SECRETARIA, actor DIRECTOR), got %v", err)
		}
		if !activoDe(t, created.Expediente.Id) {
			t.Fatal("activo debía permanecer TRUE: la baja no debió ejecutarse")
		}
	})

	// Caso 4 / Caso 11 — SOLICITANTE no tiene "expedientes.delete": Auth
	// responde allowed=false (no un error), incluso sobre su propio
	// expediente PENDIENTE. En el catálogo actual ningún interno carece de
	// "expedientes.delete", así que este es también el caso real de
	// "Auth responde false" pedido como Caso 11.
	t.Run("DeleteExpediente_Solicitante_Denegado", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitanted", "SOLICITANTE")
		created, err := client.CreateExpediente(authCtx(solicitanteID, "SOLICITANTE"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test delete solicitante", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		_, err = client.DeleteExpediente(authCtx(solicitanteID, "SOLICITANTE"), &expedientes.DeleteExpedienteRequest{Id: created.Expediente.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: SOLICITANTE no tiene expedientes.delete ni sobre su propio expediente, got %v", err)
		}
		if !activoDe(t, created.Expediente.Id) {
			t.Fatal("activo debía permanecer TRUE: la baja no debió ejecutarse")
		}
	})

	// Casos 5/6/7 — estado distinto de PENDIENTE: el permiso y el área se
	// cumplen (SECRETARIA, CanViewAll, sobre su propio expediente), pero el
	// estado ya no es PENDIENTE -> PermissionDenied (regla explícita del
	// Paso 18C), activo permanece TRUE.
	estadoNoPendienteCases := []struct {
		nombre      string
		transitions []string // NuevoEstado aplicados en orden vía ChangeEstado
	}{
		{"EnProceso", []string{"EN_PROCESO"}},
		{"Observado", []string{"OBSERVADO"}},
		{"Atendido", []string{"EN_PROCESO", "ATENDIDO"}},
	}
	for _, tc := range estadoNoPendienteCases {
		tc := tc
		t.Run("DeleteExpediente_Denegado_"+tc.nombre, func(t *testing.T) {
			secretariaID := createRealUser(t, "secretaria_"+strings.ToLower(tc.nombre), "SECRETARIA")
			created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
				Tipo: "CERTIFICADO", Asunto: "Test delete estado " + tc.nombre, Prioridad: "NORMAL",
			})
			if err != nil {
				t.Fatalf("no se pudo crear el expediente base: %v", err)
			}
			trackExpediente(created.Expediente.Id)

			for _, nuevoEstado := range tc.transitions {
				if _, err := client.ChangeEstado(authCtx(secretariaID, "SECRETARIA"), &expedientes.ChangeEstadoRequest{
					Id: created.Expediente.Id, NuevoEstado: nuevoEstado,
				}); err != nil {
					t.Fatalf("no se pudo preparar el estado %s: %v", nuevoEstado, err)
				}
			}

			_, err = client.DeleteExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.DeleteExpedienteRequest{Id: created.Expediente.Id})
			if status.Code(err) != codes.PermissionDenied {
				t.Fatalf("se esperaba PermissionDenied por estado != PENDIENTE (%s), got %v", tc.nombre, err)
			}
			if !activoDe(t, created.Expediente.Id) {
				t.Fatal("activo debía permanecer TRUE: la baja no debió ejecutarse")
			}
		})
	}

	// Caso 9 — expediente inexistente: el permiso se concede (SECRETARIA sí
	// tiene expedientes.delete), pero el id no existe -> NotFound, mismo
	// comportamiento previo a este paso.
	t.Run("DeleteExpediente_Inexistente_NotFound", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria12", "SECRETARIA")
		_, err := client.DeleteExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.DeleteExpedienteRequest{
			Id: "00000000-0000-4000-8000-000000000000",
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un expediente inexistente, got %v", err)
		}
	})

	// Caso 10 — expediente ya inactivo (eliminado antes): FindByIDOrCodigo
	// ya filtra activo = TRUE, así que un segundo intento debe conservar el
	// mismo NotFound que ya producía el comportamiento previo a este paso.
	t.Run("DeleteExpediente_YaInactivo_NotFound", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria13", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test delete ya inactivo", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		if _, err := client.DeleteExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.DeleteExpedienteRequest{Id: created.Expediente.Id}); err != nil {
			t.Fatalf("primera baja debía tener éxito: %v", err)
		}

		_, err = client.DeleteExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.DeleteExpedienteRequest{Id: created.Expediente.Id})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un expediente ya inactivo, got %v", err)
		}
	})

	// Tests de PASO 20B.2 — ValidateDerivacion: RPC de solo lectura, sin
	// efectos secundarios, que expone estados.IsValidArea + estados.CanDerivar
	// (las MISMAS funciones ya usadas por DerivarExpediente) para que
	// Derivaciones pueda validar una transición sin ejecutarla.

	// areaYEstado consulta directamente expedientes_db para confirmar que
	// ValidateDerivacion no mutó nada (Caso 12).
	areaYEstado := func(t *testing.T, id string) (string, string) {
		t.Helper()
		var area, estado string
		if err := expedientesPool.QueryRow(ctx, "SELECT area_actual, estado FROM expedientes WHERE id = $1", id).Scan(&area, &estado); err != nil {
			t.Fatalf("consultar area/estado de %s: %v", id, err)
		}
		return area, estado
	}

	// Caso 1 — SECRETARIA -> DIRECTOR, CERTIFICADO (F2 real): valido=true.
	t.Run("ValidateDerivacion_F2_SecretariaADirector_Valido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria14", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test ValidateDerivacion F2", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)
		areaAntes, estadoAntes := areaYEstado(t, created.Expediente.Id)

		resp, err := client.ValidateDerivacion(authCtx(secretariaID, "SECRETARIA"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "DIRECTOR",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito, got err=%v", err)
		}
		if !resp.Valido {
			t.Fatal("se esperaba valido=true para SECRETARIA->DIRECTOR en CERTIFICADO (F2 real)")
		}
		areaDespues, estadoDespues := areaYEstado(t, created.Expediente.Id)
		if areaAntes != areaDespues || estadoAntes != estadoDespues {
			t.Fatalf("ValidateDerivacion NO debía mutar nada: area %s->%s, estado %s->%s", areaAntes, areaDespues, estadoAntes, estadoDespues)
		}
	})

	// Caso 2 — SECRETARIA -> AREA_INVENTADA: valido=false (IsValidArea).
	t.Run("ValidateDerivacion_AreaInventada_Invalido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria15", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test area inventada", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		resp, err := client.ValidateDerivacion(authCtx(secretariaID, "SECRETARIA"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "AREA_INVENTADA",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito (con valido=false), got err=%v", err)
		}
		if resp.Valido {
			t.Fatal("se esperaba valido=false: AREA_INVENTADA no es un área real (IsValidArea)")
		}
	})

	// Caso 3 — CERTIFICADO, SECRETARIA -> SUBDIRECTOR: valido=false (no está
	// en derivacionesPermitidas para F2).
	t.Run("ValidateDerivacion_F2_TransicionNoPermitida_Invalido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria16", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test F2 transicion no permitida", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		resp, err := client.ValidateDerivacion(authCtx(secretariaID, "SECRETARIA"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "SUBDIRECTOR",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito (con valido=false), got err=%v", err)
		}
		if resp.Valido {
			t.Fatal("se esperaba valido=false: SECRETARIA->SUBDIRECTOR no existe para CERTIFICADO (F2)")
		}
	})

	// Caso 4 — PERMISO, SECRETARIA -> DIRECTOR: valido=true (F4 real).
	t.Run("ValidateDerivacion_F4_SecretariaADirector_Valido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria17", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "PERMISO", Asunto: "Test F4 secretaria->director", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		resp, err := client.ValidateDerivacion(authCtx(secretariaID, "SECRETARIA"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "DIRECTOR",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito, got err=%v", err)
		}
		if !resp.Valido {
			t.Fatal("se esperaba valido=true para SECRETARIA->DIRECTOR en PERMISO (F4 real)")
		}
	})

	// Caso 5 — PERMISO, DIRECTOR -> SUBDIRECTOR: valido=true (F4 real,
	// "derivación aprobatoria"). Requiere el expediente REALMENTE en
	// DIRECTOR (derivado ahí primero).
	t.Run("ValidateDerivacion_F4_DirectorASubdirector_Valido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria18", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "PERMISO", Asunto: "Test F4 director->subdirector", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)
		if _, err := client.DerivarExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.DerivarExpedienteRequest{
			Id: created.Expediente.Id, AreaDestino: "DIRECTOR",
		}); err != nil {
			t.Fatalf("no se pudo derivar a DIRECTOR: %v", err)
		}

		directorID := createRealUser(t, "director6", "DIRECTOR")
		resp, err := client.ValidateDerivacion(authCtx(directorID, "DIRECTOR"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "SUBDIRECTOR",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito, got err=%v", err)
		}
		if !resp.Valido {
			t.Fatal("se esperaba valido=true para DIRECTOR->SUBDIRECTOR en PERMISO (F4 real)")
		}
	})

	// Caso 6 — PERMISO, SUBDIRECTOR -> DOCENTE: valido=true (F4 real,
	// cierre). Requiere el expediente REALMENTE en SUBDIRECTOR.
	t.Run("ValidateDerivacion_F4_SubdirectorADocente_Valido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria19", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "PERMISO", Asunto: "Test F4 subdirector->docente", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)
		if _, err := client.DerivarExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.DerivarExpedienteRequest{
			Id: created.Expediente.Id, AreaDestino: "DIRECTOR",
		}); err != nil {
			t.Fatalf("no se pudo derivar a DIRECTOR: %v", err)
		}
		directorID := createRealUser(t, "director7", "DIRECTOR")
		if _, err := client.DerivarExpediente(authCtx(directorID, "DIRECTOR"), &expedientes.DerivarExpedienteRequest{
			Id: created.Expediente.Id, AreaDestino: "SUBDIRECTOR",
		}); err != nil {
			t.Fatalf("no se pudo derivar a SUBDIRECTOR: %v", err)
		}

		subdirectorID := createRealUser(t, "subdirector1", "SUBDIRECTOR")
		resp, err := client.ValidateDerivacion(authCtx(subdirectorID, "SUBDIRECTOR"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "DOCENTE",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito, got err=%v", err)
		}
		if !resp.Valido {
			t.Fatal("se esperaba valido=true para SUBDIRECTOR->DOCENTE en PERMISO (F4 real, cierre)")
		}
	})

	// Caso 7 — PERMISO, SECRETARIA -> DOCENTE: valido=false (salto directo,
	// no existe en derivacionesPermitidas para F4).
	t.Run("ValidateDerivacion_F4_SaltoDirecto_Invalido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria20", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "PERMISO", Asunto: "Test F4 salto directo", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		resp, err := client.ValidateDerivacion(authCtx(secretariaID, "SECRETARIA"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "DOCENTE",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito (con valido=false), got err=%v", err)
		}
		if resp.Valido {
			t.Fatal("se esperaba valido=false: SECRETARIA->DOCENTE no existe para PERMISO (F4), salta SUBDIRECTOR")
		}
	})

	// Caso 8 — tipo genérico (OTRO): permisivo, cualquier área válida.
	t.Run("ValidateDerivacion_TipoGenerico_Permisivo", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria21", "SECRETARIA")
		created, err := client.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "OTRO", Asunto: "Test tipo generico", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		resp, err := client.ValidateDerivacion(authCtx(secretariaID, "SECRETARIA"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "SUBDIRECTOR",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito, got err=%v", err)
		}
		if !resp.Valido {
			t.Fatal("se esperaba valido=true: tipo genérico (OTRO) es permisivo hacia cualquier área válida")
		}
	})

	// Caso 9 — expediente inexistente: NotFound.
	t.Run("ValidateDerivacion_ExpedienteInexistente_NotFound", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria22", "SECRETARIA")
		_, err := client.ValidateDerivacion(authCtx(secretariaID, "SECRETARIA"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: "00000000-0000-4000-8000-000000000000", AreaDestino: "DIRECTOR",
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound, got %v", err)
		}
	})

	// Caso 10 — expediente ajeno: SOLICITANTE B no puede ni siquiera
	// validar transiciones sobre el expediente de SOLICITANTE A
	// (PermissionDenied, misma regla de ownership que ya usa GetExpediente
	// — no se revela nada sobre el expediente ajeno).
	t.Run("ValidateDerivacion_ExpedienteAjeno_PermissionDenied", func(t *testing.T) {
		solicitanteA := createRealUser(t, "solicitantex", "SOLICITANTE")
		created, err := client.CreateExpediente(authCtx(solicitanteA, "SOLICITANTE"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test expediente ajeno", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base: %v", err)
		}
		trackExpediente(created.Expediente.Id)

		solicitanteB := createRealUser(t, "solicitantey", "SOLICITANTE")
		_, err = client.ValidateDerivacion(authCtx(solicitanteB, "SOLICITANTE"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: created.Expediente.Id, AreaDestino: "DIRECTOR",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: el expediente es de otro solicitante, got %v", err)
		}
	})

	// Caso 11 — JWT inválido/ausente: Unauthenticated.
	t.Run("ValidateDerivacion_SinJWT_Unauthenticated", func(t *testing.T) {
		_, err := client.ValidateDerivacion(context.Background(), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: "00000000-0000-4000-8000-000000000000", AreaDestino: "DIRECTOR",
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
	})

	// Test G — Auth no disponible: un AuthClient apuntando a un puerto sin
	// nada escuchando debe producir un ERROR (fail-closed), nunca
	// permitir la operación por fallback al rol local.
	t.Run("AuthNoDisponible", func(t *testing.T) {
		downSvcCtx := &svc.ServiceContext{
			Config:               config.Config{JWTSecret: realSharedJWTSecret},
			DB:                   expedientesPool,
			ExpedienteRepository: repository.NewExpedienteRepository(expedientesPool),
			JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
			AuthClient: authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19999", // puerto sin servicio, a propósito
				NonBlock: true,              // no bloquear/fallar al construir: la
				// falla real debe verse en la llamada (fail-closed), no en el arranque
				Timeout: 2000,
			})),
		}
		downListener := bufconn.Listen(1024 * 1024)
		downServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downSvcCtx.JWTValidator),
		))
		expedientes.RegisterExpedientesServer(downServer, server.NewExpedientesServer(downSvcCtx))
		go func() { _ = downServer.Serve(downListener) }()
		defer downServer.Stop()

		downConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Auth caído): %v", err)
		}
		defer downConn.Close()
		downClient := expedientes.NewExpedientesClient(downConn)

		userID := createRealUser(t, "authcaido", "SECRETARIA")
		_, err = downClient.CreateExpediente(authCtx(userID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test Auth no disponible", Prioridad: "NORMAL",
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Auth Service no está disponible, la operación se permitió")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}

		// Caso 7 (Paso 18B) — UpdateExpediente también debe fallar cerrado:
		// el expediente se crea con Auth arriba (vía el cliente normal), y
		// luego se intenta editar a través del cliente conectado al Auth
		// caído. Debe fallar en RequirePermission, sin llegar siquiera a
		// comparar el área.
		created, err := client.CreateExpediente(authCtx(userID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test update area Auth no disponible", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base (con Auth arriba): %v", err)
		}
		trackExpediente(created.Expediente.Id)

		_, err = downClient.UpdateExpediente(authCtx(userID, "SECRETARIA"), &expedientes.UpdateExpedienteRequest{
			Id: created.Expediente.Id, Asunto: "No debería aplicarse", Prioridad: "NORMAL",
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para UpdateExpediente cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para UpdateExpediente, got OK")
		}

		// Caso 12 (Paso 18C) — DeleteExpediente también debe fallar cerrado,
		// y la baja NO debe ejecutarse (activo permanece TRUE).
		deleted, err := client.CreateExpediente(authCtx(userID, "SECRETARIA"), &expedientes.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test delete Auth no disponible", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente base (con Auth arriba): %v", err)
		}
		trackExpediente(deleted.Expediente.Id)

		_, err = downClient.DeleteExpediente(authCtx(userID, "SECRETARIA"), &expedientes.DeleteExpedienteRequest{Id: deleted.Expediente.Id})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para DeleteExpediente cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para DeleteExpediente, got OK")
		}
		if !activoDe(t, deleted.Expediente.Id) {
			t.Fatal("activo debía permanecer TRUE: la baja no debió ejecutarse con Auth caído")
		}

		// Caso (Paso 20B.2) — ValidateDerivacion también debe fallar
		// cerrado con Auth caído: la consulta a Auth.HasPermission
		// ("expedientes.view"/"expedientes.view_own") ya falla, sin
		// llegar siquiera a leer el expediente.
		_, err = downClient.ValidateDerivacion(authCtx(userID, "SECRETARIA"), &expedientes.ValidateDerivacionRequest{
			ExpedienteId: deleted.Expediente.Id, AreaDestino: "DIRECTOR",
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para ValidateDerivacion cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para ValidateDerivacion, got OK")
		}
	})
}

func authDatabaseURL() string {
	if value := os.Getenv("AUTH_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://auth_user:auth_password@127.0.0.1:5433/auth_db?sslmode=disable"
}
