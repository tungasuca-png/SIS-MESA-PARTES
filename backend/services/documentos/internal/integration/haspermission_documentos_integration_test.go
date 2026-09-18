package integration

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"auth/authclient"
	"documentos/documentos"
	"documentos/internal/config"
	"documentos/internal/interceptor"
	"documentos/internal/repository"
	"documentos/internal/security"
	"documentos/internal/server"
	"documentos/internal/svc"
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
// Documentos -> Auth.HasPermission -> PostgreSQL, no un doble de prueba.
const realSharedJWTSecret = "41807ca75b3f05df6db6a17da41503e4b4fcb2521f6edf5e7651984e63db6d6c"

// TestHasPermissionIntegration prueba la integración REAL (Paso 14B):
// Documentos -> Auth.HasPermission (RPC real, contra un Auth Service real
// corriendo en 127.0.0.1:8080) -> AuthorizationService -> PostgreSQL real
// (auth_db), combinada con las reglas de negocio existentes de Documentos
// (CanUpload/CanDelete/CanViewAll y el filtro SubidoPor), que NO cambian.
func TestHasPermissionIntegration(t *testing.T) {
	if strings.ToLower(os.Getenv("INTEGRATION_TEST")) != "true" {
		t.Skip("set INTEGRATION_TEST=true to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	documentosPool, err := pgxpool.New(ctx, integrationDatabaseURL())
	if err != nil {
		t.Fatalf("create documentos_db pool: %v", err)
	}
	defer documentosPool.Close()
	if err := documentosPool.Ping(ctx); err != nil {
		t.Fatalf("connect to documentos_db: %v", err)
	}

	authPool, err := pgxpool.New(ctx, authDatabaseURL())
	if err != nil {
		t.Fatalf("create auth_db pool: %v", err)
	}
	defer authPool.Close()
	if err := authPool.Ping(ctx); err != nil {
		t.Fatalf("connect to auth_db: %v", err)
	}

	// expedientesPool (Paso 19B): limpieza directa de los expedientes
	// REALES creados para probar el ownership de "documentos.view_own" —
	// mismo patrón de cleanup ya usado por los propios tests de Expedientes
	// (DELETE FROM directo, aunque en producción la baja sea lógica).
	expedientesPool, err := pgxpool.New(ctx, expedientesDatabaseURL())
	if err != nil {
		t.Fatalf("create expedientes_db pool: %v", err)
	}
	defer expedientesPool.Close()
	if err := expedientesPool.Ping(ctx); err != nil {
		t.Fatalf("connect to expedientes_db: %v", err)
	}

	testID := strconv.FormatInt(time.Now().UnixNano()%1e8, 10)
	usernamePrefix := "hpdoc" + testID + "_"
	var createdAuthUserIDs []string
	var createdDocumentIDs []string
	var createdExpedienteIDs []string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, id := range createdDocumentIDs {
			if _, err := documentosPool.Exec(cleanupCtx, "DELETE FROM documentos WHERE id = $1", id); err != nil {
				t.Logf("cleanup documento %s: %v", id, err)
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

	// createRealUser inserta un usuario REAL en auth_db y le asigna, en
	// usuario_roles, el/los rol(es) indicados (ninguno si no se pasa
	// ningún roleName, para el caso "usuario sin ningún permiso").
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

	// expedientesRealClient (Paso 19B): cliente REAL contra Expedientes
	// Service (127.0.0.1:8082, ya corriendo) — se usa tanto para crear los
	// expedientes reales que necesitan los tests de ownership como para el
	// propio GetDocumento bajo prueba (vía svcCtx.ExpedientesClient).
	expedientesRealClient := expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8082"}))

	// createRealExpediente crea un expediente REAL (Expedientes Service +
	// expedientes_db reales) cuyo SolicitanteID es, de verdad,
	// solicitanteID — necesario para probar el ownership real que
	// GetDocumento ahora delega en Expedientes.GetExpediente (Paso 19B).
	createRealExpediente := func(t *testing.T, solicitanteID string) string {
		t.Helper()
		resp, err := expedientesRealClient.CreateExpediente(authCtx(solicitanteID, "SOLICITANTE"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test ownership GetDocumento", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		createdExpedienteIDs = append(createdExpedienteIDs, resp.Expediente.Id)
		return resp.Expediente.Id
	}

	svcCtx := &svc.ServiceContext{
		Config:              config.Config{JWTSecret: realSharedJWTSecret, AuthRpc: zrpc.RpcClientConf{Target: "127.0.0.1:8080"}},
		DB:                  documentosPool,
		DocumentoRepository: repository.NewDocumentoRepository(documentosPool),
		JWTValidator:        security.NewJWTValidator(realSharedJWTSecret),
		AuthClient:          authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
		ExpedientesClient:   expedientesRealClient,
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(svcCtx.JWTValidator),
	))
	documentos.RegisterDocumentosServer(grpcServer, server.NewDocumentosServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC test server: %v", err)
	}
	defer conn.Close()
	client := documentos.NewDocumentosClient(conn)

	trackDoc := func(id string) { createdDocumentIDs = append(createdDocumentIDs, id) }
	baseNano := time.Now().UnixNano()
	expedienteID := func(index int) string {
		return fixedUUID(baseNano, index)
	}

	// Test — Upload: permiso concedido (SECRETARIA tiene documentos.create).
	t.Run("Upload_PermisoConcedido", func(t *testing.T) {
		userID := createRealUser(t, "secretaria", "SECRETARIA")
		resp, err := client.UploadDocumento(authCtx(userID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(1), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("se esperaba éxito para SECRETARIA/documentos.create, got err=%v", err)
		}
		trackDoc(resp.Documento.Id)
	})

	// Test — Upload: permiso denegado. Los 7 roles reales tienen
	// documentos.create en el catálogo actual (Paso 9) — la única forma
	// real de probar la denegación por PERMISO (no por CanUpload) es un
	// usuario sin ningún rol asignado.
	t.Run("Upload_PermisoDenegado_SinRoles", func(t *testing.T) {
		userID := createRealUser(t, "sinroles")
		_, err := client.UploadDocumento(authCtx(userID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(2), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para un usuario sin roles, got %v", err)
		}
	})

	// Test — Upload: CanUpload (regla de negocio existente) sigue
	// aplicando incluso con el permiso concedido: SOLICITANTE tiene
	// documentos.create, pero no puede subir tipo PROVEIDO.
	t.Run("Upload_CanUploadConservado", func(t *testing.T) {
		userID := createRealUser(t, "solicitante", "SOLICITANTE")
		_, err := client.UploadDocumento(authCtx(userID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(3), Nombre: "proveido.pdf", TipoDocumento: "PROVEIDO", Extension: "pdf", Contenido: []byte("x"),
		})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por CanUpload (SOLICITANTE + PROVEIDO), got %v", err)
		}
	})

	// Test — Get: permiso concedido (documentos.view, rol interno).
	t.Run("Get_PermisoConcedido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria2", "SECRETARIA")
		uploaded, err := client.UploadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(4), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		docenteID := createRealUser(t, "docente", "DOCENTE")
		if _, err := client.GetDocumento(authCtx(docenteID, "DOCENTE"), &documentos.GetDocumentoRequest{Id: uploaded.Documento.Id}); err != nil {
			t.Fatalf("se esperaba éxito para DOCENTE/documentos.view, got err=%v", err)
		}
	})

	// Test — Get: permiso denegado (usuario sin ningún rol).
	t.Run("Get_PermisoDenegado_SinRoles", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria3", "SECRETARIA")
		uploaded, err := client.UploadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(5), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		sinRolID := createRealUser(t, "sinroles2")
		_, err = client.GetDocumento(authCtx(sinRolID, "SOLICITANTE"), &documentos.GetDocumentoRequest{Id: uploaded.Documento.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para un usuario sin roles, got %v", err)
		}
	})

	// Test — Get: documentos.view_own alcanza (SOLICITANTE, sin
	// documentos.view, sí tiene documentos.view_own) SOBRE SU PROPIO
	// expediente REAL (Paso 19B: ownership real, ya no basta el permiso).
	t.Run("Get_ViewOwnAlcanza", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitante2", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)
		uploaded, err := client.UploadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		if _, err := client.GetDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.GetDocumentoRequest{Id: uploaded.Documento.Id}); err != nil {
			t.Fatalf("se esperaba éxito para SOLICITANTE/documentos.view_own sobre su propio expediente, got err=%v", err)
		}
	})

	// Test (Paso 19B, Caso 3) — documentos.view_own NO alcanza para un
	// documento cuyo expediente pertenece a OTRO solicitante: la única
	// forma de acceder era antes tener view_own (sin importar de quién era
	// el expediente); ahora Expedientes.GetExpediente deniega por
	// ownership real (SolicitanteID != el que consulta).
	t.Run("Get_ViewOwn_ExpedienteAjeno_Denegado", func(t *testing.T) {
		solicitanteA := createRealUser(t, "solicitantec", "SOLICITANTE")
		expA := createRealExpediente(t, solicitanteA)
		uploaded, err := client.UploadDocumento(authCtx(solicitanteA, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expA, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		solicitanteB := createRealUser(t, "solicitanted", "SOLICITANTE")
		_, err = client.GetDocumento(authCtx(solicitanteB, "SOLICITANTE"), &documentos.GetDocumentoRequest{Id: uploaded.Documento.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: el expediente es de otro solicitante, got %v", err)
		}
	})

	// Test (Paso 19B, Caso 4 — problema institucional del Paso 19) — un
	// PROVEIDO subido por DIRECCIÓN (subido_por = DIRECCIÓN) en el
	// expediente REAL del solicitante debe ser accesible para ese
	// solicitante vía GetDocumento, aunque SubidoPor != solicitanteID —
	// porque el ownership ya NO se decide por SubidoPor, sino por
	// Expediente.SolicitanteID (delegado a Expedientes.GetExpediente).
	t.Run("Get_ViewOwn_ProveidoDeDireccion_Permitido", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitantee", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)

		direccionID := createRealUser(t, "direccion1", "DIRECTOR")
		uploaded, err := client.UploadDocumento(authCtx(direccionID, "DIRECTOR"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "proveido.pdf", TipoDocumento: "PROVEIDO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el PROVEIDO base (subido por DIRECTOR): %v", err)
		}
		trackDoc(uploaded.Documento.Id)
		if uploaded.Documento.SubidoPor == solicitanteID {
			t.Fatalf("precondición inválida: el PROVEIDO debe estar subido por DIRECCIÓN, no por el solicitante")
		}

		if _, err := client.GetDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.GetDocumentoRequest{Id: uploaded.Documento.Id}); err != nil {
			t.Fatalf("se esperaba éxito: el PROVEIDO pertenece al expediente del solicitante aunque SubidoPor sea DIRECCIÓN, got err=%v", err)
		}
	})

	// Test (Paso 19B, Caso 5) — expediente_id inexistente: Documentos no
	// valida expediente_id contra Expedientes al subir (Paso 19, sección
	// 9), así que puede existir un documento cuyo expediente_id no
	// corresponde a ningún expediente real. Para "view_own", eso debe
	// fallar como NotFound (mismo error que ya usa Expedientes.GetExpediente
	// para un id inexistente), nunca como acceso permitido.
	t.Run("Get_ViewOwn_ExpedienteInexistente_NotFound", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitantef", "SOLICITANTE")
		uploaded, err := client.UploadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: "00000000-0000-4000-8000-000000000000", Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base (expediente_id inexistente, permitido por diseño de Documentos): %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		_, err = client.GetDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.GetDocumentoRequest{Id: uploaded.Documento.Id})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound (expediente inexistente), got %v", err)
		}
	})

	// Test (Paso 19B, Caso 7) — Expedientes no disponible: para un usuario
	// que depende de documentos.view_own, si Expedientes.GetExpediente no
	// se puede consultar, debe fallar cerrado (nunca permitir el acceso).
	t.Run("Get_ViewOwn_ExpedientesNoDisponible", func(t *testing.T) {
		downExpSvcCtx := &svc.ServiceContext{
			Config:              config.Config{JWTSecret: realSharedJWTSecret},
			DB:                  documentosPool,
			DocumentoRepository: repository.NewDocumentoRepository(documentosPool),
			JWTValidator:        security.NewJWTValidator(realSharedJWTSecret),
			AuthClient:          authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
			ExpedientesClient: expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19998", // puerto sin servicio, a propósito
				NonBlock: true,
				Timeout:  2000,
			})),
		}
		downExpListener := bufconn.Listen(1024 * 1024)
		downExpServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downExpSvcCtx.JWTValidator),
		))
		documentos.RegisterDocumentosServer(downExpServer, server.NewDocumentosServer(downExpSvcCtx))
		go func() { _ = downExpServer.Serve(downExpListener) }()
		defer downExpServer.Stop()

		downExpConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downExpListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Expedientes caído): %v", err)
		}
		defer downExpConn.Close()
		downExpClient := documentos.NewDocumentosClient(downExpConn)

		solicitanteID := createRealUser(t, "solicitanteg", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)
		uploaded, err := client.UploadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		_, err = downExpClient.GetDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.GetDocumentoRequest{Id: uploaded.Documento.Id})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Expedientes Service no está disponible para view_own")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}
	})

	// Test — Download: permiso concedido / denegado, mismo criterio que Get.
	t.Run("Download_PermisoConcedido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria4", "SECRETARIA")
		uploaded, err := client.UploadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(7), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("contenido-real"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		resp, err := client.DownloadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.DownloadDocumentoRequest{Id: uploaded.Documento.Id})
		if err != nil {
			t.Fatalf("se esperaba éxito para SECRETARIA/documentos.view, got err=%v", err)
		}
		if string(resp.Contenido) != "contenido-real" {
			t.Fatalf("contenido inesperado: %q", resp.Contenido)
		}
	})

	t.Run("Download_PermisoDenegado_SinRoles", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria5", "SECRETARIA")
		uploaded, err := client.UploadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(8), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		sinRolID := createRealUser(t, "sinroles3")
		_, err = client.DownloadDocumento(authCtx(sinRolID, "SOLICITANTE"), &documentos.DownloadDocumentoRequest{Id: uploaded.Documento.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para un usuario sin roles, got %v", err)
		}
	})

	// Test (Paso 19D, caso 2) — documentos.view_own sobre expediente
	// PROPIO real: ownership confirmado vía Expedientes.GetExpediente.
	t.Run("Download_ViewOwn_ExpedientePropio_Permitido", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitantem", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)
		uploaded, err := client.UploadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("contenido-propio"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		resp, err := client.DownloadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.DownloadDocumentoRequest{Id: uploaded.Documento.Id})
		if err != nil {
			t.Fatalf("se esperaba éxito para SOLICITANTE/documentos.view_own sobre su propio expediente, got err=%v", err)
		}
		if string(resp.Contenido) != "contenido-propio" {
			t.Fatalf("contenido inesperado: %q", resp.Contenido)
		}
	})

	// Test (Paso 19D, casos 3 y 8) — documentos.view_own sobre expediente
	// AJENO: PermissionDenied, delegado a Expedientes.GetExpediente. Este
	// es también el caso de "acceso directo al RPC" (sin pasar por el
	// Gateway: aquí se llama a client.DownloadDocumento directamente, vía
	// bufconn) que demuestra que la vulnerabilidad detectada en la
	// auditoría (Paso 19) ya no depende exclusivamente del Gateway.
	t.Run("Download_ViewOwn_ExpedienteAjeno_Denegado_AccesoDirecto", func(t *testing.T) {
		solicitanteA := createRealUser(t, "solicitanten", "SOLICITANTE")
		expA := createRealExpediente(t, solicitanteA)
		uploadedA, err := client.UploadDocumento(authCtx(solicitanteA, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expA, Nombre: "a.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("contenido-de-a"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento de A: %v", err)
		}
		trackDoc(uploadedA.Documento.Id)

		solicitanteB := createRealUser(t, "solicitanteo", "SOLICITANTE")
		_, err = client.DownloadDocumento(authCtx(solicitanteB, "SOLICITANTE"), &documentos.DownloadDocumentoRequest{Id: uploadedA.Documento.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: el expediente es de otro solicitante (llamada directa al RPC, sin Gateway), got %v", err)
		}
	})

	// Test (Paso 19D, caso 4 — problema institucional del Paso 19) — el
	// PROVEIDO subido por DIRECCIÓN (SubidoPor != solicitanteID) debe
	// poder descargarse por el solicitante dueño real del expediente.
	t.Run("Download_ViewOwn_ProveidoDeDireccion_Permitido", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitantep", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)

		direccionID := createRealUser(t, "direccion4", "DIRECTOR")
		proveido, err := client.UploadDocumento(authCtx(direccionID, "DIRECTOR"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "proveido.pdf", TipoDocumento: "PROVEIDO", Extension: "pdf", Contenido: []byte("contenido-proveido"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el PROVEIDO (subido por DIRECTOR): %v", err)
		}
		trackDoc(proveido.Documento.Id)

		resp, err := client.DownloadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.DownloadDocumentoRequest{Id: proveido.Documento.Id})
		if err != nil {
			t.Fatalf("se esperaba éxito: el PROVEIDO pertenece al expediente del solicitante aunque SubidoPor sea DIRECCIÓN, got err=%v", err)
		}
		if string(resp.Contenido) != "contenido-proveido" {
			t.Fatalf("contenido inesperado: %q", resp.Contenido)
		}
	})

	// Test (Paso 19D, caso 5) — documento inexistente: se conserva el
	// comportamiento previo (NotFound), incluso tras reordenar el flujo
	// para leer metadata antes que el contenido.
	t.Run("Download_Inexistente_NotFound", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria9b", "SECRETARIA")
		_, err := client.DownloadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.DownloadDocumentoRequest{
			Id: "00000000-0000-4000-8000-000000000000",
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("se esperaba NotFound para un documento inexistente, got %v", err)
		}
	})

	// Test (Paso 19D, caso 7) — Expedientes no disponible, para
	// documentos.view_own: debe fallar cerrado, sin llegar a leer el
	// contenido.
	t.Run("Download_ViewOwn_ExpedientesNoDisponible", func(t *testing.T) {
		downExpSvcCtx := &svc.ServiceContext{
			Config:              config.Config{JWTSecret: realSharedJWTSecret},
			DB:                  documentosPool,
			DocumentoRepository: repository.NewDocumentoRepository(documentosPool),
			JWTValidator:        security.NewJWTValidator(realSharedJWTSecret),
			AuthClient:          authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
			ExpedientesClient: expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19996", // puerto sin servicio, a propósito
				NonBlock: true,
				Timeout:  2000,
			})),
		}
		downExpListener := bufconn.Listen(1024 * 1024)
		downExpServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downExpSvcCtx.JWTValidator),
		))
		documentos.RegisterDocumentosServer(downExpServer, server.NewDocumentosServer(downExpSvcCtx))
		go func() { _ = downExpServer.Serve(downExpListener) }()
		defer downExpServer.Stop()

		downExpConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downExpListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Expedientes caído): %v", err)
		}
		defer downExpConn.Close()
		downExpClient := documentos.NewDocumentosClient(downExpConn)

		solicitanteID := createRealUser(t, "solicitanteq", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)
		uploaded, err := client.UploadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		_, err = downExpClient.DownloadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.DownloadDocumentoRequest{Id: uploaded.Documento.Id})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Expedientes Service no está disponible para view_own")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}
	})

	// Test — List: view global (DIRECTOR) ve documentos de un expediente
	// aunque no los haya subido él.
	t.Run("List_ViewGlobal", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria6", "SECRETARIA")
		exp := expedienteID(9)
		uploaded, err := client.UploadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		directorID := createRealUser(t, "director", "DIRECTOR")
		listResp, err := client.ListDocumentos(authCtx(directorID, "DIRECTOR"), &documentos.ListDocumentosRequest{ExpedienteId: exp})
		if err != nil {
			t.Fatalf("se esperaba éxito para DIRECTOR/documentos.view, got err=%v", err)
		}
		if listResp.Total != 1 {
			t.Fatalf("se esperaba ver el documento ajeno (view global), total=%d", listResp.Total)
		}
	})

	// Test (Paso 19C) — con expediente_id: ownership real, NO SubidoPor. Un
	// SOLICITANTE (solo documentos.view_own) que pide el expediente_id de
	// OTRO solicitante debe ser denegado por Expedientes.GetExpediente
	// (SolicitanteID != el que consulta), no simplemente filtrado a 0
	// resultados como antes del Paso 19C.
	t.Run("List_ConExpedienteId_ViewOwn_ExpedienteAjeno_Denegado", func(t *testing.T) {
		solicitanteA := createRealUser(t, "solicitantea", "SOLICITANTE")
		expA := createRealExpediente(t, solicitanteA)
		uploadedA, err := client.UploadDocumento(authCtx(solicitanteA, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expA, Nombre: "a.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento de A: %v", err)
		}
		trackDoc(uploadedA.Documento.Id)

		solicitanteB := createRealUser(t, "solicitanteb", "SOLICITANTE")
		_, err = client.ListDocumentos(authCtx(solicitanteB, "SOLICITANTE"), &documentos.ListDocumentosRequest{ExpedienteId: expA})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: el expediente es de otro solicitante, got %v", err)
		}
	})

	// Test (Paso 19C, casos 2/4/5 obligatorios) — con expediente_id propio:
	// el SOLICITANTE ve TODOS los documentos de su expediente, incluyendo
	// el PROVEIDO subido por DIRECCIÓN (SubidoPor != solicitanteID) — este
	// es el caso institucional del Paso 19, ahora resuelto también sin
	// expediente_id puntual (siguiente test).
	t.Run("List_ConExpedienteId_ViewOwn_IncluyeProveidoDeDireccion", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitanteh", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)

		propio, err := client.UploadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "adjunto.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento propio: %v", err)
		}
		trackDoc(propio.Documento.Id)

		direccionID := createRealUser(t, "direccion2", "DIRECTOR")
		proveido, err := client.UploadDocumento(authCtx(direccionID, "DIRECTOR"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "proveido.pdf", TipoDocumento: "PROVEIDO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el PROVEIDO (subido por DIRECTOR): %v", err)
		}
		trackDoc(proveido.Documento.Id)

		listResp, err := client.ListDocumentos(authCtx(solicitanteID, "SOLICITANTE"), &documentos.ListDocumentosRequest{ExpedienteId: exp})
		if err != nil {
			t.Fatalf("se esperaba éxito para el dueño real del expediente, got err=%v", err)
		}
		if listResp.Total != 2 {
			t.Fatalf("se esperaban 2 documentos (propio + PROVEIDO de Dirección), total=%d", listResp.Total)
		}
	})

	// Test (Paso 19C, caso obligatorio 4, SIN expediente_id) — mismo caso
	// institucional que arriba, pero listando SIN expediente_id: el
	// PROVEIDO de Dirección debe aparecer igual, resuelto vía
	// Expedientes.ListExpedientes (expedientes propios del solicitante),
	// nunca vía SubidoPor.
	t.Run("List_SinExpedienteId_ViewOwn_IncluyeProveidoDeDireccion", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitantei", "SOLICITANTE")
		exp := createRealExpediente(t, solicitanteID)

		propio, err := client.UploadDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "adjunto.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento propio: %v", err)
		}
		trackDoc(propio.Documento.Id)

		direccionID := createRealUser(t, "direccion3", "DIRECTOR")
		proveido, err := client.UploadDocumento(authCtx(direccionID, "DIRECTOR"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "proveido.pdf", TipoDocumento: "PROVEIDO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el PROVEIDO (subido por DIRECTOR): %v", err)
		}
		trackDoc(proveido.Documento.Id)

		// Otro solicitante, sin relación con "exp", no debe aparecer
		// mezclado en el resultado de A — prueba de aislamiento además del
		// caso positivo.
		solicitanteOtro := createRealUser(t, "solicitantej", "SOLICITANTE")
		expOtro := createRealExpediente(t, solicitanteOtro)
		ajenoDoc, err := client.UploadDocumento(authCtx(solicitanteOtro, "SOLICITANTE"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expOtro, Nombre: "otro.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento ajeno: %v", err)
		}
		trackDoc(ajenoDoc.Documento.Id)

		listResp, err := client.ListDocumentos(authCtx(solicitanteID, "SOLICITANTE"), &documentos.ListDocumentosRequest{})
		if err != nil {
			t.Fatalf("se esperaba éxito para SOLICITANTE/documentos.view_own sin expediente_id, got err=%v", err)
		}
		if listResp.Total != 2 {
			t.Fatalf("se esperaban 2 documentos (propio + PROVEIDO), total=%d", listResp.Total)
		}
		seen := map[string]bool{}
		for _, d := range listResp.Documentos {
			seen[d.Id] = true
		}
		if !seen[propio.Documento.Id] || !seen[proveido.Documento.Id] {
			t.Fatalf("faltan documentos esperados en la lista: %+v", listResp.Documentos)
		}
		if seen[ajenoDoc.Documento.Id] {
			t.Fatal("el documento de otro solicitante NO debía aparecer")
		}
	})

	// Test (Paso 19C) — SIN expediente_id, solicitante sin ningún
	// expediente propio: debe listar 0 documentos (el "1 = 0" de
	// seguridad en el repositorio), nunca "sin filtro".
	t.Run("List_SinExpedienteId_ViewOwn_SinExpedientesPropios", func(t *testing.T) {
		solicitanteID := createRealUser(t, "solicitantek", "SOLICITANTE")
		listResp, err := client.ListDocumentos(authCtx(solicitanteID, "SOLICITANTE"), &documentos.ListDocumentosRequest{})
		if err != nil {
			t.Fatalf("se esperaba éxito (0 resultados, no error), got err=%v", err)
		}
		if listResp.Total != 0 {
			t.Fatalf("se esperaban 0 documentos (sin expedientes propios), total=%d", listResp.Total)
		}
	})

	// Test — List: sin ningún permiso -> PermissionDenied.
	t.Run("List_SinPermiso", func(t *testing.T) {
		sinRolID := createRealUser(t, "sinroles4")
		_, err := client.ListDocumentos(authCtx(sinRolID, "SOLICITANTE"), &documentos.ListDocumentosRequest{ExpedienteId: expedienteID(11)})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied para un usuario sin roles, got %v", err)
		}
	})

	// Test (Paso 19C, caso obligatorio 8) — Expedientes no disponible, rama
	// SIN expediente_id (la nueva: pagina Expedientes.ListExpedientes). La
	// rama CON expediente_id reutiliza el mismo GetExpediente ya probado
	// fail-closed en Get_ViewOwn_ExpedientesNoDisponible (Paso 19B).
	t.Run("List_SinExpedienteId_ViewOwn_ExpedientesNoDisponible", func(t *testing.T) {
		downExpSvcCtx := &svc.ServiceContext{
			Config:              config.Config{JWTSecret: realSharedJWTSecret},
			DB:                  documentosPool,
			DocumentoRepository: repository.NewDocumentoRepository(documentosPool),
			JWTValidator:        security.NewJWTValidator(realSharedJWTSecret),
			AuthClient:          authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
			ExpedientesClient: expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19997", // puerto sin servicio, a propósito
				NonBlock: true,
				Timeout:  2000,
			})),
		}
		downExpListener := bufconn.Listen(1024 * 1024)
		downExpServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downExpSvcCtx.JWTValidator),
		))
		documentos.RegisterDocumentosServer(downExpServer, server.NewDocumentosServer(downExpSvcCtx))
		go func() { _ = downExpServer.Serve(downExpListener) }()
		defer downExpServer.Stop()

		downExpConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downExpListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Expedientes caído): %v", err)
		}
		defer downExpConn.Close()
		downExpClient := documentos.NewDocumentosClient(downExpConn)

		solicitanteID := createRealUser(t, "solicitantel", "SOLICITANTE")
		_, err = downExpClient.ListDocumentos(authCtx(solicitanteID, "SOLICITANTE"), &documentos.ListDocumentosRequest{})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Expedientes Service no está disponible para view_own sin expediente_id")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}
	})

	// documentoActivo (Paso 19E) consulta directamente documentos_db para
	// confirmar sin ambigüedad si la baja lógica ocurrió o no.
	documentoActivo := func(t *testing.T, id string) bool {
		t.Helper()
		var estado string
		if err := documentosPool.QueryRow(ctx, "SELECT estado FROM documentos WHERE id = $1", id).Scan(&estado); err != nil {
			t.Fatalf("consultar estado de %s: %v", id, err)
		}
		return estado == "ACTIVO"
	}

	// derivarExpedienteReal mueve un expediente real de área (vía
	// Expedientes.DerivarExpediente, ya probado en Expedientes Service)
	// usando un actor con CanViewAll (SECRETARIA) para no depender de las
	// reglas de área en cada paso de preparación — el foco de estos tests
	// es DeleteDocumento, no el flujo de derivación en sí.
	derivarExpedienteReal := func(t *testing.T, secretariaID, expID, areaDestino string) {
		t.Helper()
		if _, err := expedientesRealClient.DerivarExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.DerivarExpedienteRequest{
			Id: expID, AreaDestino: areaDestino,
		}); err != nil {
			t.Fatalf("no se pudo derivar el expediente real a %s: %v", areaDestino, err)
		}
	}

	// Test (Paso 19E, Caso 1) — permitido: DIRECTOR (rol interno SIN
	// CanViewAll) elimina un documento de un expediente PENDIENTE cuya
	// área SÍ coincide con la suya (derivado ahí con un tipo genérico, que
	// no dispara ningún cambio de estado especial — ver estados.CanDerivar).
	t.Run("Delete_Permitido_AreaCorrespondiente", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria7", "SECRETARIA")
		resp, err := expedientesRealClient.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "OTRO", Asunto: "Test delete permitido", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		exp := resp.Expediente.Id
		createdExpedienteIDs = append(createdExpedienteIDs, exp)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR")

		directorID := createRealUser(t, "director3", "DIRECTOR")
		uploaded, err := client.UploadDocumento(authCtx(directorID, "DIRECTOR"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		// DeleteDocumento es una baja lógica (UPDATE estado='ELIMINADO'),
		// no un DELETE FROM — la fila sigue existiendo, así que igual hay
		// que trackearla para la limpieza.
		trackDoc(uploaded.Documento.Id)

		if _, err := client.DeleteDocumento(authCtx(directorID, "DIRECTOR"), &documentos.DeleteDocumentoRequest{Id: uploaded.Documento.Id}); err != nil {
			t.Fatalf("se esperaba éxito para DIRECTOR/documentos.delete sobre su propia área, got err=%v", err)
		}
		if documentoActivo(t, uploaded.Documento.Id) {
			t.Fatal("se esperaba estado=ELIMINADO tras la baja lógica")
		}
	})

	// Test (Paso 19E, Caso 2 / Caso 9 "RPC directo") — área incorrecta:
	// AUXILIAR (interno, sin CanViewAll) intenta eliminar un documento de
	// un expediente cuya área es DIRECTOR. Llamada directa al RPC (bufconn,
	// sin Gateway) — demuestra que la protección ya no depende del
	// Gateway (que para Delete siempre fue passthrough, ver Paso 19E).
	t.Run("Delete_RpcDirecto_AreaIncorrecta_Denegado", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria9c", "SECRETARIA")
		resp, err := expedientesRealClient.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "OTRO", Asunto: "Test delete area incorrecta", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		exp := resp.Expediente.Id
		createdExpedienteIDs = append(createdExpedienteIDs, exp)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR")

		directorID := createRealUser(t, "director4", "DIRECTOR")
		uploaded, err := client.UploadDocumento(authCtx(directorID, "DIRECTOR"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		auxiliarID := createRealUser(t, "auxiliar1", "AUXILIAR")
		_, err = client.DeleteDocumento(authCtx(auxiliarID, "AUXILIAR"), &documentos.DeleteDocumentoRequest{Id: uploaded.Documento.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por área (expediente en DIRECTOR, actor AUXILIAR), got %v", err)
		}
		if !documentoActivo(t, uploaded.Documento.Id) {
			t.Fatal("activo debía permanecer TRUE: la baja no debió ejecutarse")
		}
	})

	// Test (Paso 19E, Caso 3) — estado EN_PROCESO: DIRECTOR, con área
	// correcta y permiso correcto, intenta eliminar un documento de un
	// expediente F2 (CERTIFICADO) recién derivado a Dirección — ese caso
	// especial SÍ cambia el estado a EN_PROCESO (ver DerivarExpediente).
	t.Run("Delete_Denegado_EnProceso", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria9d", "SECRETARIA")
		resp, err := expedientesRealClient.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test delete en proceso", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		exp := resp.Expediente.Id
		createdExpedienteIDs = append(createdExpedienteIDs, exp)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR") // F2 SECRETARIA->DIRECTOR: PENDIENTE -> EN_PROCESO

		directorID := createRealUser(t, "director5", "DIRECTOR")
		uploaded, err := client.UploadDocumento(authCtx(directorID, "DIRECTOR"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		_, err = client.DeleteDocumento(authCtx(directorID, "DIRECTOR"), &documentos.DeleteDocumentoRequest{Id: uploaded.Documento.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied por estado EN_PROCESO, got %v", err)
		}
		if !documentoActivo(t, uploaded.Documento.Id) {
			t.Fatal("activo debía permanecer TRUE: la baja no debió ejecutarse")
		}
	})

	// Test (Paso 19E, Casos 4 y 5) — estado ATENDIDO, incluyendo el
	// PROVEIDO: se completa el flujo F2 real (SECRETARIA->DIRECTOR,
	// DIRECTOR->SECRETARIA, ResolverExpediente) hasta ATENDIDO, con un
	// PROVEIDO subido por DIRECTOR ANTES del cierre. SECRETARIA
	// (CanViewAll, área correcta por definición) intenta eliminar ese
	// PROVEIDO una vez el expediente ya está ATENDIDO -> debe negarse.
	// Este test demuestra que el documento final no puede eliminarse de
	// un expediente ya cerrado.
	t.Run("Delete_Denegado_Atendido_Proveido", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria9e", "SECRETARIA")
		resp, err := expedientesRealClient.CreateExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.CreateExpedienteRequest{
			Tipo: "CERTIFICADO", Asunto: "Test delete atendido proveido", Prioridad: "NORMAL",
		})
		if err != nil {
			t.Fatalf("no se pudo crear el expediente real base: %v", err)
		}
		exp := resp.Expediente.Id
		createdExpedienteIDs = append(createdExpedienteIDs, exp)
		derivarExpedienteReal(t, secretariaID, exp, "DIRECTOR") // PENDIENTE -> EN_PROCESO, area DIRECTOR

		direccionID := createRealUser(t, "direccion5", "DIRECTOR")
		proveido, err := client.UploadDocumento(authCtx(direccionID, "DIRECTOR"), &documentos.UploadDocumentoRequest{
			ExpedienteId: exp, Nombre: "proveido.pdf", TipoDocumento: "PROVEIDO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el PROVEIDO: %v", err)
		}
		trackDoc(proveido.Documento.Id)

		// DIRECTOR devuelve el expediente a Secretaría (área), y
		// Secretaría cierra el F2 (ResolverExpediente: EN_PROCESO ->
		// ATENDIDO) — mismo flujo real ya probado en Expedientes Service.
		if _, err := expedientesRealClient.DerivarExpediente(authCtx(direccionID, "DIRECTOR"), &expedientesclient.DerivarExpedienteRequest{
			Id: exp, AreaDestino: "SECRETARIA",
		}); err != nil {
			t.Fatalf("no se pudo devolver el expediente a Secretaría: %v", err)
		}
		if _, err := expedientesRealClient.ResolverExpediente(authCtx(secretariaID, "SECRETARIA"), &expedientesclient.ResolverExpedienteRequest{Id: exp}); err != nil {
			t.Fatalf("no se pudo resolver (cerrar) el expediente: %v", err)
		}

		_, err = client.DeleteDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.DeleteDocumentoRequest{Id: proveido.Documento.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: el expediente ya está ATENDIDO, got %v", err)
		}
		if !documentoActivo(t, proveido.Documento.Id) {
			t.Fatal("el PROVEIDO debía permanecer activo: no debe poder eliminarse de un expediente cerrado")
		}
	})

	// Test (Paso 19E, Caso 6) — SOLICITANTE no tiene documentos.delete en
	// el catálogo real, así que queda denegado antes de llegar a
	// consultar ningún expediente.
	t.Run("Delete_PermisoDenegado_Solicitante", func(t *testing.T) {
		secretariaID := createRealUser(t, "secretaria8", "SECRETARIA")
		uploaded, err := client.UploadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(13), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		solicitanteID := createRealUser(t, "solicitante3", "SOLICITANTE")
		_, err = client.DeleteDocumento(authCtx(solicitanteID, "SOLICITANTE"), &documentos.DeleteDocumentoRequest{Id: uploaded.Documento.Id})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("se esperaba PermissionDenied: SOLICITANTE no tiene documentos.delete, got %v", err)
		}
		if !documentoActivo(t, uploaded.Documento.Id) {
			t.Fatal("activo debía permanecer TRUE: la baja no debió ejecutarse")
		}
	})

	// Test (Paso 19E, Caso 8) — Expedientes no disponible: debe fallar
	// cerrado, sin ejecutar la baja.
	t.Run("Delete_ExpedientesNoDisponible", func(t *testing.T) {
		downExpSvcCtx := &svc.ServiceContext{
			Config:              config.Config{JWTSecret: realSharedJWTSecret},
			DB:                  documentosPool,
			DocumentoRepository: repository.NewDocumentoRepository(documentosPool),
			JWTValidator:        security.NewJWTValidator(realSharedJWTSecret),
			AuthClient:          authclient.NewAuth(zrpc.MustNewClient(zrpc.RpcClientConf{Target: "127.0.0.1:8080"})),
			ExpedientesClient: expedientesclient.NewExpedientes(zrpc.MustNewClient(zrpc.RpcClientConf{
				Target:   "127.0.0.1:19995", // puerto sin servicio, a propósito
				NonBlock: true,
				Timeout:  2000,
			})),
		}
		downExpListener := bufconn.Listen(1024 * 1024)
		downExpServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
			interceptor.AuthenticationInterceptor(downExpSvcCtx.JWTValidator),
		))
		documentos.RegisterDocumentosServer(downExpServer, server.NewDocumentosServer(downExpSvcCtx))
		go func() { _ = downExpServer.Serve(downExpListener) }()
		defer downExpServer.Stop()

		downExpConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downExpListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Expedientes caído): %v", err)
		}
		defer downExpConn.Close()
		downExpClient := documentos.NewDocumentosClient(downExpConn)

		secretariaID := createRealUser(t, "secretaria9f", "SECRETARIA")
		uploaded, err := client.UploadDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(16), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base: %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		_, err = downExpClient.DeleteDocumento(authCtx(secretariaID, "SECRETARIA"), &documentos.DeleteDocumentoRequest{Id: uploaded.Documento.Id})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Expedientes Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}
		if !documentoActivo(t, uploaded.Documento.Id) {
			t.Fatal("activo debía permanecer TRUE: la baja no debió ejecutarse con Expedientes caído")
		}
	})

	// Test — Auth no disponible: un AuthClient apuntando a un puerto sin
	// nada escuchando debe producir un ERROR (fail-closed), nunca permitir
	// la operación por fallback al rol local.
	t.Run("AuthNoDisponible", func(t *testing.T) {
		downSvcCtx := &svc.ServiceContext{
			Config:              config.Config{JWTSecret: realSharedJWTSecret},
			DB:                  documentosPool,
			DocumentoRepository: repository.NewDocumentoRepository(documentosPool),
			JWTValidator:        security.NewJWTValidator(realSharedJWTSecret),
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
		documentos.RegisterDocumentosServer(downServer, server.NewDocumentosServer(downSvcCtx))
		go func() { _ = downServer.Serve(downListener) }()
		defer downServer.Stop()

		downConn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return downListener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("dial gRPC test server (Auth caído): %v", err)
		}
		defer downConn.Close()
		downClient := documentos.NewDocumentosClient(downConn)

		userID := createRealUser(t, "authcaido", "SECRETARIA")
		_, err = downClient.UploadDocumento(authCtx(userID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(14), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) cuando Auth Service no está disponible, la operación se permitió")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real, got OK")
		}

		// Caso 6 (Paso 19B) — GetDocumento también debe fallar cerrado con
		// Auth caído: la primera consulta a Auth.HasPermission
		// ("documentos.view") ya falla, sin llegar siquiera a
		// ExpedientesClient (nil en downSvcCtx a propósito, para probar
		// justamente que nunca se llega tan lejos).
		uploaded, err := client.UploadDocumento(authCtx(userID, "SECRETARIA"), &documentos.UploadDocumentoRequest{
			ExpedienteId: expedienteID(15), Nombre: "test.pdf", TipoDocumento: "ADJUNTO", Extension: "pdf", Contenido: []byte("x"),
		})
		if err != nil {
			t.Fatalf("no se pudo crear el documento base (con Auth arriba): %v", err)
		}
		trackDoc(uploaded.Documento.Id)

		_, err = downClient.GetDocumento(authCtx(userID, "SECRETARIA"), &documentos.GetDocumentoRequest{Id: uploaded.Documento.Id})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para GetDocumento cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para GetDocumento, got OK")
		}

		// Caso 7 (Paso 19C) — ListDocumentos también debe fallar cerrado con
		// Auth caído: la consulta a Auth.HasPermission ("documentos.view")
		// ya falla, sin llegar siquiera a ExpedientesClient.
		_, err = downClient.ListDocumentos(authCtx(userID, "SECRETARIA"), &documentos.ListDocumentosRequest{})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para ListDocumentos cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para ListDocumentos, got OK")
		}

		// Caso 6 (Paso 19D) — DownloadDocumento también debe fallar cerrado
		// con Auth caído: la consulta a Auth.HasPermission ("documentos.view")
		// ya falla, sin llegar siquiera a leer metadata ni contenido.
		_, err = downClient.DownloadDocumento(authCtx(userID, "SECRETARIA"), &documentos.DownloadDocumentoRequest{Id: uploaded.Documento.Id})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para DownloadDocumento cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para DownloadDocumento, got OK")
		}

		// Caso 7 (Paso 19E) — DeleteDocumento también debe fallar cerrado
		// con Auth caído: la consulta a Auth.HasPermission ("documentos.delete")
		// ya falla, sin llegar siquiera a consultar Expedientes ni a
		// ejecutar la baja.
		_, err = downClient.DeleteDocumento(authCtx(userID, "SECRETARIA"), &documentos.DeleteDocumentoRequest{Id: uploaded.Documento.Id})
		if err == nil {
			t.Fatal("se esperaba un error (fail-closed) para DeleteDocumento cuando Auth Service no está disponible")
		}
		if status.Code(err) == codes.OK {
			t.Fatalf("se esperaba un código de error real para DeleteDocumento, got OK")
		}
	})
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
