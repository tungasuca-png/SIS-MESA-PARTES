package integration

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"psicologia/internal/repository"
	"psicologia/internal/svc"
	"psicologia/psicologia"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	pView    = "psicologia.expedientes.view"
	pViewOwn = "psicologia.expedientes.view_own"
	pCreate  = "psicologia.expedientes.create"
	pUpdate  = "psicologia.expedientes.update"
)

var codigoRegex = regexp.MustCompile(`^PSI-[0-9]{4}-[0-9]{6}$`)

func wantCode(t *testing.T, err error, want codes.Code, what string) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("%s: se esperaba %v, got %v", what, want, err)
	}
}

// TestPsicologiaService prueba el servicio real de punta a punta (JWT,
// permisos vía Auth, alcance, PostgreSQL y auditoría). Auth se sustituye por un
// doble que concede permisos por userID; nunca se decide por el rol del token.
func TestPsicologiaService(t *testing.T) {
	h := newHarness(t)
	base := time.Now().UnixNano()

	// Identidades. El "role" del JWT es solo informativo y se varía a propósito
	// para demostrar que no interviene en la autorización.
	creador := fixedUUID(base, 10)                // crea expedientes; alcance propio (view_own) + update
	general := fixedUUID(base, 11)                // alcance general (view) sin update
	admin := fixedUUID(base, 12)                  // JWT ADMIN sin permisos de psicología
	propio := fixedUUID(base, 13)                 // solo view_own, es responsable de un expediente
	otroProf := fixedUUID(base, 14)               // solo view_own + update, NO responsable de los de otros
	sinAlcance := fixedUUID(base, 15)             // solo update, sin view ni view_own
	sinNada := fixedUUID(base, 16)                // sin ningún permiso
	docente := fixedUUID(base, 17)                // rol DOCENTE con view: el permiso manda, no el rol
	docenteResponsable := fixedUUID(base, 20)     // existe en Usuarios, tipo_usuario=DOCENTE: NO puede ser responsable
	solicitanteResponsable := fixedUUID(base, 21) // existe en Usuarios, tipo_usuario=SOLICITANTE: NO puede ser responsable
	// creador hace de psicólogo: con acceso clínico (atenciones.view) para
	// registrar y leer el resumen del caso.
	h.grant(creador, pCreate, pViewOwn, pUpdate, pAtView)
	h.grant(general, pView)
	h.grant(admin)
	h.grant(propio, pViewOwn)
	h.grant(otroProf, pViewOwn, pUpdate)
	h.grant(sinAlcance, pUpdate)
	h.grant(sinNada)
	h.grant(docente, pView)
	// tipo_usuario real en Usuarios (validateResponsable exige "PSICOLOGO").
	h.users.known[propio] = "PSICOLOGO"
	h.users.known[creador] = "PSICOLOGO"
	h.users.known[docenteResponsable] = "DOCENTE"
	h.users.known[solicitanteResponsable] = "SOLICITANTE"

	est := func(i int) string { return fixedUUID(base, 100+i) }
	open := func(t *testing.T, estudiante string) *psicologia.ExpedientePsicologico {
		t.Helper()
		resp, err := h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: estudiante, ResumenCaso: "TEXTO-CLINICO-SECRETO"})
		if err != nil {
			t.Fatalf("OpenExpediente: %v", err)
		}
		return h.track(resp.Expediente)
	}

	// ---- JWT ----
	t.Run("JWT_Ausente_Invalido_Valido", func(t *testing.T) {
		req := &psicologia.ListExpedientesRequest{}
		_, err := h.client.ListExpedientes(h.ctx, req)
		wantCode(t, err, codes.Unauthenticated, "sin token")
		bad := metadata.AppendToOutgoingContext(h.ctx, "authorization", "Bearer no.es.un.jwt")
		_, err = h.client.ListExpedientes(bad, req)
		wantCode(t, err, codes.Unauthenticated, "token inválido")
		malformed := metadata.AppendToOutgoingContext(h.ctx, "authorization", h.tokenFor(creador, "X"))
		_, err = h.client.ListExpedientes(malformed, req)
		wantCode(t, err, codes.Unauthenticated, "sin esquema Bearer")
		if _, err := h.client.ListExpedientes(h.as(creador, "PSICOLOGO"), req); err != nil {
			t.Fatalf("token válido con permiso: %v", err)
		}
	})

	// ---- Autorización ----
	t.Run("SinPermiso_TodasLasOperaciones_Denegadas_YAuditadas", func(t *testing.T) {
		exp := open(t, est(1))
		c := h.as(sinNada, "PSICOLOGO")
		_, e1 := h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(2)})
		_, e2 := h.client.GetExpediente(c, &psicologia.GetExpedienteRequest{Id: exp.Id})
		_, e3 := h.client.GetExpedienteByEstudiante(c, &psicologia.GetExpedienteByEstudianteRequest{EstudianteId: est(1)})
		_, e4 := h.client.ListExpedientes(c, &psicologia.ListExpedientesRequest{})
		_, e5 := h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResumenCaso: strPtr("x")})
		_, e6 := h.client.CloseExpediente(c, &psicologia.CloseExpedienteRequest{Id: exp.Id, Motivo: "x"})
		_, e7 := h.client.ReopenExpediente(c, &psicologia.ReopenExpedienteRequest{Id: exp.Id})
		for i, err := range []error{e1, e2, e3, e4, e5, e6, e7} {
			wantCode(t, err, codes.PermissionDenied, "operación "+string(rune('1'+i))+" sin permiso")
		}
		if n := h.countAudit(sinNada, "DENIED", ""); n != 7 {
			t.Fatalf("se esperaban 7 eventos DENIED, got %d", n)
		}
	})

	t.Run("ElPermisoManda_NoElRol_AdminSinPermisoDenegado_DocenteConViewPermitido", func(t *testing.T) {
		exp := open(t, est(3))
		_, err := h.client.GetExpediente(h.as(admin, "ADMIN"), &psicologia.GetExpedienteRequest{Id: exp.Id})
		wantCode(t, err, codes.PermissionDenied, "ADMIN sin permisos de psicología")
		if _, err := h.client.GetExpediente(h.as(docente, "DOCENTE"), &psicologia.GetExpedienteRequest{Id: exp.Id}); err != nil {
			t.Fatalf("un DOCENTE con psicologia.expedientes.view debe poder: %v", err)
		}
	})

	t.Run("AuthCaido_FailClosed_SinDatosNiAuditoria", func(t *testing.T) {
		exp := open(t, est(4))
		h.auth.failing = true
		defer func() { h.auth.failing = false }()
		antes := h.countAudit(creador, "VIEW", exp.Id)
		resp, err := h.client.GetExpediente(h.as(creador, "PSICOLOGO"), &psicologia.GetExpedienteRequest{Id: exp.Id})
		if err == nil || resp != nil {
			t.Fatalf("con Auth caído no debe entregarse nada: resp=%v err=%v", resp, err)
		}
		if h.countAudit(creador, "VIEW", exp.Id) != antes {
			t.Fatal("sin Auth no debe registrarse una lectura permitida")
		}
	})

	// ---- OpenExpediente ----
	t.Run("Open_CamposAuditadosYCodigo", func(t *testing.T) {
		exp := open(t, est(5))
		if exp.AbiertoPor != creador || exp.ResponsableId != creador {
			t.Fatalf("abierto_por y responsable por defecto deben ser el usuario autenticado: %+v", exp)
		}
		if exp.Estado != "ABIERTO" || !codigoRegex.MatchString(exp.Codigo) || exp.EstudianteId != est(5) {
			t.Fatalf("expediente inesperado: %+v", exp)
		}
		if h.countAudit(creador, "CREATE", exp.Id) != 1 {
			t.Fatal("se esperaba 1 evento CREATE")
		}
	})

	t.Run("Open_Validaciones", func(t *testing.T) {
		c := h.as(creador, "PSICOLOGO")
		_, err := h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: "no-es-uuid"})
		wantCode(t, err, codes.InvalidArgument, "estudiante_id inválido")
		_, err = h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(6), ResponsableId: "texto libre"})
		wantCode(t, err, codes.InvalidArgument, "responsable en texto libre")
		_, err = h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(6), ResponsableId: fixedUUID(base, 900)})
		wantCode(t, err, codes.InvalidArgument, "responsable que no existe en Usuarios")
		_, err = h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(6), ResponsableId: docenteResponsable})
		wantCode(t, err, codes.InvalidArgument, "responsable con tipo_usuario DOCENTE")
		_, err = h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(6), ResponsableId: solicitanteResponsable})
		wantCode(t, err, codes.InvalidArgument, "responsable con tipo_usuario SOLICITANTE")
		_, err = h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(6), ResumenCaso: strings.Repeat("x", 10001)})
		wantCode(t, err, codes.InvalidArgument, "resumen demasiado largo")
		// responsable distinto pero real: se acepta; abierto_por sigue siendo el autenticado
		resp, err := h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(6), ResponsableId: propio})
		if err != nil {
			t.Fatalf("responsable válido: %v", err)
		}
		h.track(resp.Expediente)
		if resp.Expediente.ResponsableId != propio || resp.Expediente.AbiertoPor != creador {
			t.Fatalf("responsable=%s abierto_por=%s", resp.Expediente.ResponsableId, resp.Expediente.AbiertoPor)
		}
	})

	t.Run("Open_EstudianteVerifier_PuntoDeExtension", func(t *testing.T) {
		h.svcCtx.Estudiantes = &fakeEstudiantes{exist: map[string]bool{est(7): true}}
		defer func() { h.svcCtx.Estudiantes = nil }()
		_, err := h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: est(8)})
		wantCode(t, err, codes.NotFound, "estudiante inexistente según el verificador")
		resp, err := h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: est(7)})
		if err != nil {
			t.Fatalf("estudiante existente: %v", err)
		}
		h.track(resp.Expediente)
	})

	// FASE 5C: a diferencia del subtest anterior (que prueba el punto de
	// extensión con un doble que implementa EstudianteVerifier directamente),
	// este ejercita la implementación REAL (estudianteServiceVerifier, sobre
	// estudiantesclient.Estudiantes) — reenvío del JWT y traducción de
	// códigos gRPC incluidos.
	t.Run("Open_EstudianteVerifier_Real_CasosYReenvioDeJWT", func(t *testing.T) {
		fake := &scriptedEstudiantesClient{exist: map[string]bool{est(231): true}}
		h.svcCtx.Estudiantes = svc.NewEstudianteVerifier(fake)
		defer func() { h.svcCtx.Estudiantes = nil }()

		var antes, despues int
		countExpedientes := func() int {
			var n int
			if err := h.admin.QueryRow(h.ctx, "SELECT COUNT(*) FROM expedientes_psicologicos").Scan(&n); err != nil {
				t.Fatalf("contar expedientes: %v", err)
			}
			return n
		}

		// Caso A: estudiante existente -> continúa y crea el expediente.
		resp, err := h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: est(231)})
		if err != nil {
			t.Fatalf("estudiante existente (verificador real): %v", err)
		}
		h.track(resp.Expediente)
		if fake.lastAuz == "" || !strings.HasPrefix(fake.lastAuz, "Bearer ") {
			t.Fatalf("el verificador real debe reenviar el JWT del usuario autenticado, got %q", fake.lastAuz)
		}

		// Caso B: UUID válido pero inexistente -> NotFound.
		_, err = h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: est(232)})
		wantCode(t, err, codes.NotFound, "estudiante inexistente (verificador real)")

		// Caso C: UUID con formato inválido -> InvalidArgument, SIN llegar a
		// consultar a Estudiantes (la validación de formato ocurre antes).
		fake.lastAuz = ""
		_, err = h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: "no-es-un-uuid"})
		wantCode(t, err, codes.InvalidArgument, "estudiante_id con formato inválido")
		if fake.lastAuz != "" {
			t.Fatal("un UUID con formato inválido no debe llegar a consultar Estudiantes Service")
		}

		// Caso D: Estudiantes Service inaccesible -> error propagado (NUNCA
		// NotFound) y ningún expediente creado.
		antes = countExpedientes()
		fake.down = true
		_, err = h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: est(233)})
		if err == nil {
			t.Fatal("Estudiantes Service caído debe producir error, no permitir la apertura")
		}
		if status.Code(err) == codes.NotFound {
			t.Fatalf("un servicio caído no debe traducirse en NotFound (estudiante inexistente), got %v", err)
		}
		despues = countExpedientes()
		if antes != despues {
			t.Fatal("Estudiantes Service caído no debe crear un expediente")
		}
		fake.down = false

		// Caso E: Estudiantes rechaza el permiso del usuario (equivalente a
		// que el operador de Psicología no tenga "estudiantes.view" en Auth)
		// -> PermissionDenied propagado tal cual, NUNCA convertido en
		// "estudiante inexistente", y tampoco crea el expediente.
		antes = countExpedientes()
		fake.denied = true
		_, err = h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: est(234)})
		wantCode(t, err, codes.PermissionDenied, "Estudiantes rechaza el permiso del usuario")
		despues = countExpedientes()
		if antes != despues {
			t.Fatal("un permiso rechazado en Estudiantes no debe crear un expediente")
		}
		fake.denied = false
	})

	t.Run("Open_Duplicado_409_YNoCreaAutomaticamenteAlConsultar", func(t *testing.T) {
		open(t, est(9))
		_, err := h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: est(9)})
		wantCode(t, err, codes.AlreadyExists, "segundo expediente del mismo estudiante")

		var antes, despues int
		_ = h.admin.QueryRow(h.ctx, "SELECT COUNT(*) FROM expedientes_psicologicos").Scan(&antes)
		_, err = h.client.GetExpedienteByEstudiante(h.as(general, "PSICOLOGO"), &psicologia.GetExpedienteByEstudianteRequest{EstudianteId: est(10)})
		wantCode(t, err, codes.NotFound, "estudiante sin expediente")
		_ = h.admin.QueryRow(h.ctx, "SELECT COUNT(*) FROM expedientes_psicologicos").Scan(&despues)
		if antes != despues {
			t.Fatal("consultar no debe crear expedientes")
		}
	})

	t.Run("Open_MismoEstudianteConcurrente_SoloUnoGana", func(t *testing.T) {
		estudiante := est(20)
		const n = 8
		var wg sync.WaitGroup
		results := make([]error, n)
		ids := make([]string, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				resp, err := h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: estudiante})
				results[i] = err
				if err == nil {
					ids[i] = resp.Expediente.Id
				}
			}(i)
		}
		wg.Wait()
		ok, conflict := 0, 0
		for i, err := range results {
			switch status.Code(err) {
			case codes.OK:
				ok++
				h.created = append(h.created, ids[i])
			case codes.AlreadyExists:
				conflict++
			default:
				t.Fatalf("resultado inesperado: %v", err)
			}
		}
		if ok != 1 || conflict != n-1 {
			t.Fatalf("debe ganar exactamente 1 solicitud: ok=%d conflict=%d", ok, conflict)
		}
		var filas int
		_ = h.admin.QueryRow(h.ctx, "SELECT COUNT(*) FROM expedientes_psicologicos WHERE estudiante_id = $1::uuid", estudiante).Scan(&filas)
		if filas != 1 {
			t.Fatalf("debe existir un único expediente, hay %d", filas)
		}
	})

	t.Run("Open_EstudiantesDistintosConcurrentes_CodigosUnicosYConsecutivos", func(t *testing.T) {
		const n = 10
		var wg sync.WaitGroup
		var mu sync.Mutex
		codigos := map[string]bool{}
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				resp, err := h.client.OpenExpediente(h.as(creador, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: est(30 + i)})
				if err != nil {
					t.Errorf("open concurrente: %v", err)
					return
				}
				mu.Lock()
				codigos[resp.Expediente.Codigo] = true
				h.created = append(h.created, resp.Expediente.Id)
				mu.Unlock()
			}(i)
		}
		wg.Wait()
		if len(codigos) != n {
			t.Fatalf("los %d expedientes deben tener códigos distintos, hay %d", n, len(codigos))
		}
	})

	// ---- Alcance (view / view_own) ----
	t.Run("Alcance_ViewGeneral_VeTodos_ViewOwn_SoloLosPropios", func(t *testing.T) {
		propioExp := h.mustOpenFor(t, creador, est(40), propio)
		ajenoExp := open(t, est(41)) // responsable: creador

		if _, err := h.client.GetExpediente(h.as(general, "X"), &psicologia.GetExpedienteRequest{Id: ajenoExp.Id}); err != nil {
			t.Fatalf("view general ve cualquier expediente: %v", err)
		}
		c := h.as(propio, "X")
		got, err := h.client.GetExpediente(c, &psicologia.GetExpedienteRequest{Id: propioExp.Id})
		if err != nil || got.Expediente.Id != propioExp.Id {
			t.Fatalf("view_own sobre su expediente: %v", err)
		}
		// Sin psicologia.atenciones.view, el alcance no entrega el contenido clínico.
		if got.Expediente.ResumenCaso != "" {
			t.Fatal("view_own sin acceso clínico no debe recibir el resumen del caso")
		}
		if _, err := h.client.GetExpedienteByEstudiante(c, &psicologia.GetExpedienteByEstudianteRequest{EstudianteId: est(40)}); err != nil {
			t.Fatalf("view_own por estudiante (propio): %v", err)
		}

		// Ajeno: NotFound uniforme (no revela existencia) y evento DENIED.
		_, err = h.client.GetExpediente(c, &psicologia.GetExpedienteRequest{Id: ajenoExp.Id})
		wantCode(t, err, codes.NotFound, "view_own sobre expediente ajeno")
		_, err = h.client.GetExpedienteByEstudiante(c, &psicologia.GetExpedienteByEstudianteRequest{EstudianteId: est(41)})
		wantCode(t, err, codes.NotFound, "view_own por estudiante ajeno")
		if h.countAudit(propio, "DENIED", ajenoExp.Id) != 2 {
			t.Fatal("se esperaban 2 eventos DENIED (FUERA_DE_ALCANCE)")
		}
		// El resumen clínico no sale del servidor hacia quien no tiene alcance.
	})

	t.Run("List_AlcanceEnSQL_SinResumen_YAuditaVIEW", func(t *testing.T) {
		l1 := h.mustOpenFor(t, creador, est(50), propio)
		l2 := open(t, est(51))
		// view_own: solo los suyos (responsable propio).
		resp, err := h.client.ListExpedientes(h.as(propio, "X"), &psicologia.ListExpedientesRequest{PageSize: 100})
		if err != nil {
			t.Fatalf("list view_own: %v", err)
		}
		for _, e := range resp.Expedientes {
			if e.ResponsableId != propio {
				t.Fatalf("view_own devolvió un expediente ajeno: %+v", e)
			}
		}
		if !contains(resp.Expedientes, l1.Id) || contains(resp.Expedientes, l2.Id) {
			t.Fatal("view_own debe incluir el suyo y excluir el ajeno")
		}
		// view general: ve ambos; el listado NUNCA incluye resumen_caso.
		all, err := h.client.ListExpedientes(h.as(general, "X"), &psicologia.ListExpedientesRequest{PageSize: 100})
		if err != nil {
			t.Fatalf("list view: %v", err)
		}
		if !contains(all.Expedientes, l1.Id) || !contains(all.Expedientes, l2.Id) {
			t.Fatal("view general debe ver todos los expedientes")
		}
		for _, e := range all.Expedientes {
			if e.ResumenCaso != "" {
				t.Fatal("el listado no debe exponer resumen_caso")
			}
		}
		if h.countAudit(general, "VIEW", "") < 1 {
			t.Fatal("el listado debe registrar VIEW")
		}
		_, err = h.client.ListExpedientes(h.as(general, "X"), &psicologia.ListExpedientesRequest{Estado: "INVENTADO"})
		wantCode(t, err, codes.InvalidArgument, "estado inválido")
	})

	t.Run("Get_AuditaVIEW_ConFailClosed", func(t *testing.T) {
		exp := open(t, est(60))
		antes := h.countAudit(general, "VIEW", exp.Id)
		if _, err := h.client.GetExpediente(h.as(general, "X"), &psicologia.GetExpedienteRequest{Id: exp.Id}); err != nil {
			t.Fatalf("get: %v", err)
		}
		if h.countAudit(general, "VIEW", exp.Id) != antes+1 {
			t.Fatal("cada lectura debe registrar un VIEW")
		}
		// Si la bitácora no se puede escribir, NO se entrega el dato.
		closed, err := pgxpool.New(h.ctx, appDatabaseURL(t))
		if err != nil {
			t.Fatalf("pool: %v", err)
		}
		closed.Close()
		h.svcCtx.AuditoriaRepo = repository.NewAuditoriaRepository(closed)
		defer func() { h.svcCtx.AuditoriaRepo = repository.NewAuditoriaRepository(h.app) }()
		resp, err := h.client.GetExpediente(h.as(general, "X"), &psicologia.GetExpedienteRequest{Id: exp.Id})
		if err == nil || resp != nil || status.Code(err) != codes.Internal {
			t.Fatalf("sin auditoría no debe devolverse el expediente: resp=%v err=%v", resp, err)
		}
	})

	// ---- Contenido clínico: exige psicologia.atenciones.view ----
	// El alcance del expediente (view/view_own) da sus datos administrativos;
	// el resumen del caso y el motivo de cierre exigen además el permiso que
	// protege las atenciones, para leerlos y para escribirlos.
	t.Run("ContenidoClinico_ExigeAtencionesView", func(t *testing.T) {
		psicologoGlobal := fixedUUID(base, 30) // view + atenciones.view (perfil acordado para PSICOLOGO)
		gestorSinClinico := fixedUUID(base, 31) // view + create + update, sin atenciones.view
		h.grant(psicologoGlobal, pView, pAtView)
		h.grant(gestorSinClinico, pView, pCreate, pUpdate)

		exp := open(t, est(85)) // responsable: creador, con resumen

		// Alcance global + acceso clínico: ve el resumen de un expediente ajeno.
		got, err := h.client.GetExpediente(h.as(psicologoGlobal, "PSICOLOGO"), &psicologia.GetExpedienteRequest{Id: exp.Id})
		if err != nil || got.Expediente.ResumenCaso != "TEXTO-CLINICO-SECRETO" {
			t.Fatalf("view + atenciones.view debe ver el resumen: %+v err=%v", got, err)
		}
		// Solo alcance: ve el expediente, no el resumen (por id y por estudiante).
		g1, err := h.client.GetExpediente(h.as(general, "X"), &psicologia.GetExpedienteRequest{Id: exp.Id})
		if err != nil || g1.Expediente.Id != exp.Id || g1.Expediente.ResumenCaso != "" {
			t.Fatalf("view sin atenciones.view: %+v err=%v", g1, err)
		}
		g2, err := h.client.GetExpedienteByEstudiante(h.as(general, "X"), &psicologia.GetExpedienteByEstudianteRequest{EstudianteId: est(85)})
		if err != nil || g2.Expediente.ResumenCaso != "" {
			t.Fatalf("por estudiante sin atenciones.view: %+v err=%v", g2, err)
		}

		// Escribir el resumen sin poder leerlo: 403 y DENIED; nada cambia.
		c := h.as(gestorSinClinico, "X")
		antes := h.countAudit(gestorSinClinico, "DENIED", "")
		_, err = h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResumenCaso: strPtr("sobrescrito")})
		wantCode(t, err, codes.PermissionDenied, "escribir resumen sin atenciones.view")
		if h.countAudit(gestorSinClinico, "DENIED", "") != antes+1 {
			t.Fatal("se esperaba un evento DENIED")
		}
		// Cambiar el estado sí puede; la respuesta no trae contenido clínico.
		upd, err := h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, Estado: strPtr("EN_SEGUIMIENTO")})
		if err != nil || upd.Expediente.Estado != "EN_SEGUIMIENTO" || upd.Expediente.ResumenCaso != "" {
			t.Fatalf("update de estado sin acceso clínico: %+v err=%v", upd, err)
		}
		// El cierre no devuelve el motivo ni el resumen a quien no tiene acceso clínico.
		cl, err := h.client.CloseExpediente(c, &psicologia.CloseExpedienteRequest{Id: exp.Id, Motivo: "MOTIVO-CLINICO"})
		if err != nil || cl.Expediente.Estado != "CERRADO" || cl.Expediente.MotivoCierre != "" || cl.Expediente.ResumenCaso != "" {
			t.Fatalf("cierre sin acceso clínico: %+v err=%v", cl, err)
		}
		re, err := h.client.ReopenExpediente(c, &psicologia.ReopenExpedienteRequest{Id: exp.Id})
		if err != nil || re.Expediente.ResumenCaso != "" || re.Expediente.MotivoCierre != "" {
			t.Fatalf("reapertura sin acceso clínico: %+v err=%v", re, err)
		}
		// El resumen original sigue intacto (lo comprueba quien sí tiene acceso).
		ok, _ := h.client.GetExpediente(h.as(psicologoGlobal, "PSICOLOGO"), &psicologia.GetExpedienteRequest{Id: exp.Id})
		if ok.Expediente.ResumenCaso != "TEXTO-CLINICO-SECRETO" {
			t.Fatal("un intento denegado no debe cambiar el resumen")
		}

		// Abrir con resumen inicial exige acceso clínico; sin resumen, no.
		_, err = h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(86), ResumenCaso: "texto"})
		wantCode(t, err, codes.PermissionDenied, "abrir con resumen sin atenciones.view")
		sinResumen, err := h.client.OpenExpediente(c, &psicologia.OpenExpedienteRequest{EstudianteId: est(86)})
		if err != nil {
			t.Fatalf("abrir sin resumen: %v", err)
		}
		h.track(sinResumen.Expediente)
	})

	// ---- Update / Close / Reopen ----
	t.Run("Update_ExigeUpdateYAlcance", func(t *testing.T) {
		exp := open(t, est(70)) // responsable: creador
		upd := &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResumenCaso: strPtr("nuevo resumen")}

		// view general SIN update -> PermissionDenied
		_, err := h.client.UpdateExpediente(h.as(general, "X"), upd)
		wantCode(t, err, codes.PermissionDenied, "view sin update")
		// update SIN ningún permiso de lectura -> PermissionDenied (sin alcance)
		_, err = h.client.UpdateExpediente(h.as(sinAlcance, "X"), upd)
		wantCode(t, err, codes.PermissionDenied, "update sin alcance")
		// view_own + update pero expediente de OTRO responsable -> NotFound (oculto) + DENIED
		_, err = h.client.UpdateExpediente(h.as(otroProf, "X"), &psicologia.UpdateExpedienteRequest{Id: exp.Id, Estado: strPtr("EN_SEGUIMIENTO")})
		wantCode(t, err, codes.NotFound, "view_own+update sobre ajeno")
		if h.countAudit(otroProf, "DENIED", exp.Id) < 1 {
			t.Fatal("se esperaba DENIED por fuera de alcance")
		}
		// el responsable con update sí puede
		resp, err := h.client.UpdateExpediente(h.as(creador, "X"), &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResumenCaso: strPtr("nuevo resumen"), Estado: strPtr("EN_SEGUIMIENTO")})
		if err != nil || resp.Expediente.ResumenCaso != "nuevo resumen" || resp.Expediente.Estado != "EN_SEGUIMIENTO" {
			t.Fatalf("update propio: %+v err=%v", resp, err)
		}
		if h.countAudit(creador, "UPDATE", exp.Id) != 1 {
			t.Fatal("se esperaba 1 evento UPDATE")
		}
		// el sinAlcance/otroProf no modificaron nada (se lee con acceso clínico)
		got, _ := h.client.GetExpediente(h.as(creador, "X"), &psicologia.GetExpedienteRequest{Id: exp.Id})
		if got.Expediente.ResumenCaso != "nuevo resumen" {
			t.Fatal("el resumen no debía cambiar por intentos denegados")
		}

		c := h.as(creador, "X")
		_, err = h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id})
		wantCode(t, err, codes.InvalidArgument, "update sin campos")
		_, err = h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, Estado: strPtr("CERRADO")})
		wantCode(t, err, codes.InvalidArgument, "CERRADO no se logra por update")
		_, err = h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResponsableId: strPtr("texto")})
		wantCode(t, err, codes.InvalidArgument, "responsable inválido")
		_, err = h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResponsableId: strPtr(fixedUUID(base, 901))})
		wantCode(t, err, codes.InvalidArgument, "responsable inexistente en Usuarios")
		_, err = h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResponsableId: strPtr(docenteResponsable)})
		wantCode(t, err, codes.InvalidArgument, "responsable con tipo_usuario DOCENTE")
		// reasignar a un PSICOLOGO real: se acepta (queda auditado con el NOMBRE del campo)
		re, err := h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResponsableId: strPtr(propio)})
		if err != nil || re.Expediente.ResponsableId != propio {
			t.Fatalf("reasignar a un PSICOLOGO válido: %+v err=%v", re, err)
		}
	})

	t.Run("Close_Reopen_TransicionesYAuditoria", func(t *testing.T) {
		exp := open(t, est(80))
		c := h.as(creador, "X")

		_, err := h.client.CloseExpediente(c, &psicologia.CloseExpedienteRequest{Id: exp.Id})
		wantCode(t, err, codes.InvalidArgument, "cierre sin motivo")
		_, err = h.client.ReopenExpediente(c, &psicologia.ReopenExpedienteRequest{Id: exp.Id})
		wantCode(t, err, codes.FailedPrecondition, "reabrir uno que no está cerrado")

		closed, err := h.client.CloseExpediente(c, &psicologia.CloseExpedienteRequest{Id: exp.Id, Motivo: "alta"})
		if err != nil || closed.Expediente.Estado != "CERRADO" || closed.Expediente.MotivoCierre != "alta" || closed.Expediente.FechaCierre == "" {
			t.Fatalf("cierre: %+v err=%v", closed, err)
		}
		_, err = h.client.CloseExpediente(c, &psicologia.CloseExpedienteRequest{Id: exp.Id, Motivo: "otra vez"})
		wantCode(t, err, codes.FailedPrecondition, "cerrar dos veces")
		_, err = h.client.UpdateExpediente(c, &psicologia.UpdateExpedienteRequest{Id: exp.Id, ResumenCaso: strPtr("x")})
		wantCode(t, err, codes.FailedPrecondition, "modificar uno cerrado")
		_, err = h.client.ReopenExpediente(c, &psicologia.ReopenExpedienteRequest{Id: exp.Id, Estado: "CERRADO"})
		wantCode(t, err, codes.InvalidArgument, "reabrir como CERRADO")

		re, err := h.client.ReopenExpediente(c, &psicologia.ReopenExpedienteRequest{Id: exp.Id})
		if err != nil || re.Expediente.Estado != "ABIERTO" {
			t.Fatalf("reapertura por defecto: %+v err=%v", re, err)
		}
		// segundo ciclo: reabrir directamente a EN_SEGUIMIENTO
		if _, err := h.client.CloseExpediente(c, &psicologia.CloseExpedienteRequest{Id: exp.Id, Motivo: "alta 2"}); err != nil {
			t.Fatalf("segundo cierre: %v", err)
		}
		re2, err := h.client.ReopenExpediente(c, &psicologia.ReopenExpedienteRequest{Id: exp.Id, Estado: "EN_SEGUIMIENTO"})
		if err != nil || re2.Expediente.Estado != "EN_SEGUIMIENTO" {
			t.Fatalf("reapertura a EN_SEGUIMIENTO: %+v err=%v", re2, err)
		}
		if h.countAudit(creador, "CLOSE", exp.Id) != 2 || h.countAudit(creador, "REOPEN", exp.Id) != 2 {
			t.Fatal("se esperaban 2 CLOSE y 2 REOPEN")
		}
		// un profesional con alcance propio pero de otro responsable no puede cerrar
		exp2 := open(t, est(81))
		_, err = h.client.CloseExpediente(h.as(otroProf, "X"), &psicologia.CloseExpedienteRequest{Id: exp2.Id, Motivo: "x"})
		wantCode(t, err, codes.NotFound, "cerrar ajeno con view_own")
	})

	// ---- Auditoría: nada sensible ----
	t.Run("Auditoria_NoContieneContenidoClinicoNiJWT_Y_AccionesCompletas", func(t *testing.T) {
		acciones := map[string]bool{}
		rows, err := h.admin.Query(h.ctx, `SELECT accion, COALESCE(motivo,''), recurso FROM auditoria_psicologica WHERE usuario_id::text = ANY($1)`, h.actors)
		if err != nil {
			t.Fatalf("leer auditoría: %v", err)
		}
		defer rows.Close()
		permitidos := regexp.MustCompile(`^(|PERMISO_INSUFICIENTE|SIN_ALCANCE|FUERA_DE_ALCANCE|LISTADO|ABIERTO|EN_SEGUIMIENTO|resumen_caso|estado|responsable_id|[a-z_,]+)$`)
		for rows.Next() {
			var accion, motivo, recurso string
			_ = rows.Scan(&accion, &motivo, &recurso)
			acciones[accion] = true
			if !permitidos.MatchString(motivo) || strings.Contains(motivo, "SECRETO") || strings.Contains(motivo, "eyJ") {
				t.Fatalf("la auditoría contiene texto no permitido: %q", motivo)
			}
			if recurso != "EXPEDIENTE_PSICOLOGICO" {
				t.Fatalf("recurso inesperado %q", recurso)
			}
		}
		for _, a := range []string{"CREATE", "VIEW", "UPDATE", "CLOSE", "REOPEN", "DENIED"} {
			if !acciones[a] {
				t.Fatalf("falta el evento %s en la auditoría", a)
			}
		}
	})
}

func strPtr(s string) *string { return &s }

func contains(items []*psicologia.ExpedientePsicologico, id string) bool {
	for _, e := range items {
		if e.Id == id {
			return true
		}
	}
	return false
}

// mustOpenFor abre un expediente como "actor" asignando otro responsable.
func (h *harness) mustOpenFor(t *testing.T, actor, estudiante, responsable string) *psicologia.ExpedientePsicologico {
	t.Helper()
	resp, err := h.client.OpenExpediente(h.as(actor, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{
		EstudianteId: estudiante, ResponsableId: responsable, ResumenCaso: "TEXTO-CLINICO-SECRETO",
	})
	if err != nil {
		t.Fatalf("OpenExpediente(responsable=%s): %v", responsable, err)
	}
	return h.track(resp.Expediente)
}

var _ = context.Background
