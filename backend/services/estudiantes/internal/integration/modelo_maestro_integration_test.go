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

// TestModeloMaestro verifica en PostgreSQL real las etapas del modelo
// maestro: resolución exacta y posibles duplicados (identidad), domicilio,
// contacto y edad (dominio personal), historial de matrícula y familia
// (personas normalizadas, parentesco, apoderado, vigencia), con permisos,
// auditoría sin valores y ausencia de valores en logs.
//
// Aislamiento: solo filas creadas por esta prueba (códigos y nombres con el
// tag de la ejecución); la limpieza las borra todas (helpers_test.go).
func TestModeloMaestro(t *testing.T) {
	h := newHarness(t)
	var logBuf syncBuffer
	logx.SetWriter(logx.NewWriter(&logBuf))
	t.Cleanup(func() { logx.Reset() })

	base := time.Now().UnixNano()
	tag := strconv.FormatInt(base%100_000_000, 36)
	n := 0
	codigo := func() string { n++; return fmt.Sprintf("MM-%s-%d", tag, n) }
	dni := func(i int) string { return fmt.Sprintf("2%07d", (base/1000+int64(i))%10_000_000) }

	const (
		admin    = "00000000-0000-4000-8000-0000000000b1" // todo el maestro
		lector   = "00000000-0000-4000-8000-0000000000b2" // solo estudiantes.view
		familiaV = "00000000-0000-4000-8000-0000000000b3" // familia.view
		familiaU = "00000000-0000-4000-8000-0000000000b4" // familia.update
	)
	for _, p := range []string{"estudiantes.view", "estudiantes.create", "estudiantes.update",
		"estudiantes.personal.view", "estudiantes.personal.update"} {
		h.grant(admin, p)
	}
	h.grant(lector, "estudiantes.view")
	h.grant(familiaV, "estudiantes.familia.view")
	h.grant(familiaU, "estudiantes.familia.update")

	str := func(s string) *string { return &s }
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
	auditoriaSinValores := func(t *testing.T, id string, valores ...string) {
		t.Helper()
		var filas string
		if err := h.admin.QueryRow(h.ctx, `SELECT COALESCE(string_agg(row_to_json(a)::text, ' '), '') FROM auditoria_estudiantes a WHERE estudiante_id = $1`, id).Scan(&filas); err != nil {
			t.Fatal(err)
		}
		for _, v := range valores {
			if v != "" && strings.Contains(strings.ToLower(filas), strings.ToLower(v)) {
				t.Fatalf("la auditoría contiene un valor (%s)", v)
			}
		}
	}
	acciones := func(t *testing.T, id string) string {
		t.Helper()
		var s string
		if err := h.admin.QueryRow(h.ctx, `SELECT COALESCE(string_agg(accion || ':' || array_to_string(campos, ','), ' ' ORDER BY fecha_hora, version_resultante), '')
			FROM auditoria_estudiantes WHERE estudiante_id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// Valores con marcas reconocibles (para auditoría y logs).
	const (
		direccion = "Jr. Tupac Amaru 777"
		correo    = "familia." + "mm@correo.pe"
		telefono  = "+51 984 777 111"
		distrito  = "Tinta"
	)

	// ================= ETAPA 1: IDENTIDAD =================
	t.Run("Resolver_PorCodigoYDocumentoExactos", func(t *testing.T) {
		d := dni(1)
		e := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Resolver", Apellidos: "Exacto " + tag, TipoDocumento: "DNI", NumeroDocumento: d})
		ce := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Resolver", Apellidos: "Extranjero " + tag, TipoDocumento: "CE", NumeroDocumento: "MM" + tag})

		r1, err := h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{Codigo: " " + e.Codigo + " "})
		if err != nil || r1.Estudiante.Id != e.Id {
			t.Fatalf("por código: %+v err=%v", r1, err)
		}
		r2, err := h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{TipoDocumento: "dni", NumeroDocumento: d})
		if err != nil || r2.Estudiante.Id != e.Id {
			t.Fatalf("por DNI: %+v err=%v", r2, err)
		}
		r3, err := h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{TipoDocumento: "CE", NumeroDocumento: strings.ToLower("MM" + tag)})
		if err != nil || r3.Estudiante.Id != ce.Id {
			t.Fatalf("por CE (documento extranjero): %+v err=%v", r3, err)
		}
		// Exacto: un prefijo no resuelve (para coincidencias parciales está la búsqueda).
		_, err = h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{Codigo: e.Codigo[:len(e.Codigo)-1]})
		wantErr(t, err, codes.NotFound, "student_not_found", "código parcial")
		_, err = h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{Codigo: e.Codigo, TipoDocumento: "DNI", NumeroDocumento: d})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "dos criterios")
		_, err = h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "sin criterio")
		_, err = h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{TipoDocumento: "SIN_DOCUMENTO"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "SIN_DOCUMENTO")
		_, err = h.client.ResolveEstudiante(h.as(lector), &estudiantes.ResolveEstudianteRequest{TipoDocumento: "DNI", NumeroDocumento: "123"})
		wantErr(t, err, codes.InvalidArgument, "invalid_document", "DNI inválido")
		_, err = h.client.ResolveEstudiante(h.as(familiaV), &estudiantes.ResolveEstudianteRequest{Codigo: e.Codigo})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "sin estudiantes.view")
		_, err = h.client.ResolveEstudiante(h.ctx, &estudiantes.ResolveEstudianteRequest{Codigo: e.Codigo})
		wantCode(t, err, codes.Unauthenticated, "sin token")
	})

	t.Run("PosiblesDuplicados_ParaRevisionHumana", func(t *testing.T) {
		// Mismo nombre escrito distinto (mayúsculas, tildes, espacios) y sin
		// documento: la unicidad por documento no protege.
		a := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "José", Apellidos: "Duplicado " + tag})
		b := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "  JOSE ", Apellidos: "duplicado   " + tag})
		// Mismo nombre y misma fecha, cada uno con su DNI.
		c := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Rosa", Apellidos: "Gemela " + tag, TipoDocumento: "DNI", NumeroDocumento: dni(2), FechaNacimiento: "2014-04-04"})
		d := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Rosa", Apellidos: "Gemela " + tag, TipoDocumento: "DNI", NumeroDocumento: dni(3), FechaNacimiento: "2014-04-04"})
		// Mismo nombre, otra fecha y con documento: no es sospechoso.
		crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Rosa", Apellidos: "Gemela " + tag, TipoDocumento: "DNI", NumeroDocumento: dni(4), FechaNacimiento: "2015-05-05"})

		resp, err := h.client.ListPosiblesDuplicados(h.as(admin), &estudiantes.ListPosiblesDuplicadosRequest{PageSize: 100})
		if err != nil {
			t.Fatalf("ListPosiblesDuplicados: %v", err)
		}
		grupos := map[string]string{}
		for _, g := range resp.Grupos {
			var ids []string
			for _, e := range g.Estudiantes {
				ids = append(ids, e.Id)
				if e.NumeroDocumento != "" && e.TipoDocumento == "" {
					t.Fatal("respuesta inconsistente")
				}
			}
			grupos[g.Criterio+"|"+strings.Join(ids, ",")] = g.Criterio
		}
		orden := func(x, y string) string {
			if x < y {
				return x + "," + y
			}
			return y + "," + x
		}
		if _, ok := grupos["MISMO_NOMBRE_SIN_DOCUMENTO|"+orden(a.Id, b.Id)]; !ok {
			t.Fatalf("faltó el grupo sin documento: %v", grupos)
		}
		if _, ok := grupos["MISMO_NOMBRE_Y_FECHA|"+orden(c.Id, d.Id)]; !ok {
			t.Fatalf("faltó el grupo mismo nombre y fecha: %v", grupos)
		}
		// Nada se fusionó: los cinco siguen existiendo por separado.
		for _, id := range []string{a.Id, b.Id, c.Id, d.Id} {
			if _, err := h.client.GetEstudiante(h.as(admin), &estudiantes.GetEstudianteRequest{Id: id}); err != nil {
				t.Fatalf("el reporte no debe modificar estudiantes: %v", err)
			}
		}
		// Paginación con límite.
		p1, err := h.client.ListPosiblesDuplicados(h.as(admin), &estudiantes.ListPosiblesDuplicadosRequest{Page: 1, PageSize: 1})
		if err != nil || len(p1.Grupos) != 1 || p1.Total < 2 {
			t.Fatalf("paginación: %d grupos, total %d, err=%v", len(p1.Grupos), p1.Total, err)
		}
		_, err = h.client.ListPosiblesDuplicados(h.as(lector), &estudiantes.ListPosiblesDuplicadosRequest{})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "reporte sin estudiantes.update")
	})

	// ================= ETAPA 2: DOMICILIO, CONTACTO Y EDAD =================
	t.Run("DomicilioContacto_EnDominioPersonal", func(t *testing.T) {
		e := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Domicilio", Apellidos: "Prueba " + tag, FechaNacimiento: "2013-07-21"})
		u, err := h.client.UpdateEstudiantePersonal(h.as(admin), &estudiantes.UpdateEstudiantePersonalRequest{
			Id: e.Id, ExpectedVersion: 1,
			DomicilioDireccion: str("  " + direccion + "  "), DomicilioDepartamento: str("Cusco"),
			DomicilioProvincia: str("Canchis"), DomicilioDistrito: str(distrito),
			Correo: str(" " + correo + " "), Telefono: str(telefono),
		})
		if err != nil || u.Estudiante.Version != 2 {
			t.Fatalf("UpdateEstudiantePersonal domicilio: %+v err=%v", u, err)
		}
		// Ubigeo no informado: queda NULL (no se deduce ni se inventa).
		var ubigeoNulo bool
		if err := h.admin.QueryRow(h.ctx, `SELECT domicilio_ubigeo IS NULL FROM estudiantes WHERE id = $1`, e.Id).Scan(&ubigeoNulo); err != nil || !ubigeoNulo {
			t.Fatalf("domicilio_ubigeo debe quedar NULL: err=%v", err)
		}
		p, err := h.client.GetEstudiantePersonal(h.as(admin), &estudiantes.GetEstudiantePersonalRequest{Id: e.Id, FechaReferencia: "2026-07-20"})
		if err != nil {
			t.Fatal(err)
		}
		pp := p.Personal
		if pp.DomicilioDireccion != direccion || pp.DomicilioDistrito != distrito || pp.Correo != correo ||
			pp.Telefono != telefono || pp.DomicilioUbigeo != "" || pp.Edad == nil || *pp.Edad != 12 {
			t.Fatalf("lectura personal: %+v", pp)
		}
		// Edad a la fecha de corte (el día del cumpleaños ya cumple 13).
		p13, _ := h.client.GetEstudiantePersonal(h.as(admin), &estudiantes.GetEstudiantePersonalRequest{Id: e.Id, FechaReferencia: "2026-07-21"})
		if p13.Personal.Edad == nil || *p13.Personal.Edad != 13 {
			t.Fatalf("edad al cumpleaños: %v", p13.Personal.Edad)
		}
		// Referencia anterior al nacimiento: sin edad (no se inventa).
		pAntes, _ := h.client.GetEstudiantePersonal(h.as(admin), &estudiantes.GetEstudiantePersonalRequest{Id: e.Id, FechaReferencia: "2010-01-01"})
		if pAntes.Personal.Edad != nil {
			t.Fatalf("no debe haber edad antes del nacimiento: %v", *pAntes.Personal.Edad)
		}
		_, err = h.client.GetEstudiantePersonal(h.as(admin), &estudiantes.GetEstudiantePersonalRequest{Id: e.Id, FechaReferencia: "20-07-2026"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "fecha de referencia inválida")

		for que, req := range map[string]*estudiantes.UpdateEstudiantePersonalRequest{
			"correo inválido":   {Correo: str("sin-arroba")},
			"teléfono inválido": {Telefono: str("12ab")},
			"ubigeo inválido":   {DomicilioUbigeo: str("0808")},
		} {
			req.Id, req.ExpectedVersion = e.Id, 2
			_, err := h.client.UpdateEstudiantePersonal(h.as(admin), req)
			wantErr(t, err, codes.InvalidArgument, "validation_error", que)
		}
		// Vaciar el contacto ("" = NULL).
		if _, err := h.client.UpdateEstudiantePersonal(h.as(admin), &estudiantes.UpdateEstudiantePersonalRequest{Id: e.Id, ExpectedVersion: 2, Telefono: str("")}); err != nil {
			t.Fatalf("vaciar teléfono: %v", err)
		}
		// Nunca en listados ni en la identidad básica.
		busq, err := h.client.SearchEstudiantes(h.as(lector), &estudiantes.SearchEstudiantesRequest{Q: e.Codigo})
		if err != nil || busq.Total != 1 || strings.Contains(fmt.Sprintf("%v", busq), direccion) {
			t.Fatalf("el listado no debe exponer domicilio: err=%v", err)
		}
		if a := acciones(t, e.Id); !strings.Contains(a, "PERSONAL_UPDATE:domicilio_direccion,domicilio_departamento,domicilio_provincia,domicilio_distrito,correo,telefono") ||
			!strings.Contains(a, "PERSONAL_UPDATE:telefono") {
			t.Fatalf("auditoría del domicilio: %s", a)
		}
		auditoriaSinValores(t, e.Id, direccion, correo, telefono, distrito, "Canchis")
	})

	// ================= ETAPA 3: MATRÍCULA =================
	t.Run("Matricula_HistorialPorPeriodo", func(t *testing.T) {
		e := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Matricula", Apellidos: "Historial " + tag})
		// Fixtures (la API de esta etapa es de lectura): años de prueba
		// fuera del uso real, únicos por ejecución.
		anio := 2090 + int(base%5)*2
		periodo := func(a int, estado string) string {
			var id string
			if err := h.admin.QueryRow(h.ctx, `INSERT INTO periodos_academicos (anio, nombre, estado) VALUES ($1, $2, $3) RETURNING id`,
				a, fmt.Sprintf("Periodo %d", a), estado).Scan(&id); err != nil {
				t.Fatalf("periodo %d: %v", a, err)
			}
			h.periodos = append(h.periodos, id)
			return id
		}
		p1, p2 := periodo(anio, "CERRADO"), periodo(anio+1, "EN_CURSO")
		seccion := func(periodoID, grado, sec string) string {
			var id string
			if err := h.admin.QueryRow(h.ctx, `INSERT INTO secciones (periodo_id, nivel, grado, seccion, turno, aula) VALUES ($1, 'PRIMARIA', $2, $3, 'MAÑANA', 'Aula 1') RETURNING id`,
				periodoID, grado, sec).Scan(&id); err != nil {
				t.Fatalf("sección: %v", err)
			}
			return id
		}
		s1, s2a, s2b := seccion(p1, "3", "A"), seccion(p2, "4", "A"), seccion(p2, "4", "B")
		var m1, m2 string
		if err := h.admin.QueryRow(h.ctx, `INSERT INTO matriculas (estudiante_id, periodo_id, seccion_id, estado_matricula, fecha_matricula) VALUES ($1, $2, $3, 'CONCLUIDO', '2090-03-01') RETURNING id`, e.Id, p1, s1).Scan(&m1); err != nil {
			t.Fatal(err)
		}
		if err := h.admin.QueryRow(h.ctx, `INSERT INTO matriculas (estudiante_id, periodo_id, seccion_id) VALUES ($1, $2, $3) RETURNING id`, e.Id, p2, s2a).Scan(&m2); err != nil {
			t.Fatal(err)
		}
		// Cambio de sección dentro del periodo: el historial NO se sobrescribe.
		if _, err := h.admin.Exec(h.ctx, `INSERT INTO matricula_movimientos (matricula_id, tipo, seccion_anterior_id, seccion_nueva_id, fecha, registrado_por) VALUES ($1, 'CAMBIO_SECCION', $2, $3, '2091-06-01', $4)`, m2, s2a, s2b, admin); err != nil {
			t.Fatal(err)
		}
		if _, err := h.admin.Exec(h.ctx, `UPDATE matriculas SET seccion_id = $2 WHERE id = $1`, m2, s2b); err != nil {
			t.Fatal(err)
		}

		resp, err := h.client.ListMatriculasEstudiante(h.as(lector), &estudiantes.ListMatriculasEstudianteRequest{EstudianteId: e.Id})
		if err != nil || len(resp.Matriculas) != 2 {
			t.Fatalf("ListMatriculasEstudiante: %+v err=%v", resp, err)
		}
		actual, anterior := resp.Matriculas[0], resp.Matriculas[1]
		if actual.Periodo.Anio != int32(anio+1) || actual.Seccion.Seccion != "B" || actual.Version != 2 || actual.EstadoMatricula != "MATRICULADO" ||
			len(actual.Movimientos) != 1 || actual.Movimientos[0].Tipo != "CAMBIO_SECCION" || actual.Movimientos[0].SeccionAnteriorId != s2a {
			t.Fatalf("matrícula vigente: %+v", actual)
		}
		if anterior.Periodo.Anio != int32(anio) || anterior.EstadoMatricula != "CONCLUIDO" || anterior.Seccion.Aula != "Aula 1" || anterior.FechaMatricula != "2090-03-01" {
			t.Fatalf("matrícula anterior (historial): %+v", anterior)
		}

		// Integridad de la base.
		if _, err := h.admin.Exec(h.ctx, `INSERT INTO matriculas (estudiante_id, periodo_id) VALUES ($1, $2)`, e.Id, p1); err == nil {
			t.Fatal("dos matrículas en el mismo periodo deben rechazarse")
		}
		otro := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Otra", Apellidos: "Seccion " + tag})
		if _, err := h.admin.Exec(h.ctx, `INSERT INTO matriculas (estudiante_id, periodo_id, seccion_id) VALUES ($1, $2, $3)`, otro.Id, p1, s2a); err == nil ||
			!strings.Contains(err.Error(), "fk_matriculas_seccion_mismo_periodo") {
			t.Fatalf("una sección de otro periodo debe rechazarse: %v", err)
		}
		if _, err := h.admin.Exec(h.ctx, `INSERT INTO matriculas (estudiante_id, periodo_id, estado_matricula) VALUES ($1, $2, 'ACTIVO')`, otro.Id, p1); err == nil {
			t.Fatal("estado de matrícula fuera de D-M4 debe rechazarse")
		}
		if _, err := h.admin.Exec(h.ctx, `UPDATE matricula_movimientos SET motivo = 'x' WHERE matricula_id = $1`, m2); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("los movimientos son append-only: %v", err)
		}
		// Mínimo privilegio (etapa 5, migración 009): la aplicación escribe
		// matrículas por los RPC, pero nunca borra el historial académico.
		if _, err := h.app.Exec(h.ctx, `DELETE FROM matriculas WHERE id = $1`, m1); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("estudiantes_user no debe poder borrar matrículas: %v", err)
		}
		if _, err := h.app.Exec(h.ctx, `DELETE FROM matricula_movimientos WHERE matricula_id = $1`, m2); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("estudiantes_user no debe poder borrar movimientos: %v", err)
		}

		vacio, err := h.client.ListMatriculasEstudiante(h.as(lector), &estudiantes.ListMatriculasEstudianteRequest{EstudianteId: otro.Id})
		if err != nil || len(vacio.Matriculas) != 0 {
			t.Fatalf("sin matrículas: %+v err=%v", vacio, err)
		}
		_, err = h.client.ListMatriculasEstudiante(h.as(lector), &estudiantes.ListMatriculasEstudianteRequest{EstudianteId: "00000000-0000-4000-8000-00000000dead"})
		wantErr(t, err, codes.NotFound, "student_not_found", "matrículas de inexistente")
		_, err = h.client.ListMatriculasEstudiante(h.as(familiaV), &estudiantes.ListMatriculasEstudianteRequest{EstudianteId: e.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "matrículas sin estudiantes.view")
	})

	// ================= ETAPA 4: FAMILIA =================
	t.Run("Familia_PersonasNormalizadasYRelaciones", func(t *testing.T) {
		hijo1 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Hijo", Apellidos: "Familia " + tag})
		hijo2 := crear(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Hija", Apellidos: "Familia " + tag})
		dniPadre := dni(10)
		padre := &estudiantes.Persona{TipoDocumento: "DNI", NumeroDocumento: dniPadre, Nombres: " Juan  Carlos ", ApellidoPaterno: "Familia", ApellidoMaterno: tag}
		vincular := func(as, estudianteID string, req *estudiantes.VincularFamiliarRequest) (*estudiantes.Familiar, error) {
			req.EstudianteId = estudianteID
			resp, err := h.client.VincularFamiliar(h.as(as), req)
			if err != nil {
				return nil, err
			}
			h.personas = append(h.personas, resp.Familiar.Persona.Id)
			return resp.Familiar, nil
		}

		f1, err := vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{Persona: padre, Parentesco: "padre", EsApoderado: true, Desde: "2020-01-01"})
		if err != nil || f1.Parentesco != "PADRE" || !f1.EsApoderado || !f1.Vigente || f1.Persona.Nombres != "Juan Carlos" {
			t.Fatalf("vincular padre: %+v err=%v", f1, err)
		}
		// El mismo padre (mismo DNI) para el hermano: UNA sola persona.
		f2, err := vincular(familiaU, hijo2.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{TipoDocumento: "DNI", NumeroDocumento: dniPadre, Nombres: "JUAN CARLOS"}, Parentesco: "PADRE"})
		if err != nil || f2.Persona.Id != f1.Persona.Id {
			t.Fatalf("la persona debe reutilizarse entre hermanos: %+v err=%v", f2, err)
		}
		// Mismo DNI con otros nombres: no se sobrescribe ni se fusiona.
		_, err = vincular(familiaU, hijo2.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{TipoDocumento: "DNI", NumeroDocumento: dniPadre, Nombres: "Pedro"}, Parentesco: "TIO_A"})
		wantErr(t, err, codes.FailedPrecondition, "persona_mismatch", "mismo DNI con otros nombres")
		// Madre sin documento (no se inventa uno).
		madre, err := vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{Nombres: "María", ApellidoPaterno: "Sin Documento"}, Parentesco: "MADRE", Desde: "2019-01-01"})
		if err != nil || madre.Persona.TipoDocumento != "SIN_DOCUMENTO" || madre.Persona.NumeroDocumento != "" {
			t.Fatalf("madre sin documento: %+v err=%v", madre, err)
		}
		// Reglas: un PADRE vigente, un apoderado vigente, una relación por persona.
		_, err = vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{Nombres: "Otro Padre"}, Parentesco: "PADRE"})
		wantErr(t, err, codes.AlreadyExists, "duplicate_progenitor", "segundo padre vigente")
		_, err = vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{PersonaId: madre.Persona.Id, Parentesco: "TUTOR_LEGAL"})
		wantErr(t, err, codes.AlreadyExists, "duplicate_familiar", "misma persona dos veces")
		_, err = vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{Nombres: "Tía"}, Parentesco: "TIO_A", EsApoderado: true})
		wantErr(t, err, codes.AlreadyExists, "duplicate_apoderado", "segundo apoderado vigente")
		_, err = vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{Nombres: "Primo"}, Parentesco: "PRIMO"})
		wantErr(t, err, codes.InvalidArgument, "invalid_parentesco", "parentesco fuera del catálogo")
		_, err = vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{Nombres: "X", TipoDocumento: "DNI", NumeroDocumento: "123"}, Parentesco: "OTRO"})
		wantErr(t, err, codes.InvalidArgument, "invalid_document", "documento inválido de la persona")
		_, err = vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{Parentesco: "OTRO"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "sin persona")
		_, err = vincular(familiaU, "00000000-0000-4000-8000-00000000dead", &estudiantes.VincularFamiliarRequest{PersonaId: madre.Persona.Id, Parentesco: "MADRE"})
		wantErr(t, err, codes.NotFound, "student_not_found", "estudiante inexistente")

		// Lectura (vigentes) y desvinculación sin borrar (historial).
		lista, err := h.client.ListFamiliaresEstudiante(h.as(familiaV), &estudiantes.ListFamiliaresEstudianteRequest{EstudianteId: hijo1.Id})
		if err != nil || len(lista.Familiares) != 2 || lista.Familiares[0].Parentesco != "PADRE" || lista.Familiares[1].Parentesco != "MADRE" {
			t.Fatalf("familiares vigentes: %+v err=%v", lista, err)
		}
		cerrada, err := h.client.DesvincularFamiliar(h.as(familiaU), &estudiantes.DesvincularFamiliarRequest{EstudianteId: hijo1.Id, RelacionId: f1.RelacionId, Hasta: "2026-09-01"})
		if err != nil || cerrada.Familiar.Vigente || cerrada.Familiar.Hasta != "2026-09-01" {
			t.Fatalf("desvincular: %+v err=%v", cerrada, err)
		}
		_, err = h.client.DesvincularFamiliar(h.as(familiaU), &estudiantes.DesvincularFamiliarRequest{EstudianteId: hijo1.Id, RelacionId: f1.RelacionId})
		wantErr(t, err, codes.FailedPrecondition, "relacion_no_vigente", "desvincular dos veces")
		_, err = h.client.DesvincularFamiliar(h.as(familiaU), &estudiantes.DesvincularFamiliarRequest{EstudianteId: hijo1.Id, RelacionId: f2.RelacionId})
		wantErr(t, err, codes.NotFound, "relacion_not_found", "relación de otro estudiante")
		_, err = h.client.DesvincularFamiliar(h.as(familiaU), &estudiantes.DesvincularFamiliarRequest{EstudianteId: hijo1.Id, RelacionId: madre.RelacionId, Hasta: "1999-01-01"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "hasta anterior a desde")
		vigentes, _ := h.client.ListFamiliaresEstudiante(h.as(familiaV), &estudiantes.ListFamiliaresEstudianteRequest{EstudianteId: hijo1.Id})
		historial, _ := h.client.ListFamiliaresEstudiante(h.as(familiaV), &estudiantes.ListFamiliaresEstudianteRequest{EstudianteId: hijo1.Id, IncluirHistorial: true})
		if len(vigentes.Familiares) != 1 || len(historial.Familiares) != 2 {
			t.Fatalf("vigentes=%d historial=%d", len(vigentes.Familiares), len(historial.Familiares))
		}
		// Con el padre desvinculado, se puede registrar un nuevo apoderado.
		if _, err := vincular(familiaU, hijo1.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{Nombres: "Abuela"}, Parentesco: "ABUELO_A", EsApoderado: true}); err != nil {
			t.Fatalf("nuevo apoderado tras cerrar el anterior: %v", err)
		}

		// Permisos: dominio familiar separado.
		_, err = h.client.ListFamiliaresEstudiante(h.as(lector), &estudiantes.ListFamiliaresEstudianteRequest{EstudianteId: hijo1.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "familiares con solo estudiantes.view")
		_, err = h.client.ListFamiliaresEstudiante(h.as(familiaU), &estudiantes.ListFamiliaresEstudianteRequest{EstudianteId: hijo1.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "familiares con solo familia.update")
		_, err = vincular(familiaV, hijo1.Id, &estudiantes.VincularFamiliarRequest{Persona: &estudiantes.Persona{Nombres: "X"}, Parentesco: "OTRO"})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "vincular con solo familia.view")
		_, err = h.client.ListFamiliaresEstudiante(h.as(familiaV), &estudiantes.ListFamiliaresEstudianteRequest{EstudianteId: "00000000-0000-4000-8000-00000000dead"})
		wantErr(t, err, codes.NotFound, "student_not_found", "familiares de inexistente")

		// La identidad básica nunca trae familiares.
		for _, c := range []string{"familiares", "padre", "madre", "persona"} {
			fields := (&estudiantes.EstudianteBasico{}).ProtoReflect().Descriptor().Fields()
			for i := 0; i < fields.Len(); i++ {
				if strings.Contains(string(fields.Get(i).Name()), c) {
					t.Fatalf("EstudianteBasico expone datos familiares: %s", fields.Get(i).Name())
				}
			}
		}
		// Auditoría: acciones del dominio familiar, sin valores.
		a := acciones(t, hijo1.Id)
		if !strings.Contains(a, "FAMILIA_UPDATE:familiares") || !strings.Contains(a, "FAMILIA_VIEW:familiares") {
			t.Fatalf("auditoría familiar: %s", a)
		}
		auditoriaSinValores(t, hijo1.Id, dniPadre, "Juan Carlos", "María", "Abuela")
		// Integridad: una persona relacionada con dos estudiantes.
		var relaciones int
		if err := h.admin.QueryRow(h.ctx, `SELECT count(*) FROM estudiante_familiares WHERE persona_id = $1`, f1.Persona.Id).Scan(&relaciones); err != nil || relaciones != 2 {
			t.Fatalf("la persona compartida debe tener 2 relaciones: %d err=%v", relaciones, err)
		}
		var personasConDNI int
		if err := h.admin.QueryRow(h.ctx, `SELECT count(*) FROM personas WHERE tipo_documento = 'DNI' AND numero_documento = $1`, dniPadre).Scan(&personasConDNI); err != nil || personasConDNI != 1 {
			t.Fatalf("no debe duplicarse la persona: %d err=%v", personasConDNI, err)
		}
	})

	// ================= LOGS =================
	t.Run("Logs_SinValores", func(t *testing.T) {
		logx.Error("sonda-captura-mm")
		logs := logBuf.String()
		if !strings.Contains(logs, "sonda-captura-mm") {
			t.Fatal("la captura de logs no está funcionando")
		}
		for _, v := range []string{direccion, correo, telefono, "Juan Carlos", "María", dni(10), dni(1)} {
			if strings.Contains(logs, v) {
				t.Fatalf("los logs contienen un valor (%s)", v)
			}
		}
	})
}
