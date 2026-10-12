package integration

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"psicologia/internal/clinicalcrypto"
	"psicologia/internal/repository"
	"psicologia/psicologia"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

// Marcador de contenido clínico de prueba (FASE 6N-3, bloque 15).
const marcador = "MARCADOR_CLINICO_TEST_123"

// bufferSeguro recoge todo lo que el servicio escribe en su log durante la prueba.
type bufferSeguro struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *bufferSeguro) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *bufferSeguro) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// textoDelPDF descomprime los streams (FlateDecode) del PDF para buscar texto.
func textoDelPDF(t *testing.T, pdf []byte) string {
	t.Helper()
	var out strings.Builder
	re := regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	for _, m := range re.FindAllSubmatch(pdf, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			out.Write(m[1])
			continue
		}
		b, _ := io.ReadAll(r)
		out.Write(b)
	}
	return out.String()
}

func contenidoConMarcador(tag string) *psicologia.AtencionContenido {
	return &psicologia.AtencionContenido{
		FechaAtencion: "2026-09-24", HoraAtencion: "11:00",
		RealizadaPor: []string{"PSICOLOGO"}, RealizadaPorOtros: marcador + " otros " + tag,
		Motivos: []string{"FAMILIAR"}, MotivoOtros: marcador + " motivo otros",
		DescripcionMotivo: marcador + " descripcion " + tag, AnalisisIntegral: marcador + " analisis",
		Estrategias: marcador + " estrategias", Derivaciones: []string{"OTRO"}, DerivacionOtro: marcador + " destino",
		SeguimientoResultados: marcador + " seguimiento", Acuerdos: marcador + " acuerdos",
		CompromisoEstudiante: marcador + " compromiso est", CompromisoFamilia: marcador + " compromiso fam",
	}
}

func TestCifradoContenidoClinico(t *testing.T) {
	h := newHarness(t)

	// J: todo lo que el servicio registre en su log durante la prueba.
	logs := &bufferSeguro{}
	anterior := logx.Reset()
	logx.SetWriter(logx.NewWriter(logs))
	t.Cleanup(func() { logx.Reset(); logx.SetWriter(anterior) })

	base := time.Now().UnixNano()
	psico := fixedUUID(base, 70)
	sinPermiso := fixedUUID(base, 71) // expedientes.view sin atenciones.view
	ajeno := fixedUUID(base, 72)      // view_own, no responsable
	h.grant(psico, pCreate, pView, pUpdate, pAtView, pAtCreate, pAtCorrect, pAtAnnul)
	h.grant(sinPermiso, pView)
	h.grant(ajeno, pViewOwn, pAtView)
	h.users.known[psico] = "PSICOLOGO"
	estudiante := fixedUUID(base, 700)
	open, err := h.client.OpenExpediente(h.as(psico, "PSICOLOGO"), &psicologia.OpenExpedienteRequest{
		EstudianteId: estudiante, ResumenCaso: marcador + " resumen del caso"})
	if err != nil {
		t.Fatalf("abrir expediente: %v", err)
	}
	exp := h.track(open.Expediente)
	h.svcCtx.EstudiantesDir = &fakeDirectorio{nombres: map[string]string{estudiante: "Estudiante Cifrado"}}
	ctxP := h.as(psico, "PSICOLOGO")

	draft, err := h.client.CreateAtencionDraft(ctxP, &psicologia.CreateAtencionDraftRequest{ExpedienteId: exp.Id, Contenido: contenidoConMarcador("v1")})
	if err != nil {
		t.Fatalf("crear borrador: %v", err)
	}

	var v1Antes []byte
	t.Run("G_Registrar_v0_a_v1_sigue_descifrando", func(t *testing.T) {
		reg, err := h.client.RegisterAtencion(ctxP, &psicologia.RegisterAtencionRequest{Id: draft.Atencion.Id, Revision: draft.Atencion.Revision})
		if err != nil {
			t.Fatalf("registrar: %v", err)
		}
		if !strings.Contains(reg.Atencion.Version.Contenido.DescripcionMotivo, marcador) {
			t.Fatal("la versión 1 no se descifró tras el paso v0 -> v1")
		}
		if err := h.admin.QueryRow(h.ctx, `SELECT contenido_cifrado FROM atencion_versiones WHERE atencion_id = $1::uuid AND numero_version = 1`, draft.Atencion.Id).Scan(&v1Antes); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("B_API_autorizada_devuelve_el_contenido", func(t *testing.T) {
		got, err := h.client.GetAtencion(ctxP, &psicologia.GetAtencionRequest{Id: draft.Atencion.Id})
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		c := got.Atencion.Version.Contenido
		for nombre, v := range map[string]string{"descripcion": c.DescripcionMotivo, "analisis": c.AnalisisIntegral,
			"estrategias": c.Estrategias, "seguimiento": c.SeguimientoResultados, "acuerdos": c.Acuerdos,
			"compromiso_est": c.CompromisoEstudiante, "compromiso_fam": c.CompromisoFamilia,
			"motivo_otros": c.MotivoOtros, "derivacion_otro": c.DerivacionOtro, "realizada_otros": c.RealizadaPorOtros} {
			if !strings.Contains(v, marcador) {
				t.Errorf("%s no se devolvió descifrado", nombre)
			}
		}
		e, err := h.client.GetExpediente(ctxP, &psicologia.GetExpedienteRequest{Id: exp.Id})
		if err != nil || !strings.Contains(e.Expediente.ResumenCaso, marcador) {
			t.Fatalf("resumen del caso no descifrado: err=%v", err)
		}
	})

	t.Run("H_Corregir_v1_a_v2_no_cambia_v1", func(t *testing.T) {
		corr, err := h.client.CorrectAtencion(ctxP, &psicologia.CorrectAtencionRequest{Id: draft.Atencion.Id, VersionBase: 1,
			MotivoCorreccion: marcador + " motivo de correccion", Contenido: contenidoConMarcador("v2")})
		if err != nil {
			t.Fatalf("corregir: %v", err)
		}
		if corr.Atencion.VersionVigente != 2 || !strings.Contains(corr.Atencion.Version.Contenido.DescripcionMotivo, "v2") {
			t.Fatalf("v2 inesperada: %+v", corr.Atencion)
		}
		if corr.Atencion.Version.MotivoCorreccion != marcador+" motivo de correccion" {
			t.Fatal("el motivo de corrección no se descifró")
		}
		var v1Despues []byte
		if err := h.admin.QueryRow(h.ctx, `SELECT contenido_cifrado FROM atencion_versiones WHERE atencion_id = $1::uuid AND numero_version = 1`, draft.Atencion.Id).Scan(&v1Despues); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(v1Antes, v1Despues) {
			t.Fatal("la versión 1 cambió al corregir")
		}
		v1, err := h.client.GetAtencionVersion(ctxP, &psicologia.GetAtencionVersionRequest{Id: draft.Atencion.Id, NumeroVersion: 1})
		if err != nil || !strings.Contains(v1.Atencion.Version.Contenido.DescripcionMotivo, "v1") {
			t.Fatalf("v1 ilegible tras corregir: %v", err)
		}
	})

	t.Run("I_PDF_con_contenido_correcto", func(t *testing.T) {
		for _, n := range []int32{1, 2} {
			r, err := h.client.ExportAtencionPdf(ctxP, &psicologia.ExportAtencionPdfRequest{Id: draft.Atencion.Id, NumeroVersion: n})
			if err != nil {
				t.Fatalf("pdf v%d: %v", n, err)
			}
			txt := textoDelPDF(t, r.Contenido)
			if !strings.Contains(txt, marcador) {
				t.Fatalf("el PDF v%d no contiene el contenido descifrado", n)
			}
		}
	})

	t.Run("Anular_y_cerrar_con_motivo_cifrado", func(t *testing.T) {
		an, err := h.client.AnnulAtencion(ctxP, &psicologia.AnnulAtencionRequest{Id: draft.Atencion.Id, Motivo: marcador + " motivo de anulacion"})
		if err != nil || an.Atencion.MotivoAnulacion != marcador+" motivo de anulacion" {
			t.Fatalf("anular: %v", err)
		}
		cl, err := h.client.CloseExpediente(ctxP, &psicologia.CloseExpedienteRequest{Id: exp.Id, Motivo: marcador + " motivo de cierre"})
		if err != nil || cl.Expediente.MotivoCierre != marcador+" motivo de cierre" || !strings.Contains(cl.Expediente.ResumenCaso, marcador) {
			t.Fatalf("cerrar: %v", err)
		}
		// Bloque 13: el listado no expone ni el resumen ni el motivo de cierre.
		list, err := h.client.ListExpedientes(ctxP, &psicologia.ListExpedientesRequest{PageSize: 100})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range list.Expedientes {
			if e.Id == exp.Id && (e.MotivoCierre != "" || e.ResumenCaso != "") {
				t.Fatal("el listado de expedientes expone texto libre")
			}
		}
		if _, err := h.client.ReopenExpediente(ctxP, &psicologia.ReopenExpedienteRequest{Id: exp.Id}); err != nil {
			t.Fatalf("reabrir: %v", err)
		}
	})

	t.Run("A_SQL_directo_no_contiene_texto_clinico", func(t *testing.T) {
		for _, tabla := range []string{"atencion_versiones", "atenciones_psicologicas", "expedientes_psicologicos", "auditoria_psicologica"} {
			var n int
			if err := h.admin.QueryRow(h.ctx, `SELECT COUNT(*) FROM `+tabla+` t WHERE row_to_json(t)::text LIKE '%'||$1||'%'`, marcador).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Fatalf("%s contiene el texto clínico en claro (%d filas)", tabla, n)
			}
		}
		var n int
		if err := h.admin.QueryRow(h.ctx, `SELECT COUNT(*) FROM atencion_versiones
			WHERE position(convert_to($1, 'UTF8') IN contenido_cifrado) > 0`, marcador).Scan(&n); err != nil || n != 0 {
			t.Fatalf("el bloque cifrado contiene el marcador en claro: n=%d err=%v", n, err)
		}
	})

	t.Run("C_Sin_permiso_o_fuera_de_alcance", func(t *testing.T) {
		_, err := h.client.GetAtencion(h.as(sinPermiso, "DOCENTE"), &psicologia.GetAtencionRequest{Id: draft.Atencion.Id})
		wantCode(t, err, codes.PermissionDenied, "sin atenciones.view")
		_, err = h.client.GetAtencion(h.as(ajeno, "PSICOLOGO"), &psicologia.GetAtencionRequest{Id: draft.Atencion.Id})
		wantCode(t, err, codes.NotFound, "view_own fuera de alcance")
		_, err = h.client.ExportAtencionPdf(h.as(ajeno, "PSICOLOGO"), &psicologia.ExportAtencionPdfRequest{Id: draft.Atencion.Id})
		wantCode(t, err, codes.NotFound, "pdf fuera de alcance")
	})

	t.Run("E_Bloque_alterado_rechazado", func(t *testing.T) {
		d, err := h.client.CreateAtencionDraft(ctxP, &psicologia.CreateAtencionDraftRequest{ExpedienteId: exp.Id, Contenido: contenidoConMarcador("alterado")})
		if err != nil {
			t.Fatal(err)
		}
		// Se invierte el último byte (etiqueta GCM) del borrador (v0, modificable).
		if _, err := h.admin.Exec(h.ctx, `UPDATE atencion_versiones
			SET contenido_cifrado = set_byte(contenido_cifrado, length(contenido_cifrado) - 1, get_byte(contenido_cifrado, length(contenido_cifrado) - 1) # 255)
			WHERE atencion_id = $1::uuid AND numero_version = 0`, d.Atencion.Id); err != nil {
			t.Fatal(err)
		}
		_, err = h.client.GetAtencion(ctxP, &psicologia.GetAtencionRequest{Id: d.Atencion.Id})
		wantCode(t, err, codes.Internal, "bloque alterado")
	})

	t.Run("F_Bloque_copiado_a_otra_fila_rechazado", func(t *testing.T) {
		a1, err := h.client.CreateAtencionDraft(ctxP, &psicologia.CreateAtencionDraftRequest{ExpedienteId: exp.Id, Contenido: contenidoConMarcador("origen")})
		if err != nil {
			t.Fatal(err)
		}
		a2, err := h.client.CreateAtencionDraft(ctxP, &psicologia.CreateAtencionDraftRequest{ExpedienteId: exp.Id, Contenido: contenidoConMarcador("destino")})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.admin.Exec(h.ctx, `UPDATE atencion_versiones d SET contenido_cifrado = o.contenido_cifrado, cifrado_key_id = o.cifrado_key_id
			FROM atencion_versiones o WHERE o.atencion_id = $1::uuid AND o.numero_version = 0
			AND d.atencion_id = $2::uuid AND d.numero_version = 0`, a1.Atencion.Id, a2.Atencion.Id); err != nil {
			t.Fatal(err)
		}
		_, err = h.client.GetAtencion(ctxP, &psicologia.GetAtencionRequest{Id: a2.Atencion.Id})
		wantCode(t, err, codes.Internal, "bloque copiado")
		if _, err := h.client.GetAtencion(ctxP, &psicologia.GetAtencionRequest{Id: a1.Atencion.Id}); err != nil {
			t.Fatalf("el original debe seguir legible: %v", err)
		}
	})

	t.Run("D_Clave_ausente_o_incorrecta_impide_iniciar", func(t *testing.T) {
		// Canario de la clave de prueba (lo aprovisiona el migrador; aquí el rol de pruebas).
		tx, err := h.admin.Begin(h.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repository.InicializarCanario(h.ctx, tx, h.cifrador); err != nil {
			_ = tx.Rollback(h.ctx)
			t.Fatalf("inicializar canario: %v", err)
		}
		if err := tx.Commit(h.ctx); err != nil {
			t.Fatal(err)
		}
		// Con la clave correcta, el arranque (misma verificación que NewServiceContext) pasa.
		if err := repository.VerificarCanario(h.ctx, h.app, h.cifrador); err != nil {
			t.Fatalf("la clave correcta debe pasar el canario: %v", err)
		}
		// Misma key_id, otra clave: el servicio NO inicia.
		otra := make([]byte, 32)
		_, _ = rand.Read(otra)
		incorrecta, err := clinicalcrypto.New(base64.StdEncoding.EncodeToString(otra), h.cifrador.ActiveKeyID(), "")
		if err != nil {
			t.Fatal(err)
		}
		if err := repository.VerificarCanario(context.Background(), h.app, incorrecta); err == nil {
			t.Fatal("una clave incorrecta debe impedir el arranque")
		} else if strings.Contains(err.Error(), base64.StdEncoding.EncodeToString(otra)) {
			t.Fatal("el error expone la clave")
		}
		// key_id sin canario aprovisionado: no inicia.
		sinCanario, _ := clinicalcrypto.New(base64.StdEncoding.EncodeToString(otra), "test-sin-canario", "")
		if err := repository.VerificarCanario(context.Background(), h.app, sinCanario); err == nil {
			t.Fatal("una clave sin canario debe impedir el arranque")
		}
		// Clave ausente: no se puede construir el cifrador.
		if _, err := clinicalcrypto.New("", "k1", ""); !errors.Is(err, clinicalcrypto.ErrConfiguracion) {
			t.Fatalf("clave ausente: se esperaba ErrConfiguracion, got %v", err)
		}
	})

	t.Run("J_Logs_sin_contenido_clinico", func(t *testing.T) {
		out := logs.String()
		if strings.Contains(out, marcador) {
			t.Fatal("el log del servicio contiene contenido clínico")
		}
		// Los errores de descifrado (E, F) sí deben haberse registrado, sin contenido.
		if !strings.Contains(out, "descifrar") && !strings.Contains(out, "no se pudo consultar") {
			t.Logf("aviso: no se encontró el registro del error de descifrado en el log capturado")
		}
	})
}
