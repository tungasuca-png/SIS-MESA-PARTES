package integration

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"psicologia/psicologia"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	pAtView    = "psicologia.atenciones.view"
	pAtCreate  = "psicologia.atenciones.create"
	pAtCorrect = "psicologia.atenciones.correct"
	pAtAnnul   = "psicologia.atenciones.annul"
)

// Marca de contenido "clínico" de prueba: nunca debe aparecer en la auditoría.
const marcaClinica = "CONTENIDO-CLINICO-SECRETO"

func contenidoCompleto(sufijo string) *psicologia.AtencionContenido {
	return &psicologia.AtencionContenido{
		FechaAtencion: "2026-09-23", HoraAtencion: "10:30",
		RealizadaPor: []string{"PSICOLOGO"}, Motivos: []string{"SOCIOEMOCIONAL", "FAMILIAR"},
		DescripcionMotivo: marcaClinica + " descripcion " + sufijo, AnalisisIntegral: marcaClinica + " analisis",
		Estrategias: "estrategias", Derivaciones: []string{"TUTORIA"}, SeguimientoResultados: "seguimiento",
		Acuerdos: "acuerdos", CompromisoEstudiante: "compromiso est", CompromisoFamilia: "compromiso fam",
	}
}

// TestAtencionesPsicologicas prueba las atenciones de punta a punta (JWT,
// permisos por userID, alcance del expediente, PostgreSQL real, auditoría).
func TestAtencionesPsicologicas(t *testing.T) {
	h := newHarness(t)
	base := time.Now().UnixNano()

	psico := fixedUUID(base, 30)    // responsable del expediente: todo
	general := fixedUUID(base, 31)  // expedientes.view + atenciones completas (no responsable)
	propio := fixedUUID(base, 32)   // view_own, NO responsable del expediente de prueba
	lector := fixedUUID(base, 33)   // view + atenciones.view solamente
	sinAtenc := fixedUUID(base, 34) // expedientes.view sin permisos de atención
	sinAlcance := fixedUUID(base, 35)
	h.grant(psico, pCreate, pView, pUpdate, pAtView, pAtCreate, pAtCorrect, pAtAnnul)
	h.grant(general, pView, pAtView, pAtCreate, pAtCorrect, pAtAnnul)
	h.grant(propio, pViewOwn, pAtView, pAtCreate, pAtCorrect, pAtAnnul)
	h.grant(lector, pView, pAtView)
	h.grant(sinAtenc, pView)
	h.grant(sinAlcance, pAtView, pAtCreate) // permisos de atención sin alcance de expediente
	h.users.known[psico] = "PSICOLOGO"

	estudiante := fixedUUID(base, 300)
	openResp, err := h.client.OpenExpediente(h.as(psico, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{EstudianteId: estudiante})
	if err != nil {
		t.Fatalf("abrir expediente: %v", err)
	}
	exp := h.track(openResp.Expediente)
	dir := &fakeDirectorio{nombres: map[string]string{estudiante: "Estudiante De Prueba"}}
	h.svcCtx.EstudiantesDir = dir

	crear := func(t *testing.T, user string, c *psicologia.AtencionContenido) *psicologia.AtencionPsicologica {
		t.Helper()
		r, err := h.client.CreateAtencionDraft(h.as(user, "PSICOLOGO"), &psicologia.CreateAtencionDraftRequest{ExpedienteId: exp.Id, Contenido: c})
		if err != nil {
			t.Fatalf("crear borrador: %v", err)
		}
		return r.Atencion
	}
	registrar := func(t *testing.T, user string, a *psicologia.AtencionPsicologica) *psicologia.AtencionPsicologica {
		t.Helper()
		r, err := h.client.RegisterAtencion(h.as(user, "PSICOLOGO"), &psicologia.RegisterAtencionRequest{Id: a.Id, Revision: a.Revision})
		if err != nil {
			t.Fatalf("registrar: %v", err)
		}
		return r.Atencion
	}

	var registrada *psicologia.AtencionPsicologica

	t.Run("Borrador_CrearEditarRevision", func(t *testing.T) {
		a := crear(t, psico, &psicologia.AtencionContenido{DescripcionMotivo: "inicio"})
		if a.Estado != "BORRADOR" || a.Numero != 0 || a.Version.GetNumeroVersion() != 0 || a.Revision != 0 {
			t.Fatalf("borrador inesperado: %+v", a)
		}
		r, err := h.client.UpdateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.UpdateAtencionDraftRequest{
			Id: a.Id, Revision: a.Revision, Contenido: contenidoCompleto("A")})
		if err != nil || r.Atencion.Revision != 1 || r.Atencion.Version.Contenido.DescripcionMotivo == "inicio" {
			t.Fatalf("editar borrador: %+v err=%v", r, err)
		}
		_, err = h.client.UpdateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.UpdateAtencionDraftRequest{
			Id: a.Id, Revision: 0, Contenido: contenidoCompleto("B")})
		wantCode(t, err, codes.Aborted, "revisión obsoleta")
		if h.countAudit(psico, "CREATE", a.Id) != 1 || h.countAudit(psico, "UPDATE", a.Id) != 1 {
			t.Fatal("se esperaba CREATE y UPDATE auditados")
		}
		registrada = registrar(t, psico, r.Atencion)
		if registrada.Estado != "REGISTRADA" || registrada.Numero != 1 || registrada.VersionVigente != 1 || registrada.RegistradaPor != psico {
			t.Fatalf("registro inesperado: %+v", registrada)
		}
		if h.countAudit(psico, "REGISTER", a.Id) != 1 {
			t.Fatal("se esperaba REGISTER auditado")
		}
	})

	t.Run("Registrar_Validaciones", func(t *testing.T) {
		a := crear(t, psico, &psicologia.AtencionContenido{DescripcionMotivo: "sin fecha"})
		_, err := h.client.RegisterAtencion(h.as(psico, "PSICOLOGO"), &psicologia.RegisterAtencionRequest{Id: a.Id, Revision: a.Revision})
		wantCode(t, err, codes.InvalidArgument, "registrar sin fecha")
		c := contenidoCompleto("otro")
		c.Derivaciones = []string{"OTRO"}
		r, _ := h.client.UpdateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.UpdateAtencionDraftRequest{Id: a.Id, Revision: a.Revision, Contenido: c})
		_, err = h.client.RegisterAtencion(h.as(psico, "PSICOLOGO"), &psicologia.RegisterAtencionRequest{Id: a.Id, Revision: r.Atencion.Revision})
		wantCode(t, err, codes.InvalidArgument, "derivación Otro sin texto")
		// Datos de formato inválidos al guardar borrador.
		bad := contenidoCompleto("x")
		bad.FechaAtencion = "2026-02-30"
		_, err = h.client.UpdateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.UpdateAtencionDraftRequest{Id: a.Id, Revision: r.Atencion.Revision, Contenido: bad})
		wantCode(t, err, codes.InvalidArgument, "fecha inválida")
		bad = contenidoCompleto("x")
		bad.Motivos = []string{"NO_EXISTE"}
		_, err = h.client.UpdateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.UpdateAtencionDraftRequest{Id: a.Id, Revision: r.Atencion.Revision, Contenido: bad})
		wantCode(t, err, codes.InvalidArgument, "motivo inexistente en catálogo")
		bad = contenidoCompleto("x")
		bad.RealizadaPor = []string{"ALUMNO"}
		_, err = h.client.UpdateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.UpdateAtencionDraftRequest{Id: a.Id, Revision: r.Atencion.Revision, Contenido: bad})
		wantCode(t, err, codes.InvalidArgument, "realizada_por inválido")
		// Descartar (no se borra; deja de existir para todos).
		if _, err := h.client.DiscardAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.DiscardAtencionDraftRequest{Id: a.Id, Revision: r.Atencion.Revision}); err != nil {
			t.Fatalf("descartar: %v", err)
		}
		_, err = h.client.GetAtencion(h.as(psico, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: a.Id})
		wantCode(t, err, codes.NotFound, "descartada no visible")
		var n int
		_ = h.admin.QueryRow(h.ctx, "SELECT COUNT(*) FROM atenciones_psicologicas WHERE id = $1::uuid AND estado = 'DESCARTADA'", a.Id).Scan(&n)
		if n != 1 {
			t.Fatal("descartar no debe borrar físicamente")
		}
	})

	t.Run("Registrada_NoSeEditaDirectamente", func(t *testing.T) {
		_, err := h.client.UpdateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.UpdateAtencionDraftRequest{
			Id: registrada.Id, Revision: registrada.Revision, Contenido: contenidoCompleto("x")})
		wantCode(t, err, codes.FailedPrecondition, "editar una registrada")
		_, err = h.client.RegisterAtencion(h.as(psico, "PSICOLOGO"), &psicologia.RegisterAtencionRequest{Id: registrada.Id, Revision: registrada.Revision})
		wantCode(t, err, codes.FailedPrecondition, "registrar dos veces")
		// Capa BD: la versión oficial es inmutable incluso para la aplicación.
		_, err = h.app.Exec(h.ctx, "UPDATE atencion_versiones SET contenido_cifrado = contenido_cifrado || decode('00', 'hex') WHERE atencion_id = $1::uuid AND numero_version = 1", registrada.Id)
		if err == nil || !strings.Contains(err.Error(), "inmutable") {
			t.Fatalf("la versión 1 debe ser inmutable, err=%v", err)
		}
		_, err = h.app.Exec(h.ctx, "DELETE FROM atenciones_psicologicas WHERE id = $1::uuid", registrada.Id)
		if err == nil {
			t.Fatal("la aplicación no debe poder borrar atenciones")
		}
	})

	t.Run("BorradorAjeno_Invisible", func(t *testing.T) {
		a := crear(t, psico, contenidoCompleto("ajeno"))
		_, err := h.client.GetAtencion(h.as(general, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: a.Id})
		wantCode(t, err, codes.NotFound, "borrador ajeno")
		list, err := h.client.ListAtenciones(h.as(general, "PSICOLOGO"), &psicologia.ListAtencionesRequest{ExpedienteId: exp.Id})
		if err != nil {
			t.Fatalf("listar: %v", err)
		}
		for _, it := range list.Atenciones {
			if it.Id == a.Id {
				t.Fatal("un borrador ajeno no debe aparecer en el listado")
			}
		}
		_, _ = h.client.DiscardAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.DiscardAtencionDraftRequest{Id: a.Id, Revision: a.Revision})
	})

	t.Run("Correlativo_ConcurrenteSinDuplicados", func(t *testing.T) {
		const n = 6
		drafts := make([]*psicologia.AtencionPsicologica, n)
		for i := range drafts {
			drafts[i] = crear(t, psico, contenidoCompleto("c"))
		}
		var wg sync.WaitGroup
		numeros := make([]int32, n)
		errs := make([]error, n)
		for i := range drafts {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				r, err := h.client.RegisterAtencion(h.as(psico, "PSICOLOGO"), &psicologia.RegisterAtencionRequest{Id: drafts[i].Id, Revision: drafts[i].Revision})
				errs[i] = err
				if err == nil {
					numeros[i] = r.Atencion.Numero
				}
			}(i)
		}
		wg.Wait()
		vistos := map[int32]bool{}
		for i, num := range numeros {
			if errs[i] != nil {
				t.Fatalf("registro concurrente: %v", errs[i])
			}
			if vistos[num] || num < 2 || num > int32(1+n) {
				t.Fatalf("número duplicado o fuera de rango: %v", numeros)
			}
			vistos[num] = true
		}
	})

	t.Run("RegistroDoble_Concurrente_UnoGana", func(t *testing.T) {
		a := crear(t, psico, contenidoCompleto("doble"))
		var wg sync.WaitGroup
		results := make([]error, 4)
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, results[i] = h.client.RegisterAtencion(h.as(psico, "PSICOLOGO"), &psicologia.RegisterAtencionRequest{Id: a.Id, Revision: a.Revision})
			}(i)
		}
		wg.Wait()
		ok := 0
		for _, err := range results {
			if err == nil {
				ok++
			} else if c := status.Code(err); c != codes.FailedPrecondition && c != codes.Aborted {
				t.Fatalf("resultado inesperado: %v", err)
			}
		}
		if ok != 1 {
			t.Fatalf("exactamente un registro debe ganar, ganaron %d", ok)
		}
	})

	t.Run("Listar_SinContenidoClinico_YConsultar", func(t *testing.T) {
		list, err := h.client.ListAtenciones(h.as(lector, "DOCENTE"), &psicologia.ListAtencionesRequest{ExpedienteId: exp.Id})
		if err != nil || list.Total < 1 {
			t.Fatalf("listar: %+v err=%v", list, err)
		}
		for _, it := range list.Atenciones {
			if it.Estado == "BORRADOR" {
				t.Fatal("el lector no debe ver borradores ajenos")
			}
		}
		// El resumen no tiene campos clínicos: se verifica también el mensaje serializado.
		raw := list.String()
		if strings.Contains(raw, marcaClinica) {
			t.Fatal("el listado no debe incluir contenido clínico")
		}
		got, err := h.client.GetAtencion(h.as(lector, "DOCENTE"), &psicologia.GetAtencionRequest{Id: registrada.Id})
		if err != nil || !strings.Contains(got.Atencion.Version.Contenido.DescripcionMotivo, marcaClinica) {
			t.Fatalf("consultar: %+v err=%v", got, err)
		}
		if len(got.Atencion.Version.Motivos) != 2 || got.Atencion.Version.Motivos[0].Etiqueta == "" {
			t.Fatalf("motivos con etiqueta congelada: %+v", got.Atencion.Version.Motivos)
		}
		if h.countAudit(lector, "VIEW", registrada.Id) < 1 {
			t.Fatal("la consulta debe auditarse como VIEW")
		}
	})

	t.Run("Corregir_NuevaVersion_AnteriorIntacta", func(t *testing.T) {
		_, err := h.client.CorrectAtencion(h.as(psico, "PSICOLOGO"), &psicologia.CorrectAtencionRequest{
			Id: registrada.Id, VersionBase: 1, Contenido: contenidoCompleto("v2")})
		wantCode(t, err, codes.InvalidArgument, "corrección sin motivo")
		_, err = h.client.CorrectAtencion(h.as(psico, "PSICOLOGO"), &psicologia.CorrectAtencionRequest{
			Id: registrada.Id, VersionBase: 7, MotivoCorreccion: "error de tipeo", Contenido: contenidoCompleto("v2")})
		wantCode(t, err, codes.Aborted, "version_base obsoleta")

		// Dos correcciones simultáneas sobre la misma base: solo una gana.
		var wg sync.WaitGroup
		results := make([]error, 3)
		for i := range results {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, results[i] = h.client.CorrectAtencion(h.as(psico, "PSICOLOGO"), &psicologia.CorrectAtencionRequest{
					Id: registrada.Id, VersionBase: 1, MotivoCorreccion: "corrección", Contenido: contenidoCompleto("v2")})
			}(i)
		}
		wg.Wait()
		ok := 0
		for _, err := range results {
			if err == nil {
				ok++
			} else if status.Code(err) != codes.Aborted {
				t.Fatalf("resultado inesperado: %v", err)
			}
		}
		if ok != 1 {
			t.Fatalf("exactamente una corrección debe ganar, ganaron %d", ok)
		}
		cur, _ := h.client.GetAtencion(h.as(psico, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: registrada.Id})
		if cur.Atencion.VersionVigente != 2 || !strings.Contains(cur.Atencion.Version.Contenido.DescripcionMotivo, "v2") {
			t.Fatalf("versión vigente inesperada: %+v", cur.Atencion)
		}
		v1, err := h.client.GetAtencionVersion(h.as(psico, "PSICOLOGO"), &psicologia.GetAtencionVersionRequest{Id: registrada.Id, NumeroVersion: 1})
		if err != nil || !strings.Contains(v1.Atencion.Version.Contenido.DescripcionMotivo, " A") {
			t.Fatalf("la versión 1 debe quedar intacta: %+v err=%v", v1, err)
		}
		_, err = h.client.GetAtencionVersion(h.as(psico, "PSICOLOGO"), &psicologia.GetAtencionVersionRequest{Id: registrada.Id, NumeroVersion: 9})
		wantCode(t, err, codes.NotFound, "versión inexistente")
		vs, err := h.client.ListAtencionVersions(h.as(psico, "PSICOLOGO"), &psicologia.ListAtencionVersionsRequest{Id: registrada.Id})
		if err != nil || len(vs.Versiones) != 2 || vs.Versiones[1].MotivoCorreccion != "corrección" {
			t.Fatalf("versiones: %+v err=%v", vs, err)
		}
		if h.countAudit(psico, "CORRECT", registrada.Id) != 1 {
			t.Fatal("la corrección debe auditarse una vez")
		}
	})

	t.Run("ExportarPdf", func(t *testing.T) {
		borrador := crear(t, psico, contenidoCompleto("pdf"))
		_, err := h.client.ExportAtencionPdf(h.as(psico, "PSICOLOGO"), &psicologia.ExportAtencionPdfRequest{Id: borrador.Id})
		wantCode(t, err, codes.FailedPrecondition, "un borrador no se exporta")
		_, _ = h.client.DiscardAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.DiscardAtencionDraftRequest{Id: borrador.Id, Revision: borrador.Revision})

		pdfResp, err := h.client.ExportAtencionPdf(h.as(lector, "DOCENTE"), &psicologia.ExportAtencionPdfRequest{Id: registrada.Id})
		if err != nil {
			t.Fatalf("exportar vigente: %v", err)
		}
		if !bytes.HasPrefix(pdfResp.Contenido, []byte("%PDF")) {
			t.Fatal("el contenido no es un PDF")
		}
		want := exp.Codigo + "-AT01-v2.pdf"
		if pdfResp.Nombre != want || strings.Contains(pdfResp.Nombre, "Estudiante") {
			t.Fatalf("nombre de archivo %q, se esperaba %q", pdfResp.Nombre, want)
		}
		v1, err := h.client.ExportAtencionPdf(h.as(lector, "DOCENTE"), &psicologia.ExportAtencionPdfRequest{Id: registrada.Id, NumeroVersion: 1})
		if err != nil || v1.Nombre != exp.Codigo+"-AT01-v1.pdf" {
			t.Fatalf("exportar v1: %+v err=%v", v1.GetNombre(), err)
		}
		if h.countAudit(lector, "EXPORT", registrada.Id) != 2 {
			t.Fatal("cada descarga debe auditarse como EXPORT")
		}
		// Sin poder identificar al estudiante no hay PDF ni evento EXPORT.
		dir.failing = status.Error(codes.PermissionDenied, "sin estudiantes.view")
		_, err = h.client.ExportAtencionPdf(h.as(lector, "DOCENTE"), &psicologia.ExportAtencionPdfRequest{Id: registrada.Id})
		wantCode(t, err, codes.PermissionDenied, "estudiantes sin permiso")
		dir.failing = nil
		if h.countAudit(lector, "EXPORT", registrada.Id) != 2 {
			t.Fatal("un PDF fallido no debe auditarse como EXPORT")
		}
		// Texto muy largo y caracteres fuera de cp1252 no rompen el PDF.
		largo := crear(t, psico, &psicologia.AtencionContenido{
			FechaAtencion: "2026-09-24", HoraAtencion: "08:00", MotivoOtros: strings.Repeat("otro motivo ", 40),
			DescripcionMotivo: strings.Repeat("Texto “largo” con ñ y tildes áéíóú — ", 250),
			AnalisisIntegral:  strings.Repeat("Análisis ", 1100), Derivaciones: []string{"OTRO"}, DerivacionOtro: strings.Repeat("destino ", 40),
		})
		reg := registrar(t, psico, largo)
		big, err := h.client.ExportAtencionPdf(h.as(psico, "PSICOLOGO"), &psicologia.ExportAtencionPdfRequest{Id: reg.Id})
		if err != nil || !bytes.HasPrefix(big.Contenido, []byte("%PDF")) {
			t.Fatalf("PDF con texto largo: err=%v", err)
		}
	})

	t.Run("Permisos", func(t *testing.T) {
		_, err := h.client.GetAtencion(h.as(sinAtenc, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: registrada.Id})
		wantCode(t, err, codes.PermissionDenied, "sin atenciones.view")
		_, err = h.client.ListAtenciones(h.as(sinAtenc, "PSICOLOGO"), &psicologia.ListAtencionesRequest{ExpedienteId: exp.Id})
		wantCode(t, err, codes.PermissionDenied, "listar sin atenciones.view")
		_, err = h.client.ExportAtencionPdf(h.as(sinAtenc, "PSICOLOGO"), &psicologia.ExportAtencionPdfRequest{Id: registrada.Id})
		wantCode(t, err, codes.PermissionDenied, "exportar sin atenciones.view")
		_, err = h.client.CreateAtencionDraft(h.as(lector, "PSICOLOGO"), &psicologia.CreateAtencionDraftRequest{ExpedienteId: exp.Id})
		wantCode(t, err, codes.PermissionDenied, "crear sin atenciones.create")
		_, err = h.client.CorrectAtencion(h.as(lector, "PSICOLOGO"), &psicologia.CorrectAtencionRequest{
			Id: registrada.Id, VersionBase: 2, MotivoCorreccion: "x", Contenido: contenidoCompleto("x")})
		wantCode(t, err, codes.PermissionDenied, "corregir sin atenciones.correct")
		_, err = h.client.AnnulAtencion(h.as(lector, "PSICOLOGO"), &psicologia.AnnulAtencionRequest{Id: registrada.Id, Motivo: "x"})
		wantCode(t, err, codes.PermissionDenied, "anular sin atenciones.annul")
		// Permisos de atención pero ningún alcance de expediente.
		_, err = h.client.GetAtencion(h.as(sinAlcance, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: registrada.Id})
		wantCode(t, err, codes.PermissionDenied, "sin view ni view_own")
		if h.countAudit(sinAtenc, "DENIED", "") < 3 {
			t.Fatal("las denegaciones deben auditarse")
		}
	})

	t.Run("Alcance_ViewOwn", func(t *testing.T) {
		_, err := h.client.GetAtencion(h.as(propio, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: registrada.Id})
		wantCode(t, err, codes.NotFound, "view_own fuera de alcance")
		_, err = h.client.ListAtenciones(h.as(propio, "PSICOLOGO"), &psicologia.ListAtencionesRequest{ExpedienteId: exp.Id})
		wantCode(t, err, codes.NotFound, "listar fuera de alcance")
		_, err = h.client.CreateAtencionDraft(h.as(propio, "PSICOLOGO"), &psicologia.CreateAtencionDraftRequest{ExpedienteId: exp.Id})
		wantCode(t, err, codes.NotFound, "crear fuera de alcance")
		_, err = h.client.AnnulAtencion(h.as(propio, "PSICOLOGO"), &psicologia.AnnulAtencionRequest{Id: registrada.Id, Motivo: "x"})
		wantCode(t, err, codes.NotFound, "anular fuera de alcance")
		_, err = h.client.ExportAtencionPdf(h.as(propio, "PSICOLOGO"), &psicologia.ExportAtencionPdfRequest{Id: registrada.Id})
		wantCode(t, err, codes.NotFound, "exportar fuera de alcance")
		// El responsable (view_own sería suficiente) sí accede; "general" (view) también.
		if _, err := h.client.GetAtencion(h.as(general, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: registrada.Id}); err != nil {
			t.Fatalf("view general: %v", err)
		}
	})

	t.Run("Errores_Inexistentes", func(t *testing.T) {
		_, err := h.client.GetAtencion(h.as(psico, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: fixedUUID(base, 999)})
		wantCode(t, err, codes.NotFound, "atención inexistente")
		_, err = h.client.GetAtencion(h.as(psico, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: "no-uuid"})
		wantCode(t, err, codes.InvalidArgument, "id inválido")
		_, err = h.client.CreateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.CreateAtencionDraftRequest{ExpedienteId: fixedUUID(base, 998)})
		wantCode(t, err, codes.NotFound, "expediente inexistente")
		_, err = h.client.ListAtenciones(h.as(psico, "PSICOLOGO"), &psicologia.ListAtencionesRequest{ExpedienteId: fixedUUID(base, 998)})
		wantCode(t, err, codes.NotFound, "listar expediente inexistente")
	})

	t.Run("Anular_SinBorrar", func(t *testing.T) {
		_, err := h.client.AnnulAtencion(h.as(psico, "PSICOLOGO"), &psicologia.AnnulAtencionRequest{Id: registrada.Id, Motivo: "  "})
		wantCode(t, err, codes.InvalidArgument, "anular sin motivo")
		r, err := h.client.AnnulAtencion(h.as(psico, "PSICOLOGO"), &psicologia.AnnulAtencionRequest{Id: registrada.Id, Motivo: "registrada por error"})
		if err != nil || r.Atencion.Estado != "ANULADA" || r.Atencion.MotivoAnulacion != "registrada por error" || r.Atencion.Numero != 1 {
			t.Fatalf("anular: %+v err=%v", r, err)
		}
		_, err = h.client.AnnulAtencion(h.as(psico, "PSICOLOGO"), &psicologia.AnnulAtencionRequest{Id: registrada.Id, Motivo: "otra vez"})
		wantCode(t, err, codes.FailedPrecondition, "anular dos veces")
		_, err = h.client.CorrectAtencion(h.as(psico, "PSICOLOGO"), &psicologia.CorrectAtencionRequest{
			Id: registrada.Id, VersionBase: 2, MotivoCorreccion: "x", Contenido: contenidoCompleto("x")})
		wantCode(t, err, codes.FailedPrecondition, "corregir una anulada")
		if _, err := h.client.GetAtencion(h.as(lector, "DOCENTE"), &psicologia.GetAtencionRequest{Id: registrada.Id}); err != nil {
			t.Fatalf("una anulada sigue visible: %v", err)
		}
		if _, err := h.client.ExportAtencionPdf(h.as(lector, "DOCENTE"), &psicologia.ExportAtencionPdfRequest{Id: registrada.Id}); err != nil {
			t.Fatalf("una anulada se puede descargar (con su estado): %v", err)
		}
		if h.countAudit(psico, "ANNUL", registrada.Id) != 1 {
			t.Fatal("la anulación debe auditarse")
		}
	})

	t.Run("ExpedienteCerrado_BloqueaEscrituras", func(t *testing.T) {
		pendiente := crear(t, psico, contenidoCompleto("antes de cerrar"))
		if _, err := h.client.CloseExpediente(h.as(psico, "PSICOLOGO"), &psicologia.CloseExpedienteRequest{Id: exp.Id, Motivo: "alta"}); err != nil {
			t.Fatalf("cerrar expediente: %v", err)
		}
		_, err := h.client.CreateAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.CreateAtencionDraftRequest{ExpedienteId: exp.Id})
		wantCode(t, err, codes.FailedPrecondition, "crear en expediente cerrado")
		_, err = h.client.RegisterAtencion(h.as(psico, "PSICOLOGO"), &psicologia.RegisterAtencionRequest{Id: pendiente.Id, Revision: pendiente.Revision})
		wantCode(t, err, codes.FailedPrecondition, "registrar en expediente cerrado")
		if _, err := h.client.DiscardAtencionDraft(h.as(psico, "PSICOLOGO"), &psicologia.DiscardAtencionDraftRequest{Id: pendiente.Id, Revision: pendiente.Revision}); err != nil {
			t.Fatalf("descartar un borrador en expediente cerrado debe permitirse: %v", err)
		}
		if _, err := h.client.ListAtenciones(h.as(psico, "PSICOLOGO"), &psicologia.ListAtencionesRequest{ExpedienteId: exp.Id}); err != nil {
			t.Fatalf("las lecturas siguen permitidas: %v", err)
		}
	})

	t.Run("Auditoria_SinContenidoClinico", func(t *testing.T) {
		var n int
		if err := h.admin.QueryRow(h.ctx, `SELECT COUNT(*) FROM auditoria_psicologica
			WHERE recurso = 'ATENCION_PSICOLOGICA' AND COALESCE(motivo, '') LIKE '%' || $1 || '%'`, marcaClinica).Scan(&n); err != nil {
			t.Fatalf("auditoría: %v", err)
		}
		if n != 0 {
			t.Fatal("la auditoría no debe contener contenido de la hoja")
		}
		var acciones int
		_ = h.admin.QueryRow(h.ctx, `SELECT COUNT(DISTINCT accion) FROM auditoria_psicologica
			WHERE recurso = 'ATENCION_PSICOLOGICA' AND usuario_id = $1::uuid`, psico).Scan(&acciones)
		if acciones < 7 { // CREATE, UPDATE, REGISTER, DISCARD, VIEW, CORRECT, EXPORT, ANNUL
			t.Fatalf("faltan acciones auditadas: %d", acciones)
		}
	})
}
