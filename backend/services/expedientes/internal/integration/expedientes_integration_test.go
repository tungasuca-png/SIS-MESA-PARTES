package integration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

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
)

const testSecret = "expedientes-integration-test-secret"

func TestExpedientesIntegration(t *testing.T) {
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

	testID := time.Now().UnixNano()
	solicitanteID := fixedUUID(testID, 1)
	otroSolicitanteID := fixedUUID(testID, 2)
	defer cleanupIntegrationData(t, pool, solicitanteID, otroSolicitanteID)

	svcCtx := &svc.ServiceContext{
		Config:               config.Config{JWTSecret: testSecret},
		DB:                   pool,
		ExpedienteRepository: repository.NewExpedienteRepository(pool),
		JWTValidator:         security.NewJWTValidator(testSecret),
		// Esta suite prueba reglas de negocio (área/ownership/estado/CAS/
		// F2/F4) con identidades sintéticas que no existen en auth_db — ver
		// fake_authclient_test.go. El catálogo de permisos real se prueba
		// en TestHasPermissionIntegration, en este mismo paquete.
		AuthClient: newReplicaAuthClient(security.NewJWTValidator(testSecret)),
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		interceptor.AuthenticationInterceptor(svcCtx.JWTValidator),
	))
	expedientes.RegisterExpedientesServer(grpcServer, server.NewExpedientesServer(svcCtx))
	go func() { _ = grpcServer.Serve(listener) }()
	defer grpcServer.Stop()

	clientCtx := context.Background()
	conn, err := grpc.DialContext(clientCtx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial gRPC test server: %v", err)
	}
	defer conn.Close()
	client := expedientes.NewExpedientesClient(conn)

	ctxSolicitante := authContext(solicitanteID, "SOLICITANTE")
	ctxOtroSolicitante := authContext(otroSolicitanteID, "SOLICITANTE")
	ctxAdmin := authContext(fixedUUID(testID, 3), "ADMIN")
	ctxSecretaria := authContext(fixedUUID(testID, 4), "SECRETARIA")
	ctxDirector := authContext(fixedUUID(testID, 5), "DIRECTOR")
	ctxDocente := authContext(fixedUUID(testID, 6), "DOCENTE")
	ctxSubdirector := authContext(fixedUUID(testID, 7), "SUBDIRECTOR")

	// --- Crear expediente ---
	createResp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo:   "SOLICITUD",
		Asunto: "Solicitud de certificado de estudios",
	})
	if err != nil {
		t.Fatalf("create expediente: %v", err)
	}
	if createResp.Expediente.Estado != "PENDIENTE" {
		t.Fatalf("expected initial estado PENDIENTE, got %q", createResp.Expediente.Estado)
	}
	if createResp.Expediente.Tipo != "SOLICITUD" || createResp.Expediente.Prioridad != "NORMAL" {
		t.Fatalf("unexpected tipo/prioridad: tipo=%q prioridad=%q", createResp.Expediente.Tipo, createResp.Expediente.Prioridad)
	}
	if !strings.HasPrefix(createResp.Expediente.Codigo, "EXP-") {
		t.Fatalf("unexpected codigo format: %s", createResp.Expediente.Codigo)
	}

	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for missing asunto, got %v", err)
	}
	// Etapa 2: el tipo ya no se completa solo como SOLICITUD — vacío se
	// rechaza igual que cualquier valor inválido.
	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Asunto: "x"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for tipo vacío, got %v", err)
	}
	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Asunto: "x", Tipo: "INVALIDO"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for tipo inválido, got %v", err)
	}
	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Asunto: "x", Tipo: "ABC"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for tipo ABC, got %v", err)
	}
	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Asunto: "x", Tipo: "SOLICITUD", Prioridad: "ALTA"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for prioridad inválida, got %v", err)
	}
	if _, err := client.CreateExpediente(context.Background(), &expedientes.CreateExpedienteRequest{Asunto: "x"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without token, got %v", err)
	}

	// --- Código único ---
	createResp2, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Tipo: "SOLICITUD", Asunto: "Otro expediente"})
	if err != nil {
		t.Fatalf("create second expediente: %v", err)
	}
	if createResp2.Expediente.Codigo == createResp.Expediente.Codigo {
		t.Fatalf("expected unique codigo, got duplicate %s", createResp.Expediente.Codigo)
	}

	// --- Consultar ---
	getResp, err := client.GetExpediente(ctxSolicitante, &expedientes.GetExpedienteRequest{Id: createResp.Expediente.Codigo})
	if err != nil {
		t.Fatalf("get own expediente by codigo: %v", err)
	}
	if getResp.Expediente.Id != createResp.Expediente.Id {
		t.Fatalf("expected same expediente by codigo lookup")
	}

	if _, err := client.GetExpediente(ctxOtroSolicitante, &expedientes.GetExpedienteRequest{Id: createResp.Expediente.Id}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for foreign expediente, got %v", err)
	}
	if _, err := client.GetExpediente(ctxAdmin, &expedientes.GetExpedienteRequest{Id: "no-existe-" + time.Now().String()}); status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound for missing expediente, got %v", err)
	}

	// --- Autorización de actualización ---
	if _, err := client.UpdateExpediente(ctxSolicitante, &expedientes.UpdateExpedienteRequest{Id: createResp.Expediente.Id, Asunto: "otro asunto"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for solicitante update, got %v", err)
	}
	updateResp, err := client.UpdateExpediente(ctxAdmin, &expedientes.UpdateExpedienteRequest{
		Id:        createResp.Expediente.Id,
		Asunto:    "Solicitud de certificado (actualizado)",
		Prioridad: "URGENTE",
	})
	if err != nil {
		t.Fatalf("update as admin: %v", err)
	}
	if updateResp.Expediente.Prioridad != "URGENTE" {
		t.Fatalf("expected priority updated to URGENTE, got %q", updateResp.Expediente.Prioridad)
	}

	// --- Autorización y transición de estados ---
	if _, err := client.ChangeEstado(ctxSolicitante, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "EN_PROCESO"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for solicitante change estado, got %v", err)
	}
	if _, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "ATENDIDO"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition for PENDIENTE->ATENDIDO, got %v", err)
	}
	changeResp, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "EN_PROCESO"})
	if err != nil {
		t.Fatalf("change estado to EN_PROCESO: %v", err)
	}
	if changeResp.Expediente.Estado != "EN_PROCESO" {
		t.Fatalf("expected EN_PROCESO, got %q", changeResp.Expediente.Estado)
	}
	if _, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "ATENDIDO"}); err != nil {
		t.Fatalf("change estado to ATENDIDO: %v", err)
	}
	if _, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{Id: createResp.Expediente.Id, NuevoEstado: "PENDIENTE"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition for ATENDIDO->PENDIENTE, got %v", err)
	}

	// --- Listado con alcance por rol ---
	listAdmin, err := client.ListExpedientes(ctxAdmin, &expedientes.ListExpedientesRequest{})
	if err != nil {
		t.Fatalf("list as admin: %v", err)
	}
	if listAdmin.Total < 2 {
		t.Fatalf("expected at least 2 expedientes visible to admin, got %d", listAdmin.Total)
	}

	listSolicitante, err := client.ListExpedientes(ctxSolicitante, &expedientes.ListExpedientesRequest{})
	if err != nil {
		t.Fatalf("list as solicitante: %v", err)
	}
	for _, e := range listSolicitante.Expedientes {
		if e.SolicitanteId != solicitanteID {
			t.Fatalf("solicitante should only see its own expedientes, saw solicitante_id=%s", e.SolicitanteId)
		}
	}

	// --- Etapa 1: autorización por área en ChangeEstado/UpdateArea ---
	// Todo expediente nuevo entra con area_actual=SECRETARIA (default de la
	// migración 002_add_area_actual.sql).
	areaExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Tipo: "SOLICITUD", Asunto: "Prueba de autorización por área"})
	if err != nil {
		t.Fatalf("create expediente for area authorization tests: %v", err)
	}
	if areaExp.Expediente.AreaActual != "SECRETARIA" {
		t.Fatalf("expected new expediente area_actual=SECRETARIA, got %q", areaExp.Expediente.AreaActual)
	}

	// Caso 1 — área correcta: SECRETARIA opera sobre un expediente que está
	// en SECRETARIA.
	if _, err := client.ChangeEstado(ctxSecretaria, &expedientes.ChangeEstadoRequest{
		Id: areaExp.Expediente.Id, NuevoEstado: "EN_PROCESO",
	}); err != nil {
		t.Fatalf("Caso 1 (area correcta) esperaba éxito, obtuvo: %v", err)
	}

	// Caso 2 — área incorrecta: DIRECCION intenta operar sobre un expediente
	// que sigue en SECRETARIA.
	if _, err := client.ChangeEstado(ctxDirector, &expedientes.ChangeEstadoRequest{
		Id: areaExp.Expediente.Id, NuevoEstado: "ATENDIDO",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Caso 2 (area incorrecta) esperaba PermissionDenied, obtuvo: %v", err)
	}
	if _, err := client.UpdateArea(ctxDirector, &expedientes.UpdateAreaRequest{
		Id: areaExp.Expediente.Id, Area: "DIRECTOR",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Caso 2 UpdateArea (area incorrecta) esperaba PermissionDenied, obtuvo: %v", err)
	}

	// Caso 3 — otro rol: DOCENTE tampoco puede operar sobre un expediente que
	// no está en su área.
	if _, err := client.ChangeEstado(ctxDocente, &expedientes.ChangeEstadoRequest{
		Id: areaExp.Expediente.Id, NuevoEstado: "ATENDIDO",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Caso 3 (otro rol) esperaba PermissionDenied, obtuvo: %v", err)
	}

	// SECRETARIA (dueña actual, vía CanViewAll) deriva el expediente hacia
	// DIRECCION — a partir de acá le corresponde a Dirección, no a Secretaría.
	if _, err := client.UpdateArea(ctxSecretaria, &expedientes.UpdateAreaRequest{
		Id: areaExp.Expediente.Id, Area: "DIRECTOR",
	}); err != nil {
		t.Fatalf("SECRETARIA derivando a DIRECTOR esperaba éxito, obtuvo: %v", err)
	}

	// Tras la derivación, DOCENTE (ajeno) sigue sin poder actuar...
	if _, err := client.ChangeEstado(ctxDocente, &expedientes.ChangeEstadoRequest{
		Id: areaExp.Expediente.Id, NuevoEstado: "ATENDIDO",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("DOCENTE tras derivación esperaba PermissionDenied, obtuvo: %v", err)
	}
	// ...pero DIRECTOR (nuevo dueño real) sí puede.
	if _, err := client.ChangeEstado(ctxDirector, &expedientes.ChangeEstadoRequest{
		Id: areaExp.Expediente.Id, NuevoEstado: "ATENDIDO",
	}); err != nil {
		t.Fatalf("DIRECTOR (nueva area) esperaba éxito, obtuvo: %v", err)
	}

	// Caso 4 — SOLICITANTE no puede modificar arbitrariamente estado/área de
	// su propio expediente (ChangeEstado ya se cubre arriba; se agrega
	// UpdateArea acá).
	solicitanteExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Tipo: "SOLICITUD", Asunto: "Prueba solicitante no deriva"})
	if err != nil {
		t.Fatalf("create expediente for solicitante case: %v", err)
	}
	if _, err := client.UpdateArea(ctxSolicitante, &expedientes.UpdateAreaRequest{
		Id: solicitanteExp.Expediente.Id, Area: "SECRETARIA",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Caso 4 (solicitante UpdateArea) esperaba PermissionDenied, obtuvo: %v", err)
	}

	// Caso 5 — ADMIN conserva su comportamiento administrativo global
	// existente (CanViewAll): puede operar sin importar el área actual.
	if _, err := client.UpdateArea(ctxAdmin, &expedientes.UpdateAreaRequest{
		Id: solicitanteExp.Expediente.Id, Area: "AUXILIAR",
	}); err != nil {
		t.Fatalf("Caso 5 (ADMIN) esperaba éxito, obtuvo: %v", err)
	}

	// Caso 6 — expediente inexistente: mismo error ya usado por el proyecto
	// (NotFound) para ChangeEstado y UpdateArea.
	expedienteInexistente := "no-existe-" + time.Now().String()
	if _, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{
		Id: expedienteInexistente, NuevoEstado: "EN_PROCESO",
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("Caso 6 ChangeEstado esperaba NotFound, obtuvo: %v", err)
	}
	if _, err := client.UpdateArea(ctxAdmin, &expedientes.UpdateAreaRequest{
		Id: expedienteInexistente, Area: "SECRETARIA",
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("Caso 6 UpdateArea esperaba NotFound, obtuvo: %v", err)
	}

	// Caso 7 — expediente inactivo (baja lógica): ya no debe poder
	// modificarse (FindByIDOrCodigo filtra activo=TRUE, así que se ve igual
	// que "no encontrado", consistente con el resto de operaciones de
	// lectura/escritura del repositorio).
	inactivoExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Tipo: "SOLICITUD", Asunto: "Prueba expediente inactivo"})
	if err != nil {
		t.Fatalf("create expediente for inactive case: %v", err)
	}
	if _, err := client.DeleteExpediente(ctxAdmin, &expedientes.DeleteExpedienteRequest{Id: inactivoExp.Expediente.Id}); err != nil {
		t.Fatalf("delete (baja lógica) expediente: %v", err)
	}
	if _, err := client.ChangeEstado(ctxAdmin, &expedientes.ChangeEstadoRequest{
		Id: inactivoExp.Expediente.Id, NuevoEstado: "EN_PROCESO",
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("Caso 7 ChangeEstado sobre inactivo esperaba NotFound, obtuvo: %v", err)
	}
	if _, err := client.UpdateArea(ctxAdmin, &expedientes.UpdateAreaRequest{
		Id: inactivoExp.Expediente.Id, Area: "SECRETARIA",
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("Caso 7 UpdateArea sobre inactivo esperaba NotFound, obtuvo: %v", err)
	}

	// --- Etapa 1.1, punto 3: prueba explícita de SUBDIRECTOR ---
	subdirExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Tipo: "SOLICITUD", Asunto: "Prueba SUBDIRECTOR"})
	if err != nil {
		t.Fatalf("create expediente for subdirector case: %v", err)
	}
	// Bloquea: el expediente sigue en SECRETARIA, no en SUBDIRECCION.
	if _, err := client.ChangeEstado(ctxSubdirector, &expedientes.ChangeEstadoRequest{
		Id: subdirExp.Expediente.Id, NuevoEstado: "EN_PROCESO",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("SUBDIRECTOR sobre expediente ajeno (area=SECRETARIA) esperaba PermissionDenied, obtuvo: %v", err)
	}
	if _, err := client.UpdateArea(ctxSubdirector, &expedientes.UpdateAreaRequest{
		Id: subdirExp.Expediente.Id, Area: "SUBDIRECTOR",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("SUBDIRECTOR UpdateArea sobre expediente ajeno esperaba PermissionDenied, obtuvo: %v", err)
	}
	// Se deriva legítimamente hacia SUBDIRECCION (SECRETARIA, dueña actual,
	// vía CanViewAll)...
	if _, err := client.UpdateArea(ctxSecretaria, &expedientes.UpdateAreaRequest{
		Id: subdirExp.Expediente.Id, Area: "SUBDIRECTOR",
	}); err != nil {
		t.Fatalf("SECRETARIA derivando a SUBDIRECTOR esperaba éxito, obtuvo: %v", err)
	}
	// ...y ahora sí permite: el expediente ya está en su área.
	if _, err := client.ChangeEstado(ctxSubdirector, &expedientes.ChangeEstadoRequest{
		Id: subdirExp.Expediente.Id, NuevoEstado: "EN_PROCESO",
	}); err != nil {
		t.Fatalf("SUBDIRECTOR sobre expediente propio (area=SUBDIRECTOR) esperaba éxito, obtuvo: %v", err)
	}

	// --- Etapa 1.1, punto 1: conflicto de concurrencia optimista en UpdateArea ---
	// Se simula el escenario "lectura válida → otra operación cambia el área
	// entretanto → la escritura basada en la lectura vieja debe rechazarse"
	// operando el repositorio directamente (mismo repositorio que usa
	// UpdateAreaLogic), sin depender de una carrera real entre goroutines
	// (no determinística) para que la prueba no sea intermitente.
	concurrExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{Tipo: "SOLICITUD", Asunto: "Prueba de concurrencia en UpdateArea"})
	if err != nil {
		t.Fatalf("create expediente for concurrency case: %v", err)
	}

	// "Request A" lee el expediente (area_actual=SECRETARIA en este punto).
	leidoPorA, err := svcCtx.ExpedienteRepository.FindByIDOrCodigo(ctx, concurrExp.Expediente.Id)
	if err != nil {
		t.Fatalf("lectura previa a la carrera: %v", err)
	}
	if leidoPorA.AreaActual != "SECRETARIA" {
		t.Fatalf("expected area_actual=SECRETARIA before race, got %q", leidoPorA.AreaActual)
	}

	// "Request B" gana la carrera: deriva primero, con la misma area leída.
	if _, err := svcCtx.ExpedienteRepository.UpdateArea(ctx, concurrExp.Expediente.Id, leidoPorA.AreaActual, "DIRECTOR"); err != nil {
		t.Fatalf("Request B (gana la carrera) esperaba éxito, obtuvo: %v", err)
	}

	// "Request A" intenta escribir con la lectura vieja (area_actual ya no es
	// SECRETARIA, es DIRECTOR) — debe rechazarse, no pisar el cambio de B.
	if _, err := svcCtx.ExpedienteRepository.UpdateArea(ctx, concurrExp.Expediente.Id, leidoPorA.AreaActual, "SUBDIRECTOR"); !errors.Is(err, repository.ErrExpedienteNotFound) {
		t.Fatalf("Request A (lectura vieja) esperaba ErrExpedienteNotFound (conflicto), obtuvo: %v", err)
	}

	// El área final debe ser la de B (DIRECTOR), no la de la escritura
	// rechazada de A (SUBDIRECTOR) ni la original (SECRETARIA).
	final, err := svcCtx.ExpedienteRepository.FindByIDOrCodigo(ctx, concurrExp.Expediente.Id)
	if err != nil {
		t.Fatalf("lectura final tras la carrera: %v", err)
	}
	if final.AreaActual != "DIRECTOR" {
		t.Fatalf("expected area_actual=DIRECTOR tras el conflicto de concurrencia, got %q", final.AreaActual)
	}

	// UpdateAreaLogic (capa gRPC) usa este mismo método de repositorio y
	// solo traduce ErrExpedienteNotFound a codes.Aborted (ver
	// updatearealogic.go) — no se repite la prueba con una carrera real de
	// goroutines contra el servidor gRPC para no introducir un test
	// intermitente; la guarda queda demostrada de forma determinística
	// arriba, contra el mismo código que ejecuta la lógica real.

	// --- Etapa 2: tipos de trámite reales ---
	// Los 5 tipos nuevos (docs/analisis-5-flujos-completo.md, F2 y F4) deben
	// poder crearse y guardar el tipo exacto que se envió.
	tiposValidos := []string{"CERTIFICADO", "CONSTANCIA", "PERMISO", "JUSTIFICACION_FALTA", "JUSTIFICACION_TARDANZA"}
	for _, tipo := range tiposValidos {
		resp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
			Tipo: tipo, Asunto: "Prueba de tipo " + tipo,
		})
		if err != nil {
			t.Fatalf("crear expediente tipo=%s esperaba éxito, obtuvo: %v", tipo, err)
		}
		if resp.Expediente.Tipo != tipo {
			t.Fatalf("esperaba Tipo=%s guardado, obtuvo %q", tipo, resp.Expediente.Tipo)
		}
		// Debe aparecer correctamente en el detalle (GetExpediente).
		got, err := client.GetExpediente(ctxSolicitante, &expedientes.GetExpedienteRequest{Id: resp.Expediente.Id})
		if err != nil {
			t.Fatalf("GetExpediente tras crear tipo=%s: %v", tipo, err)
		}
		if got.Expediente.Tipo != tipo {
			t.Fatalf("detalle: esperaba Tipo=%s, obtuvo %q", tipo, got.Expediente.Tipo)
		}
	}

	// Tipos inválidos: además de "ABC"/""/"INVALIDO" (ya probados arriba),
	// un nombre parecido pero no exacto tampoco debe aceptarse (no hay
	// normalización más allá de mayúsculas/espacios — ver
	// strings.ToUpper(strings.TrimSpace(...)) en CreateExpedienteLogic).
	if _, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "JUSTIFICACION", Asunto: "x",
	}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("esperaba InvalidArgument para tipo=JUSTIFICACION (no existe, son dos tipos separados), obtuvo: %v", err)
	}

	// Compatibilidad: un expediente antiguo con tipo=SOLICITUD (createResp,
	// creado al principio de esta prueba) sigue pudiendo consultarse,
	// listarse, cambiarse de estado y derivarse con las reglas normales —
	// no requiere ninguna migración de datos ni conversión automática.
	if _, err := client.GetExpediente(ctxSolicitante, &expedientes.GetExpedienteRequest{Id: createResp.Expediente.Id}); err != nil {
		t.Fatalf("expediente antiguo (tipo=SOLICITUD) debía seguir siendo consultable: %v", err)
	}
	listSol, err := client.ListExpedientes(ctxSolicitante, &expedientes.ListExpedientesRequest{})
	if err != nil {
		t.Fatalf("listado tras Etapa 2: %v", err)
	}
	encontrado := false
	for _, e := range listSol.Expedientes {
		if e.Id == createResp.Expediente.Id && e.Tipo == "SOLICITUD" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Fatalf("expediente antiguo (tipo=SOLICITUD) debía seguir apareciendo en el listado")
	}

	// --- Etapa 3: DerivarExpediente (workflow real) ---

	// F2 (CERTIFICADO): única transición confirmada, SECRETARIA -> DIRECTOR.
	f2Exp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "CERTIFICADO", Asunto: "Certificado E2E Etapa 3",
	})
	if err != nil {
		t.Fatalf("crear expediente F2: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxDocente, &expedientes.DerivarExpedienteRequest{
		Id: f2Exp.Expediente.Id, AreaDestino: "DIRECTOR",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("DOCENTE derivando expediente ajeno esperaba PermissionDenied, obtuvo: %v", err)
	}
	derivF2, err := client.DerivarExpediente(ctxSecretaria, &expedientes.DerivarExpedienteRequest{
		Id: f2Exp.Expediente.Id, AreaDestino: "DIRECTOR",
	})
	if err != nil {
		t.Fatalf("SECRETARIA derivando F2 a DIRECTOR esperaba éxito, obtuvo: %v", err)
	}
	if derivF2.Expediente.AreaActual != "DIRECTOR" {
		t.Fatalf("esperaba area_actual=DIRECTOR tras derivar F2, obtuvo %q", derivF2.Expediente.AreaActual)
	}
	// F2: no hay transición confirmada más allá de Dirección (F2-02 sigue
	// pendiente) — debe rechazarse, no inventarse.
	if _, err := client.DerivarExpediente(ctxDirector, &expedientes.DerivarExpedienteRequest{
		Id: f2Exp.Expediente.Id, AreaDestino: "SUBDIRECTOR",
	}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("F2 DIRECTOR->SUBDIRECTOR (no confirmado) esperaba FailedPrecondition, obtuvo: %v", err)
	}

	// F4 (JUSTIFICACION_TARDANZA): dos transiciones confirmadas en cadena.
	f4Exp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "JUSTIFICACION_TARDANZA", Asunto: "Justificación de tardanza E2E Etapa 3",
	})
	if err != nil {
		t.Fatalf("crear expediente F4: %v", err)
	}
	// SECRETARIA no puede saltarse a Subdirección directamente.
	if _, err := client.DerivarExpediente(ctxSecretaria, &expedientes.DerivarExpedienteRequest{
		Id: f4Exp.Expediente.Id, AreaDestino: "SUBDIRECTOR",
	}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("F4 SECRETARIA->SUBDIRECTOR (salto no confirmado) esperaba FailedPrecondition, obtuvo: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxSecretaria, &expedientes.DerivarExpedienteRequest{
		Id: f4Exp.Expediente.Id, AreaDestino: "DIRECTOR",
	}); err != nil {
		t.Fatalf("F4 SECRETARIA->DIRECTOR esperaba éxito, obtuvo: %v", err)
	}
	derivF4, err := client.DerivarExpediente(ctxDirector, &expedientes.DerivarExpedienteRequest{
		Id: f4Exp.Expediente.Id, AreaDestino: "SUBDIRECTOR",
	})
	if err != nil {
		t.Fatalf("F4 DIRECTOR->SUBDIRECTOR esperaba éxito, obtuvo: %v", err)
	}
	if derivF4.Expediente.AreaActual != "SUBDIRECTOR" {
		t.Fatalf("esperaba area_actual=SUBDIRECTOR tras derivar F4, obtuvo %q", derivF4.Expediente.AreaActual)
	}

	// Genérico (SOLICITUD): sin regla institucional, se mantiene permisivo
	// (comportamiento previo a esta etapa) — no se rompe el registro interno.
	genExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "SOLICITUD", Asunto: "Genérico E2E Etapa 3",
	})
	if err != nil {
		t.Fatalf("crear expediente genérico: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxSecretaria, &expedientes.DerivarExpedienteRequest{
		Id: genExp.Expediente.Id, AreaDestino: "AUXILIAR",
	}); err != nil {
		t.Fatalf("genérico SECRETARIA->AUXILIAR esperaba éxito (sin regla institucional), obtuvo: %v", err)
	}

	// Expediente inexistente.
	if _, err := client.DerivarExpediente(ctxAdmin, &expedientes.DerivarExpedienteRequest{
		Id: "no-existe-" + time.Now().String(), AreaDestino: "DIRECTOR",
	}); status.Code(err) != codes.NotFound {
		t.Fatalf("DerivarExpediente sobre inexistente esperaba NotFound, obtuvo: %v", err)
	}

	// Concurrencia: mismo patrón determinístico que UpdateArea (sección
	// anterior) pero pasando por DerivarExpediente — dos "requests" leen el
	// mismo estado inicial, uno gana, el otro (con lectura vieja) se rechaza.
	concurrF4Exp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "PERMISO", Asunto: "Concurrencia DerivarExpediente",
	})
	if err != nil {
		t.Fatalf("crear expediente para concurrencia de DerivarExpediente: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxSecretaria, &expedientes.DerivarExpedienteRequest{
		Id: concurrF4Exp.Expediente.Id, AreaDestino: "DIRECTOR",
	}); err != nil {
		t.Fatalf("preparar expediente en DIRECTOR: %v", err)
	}
	// "Request A" lee area_actual=DIRECTOR. "Request B" gana y deriva a
	// SUBDIRECTOR primero (llamando al repositorio directamente, mismo
	// patrón ya usado arriba para no depender de una carrera real).
	if _, err := svcCtx.ExpedienteRepository.UpdateArea(ctx, concurrF4Exp.Expediente.Id, "DIRECTOR", "SUBDIRECTOR"); err != nil {
		t.Fatalf("Request B (gana la carrera) esperaba éxito: %v", err)
	}
	// "Request A" intenta derivar con su lectura vieja (área ya no es
	// DIRECTOR) — DerivarExpedienteLogic vuelve a leer el estado actual
	// (SUBDIRECTOR) antes de escribir, así que en la práctica ve
	// directamente que ya no pertenece a DIRECTOR: PermissionDenied, no
	// Aborted (el chequeo de propiedad ocurre antes que la escritura). Esto
	// es correcto: un actor de DIRECTOR ya no tiene autorización sobre un
	// expediente que ahora es de SUBDIRECTOR.
	if _, err := client.DerivarExpediente(ctxDirector, &expedientes.DerivarExpedienteRequest{
		Id: concurrF4Exp.Expediente.Id, AreaDestino: "SUBDIRECTOR",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("Request A (área cambió, ya no es de DIRECTOR) esperaba PermissionDenied, obtuvo: %v", err)
	}

	// --- Etapa 4: cierre real de F2 (ResolverExpediente) ---
	f2CierreExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "CERTIFICADO", Asunto: "Cierre F2 Etapa 4",
	})
	if err != nil {
		t.Fatalf("crear expediente para cierre F2: %v", err)
	}
	// No se puede resolver antes de derivar a Dirección (sigue en SECRETARIA,
	// pero el estado sigue PENDIENTE, no EN_PROCESO).
	if _, err := client.ResolverExpediente(ctxSecretaria, &expedientes.ResolverExpedienteRequest{Id: f2CierreExp.Expediente.Id}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("ResolverExpediente antes de tiempo esperaba FailedPrecondition, obtuvo: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxSecretaria, &expedientes.DerivarExpedienteRequest{
		Id: f2CierreExp.Expediente.Id, AreaDestino: "DIRECTOR",
	}); err != nil {
		t.Fatalf("derivar F2 a Dirección: %v", err)
	}
	// Tampoco se puede resolver mientras está en Dirección, ni por Dirección
	// (no es F2 su rol acá) ni por Secretaría (no es su área en este momento).
	if _, err := client.ResolverExpediente(ctxDirector, &expedientes.ResolverExpedienteRequest{Id: f2CierreExp.Expediente.Id}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("ResolverExpediente desde DIRECTOR esperaba FailedPrecondition, obtuvo: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxDirector, &expedientes.DerivarExpedienteRequest{
		Id: f2CierreExp.Expediente.Id, AreaDestino: "SECRETARIA",
	}); err != nil {
		t.Fatalf("Dirección devuelve F2 a Secretaría: %v", err)
	}
	// DOCENTE no puede resolver (no es su área ni su rol para esto).
	if _, err := client.ResolverExpediente(ctxDocente, &expedientes.ResolverExpedienteRequest{Id: f2CierreExp.Expediente.Id}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("ResolverExpediente por DOCENTE esperaba PermissionDenied, obtuvo: %v", err)
	}
	resuelto, err := client.ResolverExpediente(ctxSecretaria, &expedientes.ResolverExpedienteRequest{Id: f2CierreExp.Expediente.Id})
	if err != nil {
		t.Fatalf("SECRETARIA resuelve F2 esperaba éxito, obtuvo: %v", err)
	}
	if resuelto.Expediente.Estado != "ATENDIDO" {
		t.Fatalf("esperaba estado=ATENDIDO tras resolver F2, obtuvo %q", resuelto.Expediente.Estado)
	}
	// Resolver un F4 debe rechazarse (ResolverExpediente es exclusivo de F2).
	f4NoAplicaExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "PERMISO", Asunto: "Resolver no aplica a F4",
	})
	if err != nil {
		t.Fatalf("crear expediente F4 para el caso 'no aplica': %v", err)
	}
	if _, err := client.ResolverExpediente(ctxSecretaria, &expedientes.ResolverExpedienteRequest{Id: f4NoAplicaExp.Expediente.Id}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("ResolverExpediente sobre F4 esperaba FailedPrecondition, obtuvo: %v", err)
	}

	// --- Etapa 4: rechazo y corrección de F4 ---
	f4RechazoExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "JUSTIFICACION_TARDANZA", Asunto: "Rechazo F4 Etapa 4",
	})
	if err != nil {
		t.Fatalf("crear expediente para rechazo F4: %v", err)
	}
	// No se puede rechazar un F2 (RechazarExpediente es exclusivo de F4).
	if _, err := client.RechazarExpediente(ctxSecretaria, &expedientes.RechazarExpedienteRequest{Id: f2CierreExp.Expediente.Id, Motivo: "no aplica"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("RechazarExpediente sobre F2 esperaba FailedPrecondition, obtuvo: %v", err)
	}
	// Tampoco se puede rechazar mientras sigue en SECRETARIA (todavía PENDIENTE, no EN_PROCESO).
	if _, err := client.RechazarExpediente(ctxSecretaria, &expedientes.RechazarExpedienteRequest{Id: f4RechazoExp.Expediente.Id, Motivo: "muy pronto"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("RechazarExpediente antes de tiempo esperaba FailedPrecondition, obtuvo: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxSecretaria, &expedientes.DerivarExpedienteRequest{
		Id: f4RechazoExp.Expediente.Id, AreaDestino: "DIRECTOR",
	}); err != nil {
		t.Fatalf("derivar F4 a Dirección: %v", err)
	}
	// SUBDIRECTOR no puede rechazar lo que está en DIRECTOR (no es su área).
	if _, err := client.RechazarExpediente(ctxSubdirector, &expedientes.RechazarExpedienteRequest{Id: f4RechazoExp.Expediente.Id, Motivo: "no me corresponde"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("RechazarExpediente por SUBDIRECTOR (área ajena) esperaba PermissionDenied, obtuvo: %v", err)
	}
	rechazado, err := client.RechazarExpediente(ctxDirector, &expedientes.RechazarExpedienteRequest{
		Id: f4RechazoExp.Expediente.Id, Motivo: "Falta sustento médico",
	})
	if err != nil {
		t.Fatalf("DIRECTOR rechaza F4 esperaba éxito, obtuvo: %v", err)
	}
	if rechazado.Expediente.Estado != "OBSERVADO" || rechazado.Expediente.AreaActual != "SECRETARIA" {
		t.Fatalf("esperaba estado=OBSERVADO y area=SECRETARIA tras rechazar, obtuvo estado=%q area=%q",
			rechazado.Expediente.Estado, rechazado.Expediente.AreaActual)
	}
	// Doble rechazo: ya no está EN_PROCESO (está OBSERVADO), debe rechazarse.
	if _, err := client.RechazarExpediente(ctxSecretaria, &expedientes.RechazarExpedienteRequest{Id: f4RechazoExp.Expediente.Id, Motivo: "otra vez"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("doble rechazo esperaba FailedPrecondition, obtuvo: %v", err)
	}

	// Otro solicitante no puede corregir el expediente de e2eaud_sol.
	if _, err := client.CorregirExpediente(ctxOtroSolicitante, &expedientes.CorregirExpedienteRequest{
		Id: f4RechazoExp.Expediente.Id, Descripcion: "intento ajeno",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("CorregirExpediente por otro solicitante esperaba PermissionDenied, obtuvo: %v", err)
	}
	// Personal interno tampoco puede "corregir" en nombre del solicitante.
	if _, err := client.CorregirExpediente(ctxAdmin, &expedientes.CorregirExpedienteRequest{
		Id: f4RechazoExp.Expediente.Id, Descripcion: "intento admin",
	}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("CorregirExpediente por ADMIN esperaba PermissionDenied, obtuvo: %v", err)
	}
	corregido, err := client.CorregirExpediente(ctxSolicitante, &expedientes.CorregirExpedienteRequest{
		Id: f4RechazoExp.Expediente.Id, Descripcion: "Adjunto el sustento médico solicitado",
	})
	if err != nil {
		t.Fatalf("solicitante corrige y reenvía esperaba éxito, obtuvo: %v", err)
	}
	if corregido.Expediente.Estado != "PENDIENTE" {
		t.Fatalf("esperaba estado=PENDIENTE tras corregir, obtuvo %q", corregido.Expediente.Estado)
	}
	if corregido.Expediente.Descripcion != "Adjunto el sustento médico solicitado" {
		t.Fatalf("esperaba la descripción corregida guardada, obtuvo %q", corregido.Expediente.Descripcion)
	}
	// Ya no está OBSERVADO: una segunda corrección debe rechazarse.
	if _, err := client.CorregirExpediente(ctxSolicitante, &expedientes.CorregirExpedienteRequest{
		Id: f4RechazoExp.Expediente.Id, Descripcion: "otra vez",
	}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("segunda corrección esperaba FailedPrecondition, obtuvo: %v", err)
	}

	// --- Etapa 4: ruta completa de F4 aprobada hasta Docente (cierre) ---
	f4CompletoExp, err := client.CreateExpediente(ctxSolicitante, &expedientes.CreateExpedienteRequest{
		Tipo: "PERMISO", Asunto: "F4 completo hasta Docente",
	})
	if err != nil {
		t.Fatalf("crear expediente F4 completo: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxSecretaria, &expedientes.DerivarExpedienteRequest{
		Id: f4CompletoExp.Expediente.Id, AreaDestino: "DIRECTOR",
	}); err != nil {
		t.Fatalf("F4 completo: Secretaría->Dirección: %v", err)
	}
	if _, err := client.DerivarExpediente(ctxDirector, &expedientes.DerivarExpedienteRequest{
		Id: f4CompletoExp.Expediente.Id, AreaDestino: "SUBDIRECTOR",
	}); err != nil {
		t.Fatalf("F4 completo: Dirección->Subdirección: %v", err)
	}
	finalF4, err := client.DerivarExpediente(ctxSubdirector, &expedientes.DerivarExpedienteRequest{
		Id: f4CompletoExp.Expediente.Id, AreaDestino: "DOCENTE",
	})
	if err != nil {
		t.Fatalf("F4 completo: Subdirección->Docente esperaba éxito, obtuvo: %v", err)
	}
	if finalF4.Expediente.AreaActual != "DOCENTE" {
		t.Fatalf("esperaba area_actual=DOCENTE, obtuvo %q", finalF4.Expediente.AreaActual)
	}
	if finalF4.Expediente.Estado != "ATENDIDO" {
		t.Fatalf("esperaba estado=ATENDIDO al llegar a Docente (destino final), obtuvo %q", finalF4.Expediente.Estado)
	}
}

func authContext(userID, role string) context.Context {
	claims := security.Claims{
		Username: "integration-test",
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

// fixedUUID produce un UUID v4-like determinístico a partir del testID y un
// índice, únicamente para tener identificadores estables y DISTINTOS dentro
// de una misma corrida de test (no representa usuarios reales de Auth
// Service). Se acota explícitamente para no desbordar el campo de 12 dígitos
// hex del último grupo del UUID.
func fixedUUID(testID int64, index int) string {
	value := (testID%10_000_000_000)*10 + int64(index)
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", value)
}

func integrationDatabaseURL() string {
	if value := os.Getenv("EXPEDIENTES_TEST_DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://expedientes_user:expedientes_password@127.0.0.1:5433/expedientes_db?sslmode=disable"
}

func cleanupIntegrationData(t *testing.T, pool *pgxpool.Pool, solicitanteIDs ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, id := range solicitanteIDs {
		if _, err := pool.Exec(ctx, "DELETE FROM expedientes WHERE solicitante_id = $1", id); err != nil {
			t.Logf("cleanup expedientes for %s: %v", id, err)
		}
	}
}
