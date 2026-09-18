package integration

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"derivaciones/derivaciones"
	"derivaciones/internal/config"
	"derivaciones/internal/interceptor"
	"derivaciones/internal/repository"
	"derivaciones/internal/security"
	"derivaciones/internal/server"
	"derivaciones/internal/svc"
	"expedientes/expedientesclient"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// realSharedJWTSecret es el MISMO secreto que backend/.env (JWT_SECRET) —
// tiene que coincidir con el que usa el Auth Service real corriendo en
// 127.0.0.1:8080, porque este test valida el flujo end-to-end real:
// Derivaciones -> Auth.HasPermission -> PostgreSQL, no un doble de prueba.
// No existía ningún test de integración previo para Derivaciones (Paso 15
// lo confirmó: internal/integration/ estaba vacía) — este archivo es
// nuevo por completo, no una actualización.
const realSharedJWTSecret = "41807ca75b3f05df6db6a17da41503e4b4fcb2521f6edf5e7651984e63db6d6c"

func fixedUUID(base int64, index int) string {
	value := (base%10_000_000_000)*10 + int64(index)
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", value)
}

// TestHasPermissionIntegration prueba la integración REAL (Paso 15B):
// Derivaciones -> Auth.HasPermission (RPC real, contra un Auth Service
// real corriendo en 127.0.0.1:8080) -> AuthorizationService -> PostgreSQL
// real (auth_db), combinada con las reglas de negocio existentes de
// Derivaciones (CanCreate/CanViewAll), que NO cambian.
func TestHasPermissionIntegration(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	derivacionesPool, err := pgxpool.New(ctx, integrationDatabaseURL())
	if err != nil {
		t.Fatalf("create derivaciones_db pool: %v", err)
	}
	defer derivacionesPool.Close()
	if err := derivacionesPool.Ping(ctx); err != nil {
		t.Fatalf("connect to derivaciones_db: %v", err)
	}

	authPool, err := pgxpool.New(ctx, authDatabaseURL())
	if err != nil {
		t.Fatalf("create auth_db pool: %v", err)
	}
	defer authPool.Close()
	if err := authPool.Ping(ctx); err != nil {
		t.Fatalf("connect to auth_db: %v", err)
	}

	// expedientesPool (Paso 20A): limpieza directa de los expedientes
	// REALES creados para probar el ownership/área real de
	// ListDerivaciones — mismo patrón ya usado en Documentos (Paso 19E).
	expedientesPool, err := pgxpool.New(ctx, expedientesDatabaseURL())
	if err != nil {
		t.Fatalf("create expedientes_db pool: %v", err)
	}
	defer expedientesPool.Close()
	if err := expedientesPool.Ping(ctx); err != nil {
		t.Fatalf("connect to expedientes_db: %v", err)
	}

	testID := strconv.FormatInt(time.Now().UnixNano()%1e8, 10)
	usernamePrefix := "hpder" + testID + "_"
	baseNano := time.Now().UnixNano()
	var createdAuthUserIDs []string
	var createdDerivacionIDs []string
	var createdExpedienteIDs []string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, id := range createdDerivacionIDs {
			if _, err := derivacionesPool.Exec(cleanupCtx, "DELETE FROM derivaciones WHERE id = $1", id); err != nil {
				t.Logf("cleanup derivacion %s: %v", id, err)
			}
		}
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

	// expedientesRealClient (Paso 20A): cliente REAL contra Expedientes
	// Service (127.0.0.1:8082, ya corriendo) — se usa tanto para crear los
	// expedientes reales que necesitan los tests de ownership/área como
	// para el propio ListDerivaciones bajo prueba (vía
	// svcCtx.ExpedientesClient).
	expedientesRealClient := expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8082"}))

	// createRealExpediente crea un expediente REAL cuyo SolicitanteID es,
	// de verdad, solicitanteID — necesario para probar el ownership real
	// que ListDerivaciones ahora delega en Expedientes.GetExpediente.
	createRealExpediente := func(t *testing.T, solicitanteID string) string {
		t.Helper()
		resp, err := expedientesRealClient.CreateExpediente(authCtx(solicitanteID, "SOLICITANTE"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test ownership ListDerivaciones", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		createdExpedienteIDs = append(createdExpedienteIDs, resp.Expediente.Id)
		return resp.Expediente.Id
	}

	// derivarExpedienteReal mueve un expediente real de área (vía
	// Expedientes.DerivarExpediente) usando un actor con CanViewAll
	// (SECRETARIA) — mismo helper ya usado en Documentos (Paso 19E). Tipo
	// "OTRO" es genérico (estados.CanDerivar lo permite hacia cualquier
	// área válida, sin disparar ningún cambio de estado especial).
	derivarExpedienteReal := func(t *testing.T, secretariaID, expID, areaDestino string) {
		t.Helper()
		if _, err := expedientesRealClient.DerivarExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.DerivarExpedienteRequest{
			Id: expID, AreaDestino: areaDestino,
		}); err != nil {
			t.Fatalf("no se pudo derivar el expediente real a %s: %v", areaDestino, err)
		}
	}

	// crearExpedienteComoSecretaria (Paso 20B): expediente real creado por
	// SECRETARIA -- area_actual queda en SECRETARIA (default real de
	// CreateExpediente), necesario ahora que CrearDerivacion exige que
	// origen o destino coincidan con esa área real.
	crearExpedienteComoSecretaria := func(t *testing.T, secretariaID string) string {
		t.Helper()
		resp, err := expedientesRealClient.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test CrearDerivacion", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		createdExpedienteIDs = append(createdExpedienteIDs, resp.Expediente.Id)
		return resp.Expediente.Id
	}

	svcCtx := &svc.ServiceContext{
		Config:               config.Config{JWTSecret: realSharedJWTSecret, AuthRpc: zrpc.RpcClientConf{Target: "127.0.0.1:8080"}},
		DB:                   derivacionesPool,
		DerivacionRepository: repository.NewDerivacionRepository(derivacionesPool),
		JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
		AuthClient:           authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
		ExpedientesClient:    expedientesRealClient,
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(svcCtx.JWTValidator),
	))
	derivaciones.RegisterDerivacionesServer(grpcServer, server.NewDerivacionesServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC test server: %v", err)
	}
	defer conn.Close()
	client := derivaciones.NewDerivacionesClient(conn)

	trackDerivacion := func(id string) { createdDerivacionIDs = append(createdDerivacionIDs, id) }
	expedienteID := func(index int) string { return fixedUUID(baseNano, index) }

	// countDerivaciones (Paso 20B, sección 13) consulta directamente
	// derivaciones_db para confirmar, sin ambigüedad, cuántas filas
	// existen para un expediente — no basta con verificar el error: hay
	// que demostrar que ningún registro falso quedó insertado.
	countDerivaciones := func(t *testing.T, expedienteID string) int {
		t.Helper()
		var total int
		if err := derivacionesPool.QueryRow(ctx, "SELECT COUNT(*) FROM derivaciones WHERE expediente_id = $1", expedienteID).Scan(&total); err != nil {
			t.Fatalf("contar derivaciones de %s: %v", expedienteID, err)
		}
		return total
	}

	// Test — Crear: permiso concedido (SECRETARIA tiene derivaciones.create).
	t.Run("Crear_PermisoConcedido", func(t *testing.T) {
		userID := createRealUser(t, "secretaria", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, userID)
		resp, err := client.CrearDerivacion(authCtx(userID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito para SECRETARIA/derivaciones.create, got err=%v", err)
		}
		trackDerivacion(resp.Derivacion.Id)
	})

	// Test — Crear: permiso denegado (SOLICITANTE no tiene
	// derivaciones.create en el catálogo real — coincide con CanCreate,
	// que ya lo denegaba por rol).
	t.Run("Crear_PermisoDenegado_Solicitante", func(t *testing.T) {
		userID := createRealUser(t, "solicitante", "SOLICITANTE")
		_, err := client.CrearDerivacion(authCtx(userID, "SOLICITANTE"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: expedienteID(2), Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para SOLICITANTE/derivaciones.create, got %v", err)
		}
	})

	// Test — Crear: regla de negocio (validación de formato) sigue
	// aplicando incluso con el permiso concedido — un tipo inválido debe
	// seguir siendo InvalidArgument, no aceptarse solo porque el permiso
	// esté otorgado.
	t.Run("Crear_ValidacionDeNegocioConservada", func(t *testing.T) {
		userID := createRealUser(t, "secretaria2", "SECRETARIA")
		_, err := client.CrearDerivacion(authCtx(userID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: expedienteID(3), Tipo: "TIPO_INVALIDO", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument por tipo inválido (regla de negocio, no de permiso), got %v", err)
		}
	})

	// Test (Paso 20B, Caso 3) — expediente_id inexistente: NotFound real,
	// propagado desde Expedientes.GetExpediente, sin insertar nada.
	t.Run("Crear_ExpedienteInexistente_NotFound", func(t *testing.T) {
		inexistente := "00000000-0000-4000-8000-000000000000"
		antes := countDerivaciones(t, inexistente)

		secretariaID := createRealUser(t, "secretaria14", "SECRETARIA")
		_, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: inexistente, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un expediente inexistente, got %v", err)
		}
		if despues := countDerivaciones(t, inexistente); despues != antes {
			t.Fatalf("no debía insertarse ningún registro: antes=%d despues=%d", antes, despues)
		}
	})

	// Test (Paso 20B, Caso 4 / Caso 10 "RPC directo") — expediente ajeno:
	// un interno SIN acceso de área al expediente (AUXILIAR sobre un
	// expediente derivado a DIRECTOR) no puede registrar una derivación
	// para él, aunque tenga derivaciones.create. Llamada directa al RPC
	// (bufconn, sin Gateway).
	t.Run("Crear_RpcDirecto_ExpedienteAjeno_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria15", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR")
		antes := countDerivaciones(t, exp)

		auxiliarID := createRealUser(t, "auxiliar2", "AUXILIAR")
		_, err := client.CrearDerivacion(authCtx(auxiliarID, "AUXILIAR"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "DIRECTOR", Destino: "SUBDIRECTOR", Motivo: "test",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por área (expediente en DIRECTOR, actor AUXILIAR), got %v", err)
		}
		if despues := countDerivaciones(t, exp); despues != antes {
			t.Fatalf("no debía insertarse ningún registro: antes=%d despues=%d", antes, despues)
		}
	})

	// Test (Paso 20B, Caso 5 / Caso 10 "RPC directo") — origen falso: el
	// expediente real está en SECRETARIA, pero se declara origen=DIRECTOR
	// (y destino tampoco corresponde al área real) — ninguno de los dos
	// coincide con AreaActual, así que se rechaza sin insertar. Llamada
	// directa al RPC.
	t.Run("Crear_RpcDirecto_OrigenFalso_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria16", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		antes := countDerivaciones(t, exp)

		_, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "DIRECTOR", Destino: "SUBDIRECTOR", Motivo: "test",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: ni origen ni destino corresponden al área real (SECRETARIA), got %v", err)
		}
		if despues := countDerivaciones(t, exp); despues != antes {
			t.Fatalf("no debía insertarse ningún registro fabricado: antes=%d despues=%d", antes, despues)
		}
	})

	// Test (Paso 20B, Caso 6) — origen correcto explícito: además de
	// Crear_PermisoConcedido (que ya usa origen=SECRETARIA sobre un
	// expediente real en SECRETARIA), este caso prueba el patrón
	// complementario: destino coincide con el área real DESPUÉS de un
	// movimiento ya ocurrido (mismo patrón que usa el Gateway en
	// DerivarExpediente — ver crearderivacionlogic.go).
	t.Run("Crear_DestinoCoincideConAreaReal_Permitido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria17", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR") // AreaActual real ya es DIRECTOR

		// origen refleja el área ANTES del movimiento (SECRETARIA);
		// destino coincide con el área real ACTUAL (DIRECTOR) -- mismo
		// patrón que usa el Gateway tras un DerivarExpediente real.
		resp, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("se esperaba éxito: destino coincide con el área real actual, got err=%v", err)
		}
		trackDerivacion(resp.Derivacion.Id)
	})

	// Test (Paso 20B.2, sección 19) — destino inventado: origen coincide
	// con el área real (SECRETARIA), pero destino es un área que no
	// existe -- ValidateDerivacion (IsValidArea) lo rechaza, sin insertar.
	t.Run("Crear_DestinoInventado_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria23", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		antes := countDerivaciones(t, exp)

		_, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "AREA_INVENTADA", Motivo: "test",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: AREA_INVENTADA no es un área real, got %v", err)
		}
		if despues := countDerivaciones(t, exp); despues != antes {
			t.Fatalf("no debía insertarse ningún registro: antes=%d despues=%d", antes, despues)
		}
	})

	// Test (Paso 20B.2, sección 19) — transición inválida: origen coincide
	// con el área real (SECRETARIA), pero SECRETARIA->SUBDIRECTOR no
	// existe para CERTIFICADO (F2 real) -- ValidateDerivacion (CanDerivar)
	// lo rechaza, sin insertar.
	t.Run("Crear_TransicionInvalida_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria24", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		antes := countDerivaciones(t, exp)

		_, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "SUBDIRECTOR", Motivo: "test",
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: SECRETARIA->SUBDIRECTOR no existe para CERTIFICADO (F2), got %v", err)
		}
		if despues := countDerivaciones(t, exp); despues != antes {
			t.Fatalf("no debía insertarse ningún registro: antes=%d despues=%d", antes, despues)
		}
	})

	// Test (Paso 20B, Caso 9) — Expedientes no disponible: debe fallar
	// cerrado, sin insertar nada.
	t.Run("Crear_ExpedientesNoDisponible", func(t *testing.T) {
		downExpSvcCtx := &svc.ServiceContext{
			Config:               config.Config{JWTSecret: realSharedJWTSecret},
			DB:                   derivacionesPool,
			DerivacionRepository: repository.NewDerivacionRepository(derivacionesPool),
			JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
			AuthClient:           authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
			ExpedientesClient: expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19993", // puerto sin servicio, a propósito
				NonBlock: true,
				Timeout:  2000,
			})),
		}
		downExpListener := bufconn.Listen(1024 * 1024)
		downExpServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downExpSvcCtx.JWTValidator),
		))
		derivaciones.RegisterDerivacionesServer(downExpServer, server.NewDerivacionesServer(downExpSvcCtx))
		go func() { _ = downExpServer.Serve(downExpListener) }()
		defer downExpServer.Stop()

		downExpConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downExpListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Expedientes caído): %v", err)
		}
		defer downExpConn.Close()
		downExpClient := derivaciones.NewDerivacionesClient(downExpConn)

		secretariaID := createRealUser(t, "secretaria18", "SECRETARIA")
		exp := expedienteID(20)
		antes := countDerivaciones(t, exp)

		_, err = downExpClient.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Expedientes Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}
		if despues := countDerivaciones(t, exp); despues != antes {
			t.Fatalf("no debía insertarse ningún registro: antes=%d despues=%d", antes, despues)
		}
	})

	// Test — Get: permiso concedido (rol interno con derivaciones.view).
	t.Run("Get_PermisoConcedido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria3", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		// Paso 20C: GetDerivacion ahora también exige acceso real al
		// expediente (vía Expedientes.GetExpediente) — SECRETARIA
		// (CanViewAll) lo tiene trivialmente; el caso de un interno SIN
		// CanViewAll pero con área correcta/ajena se cubre en los tests
		// dedicados de este paso.
		if _, err := client.GetDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.GetDerivacionRequest{Id: created.Derivacion.Id}); err != nil {
			t.Fatalf("se esperaba éxito para SECRETARIA/derivaciones.view, got err=%v", err)
		}
	})

	// Test — Get: permiso denegado (SOLICITANTE no tiene
	// derivaciones.view en el catálogo real — cierra el hueco 🔴
	// detectado en el Paso 15, donde antes CUALQUIER autenticado podía
	// leer cualquier derivación).
	t.Run("Get_PermisoDenegado_Solicitante", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria4", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		solicitanteID := createRealUser(t, "solicitante2", "SOLICITANTE")
		resp, err := client.GetDerivacion(authCtx(solicitanteID, "SOLICITANTE"), &derivaciones.GetDerivacionRequest{Id: created.Derivacion.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para SOLICITANTE/derivaciones.view, got %v", err)
		}
		if resp != nil {
			t.Fatal("no debía filtrarse ningún dato de la derivación sin el permiso")
		}
	})

	// Test (Paso 20C, Caso 1 / Caso 10 "RPC directo") — usuario autorizado:
	// interno SIN CanViewAll (DIRECTOR), con área real correcta (expediente
	// derivado a DIRECTOR), y derivaciones.view -> PERMITIDO. Llamada
	// directa al RPC (bufconn, sin Gateway).
	t.Run("Get_RpcDirecto_AreaCorrecta_Permitido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria25", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR") // F2: SECRETARIA->DIRECTOR

		directorID := createRealUser(t, "director8", "DIRECTOR")
		created, err := client.CrearDerivacion(authCtx(directorID, "DIRECTOR"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "DIRECTOR", Destino: "SECRETARIA", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		resp, err := client.GetDerivacion(authCtx(directorID, "DIRECTOR"), &derivaciones.GetDerivacionRequest{Id: created.Derivacion.Id})
		if err != nil {
			t.Fatalf("se esperaba éxito para DIRECTOR sobre su propia área, got err=%v", err)
		}
		if resp.Derivacion.Id != created.Derivacion.Id {
			t.Fatalf("se esperaba la derivación correcta, got %+v", resp.Derivacion)
		}
	})

	// Test (Paso 20C, Caso 2 / Caso 10 "RPC directo") — expediente ajeno:
	// AUXILIAR (interno, sin CanViewAll) con derivaciones.view, pero SIN
	// acceso de área al expediente (está en DIRECTOR) -> PermissionDenied,
	// sin filtrar ningún dato.
	t.Run("Get_RpcDirecto_ExpedienteAjeno_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria26", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR")

		directorID := createRealUser(t, "director9", "DIRECTOR")
		created, err := client.CrearDerivacion(authCtx(directorID, "DIRECTOR"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "DIRECTOR", Destino: "SECRETARIA", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		auxiliarID := createRealUser(t, "auxiliar3", "AUXILIAR")
		resp, err := client.GetDerivacion(authCtx(auxiliarID, "AUXILIAR"), &derivaciones.GetDerivacionRequest{Id: created.Derivacion.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por área (expediente en DIRECTOR, actor AUXILIAR), got %v", err)
		}
		if resp != nil {
			t.Fatal("no debía filtrarse ningún dato de la derivación de un expediente ajeno")
		}
	})

	// Test (Paso 20C, Caso 3) — sin derivaciones.view: usuario sin ningún
	// rol real (JWT reclama SECRETARIA) -> PermissionDenied, sin llegar
	// siquiera a consultar Expedientes.
	t.Run("Get_SinPermisoView_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria27", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		sinRolID := createRealUser(t, "sinroles5")
		resp, err := client.GetDerivacion(authCtx(sinRolID, "SECRETARIA"), &derivaciones.GetDerivacionRequest{Id: created.Derivacion.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por falta de derivaciones.view, got %v", err)
		}
		if resp != nil {
			t.Fatal("no debía filtrarse ningún dato de la derivación sin el permiso")
		}
	})

	// Test (Paso 20C, Caso 4) — UUID de derivación inexistente: NotFound.
	t.Run("Get_DerivacionInexistente_NotFound", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria28", "SECRETARIA")
		resp, err := client.GetDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.GetDerivacionRequest{
			Id: "00000000-0000-4000-8000-000000000000",
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound, got %v", err)
		}
		if resp != nil {
			t.Fatal("no debía devolverse ninguna derivación inexistente")
		}
	})

	// Test (Paso 20C, Caso 5) — expediente asociado inexistente: la
	// derivación existe (registrada directamente en derivaciones_db, con
	// un expediente_id que nunca existió en Expedientes), pero al validar
	// acceso, Expedientes.GetExpediente devuelve NotFound -> se propaga,
	// NUNCA se devuelve la derivación.
	t.Run("Get_ExpedienteAsociadoInexistente_NotFound", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria29", "SECRETARIA")
		expedienteFantasma := "00000000-0000-4000-8000-000000000099"
		var derivacionID string
		if err := derivacionesPool.QueryRow(ctx, `
			INSERT INTO derivaciones (expediente_id, tipo, origen, destino, motivo, condicion, registrado_por)
			VALUES ($1, 'DERIVACION', 'SECRETARIA', 'DIRECTOR', 'test', '', $2)
			RETURNING id
		`, expedienteFantasma, secretariaID).Scan(&derivacionID); err != nil {
			t.Fatalf("no se pudo insertar la derivación fantasma directamente: %v", err)
		}
		trackDerivacion(derivacionID)

		resp, err := client.GetDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.GetDerivacionRequest{Id: derivacionID})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound (expediente asociado inexistente), got %v", err)
		}
		if resp != nil {
			t.Fatal("no debía devolverse la derivación de un expediente que no existe")
		}
	})

	// Test (Paso 20C, Caso 6) — JWT inválido (firma corrupta): Unauthenticated.
	t.Run("Get_JWTInvalido_Unauthenticated", func(t *testing.T) {
		badCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer token-invalido-no-firmado")
		resp, err := client.GetDerivacion(badCtx, &derivaciones.GetDerivacionRequest{
			Id: "00000000-0000-4000-8000-000000000000",
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated, got %v", err)
		}
		if resp != nil {
			t.Fatal("no debía filtrarse nada con un JWT inválido")
		}
	})

	// Test (Paso 20C, Caso 7) — sin JWT: Unauthenticated.
	t.Run("Get_SinJWT_Unauthenticated", func(t *testing.T) {
		resp, err := client.GetDerivacion(context.Background(), &derivaciones.GetDerivacionRequest{
			Id: "00000000-0000-4000-8000-000000000000",
		})
		if status.Code(err) != codes.Unauthenticated {
			t.Fatalf("se esperaba Unauthenticated sin JWT, got %v", err)
		}
		if resp != nil {
			t.Fatal("no debía filtrarse nada sin JWT")
		}
	})

	// Test (Paso 20C, Caso 9) — Expedientes no disponible: debe fallar
	// cerrado, sin devolver la derivación.
	t.Run("Get_ExpedientesNoDisponible", func(t *testing.T) {
		downExpSvcCtx := &svc.ServiceContext{
			Config:               config.Config{JWTSecret: realSharedJWTSecret},
			DB:                   derivacionesPool,
			DerivacionRepository: repository.NewDerivacionRepository(derivacionesPool),
			JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
			AuthClient:           authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
			ExpedientesClient: expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19992", // puerto sin servicio, a propósito
				NonBlock: true,
				Timeout:  2000,
			})),
		}
		downExpListener := bufconn.Listen(1024 * 1024)
		downExpServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downExpSvcCtx.JWTValidator),
		))
		derivaciones.RegisterDerivacionesServer(downExpServer, server.NewDerivacionesServer(downExpSvcCtx))
		go func() { _ = downExpServer.Serve(downExpListener) }()
		defer downExpServer.Stop()

		downExpConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downExpListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Expedientes caído): %v", err)
		}
		defer downExpConn.Close()
		downExpClient := derivaciones.NewDerivacionesClient(downExpConn)

		secretariaID := createRealUser(t, "secretaria30", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		resp, err := downExpClient.GetDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.GetDerivacionRequest{Id: created.Derivacion.Id})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Expedientes Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}
		if resp != nil {
			t.Fatal("no debía filtrarse ningún dato con Expedientes caído")
		}
	})

	// Test — Listar: view global (rol interno) sin expediente_id sigue
	// funcionando (antes decidía CanViewAll(role), ahora decide el
	// permiso real, mismo resultado).
	t.Run("Listar_ViewGlobal", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria5", "SECRETARIA")
		exp := crearExpedienteComoSecretaria(t, secretariaID)
		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		if _, err := client.ListDerivaciones(authCtx(secretariaID, "SECRETARIA"), &derivaciones.ListDerivacionesRequest{}); err != nil {
			t.Fatalf("se esperaba éxito para SECRETARIA listando sin expediente_id, got err=%v", err)
		}
	})

	// Test — Listar: SOLICITANTE sin expediente_id sigue denegado
	// (InvalidArgument, mismo comportamiento de siempre — no se convirtió
	// en PermissionDenied ni se abrió el listado global).
	t.Run("Listar_SinExpedienteId_Solicitante", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitante3", "SOLICITANTE")
		_, err := client.ListDerivaciones(authCtx(solicitanteID, "SOLICITANTE"), &derivaciones.ListDerivacionesRequest{})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("se esperaba InvalidArgument (mismo comportamiento previo), got %v", err)
		}
	})

	// Test (Paso 20A, Caso 2) — SOLICITANTE CON expediente_id PROPIO sigue
	// funcionando — el uso legítimo se conserva, pero ahora requiere un
	// expediente REAL cuyo SolicitanteID sea, de verdad, el que consulta
	// (antes de este paso, cualquier UUID bastaba — ver vulnerabilidad
	// crítica del Paso 20). El permiso "derivaciones.view" sigue sin
	// exigírsele al SOLICITANTE (no existe "view_own" en el catálogo);
	// el ownership real es lo que ahora decide.
	t.Run("Listar_ConExpedienteId_SolicitantePropio_Permitido", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitante4", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)

		secretariaID := createRealUser(t, "secretaria6", "SECRETARIA")
		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		resp, err := client.ListDerivaciones(authCtx(solicitanteID, "SOLICITANTE"), &derivaciones.ListDerivacionesRequest{ExpedienteId: exp})
		if err != nil {
			t.Fatalf("se esperaba éxito: el expediente es realmente del solicitante, got err=%v", err)
		}
		if resp.Total != 1 {
			t.Fatalf("se esperaba ver la derivación de ese expediente, total=%d", resp.Total)
		}
	})

	// Test (Paso 20A, Caso 3 / Caso 9 "RPC directo") — SOLICITANTE A con
	// el expediente_id de SOLICITANTE B: el UUID conocido ya NO es
	// suficiente. Llamada directa al RPC (bufconn, sin Gateway) — cierra
	// la vulnerabilidad crítica del Paso 20 dentro del propio
	// microservicio.
	t.Run("Listar_RpcDirecto_SolicitanteAjeno_Denegado", func(t *testing.T) {
		solicitanteB := createRealUser(t, "solicitante5", "SOLICITANTE")
		expB := createRealExpediente(t, solicitanteB)

		secretariaID := createRealUser(t, "secretaria9", "SECRETARIA")
		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: expB, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		solicitanteA := createRealUser(t, "solicitante6", "SOLICITANTE")
		_, err = client.ListDerivaciones(authCtx(solicitanteA, "SOLICITANTE"), &derivaciones.ListDerivacionesRequest{ExpedienteId: expB})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: el expediente es de otro solicitante, got %v", err)
		}
	})

	// Test (Paso 20A, Caso 1) — usuario SIN NINGÚN ROL asignado, con
	// expediente_id ajeno: el UUID no basta, y este caso ni siquiera tiene
	// un permiso local qué perder (JWT reclama SOLICITANTE, pero el
	// usuario no tiene ese rol —ni ningún otro— realmente asignado).
	t.Run("Listar_UsuarioSinRol_ExpedienteAjeno_Denegado", func(t *testing.T) {
		solicitanteB := createRealUser(t, "solicitante7", "SOLICITANTE")
		expB := createRealExpediente(t, solicitanteB)

		sinRolID := createRealUser(t, "sinroles1")
		_, err := client.ListDerivaciones(authCtx(sinRolID, "SOLICITANTE"), &derivaciones.ListDerivacionesRequest{ExpedienteId: expB})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para un usuario sin roles, got %v", err)
		}
	})

	// Test (Paso 20A, Caso 4) — interno SIN CanViewAll (DIRECTOR) sobre un
	// expediente derivado realmente a su área: PERMITIDO.
	t.Run("Listar_Interno_AreaCorrecta_Permitido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria10", "SECRETARIA")
		// Creado como SECRETARIA (interno), tipo genérico "OTRO" -- a
		// diferencia de createRealExpediente (pensado para el caso
		// SOLICITANTE), acá el creador y dueño real (SolicitanteID) es
		// irrelevante: lo que importa es AreaActual tras derivar.
		expResp, err := expedientesRealClient.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "OTRO", Asunto: "Test area correcta", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		exp := expResp.Expediente.Id
		createdExpedienteIDs = append(createdExpedienteIDs, exp)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR")

		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		directorID := createRealUser(t, "director1", "DIRECTOR")
		resp, err := client.ListDerivaciones(authCtx(directorID, "DIRECTOR"), &derivaciones.ListDerivacionesRequest{ExpedienteId: exp})
		if err != nil {
			t.Fatalf("se esperaba éxito para DIRECTOR sobre su propia área, got err=%v", err)
		}
		if resp.Total != 1 {
			t.Fatalf("se esperaba ver la derivación, total=%d", resp.Total)
		}
	})

	// Test (Paso 20A, Caso 5) — interno SIN CanViewAll (AUXILIAR) sobre un
	// expediente de OTRA área (DIRECTOR): PermissionDenied.
	t.Run("Listar_Interno_AreaAjena_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria11", "SECRETARIA")
		expResp, err := expedientesRealClient.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "OTRO", Asunto: "Test area ajena", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		exp := expResp.Expediente.Id
		createdExpedienteIDs = append(createdExpedienteIDs, exp)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR")

		created, err := client.CrearDerivacion(authCtx(secretariaID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: exp, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err != nil {
			t.Fatalf("no se pudo crear la derivación base: %v", err)
		}
		trackDerivacion(created.Derivacion.Id)

		auxiliarID := createRealUser(t, "auxiliar1", "AUXILIAR")
		_, err = client.ListDerivaciones(authCtx(auxiliarID, "AUXILIAR"), &derivaciones.ListDerivacionesRequest{ExpedienteId: exp})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por área (expediente en DIRECTOR, actor AUXILIAR), got %v", err)
		}
	})

	// Test (Paso 20A, Caso 6) — interno SIN "derivaciones.view" (usuario
	// sin ningún rol real, JWT reclama SECRETARIA) con expediente_id: debe
	// negarse por el permiso, SIN llegar siquiera a consultar Expedientes
	// (a diferencia de Caso 1, aquí el rol reclamado SÍ es interno).
	t.Run("Listar_Interno_SinPermisoView_Denegado", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitante8", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)

		sinRolID := createRealUser(t, "sinroles2")
		_, err := client.ListDerivaciones(authCtx(sinRolID, "SECRETARIA"), &derivaciones.ListDerivacionesRequest{ExpedienteId: exp})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por falta de derivaciones.view, got %v", err)
		}
	})

	// Test (Paso 20A, Caso 8) — expediente inexistente: se propaga el
	// NotFound real de Expedientes, nunca una lista vacía silenciosa.
	t.Run("Listar_ExpedienteInexistente_NotFound", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria12", "SECRETARIA")
		_, err := client.ListDerivaciones(authCtx(secretariaID, "SECRETARIA"), &derivaciones.ListDerivacionesRequest{
			ExpedienteId: "00000000-0000-4000-8000-000000000000",
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un expediente inexistente, got %v", err)
		}
	})

	// Test — Auth no disponible: un AuthClient apuntando a un puerto sin
	// nada escuchando debe producir un ERROR (fail-closed), nunca
	// permitir la operación por fallback al rol local.
	t.Run("AuthNoDisponible", func(t *testing.T) {
		downSvcCtx := &svc.ServiceContext{
			Config:               config.Config{JWTSecret: realSharedJWTSecret},
			DB:                   derivacionesPool,
			DerivacionRepository: repository.NewDerivacionRepository(derivacionesPool),
			JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
			AuthClient: authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19999", // puerto sin servicio, a propósito
				NonBlock: true,
				Timeout:  2000,
			})),
		}
		downListener := bufconn.Listen(1024 * 1024)
		downServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downSvcCtx.JWTValidator),
		))
		derivaciones.RegisterDerivacionesServer(downServer, server.NewDerivacionesServer(downSvcCtx))
		go func() { _ = downServer.Serve(downListener) }()
		defer downServer.Stop()

		downConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Auth caído): %v", err)
		}
		defer downConn.Close()
		downClient := derivaciones.NewDerivacionesClient(downConn)

		userID := createRealUser(t, "authcaido", "SECRETARIA")
		crearExpediente8 := expedienteID(8)
		antesCrear := countDerivaciones(t, crearExpediente8)
		_, err = downClient.CrearDerivacion(authCtx(userID, "SECRETARIA"), &derivaciones.CrearDerivacionRequest{
			ExpedienteId: crearExpediente8, Tipo: "DERIVACION", Origen: "SECRETARIA", Destino: "DIRECTOR", Motivo: "test",
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Auth Service no está disponible, la operación se permitió")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}
		// Caso 8 (Paso 20B, sección 13) — confirmar que, además del
		// error, NO se insertó ningún registro.
		if despues := countDerivaciones(t, crearExpediente8); despues != antesCrear {
			t.Fatalf("no debía insertarse ningún registro con Auth caído: antes=%d despues=%d", antesCrear, despues)
		}

		// Caso 7 (Paso 20A) — ListDerivaciones también debe fallar cerrado
		// con Auth caído: la consulta a Auth.HasPermission
		// ("derivaciones.view") ya falla, sin llegar siquiera a consultar
		// Expedientes.
		_, err = downClient.ListDerivaciones(authCtx(userID, "SECRETARIA"), &derivaciones.ListDerivacionesRequest{
			ExpedienteId: expedienteID(9),
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para ListDerivaciones cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para ListDerivaciones, got OK")
		}

		// Caso 8 (Paso 20C) — GetDerivacion también debe fallar cerrado con
		// Auth caído: la consulta a Auth.HasPermission ("derivaciones.view")
		// ya falla, sin llegar siquiera a consultar la derivación ni a
		// Expedientes. No hace falta una derivación real: el fallo ocurre
		// antes de leer nada.
		getResp, err := downClient.GetDerivacion(authCtx(userID, "SECRETARIA"), &derivaciones.GetDerivacionRequest{
			Id: "00000000-0000-4000-8000-000000000000",
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para GetDerivacion cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para GetDerivacion, got OK")
		}
		if getResp != nil {
			t.Fatal("no debía filtrarse ningún dato con Auth caído")
		}
	})

	// Test (Paso 20A, sección 11) — Expedientes no disponible: debe
	// fallar cerrado, nunca listar (ni siquiera con derivaciones.view
	// concedido y expediente_id presente).
	t.Run("Listar_ExpedientesNoDisponible", func(t *testing.T) {
		downExpSvcCtx := &svc.ServiceContext{
			Config:               config.Config{JWTSecret: realSharedJWTSecret},
			DB:                   derivacionesPool,
			DerivacionRepository: repository.NewDerivacionRepository(derivacionesPool),
			JWTValidator:         security.NewJWTValidator(realSharedJWTSecret),
			AuthClient:           authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
			ExpedientesClient: expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19994", // puerto sin servicio, a propósito
				NonBlock: true,
				Timeout:  2000,
			})),
		}
		downExpListener := bufconn.Listen(1024 * 1024)
		downExpServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downExpSvcCtx.JWTValidator),
		))
		derivaciones.RegisterDerivacionesServer(downExpServer, server.NewDerivacionesServer(downExpSvcCtx))
		go func() { _ = downExpServer.Serve(downExpListener) }()
		defer downExpServer.Stop()

		downExpConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downExpListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Expedientes caído): %v", err)
		}
		defer downExpConn.Close()
		downExpClient := derivaciones.NewDerivacionesClient(downExpConn)

		secretariaID := createRealUser(t, "secretaria13", "SECRETARIA")
		_, err = downExpClient.ListDerivaciones(authCtx(secretariaID, "SECRETARIA"), &derivaciones.ListDerivacionesRequest{
			ExpedienteId: expedienteID(10),
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Expedientes Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}
	})
}

func integrationDatabaseURL() string {
	if value := os.Getenv("DERIVACIONES_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://derivaciones_user:derivaciones_password@127.0.0.1:5433/derivaciones_db?sslmode=disable"
}

func authDatabaseURL() string {
	if value := os.Getenv("AUTH_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://auth_user:auth_password@127.0.0.1:5433/auth_db?sslmode=disable"
}

func expedientesDatabaseURL() string {
	if value := os.Getenv("EXPEDIENTES_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://expedientes_user:expedientes_password@127.0.0.1:5433/expedientes_db?sslmode=disable"
}
