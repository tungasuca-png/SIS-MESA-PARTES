package integration

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"estudiantes/estudiantes"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestEtapa5 verifica en PostgreSQL real la etapa 5 del modelo maestro:
// periodos, secciones, escritura de matrículas con su historial de
// movimientos, personas (padres y apoderados) con sus vínculos, el rol de
// apoderado y la resolución por código SIAGIE. Comprueba permisos, errores
// estables, auditoría sin valores y ausencia de valores en los logs.
//
// Aislamiento: años de periodo 2050–2089 derivados de la ejecución, códigos y
// nombres con el tag; la limpieza (helpers_test.go) borra todo lo creado.
func TestEtapa5(t *testing.T) {
	h := newHarness(t)
	var logBuf syncBuffer
	logx.SetWriter(logx.NewWriter(&logBuf))
	t.Cleanup(func() { logx.Reset() })

	base := time.Now().UnixNano()
	tag := strconv.FormatInt(base%100_000_000, 36)
	n := 0
	codigo := func() string { n++; return fmt.Sprintf("E5-%s-%d", tag, n) }
	dni := func(i int) string { return fmt.Sprintf("3%07d", (base/1000+int64(i))%10_000_000) }
	anioBase := int32(2050 + (base/1000)%20*2)

	const (
		admin     = "00000000-0000-4000-8000-0000000000c1" // maestro + matrícula
		lector    = "00000000-0000-4000-8000-0000000000c2" // solo estudiantes.view
		familiaV  = "00000000-0000-4000-8000-0000000000c3"
		familiaU  = "00000000-0000-4000-8000-0000000000c4"
		sinPermis = "00000000-0000-4000-8000-0000000000c5"
	)
	for _, p := range []string{"estudiantes.view", "estudiantes.create", "estudiantes.update",
		"estudiantes.change_estado", "estudiantes.matricula.update"} {
		h.grant(admin, p)
	}
	h.grant(lector, "estudiantes.view")
	h.grant(familiaV, "estudiantes.familia.view")
	h.grant(familiaU, "estudiantes.familia.update")
	h.grant(familiaU, "estudiantes.familia.view")

	wantErr := func(t *testing.T, err error, code codes.Code, prefijo, que string) {
		t.Helper()
		st := status.Convert(err)
		if err == nil || st.Code() != code || !strings.HasPrefix(st.Message(), prefijo+":") {
			t.Fatalf("%s: se esperaba %v %q, se obtuvo %v", que, code, prefijo, err)
		}
	}
	crear := func(t *testing.T, req *estudiantes.CreateEstudianteRequest) *estudiantes.EstudianteBasico {
		t.Helper()
		resp, err := h.client.CreateEstudiante(h.as(admin), req)
		if err != nil {
			t.Fatalf("CreateEstudiante: %v", err)
		}
		h.created = append(h.created, resp.Estudiante.Id)
		return resp.Estudiante
	}
	crearPeriodo := func(t *testing.T, anio int32) *estudiantes.PeriodoAcademico {
		t.Helper()
		resp, err := h.client.CreatePeriodo(h.as(admin), &estudiantes.CreatePeriodoRequest{Anio: anio, Nombre: fmt.Sprintf("Año escolar %d", anio),
			FechaInicio: fmt.Sprintf("%d-03-01", anio), FechaFin: fmt.Sprintf("%d-12-20", anio)})
		if err != nil {
			t.Fatalf("CreatePeriodo: %v", err)
		}
		h.periodos = append(h.periodos, resp.Periodo.Id)
		return resp.Periodo
	}
	avanzar := func(t *testing.T, id, estado string) {
		t.Helper()
		if _, err := h.client.ChangeEstadoPeriodo(h.as(admin), &estudiantes.ChangeEstadoPeriodoRequest{Id: id, Estado: estado}); err != nil {
			t.Fatalf("ChangeEstadoPeriodo %s: %v", estado, err)
		}
	}
	crearSeccion := func(t *testing.T, req *estudiantes.CreateSeccionRequest) *estudiantes.SeccionAcademica {
		t.Helper()
		resp, err := h.client.CreateSeccion(h.as(admin), req)
		if err != nil {
			t.Fatalf("CreateSeccion: %v", err)
		}
		return resp.Seccion
	}
	acciones := func(t *testing.T, col, id string) string {
		t.Helper()
		var out string
		if err := h.admin.QueryRow(h.ctx, `SELECT COALESCE(string_agg(accion || '[' || array_to_string(campos, ',') || ']', ' ' ORDER BY fecha_hora, id), '')
			FROM auditoria_estudiantes WHERE `+col+` = $1`, id).Scan(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	vacantes := func(v int32) *int32 { return &v }
	str := func(s string) *string { return &s }
	hoy := time.Now().Format("2006-01-02")

	// ================= PERIODOS =================
	t.Run("Periodos_CicloYValidaciones", func(t *testing.T) {
		p := crearPeriodo(t, anioBase)
		if p.Estado != "PLANIFICADO" || p.FechaInicio == "" || p.FechaFin == "" {
			t.Fatalf("periodo nuevo: %+v", p)
		}
		_, err := h.client.CreatePeriodo(h.as(admin), &estudiantes.CreatePeriodoRequest{Anio: anioBase, Nombre: "Repetido"})
		wantErr(t, err, codes.AlreadyExists, "duplicate_periodo", "año repetido")
		_, err = h.client.CreatePeriodo(h.as(admin), &estudiantes.CreatePeriodoRequest{Anio: 1990, Nombre: "Fuera"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "año fuera de rango")
		_, err = h.client.CreatePeriodo(h.as(admin), &estudiantes.CreatePeriodoRequest{Anio: anioBase + 40, Nombre: "X", FechaInicio: "2060-12-01", FechaFin: "2060-01-01"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "fechas invertidas")
		_, err = h.client.CreatePeriodo(h.as(lector), &estudiantes.CreatePeriodoRequest{Anio: anioBase + 41, Nombre: "X"})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "crear periodo sin matricula.update")

		// Solo hacia adelante.
		_, err = h.client.ChangeEstadoPeriodo(h.as(admin), &estudiantes.ChangeEstadoPeriodoRequest{Id: p.Id, Estado: "CERRADO"})
		wantErr(t, err, codes.FailedPrecondition, "invalid_transition", "saltar de PLANIFICADO a CERRADO")
		avanzar(t, p.Id, "EN_CURSO")
		avanzar(t, p.Id, "CERRADO")
		_, err = h.client.ChangeEstadoPeriodo(h.as(admin), &estudiantes.ChangeEstadoPeriodoRequest{Id: p.Id, Estado: "EN_CURSO"})
		wantErr(t, err, codes.FailedPrecondition, "invalid_transition", "reabrir un periodo cerrado")
		_, err = h.client.ChangeEstadoPeriodo(h.as(admin), &estudiantes.ChangeEstadoPeriodoRequest{Id: "00000000-0000-4000-8000-00000000dead", Estado: "EN_CURSO"})
		wantErr(t, err, codes.NotFound, "periodo_not_found", "periodo inexistente")

		_, err = h.client.CreateSeccion(h.as(admin), &estudiantes.CreateSeccionRequest{PeriodoId: p.Id, Nivel: "PRIMARIA", Grado: "1", Seccion: "A"})
		wantErr(t, err, codes.FailedPrecondition, "periodo_cerrado", "sección en periodo cerrado")

		lista, err := h.client.ListPeriodos(h.as(lector), &estudiantes.ListPeriodosRequest{})
		if err != nil {
			t.Fatal(err)
		}
		encontrado := false
		for _, x := range lista.Periodos {
			encontrado = encontrado || (x.Id == p.Id && x.Estado == "CERRADO")
		}
		if !encontrado {
			t.Fatal("ListPeriodos no devuelve el periodo cerrado")
		}
	})

	// ================= MATRÍCULA E HISTORIAL =================
	t.Run("Matricula_TransicionesEHistorial", func(t *testing.T) {
		p := crearPeriodo(t, anioBase+1)
		sA := crearSeccion(t, &estudiantes.CreateSeccionRequest{PeriodoId: p.Id, Nivel: "secundaria", Grado: "5", Seccion: "a", Turno: "mañana", Aula: "Aula 5A", Vacantes: vacantes(2)})
		sB := crearSeccion(t, &estudiantes.CreateSeccionRequest{PeriodoId: p.Id, Nivel: "SECUNDARIA", Grado: "5", Seccion: "B", Turno: "MAÑANA"})
		sAtarde := crearSeccion(t, &estudiantes.CreateSeccionRequest{PeriodoId: p.Id, Nivel: "SECUNDARIA", Grado: "5", Seccion: "A", Turno: "TARDE"})
		if sA.Nivel != "SECUNDARIA" || sA.Seccion != "A" || sA.Turno != "MAÑANA" || sA.GetVacantes() != 2 || sA.PeriodoId != p.Id {
			t.Fatalf("sección normalizada: %+v", sA)
		}
		_, err := h.client.CreateSeccion(h.as(admin), &estudiantes.CreateSeccionRequest{PeriodoId: p.Id, Nivel: "SECUNDARIA", Grado: "5", Seccion: "A", Turno: "MAÑANA"})
		wantErr(t, err, codes.AlreadyExists, "duplicate_seccion", "sección repetida")
		_, err = h.client.CreateSeccion(h.as(admin), &estudiantes.CreateSeccionRequest{PeriodoId: p.Id, Nivel: "UNIVERSIDAD", Grado: "1", Seccion: "A"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "nivel inválido")

		e1 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Ana", ApellidoPaterno: "Quispe", ApellidoMaterno: tag, TipoDocumento: "DNI", NumeroDocumento: dni(1)})
		e2 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Luis", ApellidoPaterno: "Mamani", ApellidoMaterno: tag})
		e3 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Rosa", ApellidoPaterno: "Huaman", ApellidoMaterno: tag})
		inactivo := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Inactivo", ApellidoPaterno: "Prueba", ApellidoMaterno: tag})
		if _, err := h.client.ChangeEstadoEstudiante(h.as(admin), &estudiantes.ChangeEstadoEstudianteRequest{Id: inactivo.Id, ExpectedVersion: inactivo.Version, Estado: "INACTIVO"}); err != nil {
			t.Fatal(err)
		}

		matricular := func(estudianteID, seccionID, fecha string) (*estudiantes.Matricula, error) {
			resp, err := h.client.MatricularEstudiante(h.as(admin), &estudiantes.MatricularEstudianteRequest{
				EstudianteId: estudianteID, PeriodoId: p.Id, SeccionId: seccionID, FechaMatricula: fecha})
			if err != nil {
				return nil, err
			}
			return resp.Matricula, nil
		}
		m1, err := matricular(e1.Id, sA.Id, "2020-02-15")
		if err != nil || m1.EstadoMatricula != "MATRICULADO" || m1.Seccion.GetId() != sA.Id || m1.EstudianteId != e1.Id || m1.Version != 1 || m1.Origen != "REGISTRO" {
			t.Fatalf("matricular: %+v err=%v", m1, err)
		}
		_, err = matricular(e1.Id, sB.Id, "")
		wantErr(t, err, codes.AlreadyExists, "duplicate_matricula", "segunda matrícula en el periodo")
		_, err = matricular(inactivo.Id, "", "")
		wantErr(t, err, codes.FailedPrecondition, "estudiante_inactivo", "matricular registro INACTIVO")
		_, err = matricular("00000000-0000-4000-8000-00000000dead", "", "")
		wantErr(t, err, codes.NotFound, "student_not_found", "matricular inexistente")
		_, err = matricular(e2.Id, "", "2999-01-01")
		wantErr(t, err, codes.InvalidArgument, "validation_error", "fecha de matrícula futura")
		_, err = h.client.MatricularEstudiante(h.as(lector), &estudiantes.MatricularEstudianteRequest{EstudianteId: e2.Id, PeriodoId: p.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "matricular sin matricula.update")

		// Vacantes: sA admite 2.
		m2, err := matricular(e2.Id, sA.Id, "2020-03-01")
		if err != nil {
			t.Fatalf("segunda matrícula en la sección: %v", err)
		}
		_, err = matricular(e3.Id, sA.Id, "")
		wantErr(t, err, codes.FailedPrecondition, "sin_vacantes", "sección llena")

		// Sección de otro periodo.
		otroP := crearPeriodo(t, anioBase+2)
		sOtro := crearSeccion(t, &estudiantes.CreateSeccionRequest{PeriodoId: otroP.Id, Nivel: "PRIMARIA", Grado: "1", Seccion: "A"})
		_, err = matricular(e3.Id, sOtro.Id, "")
		wantErr(t, err, codes.NotFound, "seccion_not_found", "sección de otro periodo")
		m3, err := matricular(e3.Id, "", "")
		if err != nil || m3.Seccion != nil || m3.FechaMatricula != hoy {
			t.Fatalf("matrícula sin sección y con fecha por defecto: %+v err=%v", m3, err)
		}

		// Cambio de sección (asignación desde vacío) y de turno.
		cambio, err := h.client.CambiarSeccionMatricula(h.as(admin), &estudiantes.CambiarSeccionMatriculaRequest{MatriculaId: m3.Id, ExpectedVersion: m3.Version, SeccionId: sB.Id})
		if err != nil || cambio.Matricula.Seccion.GetId() != sB.Id || cambio.Matricula.Version != 2 ||
			len(cambio.Matricula.Movimientos) != 1 || cambio.Matricula.Movimientos[0].Tipo != "CAMBIO_SECCION" || cambio.Matricula.Movimientos[0].SeccionAnteriorId != "" {
			t.Fatalf("asignar sección: %+v err=%v", cambio, err)
		}
		_, err = h.client.CambiarSeccionMatricula(h.as(admin), &estudiantes.CambiarSeccionMatriculaRequest{MatriculaId: m3.Id, ExpectedVersion: m3.Version, SeccionId: sA.Id})
		wantErr(t, err, codes.Aborted, "version_conflict", "versión vieja")
		_, err = h.client.CambiarSeccionMatricula(h.as(admin), &estudiantes.CambiarSeccionMatriculaRequest{MatriculaId: m3.Id, ExpectedVersion: 2, SeccionId: sB.Id})
		wantErr(t, err, codes.FailedPrecondition, "no_changes", "misma sección")
		turno, err := h.client.CambiarSeccionMatricula(h.as(admin), &estudiantes.CambiarSeccionMatriculaRequest{MatriculaId: m1.Id, ExpectedVersion: 1, SeccionId: sAtarde.Id, Motivo: "Pedido familiar"})
		if err != nil || turno.Matricula.Movimientos[0].Tipo != "CAMBIO_TURNO" || turno.Matricula.Movimientos[0].SeccionAnteriorId != sA.Id {
			t.Fatalf("cambio de turno: %+v err=%v", turno, err)
		}

		// Retiro, reingreso y traslado.
		_, err = h.client.RetirarMatricula(h.as(admin), &estudiantes.RetirarMatriculaRequest{MatriculaId: m2.Id, ExpectedVersion: 1, Tipo: "RETIRO"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "retiro sin motivo")
		_, err = h.client.RetirarMatricula(h.as(admin), &estudiantes.RetirarMatriculaRequest{MatriculaId: m2.Id, ExpectedVersion: 1, Tipo: "EXPULSION", Motivo: "x"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "tipo de retiro inválido")
		_, err = h.client.RetirarMatricula(h.as(admin), &estudiantes.RetirarMatriculaRequest{MatriculaId: m1.Id, ExpectedVersion: 2, Tipo: "RETIRO", Fecha: "2019-01-01", Motivo: "Antes"})
		wantErr(t, err, codes.InvalidArgument, "invalid_date", "retiro anterior a la matrícula")
		ret, err := h.client.RetirarMatricula(h.as(admin), &estudiantes.RetirarMatriculaRequest{MatriculaId: m2.Id, ExpectedVersion: 1, Tipo: "RETIRO", Fecha: "2021-05-10", Motivo: "Motivo reservado " + tag})
		if err != nil || ret.Matricula.EstadoMatricula != "RETIRADO" || ret.Matricula.FechaRetiro != "2021-05-10" {
			t.Fatalf("retiro: %+v err=%v", ret, err)
		}
		_, err = h.client.RetirarMatricula(h.as(admin), &estudiantes.RetirarMatriculaRequest{MatriculaId: m2.Id, ExpectedVersion: ret.Matricula.Version, Tipo: "RETIRO", Motivo: "Otra vez"})
		wantErr(t, err, codes.FailedPrecondition, "invalid_transition", "retirar dos veces")
		_, err = h.client.ReingresarMatricula(h.as(admin), &estudiantes.ReingresarMatriculaRequest{MatriculaId: m2.Id, ExpectedVersion: ret.Matricula.Version, Fecha: "2021-05-01"})
		wantErr(t, err, codes.InvalidArgument, "invalid_date", "reingreso antes del retiro")
		// La vacante liberada se ocupa: el reingreso a una sección llena se rechaza.
		e4 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Ocupa", ApellidoPaterno: "Vacante", ApellidoMaterno: tag})
		if _, err := matricular(e4.Id, sA.Id, ""); err != nil {
			t.Fatalf("ocupar vacante liberada: %v", err)
		}
		e5 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Llena", ApellidoPaterno: "Seccion", ApellidoMaterno: tag})
		if _, err := matricular(e5.Id, sA.Id, ""); err != nil {
			t.Fatalf("segunda vacante: %v", err)
		}
		_, err = h.client.ReingresarMatricula(h.as(admin), &estudiantes.ReingresarMatriculaRequest{MatriculaId: m2.Id, ExpectedVersion: ret.Matricula.Version})
		wantErr(t, err, codes.FailedPrecondition, "sin_vacantes", "reingreso a sección llena")
		tras, err := h.client.RetirarMatricula(h.as(admin), &estudiantes.RetirarMatriculaRequest{MatriculaId: m3.Id, ExpectedVersion: 2, Tipo: "traslado", Motivo: "Traslado a otra IE"})
		if err != nil || tras.Matricula.EstadoMatricula != "TRASLADADO" || tras.Matricula.Movimientos[len(tras.Matricula.Movimientos)-1].Tipo != "TRASLADO_SALIDA" {
			t.Fatalf("traslado: %+v err=%v", tras, err)
		}
		rein, err := h.client.ReingresarMatricula(h.as(admin), &estudiantes.ReingresarMatriculaRequest{MatriculaId: m3.Id, ExpectedVersion: tras.Matricula.Version, Motivo: "Regresa"})
		if err != nil || rein.Matricula.EstadoMatricula != "MATRICULADO" || rein.Matricula.FechaRetiro != "" {
			t.Fatalf("reingreso: %+v err=%v", rein, err)
		}

		// Conclusión: no en PLANIFICADO; sí en EN_CURSO.
		_, err = h.client.ConcluirMatricula(h.as(admin), &estudiantes.ConcluirMatriculaRequest{MatriculaId: rein.Matricula.Id, ExpectedVersion: rein.Matricula.Version})
		wantErr(t, err, codes.FailedPrecondition, "invalid_transition", "concluir en periodo PLANIFICADO")
		avanzar(t, p.Id, "EN_CURSO")
		fin, err := h.client.ConcluirMatricula(h.as(admin), &estudiantes.ConcluirMatriculaRequest{MatriculaId: rein.Matricula.Id, ExpectedVersion: rein.Matricula.Version})
		if err != nil || fin.Matricula.EstadoMatricula != "CONCLUIDO" {
			t.Fatalf("concluir: %+v err=%v", fin, err)
		}
		_, err = h.client.RetirarMatricula(h.as(admin), &estudiantes.RetirarMatriculaRequest{MatriculaId: fin.Matricula.Id, ExpectedVersion: fin.Matricula.Version, Tipo: "RETIRO", Motivo: "x"})
		wantErr(t, err, codes.FailedPrecondition, "invalid_transition", "retirar una matrícula concluida")

		// Periodo cerrado: solo se puede concluir.
		avanzar(t, p.Id, "CERRADO")
		_, err = h.client.CambiarSeccionMatricula(h.as(admin), &estudiantes.CambiarSeccionMatriculaRequest{MatriculaId: m1.Id, ExpectedVersion: turno.Matricula.Version, SeccionId: sB.Id})
		wantErr(t, err, codes.FailedPrecondition, "periodo_cerrado", "cambiar sección en periodo cerrado")
		_, err = matricular(inactivo.Id, "", "")
		wantErr(t, err, codes.FailedPrecondition, "estudiante_inactivo", "el registro se valida antes que el periodo")
		if _, err := h.client.ConcluirMatricula(h.as(admin), &estudiantes.ConcluirMatriculaRequest{MatriculaId: m1.Id, ExpectedVersion: turno.Matricula.Version}); err != nil {
			t.Fatalf("concluir en periodo cerrado: %v", err)
		}

		// Historial completo y en orden, sin perder nada.
		hist, err := h.client.ListMatriculasEstudiante(h.as(lector), &estudiantes.ListMatriculasEstudianteRequest{EstudianteId: e3.Id})
		if err != nil || len(hist.Matriculas) != 1 {
			t.Fatalf("historial: %+v err=%v", hist, err)
		}
		var tipos []string
		for _, mv := range hist.Matriculas[0].Movimientos {
			tipos = append(tipos, mv.Tipo)
		}
		if got := strings.Join(tipos, ","); got != "CAMBIO_SECCION,TRASLADO_SALIDA,REINGRESO,CONCLUSION" {
			t.Fatalf("movimientos del historial: %s", got)
		}

		// Nómina por periodo, filtros y ocupación.
		nom, err := h.client.ListMatriculasPeriodo(h.as(lector), &estudiantes.ListMatriculasPeriodoRequest{PeriodoId: p.Id})
		if err != nil || nom.Total != 5 || len(nom.Matriculas) != 5 || nom.Matriculas[0].Estudiante == nil {
			t.Fatalf("nómina: total=%d err=%v", nom.GetTotal(), err)
		}
		ret2, err := h.client.ListMatriculasPeriodo(h.as(lector), &estudiantes.ListMatriculasPeriodoRequest{PeriodoId: p.Id, EstadoMatricula: "retirado"})
		if err != nil || ret2.Total != 1 || ret2.Matriculas[0].Estudiante.Id != e2.Id {
			t.Fatalf("nómina de retirados: %+v err=%v", ret2, err)
		}
		porSeccion, err := h.client.ListMatriculasPeriodo(h.as(lector), &estudiantes.ListMatriculasPeriodoRequest{PeriodoId: p.Id, SeccionId: sA.Id, PageSize: 1})
		if err != nil || porSeccion.Total != 3 || len(porSeccion.Matriculas) != 1 {
			t.Fatalf("nómina por sección y paginada: total=%d err=%v", porSeccion.GetTotal(), err)
		}
		_, err = h.client.ListMatriculasPeriodo(h.as(lector), &estudiantes.ListMatriculasPeriodoRequest{PeriodoId: p.Id, EstadoMatricula: "ACTIVO"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "estado de nómina inválido")
		secs, err := h.client.ListSecciones(h.as(lector), &estudiantes.ListSeccionesRequest{PeriodoId: p.Id})
		if err != nil || len(secs.Secciones) != 3 {
			t.Fatalf("ListSecciones: %+v err=%v", secs, err)
		}
		for _, s := range secs.Secciones {
			if s.Id == sA.Id && s.Matriculados != 2 {
				t.Fatalf("ocupación de la sección A: %d", s.Matriculados)
			}
		}

		// Auditoría: una fila MATRICULA_UPDATE por operación, sin valores.
		if got := acciones(t, "estudiante_id", e3.Id); !strings.HasPrefix(got, "CREATE[") || !strings.HasSuffix(got, " "+
			"MATRICULA_UPDATE[matricula] MATRICULA_UPDATE[matricula,seccion] MATRICULA_UPDATE[matricula,estado_matricula] "+
			"MATRICULA_UPDATE[matricula,estado_matricula] MATRICULA_UPDATE[matricula,estado_matricula]") {
			t.Fatalf("auditoría de matrícula: %s", got)
		}
		var conMotivo int
		if err := h.admin.QueryRow(h.ctx, `SELECT count(*) FROM auditoria_estudiantes a WHERE row_to_json(a)::text ILIKE '%' || $1 || '%'`, "Motivo reservado").Scan(&conMotivo); err != nil || conMotivo != 0 {
			t.Fatalf("la auditoría no debe contener el motivo: %d err=%v", conMotivo, err)
		}
		// El motivo de retiro se guarda pero no se expone por la API.
		var motivo string
		if err := h.admin.QueryRow(h.ctx, `SELECT motivo FROM matricula_movimientos WHERE matricula_id = $1 AND tipo = 'RETIRO'`, m2.Id).Scan(&motivo); err != nil || !strings.HasSuffix(motivo, tag) {
			t.Fatalf("motivo del movimiento: err=%v", err)
		}
		// Las columnas académicas heredadas no se tocan (ADR-08 pendiente).
		var nivel string
		if err := h.admin.QueryRow(h.ctx, `SELECT nivel FROM estudiantes WHERE id = $1`, e1.Id).Scan(&nivel); err != nil || nivel != "" {
			t.Fatalf("columna heredada nivel modificada: %q err=%v", nivel, err)
		}
	})

	// ================= PERSONAS Y APODERADO =================
	t.Run("Personas_RegistroVinculosYApoderado", func(t *testing.T) {
		doc := dni(50)
		creada, err := h.client.CreatePersona(h.as(familiaU), &estudiantes.CreatePersonaRequest{Persona: &estudiantes.Persona{
			TipoDocumento: "dni", NumeroDocumento: doc, Nombres: "  María   José ", ApellidoPaterno: "Condori", ApellidoMaterno: tag}})
		if err != nil || creada.Persona.Nombres != "María José" || creada.Persona.TipoDocumento != "DNI" || creada.Persona.Version != 1 {
			t.Fatalf("CreatePersona: %+v err=%v", creada, err)
		}
		madre := creada.Persona
		h.personas = append(h.personas, madre.Id)
		_, err = h.client.CreatePersona(h.as(familiaU), &estudiantes.CreatePersonaRequest{Persona: &estudiantes.Persona{TipoDocumento: "DNI", NumeroDocumento: doc, Nombres: "Otra"}})
		wantErr(t, err, codes.AlreadyExists, "duplicate_persona", "persona con documento repetido")
		_, err = h.client.CreatePersona(h.as(familiaU), &estudiantes.CreatePersonaRequest{Persona: &estudiantes.Persona{TipoDocumento: "DNI", NumeroDocumento: "123", Nombres: "X"}})
		wantErr(t, err, codes.InvalidArgument, "invalid_document", "DNI inválido")
		_, err = h.client.CreatePersona(h.as(familiaV), &estudiantes.CreatePersonaRequest{Persona: &estudiantes.Persona{Nombres: "X"}})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "registrar sin familia.update")

		// Una persona, dos estudiantes.
		h1 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Hijo", ApellidoPaterno: "Uno", ApellidoMaterno: tag})
		h2 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Hija", ApellidoPaterno: "Dos", ApellidoMaterno: tag})
		var relaciones []string
		for _, hijo := range []string{h1.Id, h2.Id} {
			r, err := h.client.VincularFamiliar(h.as(familiaU), &estudiantes.VincularFamiliarRequest{EstudianteId: hijo, PersonaId: madre.Id, Parentesco: "MADRE"})
			if err != nil {
				t.Fatalf("vincular madre: %v", err)
			}
			relaciones = append(relaciones, r.Familiar.RelacionId)
		}

		// Rol de apoderado.
		ap, err := h.client.ActualizarFamiliar(h.as(familiaU), &estudiantes.ActualizarFamiliarRequest{EstudianteId: h1.Id, RelacionId: relaciones[0], EsApoderado: true})
		if err != nil || !ap.Familiar.EsApoderado {
			t.Fatalf("asignar apoderado: %+v err=%v", ap, err)
		}
		_, err = h.client.ActualizarFamiliar(h.as(familiaU), &estudiantes.ActualizarFamiliarRequest{EstudianteId: h1.Id, RelacionId: relaciones[0], EsApoderado: true})
		wantErr(t, err, codes.FailedPrecondition, "no_changes", "ya es apoderado")
		tio, err := h.client.VincularFamiliar(h.as(familiaU), &estudiantes.VincularFamiliarRequest{EstudianteId: h1.Id, Parentesco: "TIO_A",
			Persona: &estudiantes.Persona{Nombres: "Pedro", ApellidoPaterno: "Tío " + tag}})
		if err != nil {
			t.Fatal(err)
		}
		h.personas = append(h.personas, tio.Familiar.Persona.Id)
		_, err = h.client.ActualizarFamiliar(h.as(familiaU), &estudiantes.ActualizarFamiliarRequest{EstudianteId: h1.Id, RelacionId: tio.Familiar.RelacionId, EsApoderado: true})
		wantErr(t, err, codes.AlreadyExists, "duplicate_apoderado", "segundo apoderado vigente")
		_, err = h.client.ActualizarFamiliar(h.as(familiaU), &estudiantes.ActualizarFamiliarRequest{EstudianteId: h2.Id, RelacionId: relaciones[0], EsApoderado: true})
		wantErr(t, err, codes.NotFound, "relacion_not_found", "relación de otro estudiante")
		if _, err := h.client.ActualizarFamiliar(h.as(familiaU), &estudiantes.ActualizarFamiliarRequest{EstudianteId: h1.Id, RelacionId: relaciones[0], EsApoderado: false}); err != nil {
			t.Fatalf("quitar apoderado: %v", err)
		}
		if _, err := h.client.ActualizarFamiliar(h.as(familiaU), &estudiantes.ActualizarFamiliarRequest{EstudianteId: h1.Id, RelacionId: tio.Familiar.RelacionId, EsApoderado: true}); err != nil {
			t.Fatalf("cambiar de apoderado: %v", err)
		}
		if _, err := h.client.DesvincularFamiliar(h.as(familiaU), &estudiantes.DesvincularFamiliarRequest{EstudianteId: h2.Id, RelacionId: relaciones[1]}); err != nil {
			t.Fatal(err)
		}
		_, err = h.client.ActualizarFamiliar(h.as(familiaU), &estudiantes.ActualizarFamiliarRequest{EstudianteId: h2.Id, RelacionId: relaciones[1], EsApoderado: true})
		wantErr(t, err, codes.FailedPrecondition, "relacion_no_vigente", "apoderado en relación cerrada")

		// Consulta inversa: vigentes y con historial.
		g, err := h.client.GetPersona(h.as(familiaV), &estudiantes.GetPersonaRequest{Id: madre.Id})
		if err != nil || len(g.Vinculos) != 1 || g.Vinculos[0].Estudiante.GetId() != h1.Id || g.Vinculos[0].Parentesco != "MADRE" {
			t.Fatalf("GetPersona vigentes: %+v err=%v", g, err)
		}
		g, err = h.client.GetPersona(h.as(familiaV), &estudiantes.GetPersonaRequest{Id: madre.Id, IncluirHistorial: true})
		if err != nil || len(g.Vinculos) != 2 || g.Vinculos[1].Vigente || g.Vinculos[1].Hasta == "" {
			t.Fatalf("GetPersona con historial: %+v err=%v", g, err)
		}
		_, err = h.client.GetPersona(h.as(familiaV), &estudiantes.GetPersonaRequest{Id: "00000000-0000-4000-8000-00000000dead"})
		wantErr(t, err, codes.NotFound, "persona_not_found", "persona inexistente")
		_, err = h.client.GetPersona(h.as(lector), &estudiantes.GetPersonaRequest{Id: madre.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "persona sin familia.view")

		// Búsqueda por documento y por nombre sin tildes, en cualquier orden.
		for _, q := range []string{doc, "condori maria " + tag} {
			s, err := h.client.SearchPersonas(h.as(familiaV), &estudiantes.SearchPersonasRequest{Q: q})
			if err != nil || s.Total != 1 || s.Personas[0].Id != madre.Id {
				t.Fatalf("SearchPersonas: total=%d err=%v", s.GetTotal(), err)
			}
		}
		_, err = h.client.SearchPersonas(h.as(familiaV), &estudiantes.SearchPersonasRequest{Q: "x"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "búsqueda demasiado corta")

		// Edición con versión.
		up, err := h.client.UpdatePersona(h.as(familiaU), &estudiantes.UpdatePersonaRequest{Id: madre.Id, ExpectedVersion: 1, ApellidoMaterno: str("Corregido " + tag)})
		if err != nil || up.Persona.Version != 2 || up.Persona.ApellidoMaterno != "Corregido "+tag {
			t.Fatalf("UpdatePersona: %+v err=%v", up, err)
		}
		_, err = h.client.UpdatePersona(h.as(familiaU), &estudiantes.UpdatePersonaRequest{Id: madre.Id, ExpectedVersion: 1, Nombres: str("Otra")})
		wantErr(t, err, codes.Aborted, "version_conflict", "editar persona con versión vieja")
		_, err = h.client.UpdatePersona(h.as(familiaU), &estudiantes.UpdatePersonaRequest{Id: madre.Id, ExpectedVersion: 2, ApellidoMaterno: str("Corregido " + tag)})
		wantErr(t, err, codes.FailedPrecondition, "no_changes", "edición sin cambios")
		_, err = h.client.UpdatePersona(h.as(familiaU), &estudiantes.UpdatePersonaRequest{Id: madre.Id, ExpectedVersion: 2, TipoDocumento: str("SIN_DOCUMENTO")})
		wantErr(t, err, codes.InvalidArgument, "invalid_document", "SIN_DOCUMENTO con número")
		_, err = h.client.UpdatePersona(h.as(familiaU), &estudiantes.UpdatePersonaRequest{Id: madre.Id, ExpectedVersion: 2, Nombres: str("")})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "nombres vacío")
		_, err = h.client.UpdatePersona(h.as(familiaU), &estudiantes.UpdatePersonaRequest{Id: tio.Familiar.Persona.Id, ExpectedVersion: 1, TipoDocumento: str("DNI"), NumeroDocumento: str(doc)})
		wantErr(t, err, codes.AlreadyExists, "duplicate_persona", "documento de otra persona")

		// Auditoría de la persona: filas propias (sin estudiante) y sin valores.
		if got := acciones(t, "persona_id", madre.Id); got != "PERSONA_UPDATE[tipo_documento,numero_documento,nombres,apellido_paterno,apellido_materno] "+
			"PERSONA_VIEW[familiares] PERSONA_VIEW[familiares] PERSONA_UPDATE[apellido_materno]" {
			t.Fatalf("auditoría de la persona: %s", got)
		}
		var filas string
		if err := h.admin.QueryRow(h.ctx, `SELECT string_agg(row_to_json(a)::text, ' ') FROM auditoria_estudiantes a WHERE persona_id = $1 AND estudiante_id IS NULL`, madre.Id).Scan(&filas); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(filas, doc) || strings.Contains(filas, "Condori") || strings.Contains(filas, "Corregido") {
			t.Fatal("la auditoría de la persona contiene valores")
		}
		if got := acciones(t, "estudiante_id", h1.Id); !strings.Contains(got, "FAMILIA_UPDATE[familiares,es_apoderado]") {
			t.Fatalf("auditoría del cambio de apoderado: %s", got)
		}
	})

	// ================= RESOLUCIÓN PARA CONSUMIDORES =================
	t.Run("Resolver_PorCodigoSiagie", func(t *testing.T) {
		siagie := fmt.Sprintf("%014d", base%100_000_000_000_000)
		e := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Siagie", ApellidoPaterno: "Prueba", ApellidoMaterno: tag, CodigoSiagie: siagie})
		r, err := h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{CodigoSiagie: siagie})
		if err != nil || r.Estudiante.Id != e.Id {
			t.Fatalf("resolver por SIAGIE: %+v err=%v", r, err)
		}
		_, err = h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{CodigoSiagie: "123"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "SIAGIE con formato inválido")
		_, err = h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{CodigoSiagie: siagie, Codigo: e.Codigo})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "dos criterios")
		_, err = h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{CodigoSiagie: "99999999999999"})
		wantErr(t, err, codes.NotFound, "student_not_found", "SIAGIE inexistente")
		_, err = h.client.ResolveEstudiante(h.as(sinPermis), &estudiantes.ResolveEstudianteRequest{CodigoSiagie: siagie})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "resolver sin estudiantes.view")
	})

	// Ningún valor personal en los logs del servicio.
	if out := logBuf.String(); strings.Contains(out, dni(1)) || strings.Contains(out, dni(50)) || strings.Contains(out, "Motivo reservado") {
		t.Fatal("los logs contienen valores personales")
	}
}
