package integration

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"estudiantes/estudiantes"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestEscrituraMaestroE05 verifica en PostgreSQL real la escritura del
// maestro (FASE E-05): alta, edición parcial, cambio de estado, control de
// concurrencia por version, auditoría append-only y permisos.
func TestEscrituraMaestroE05(t *testing.T) {
	h := newHarness(t)
	base := time.Now().UnixNano()
	tag := strconv.FormatInt(base%100_000_000, 36)
	n := 0
	codigo := func() string { n++; return fmt.Sprintf("E05-%s-%d", tag, n) }
	dni := func(i int) string { return fmt.Sprintf("3%07d", (base/1000+int64(i))%10_000_000) }
	siagie := func(i int) string { return fmt.Sprintf("2%013d", (base+int64(i))%10_000_000_000_000) }

	// actor: todos los permisos de escritura. otro: también escribe (para
	// verificar que la auditoría registra a quien de verdad llamó).
	actor := "00000000-0000-4000-8000-0000000000e5"
	otro := "00000000-0000-4000-8000-0000000000e6"
	soloLectura := "00000000-0000-4000-8000-0000000000e7"
	for _, u := range []string{actor, otro} {
		// FASE E-08: el alta con datos personales exige además personal.update.
		for _, p := range []string{"estudiantes.view", "estudiantes.create", "estudiantes.update", "estudiantes.change_estado", "estudiantes.personal.update"} {
			h.grant(u, p)
		}
	}
	h.grant(soloLectura, "estudiantes.view")

	crear := func(as string, req *estudiantes.CreateEstudianteRequest) (*estudiantes.EstudianteBasico, error) {
		resp, err := h.client.CreateEstudiante(h.as(as), req)
		if err != nil {
			return nil, err
		}
		h.created = append(h.created, resp.Estudiante.Id)
		return resp.Estudiante, nil
	}
	crearOK := func(t *testing.T, req *estudiantes.CreateEstudianteRequest) *estudiantes.EstudianteBasico {
		t.Helper()
		e, err := crear(actor, req)
		if err != nil {
			t.Fatalf("CreateEstudiante: %v", err)
		}
		return e
	}
	basico := func(nombres, apellidos string) *estudiantes.CreateEstudianteRequest {
		return &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: nombres, Apellidos: apellidos}
	}
	update := func(as string, req *estudiantes.UpdateEstudianteRequest) (*estudiantes.EstudianteBasico, error) {
		resp, err := h.client.UpdateEstudiante(h.as(as), req)
		if err != nil {
			return nil, err
		}
		return resp.Estudiante, nil
	}
	cambiarEstado := func(as, id string, version int32, estado string) (*estudiantes.EstudianteBasico, error) {
		resp, err := h.client.ChangeEstadoEstudiante(h.as(as), &estudiantes.ChangeEstadoEstudianteRequest{Id: id, ExpectedVersion: version, Estado: estado})
		if err != nil {
			return nil, err
		}
		return resp.Estudiante, nil
	}
	// wantErr: código gRPC + código estable al inicio del mensaje.
	wantErr := func(t *testing.T, err error, code codes.Code, prefijo, que string) {
		t.Helper()
		st := status.Convert(err)
		if err == nil || st.Code() != code || !strings.HasPrefix(st.Message(), prefijo+":") {
			t.Fatalf("%s: se esperaba %v %q, se obtuvo %v", que, code, prefijo, err)
		}
	}
	str := func(s string) *string { return &s }
	auditorias := func(t *testing.T, id string) []string {
		t.Helper()
		rows, err := h.admin.Query(h.ctx, `SELECT accion || ':' || usuario_id || ':' || version_resultante || ':' ||
			array_to_string(campos, ',') || ':' || COALESCE(estado_anterior, '') || '>' || COALESCE(estado_nuevo, '')
			FROM auditoria_estudiantes WHERE estudiante_id = $1 ORDER BY version_resultante, fecha_hora`, id)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			out = append(out, s)
		}
		return out
	}
	contarAuditoria := func(t *testing.T) int {
		t.Helper()
		var c int
		if err := h.admin.QueryRow(h.ctx, `SELECT count(*) FROM auditoria_estudiantes WHERE usuario_id = ANY($1::uuid[])`, []string{actor, otro}).Scan(&c); err != nil {
			t.Fatal(err)
		}
		return c
	}

	// ================= CREATE =================
	t.Run("Create_Valido_Completo_YNormalizado", func(t *testing.T) {
		cod := codigo()
		e := crearOK(t, &estudiantes.CreateEstudianteRequest{
			Codigo: "  " + cod + " ", Nombres: "  josé   Luis ", ApellidoPaterno: " DE  LA CRUZ ", ApellidoMaterno: "quispe",
			TipoDocumento: " dni ", NumeroDocumento: " " + dni(1) + " ", CodigoSiagie: siagie(1), FechaNacimiento: "2015-03-04",
			Sexo: "m", PaisNacimiento: "pe", UbigeoNacimiento: "150101", LenguaMaterna: "castellano", SegundaLengua: "quechua",
			FotoRef: "fotos/e05.jpg",
		})
		if e.Codigo != cod || e.Nombres != "josé Luis" || e.ApellidoPaterno != "DE LA CRUZ" || e.ApellidoMaterno != "quispe" ||
			e.Apellidos != "DE LA CRUZ quispe" || e.TipoDocumento != "DNI" || e.NumeroDocumento != dni(1) ||
			e.CodigoSiagie != siagie(1) || e.Estado != "ACTIVO" || e.Version != 1 || e.EstadoMatricula != "ACTIVO" {
			t.Fatalf("respuesta del alta: %+v", e)
		}
		var sexo, pais, lm, sl, fecha, nivel string
		if err := h.admin.QueryRow(h.ctx, `SELECT sexo, pais_nacimiento, lengua_materna, segunda_lengua, fecha_nacimiento::text, nivel
			FROM estudiantes WHERE id = $1`, e.Id).Scan(&sexo, &pais, &lm, &sl, &fecha, &nivel); err != nil {
			t.Fatal(err)
		}
		if sexo != "M" || pais != "PE" || lm != "CASTELLANO" || sl != "QUECHUA" || fecha != "2015-03-04" || nivel != "" {
			t.Fatalf("valores guardados: sexo=%s pais=%s lenguas=%s/%s fecha=%s nivel=%q", sexo, pais, lm, sl, fecha, nivel)
		}
		aud := auditorias(t, e.Id)
		if len(aud) != 1 || !strings.HasPrefix(aud[0], "CREATE:"+actor+":1:") || !strings.Contains(aud[0], "numero_documento") {
			t.Fatalf("auditoría del alta: %v", aud)
		}
		// La auditoría guarda nombres de campos, nunca valores.
		var fila string
		if err := h.admin.QueryRow(h.ctx, `SELECT row_to_json(a)::text FROM auditoria_estudiantes a WHERE estudiante_id = $1`, e.Id).Scan(&fila); err != nil {
			t.Fatal(err)
		}
		for _, valor := range []string{dni(1), siagie(1), "2015-03-04", "José", "josé", "CRUZ", "150101"} {
			if strings.Contains(fila, valor) {
				t.Fatalf("la auditoría contiene un valor del estudiante (%s)", valor)
			}
		}
	})

	t.Run("Create_SinDocumento_Varios_YPorDefecto", func(t *testing.T) {
		a := crearOK(t, basico("Sin", "Documento Uno"))
		b := crearOK(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Sin", Apellidos: "Documento Dos", TipoDocumento: "SIN_DOCUMENTO"})
		for _, e := range []*estudiantes.EstudianteBasico{a, b} {
			if e.TipoDocumento != "SIN_DOCUMENTO" || e.NumeroDocumento != "" {
				t.Fatalf("sin documento: %+v", e)
			}
		}
	})

	t.Run("Create_CE_Pasaporte_YMismoNumeroDistintoTipo", func(t *testing.T) {
		num := fmt.Sprintf("ce%06d", base%1_000_000)
		ce := crearOK(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Ana", Apellidos: "Extranjera", TipoDocumento: "ce", NumeroDocumento: num})
		if ce.TipoDocumento != "CE" || ce.NumeroDocumento != strings.ToUpper(num) {
			t.Fatalf("CE normalizado: %+v", ce)
		}
		pas := crearOK(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Ana", Apellidos: "Viajera", TipoDocumento: "PASAPORTE", NumeroDocumento: num})
		if pas.TipoDocumento != "PASAPORTE" {
			t.Fatalf("pasaporte: %+v", pas)
		}
	})

	t.Run("Create_Duplicados", func(t *testing.T) {
		cod := codigo()
		d := dni(2)
		ce := fmt.Sprintf("DUP%06d", base%1_000_000)
		crearOK(t, &estudiantes.CreateEstudianteRequest{Codigo: cod, Nombres: "Uno", Apellidos: "Original", NumeroDocumento: d, TipoDocumento: "DNI", CodigoSiagie: siagie(2)})
		crearOK(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Dos", Apellidos: "Original", TipoDocumento: "CE", NumeroDocumento: ce})
		antes := contarAuditoria(t)

		_, err := crear(actor, &estudiantes.CreateEstudianteRequest{Codigo: cod, Nombres: "X", Apellidos: "Y"})
		wantErr(t, err, codes.AlreadyExists, "duplicate_codigo", "código duplicado")
		_, err = crear(actor, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", TipoDocumento: "DNI", NumeroDocumento: d})
		wantErr(t, err, codes.AlreadyExists, "duplicate_document", "DNI duplicado")
		if strings.Contains(err.Error(), d) {
			t.Fatalf("el error expone el DNI")
		}
		_, err = crear(actor, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", TipoDocumento: "CE", NumeroDocumento: strings.ToLower(ce)})
		wantErr(t, err, codes.AlreadyExists, "duplicate_document", "CE duplicado")
		_, err = crear(actor, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", CodigoSiagie: siagie(2)})
		wantErr(t, err, codes.AlreadyExists, "duplicate_codigo_siagie", "SIAGIE duplicado")
		if strings.Contains(err.Error(), "SQLSTATE") || strings.Contains(err.Error(), "duplicate key") {
			t.Fatalf("el error expone detalles de PostgreSQL: %v", err)
		}
		if despues := contarAuditoria(t); despues != antes {
			t.Fatalf("las altas fallidas generaron auditoría (%d -> %d)", antes, despues)
		}
	})

	t.Run("Create_DocumentoInvalido", func(t *testing.T) {
		casos := []struct{ que, tipo, num string }{
			{"DNI de 7 dígitos", "DNI", "1234567"},
			{"DNI con letras", "DNI", "1234567A"},
			{"DNI sin número", "DNI", ""},
			{"CE sin número", "CE", ""},
			{"PASAPORTE sin número", "PASAPORTE", ""},
			{"CE demasiado corto", "CE", "AB1"},
			{"CE con símbolos", "CE", "AB-123"},
			{"SIN_DOCUMENTO con número", "SIN_DOCUMENTO", "12345678"},
			{"tipo fuera del catálogo", "OTRO", "12345678"},
		}
		for _, c := range casos {
			_, err := crear(actor, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", TipoDocumento: c.tipo, NumeroDocumento: c.num})
			wantErr(t, err, codes.InvalidArgument, "invalid_document", c.que)
		}
	})

	t.Run("Create_DatosInvalidos", func(t *testing.T) {
		antes := contarAuditoria(t)
		casos := []struct {
			que    string
			req    *estudiantes.CreateEstudianteRequest
			codigo string
		}{
			{"sin código", &estudiantes.CreateEstudianteRequest{Nombres: "X", Apellidos: "Y"}, "validation_error"},
			{"código demasiado largo", &estudiantes.CreateEstudianteRequest{Codigo: strings.Repeat("C", 21), Nombres: "X", Apellidos: "Y"}, "validation_error"},
			{"sin nombres", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "   ", Apellidos: "Y"}, "validation_error"},
			{"sin apellidos", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X"}, "validation_error"},
			{"materno sin paterno", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", ApellidoMaterno: "Y"}, "validation_error"},
			{"apellidos incoherentes", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", ApellidoPaterno: "PEREZ", ApellidoMaterno: "LOPEZ", Apellidos: "GARCIA LOPEZ"}, "validation_error"},
			{"SIAGIE con formato inválido", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", CodigoSiagie: "123"}, "validation_error"},
			{"fecha imposible", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", FechaNacimiento: "2015-13-40"}, "validation_error"},
			{"fecha futura", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", FechaNacimiento: time.Now().AddDate(1, 0, 0).Format("2006-01-02")}, "validation_error"},
			{"sexo inválido", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", Sexo: "X"}, "validation_error"},
			{"país de 3 letras", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", PaisNacimiento: "PER"}, "validation_error"},
			{"ubigeo sin país PE", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", PaisNacimiento: "CL", UbigeoNacimiento: "150101"}, "validation_error"},
			{"lenguas iguales", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", LenguaMaterna: "QUECHUA", SegundaLengua: "quechua"}, "validation_error"},
			{"foto como URL", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", FotoRef: "https://x.com/f.jpg"}, "validation_error"},
			{"nombre con caracteres de control", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X\x00", Apellidos: "Y"}, "validation_error"},
			{"estado de matrícula", &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "X", Apellidos: "Y", Estado: "RETIRADO"}, "invalid_state"},
		}
		for _, c := range casos {
			_, err := crear(actor, c.req)
			wantErr(t, err, codes.InvalidArgument, c.codigo, c.que)
		}
		if despues := contarAuditoria(t); despues != antes {
			t.Fatalf("las altas inválidas generaron auditoría (%d -> %d)", antes, despues)
		}
		var huerfanos int
		if err := h.admin.QueryRow(h.ctx, `SELECT count(*) FROM estudiantes e WHERE codigo LIKE $1
			AND NOT EXISTS (SELECT 1 FROM auditoria_estudiantes a WHERE a.estudiante_id = e.id)`, "E05-"+tag+"-%").Scan(&huerfanos); err != nil {
			t.Fatal(err)
		}
		if huerfanos != 0 {
			t.Fatalf("%d estudiante(s) creados sin auditoría", huerfanos)
		}
	})

	// ================= UPDATE =================
	t.Run("Update_Valido_Parcial_YAuditado", func(t *testing.T) {
		e := crearOK(t, basico("Rosa", "Tello"))
		u, err := update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str("  Rosa   María ")})
		if err != nil || u.Nombres != "Rosa María" || u.Version != 2 || u.Apellidos != "Tello" {
			t.Fatalf("editar nombres: %+v err=%v", u, err)
		}
		// Apellidos separados: apellidos se deriva.
		u, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 2, ApellidoPaterno: str("TELLO"), ApellidoMaterno: str("DE LOS SANTOS")})
		if err != nil || u.Apellidos != "TELLO DE LOS SANTOS" || u.ApellidoMaterno != "DE LOS SANTOS" || u.Version != 3 {
			t.Fatalf("editar apellidos: %+v err=%v", u, err)
		}
		// Documento: SIN_DOCUMENTO -> DNI.
		u, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 3, TipoDocumento: str("DNI"), NumeroDocumento: str(dni(3))})
		if err != nil || u.TipoDocumento != "DNI" || u.NumeroDocumento != dni(3) || u.Version != 4 {
			t.Fatalf("editar documento: %+v err=%v", u, err)
		}
		// SIAGIE y un opcional que luego se limpia (FASE E-08: foto_ref; los
		// datos personales ya no se editan con UpdateEstudiante).
		u, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 4, CodigoSiagie: str(siagie(3)), FotoRef: str("fotos/e05.jpg")})
		if err != nil || u.CodigoSiagie != siagie(3) || u.Version != 5 {
			t.Fatalf("editar SIAGIE: %+v err=%v", u, err)
		}
		u, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 5, FotoRef: str(""), CodigoSiagie: str("")})
		if err != nil || u.CodigoSiagie != "" || u.Version != 6 {
			t.Fatalf("limpiar opcionales: %+v err=%v", u, err)
		}
		var fotoNula bool
		if err := h.admin.QueryRow(h.ctx, `SELECT foto_ref IS NULL FROM estudiantes WHERE id = $1`, e.Id).Scan(&fotoNula); err != nil || !fotoNula {
			t.Fatalf("foto_ref debía quedar NULL (err=%v)", err)
		}
		aud := auditorias(t, e.Id)
		want := []string{
			"CREATE:" + actor + ":1:",
			"UPDATE:" + actor + ":2:nombres:>",
			"UPDATE:" + actor + ":3:apellido_paterno,apellido_materno,apellidos:>",
			"UPDATE:" + actor + ":4:tipo_documento,numero_documento:>",
			"UPDATE:" + actor + ":5:codigo_siagie,foto_ref:>",
			"UPDATE:" + actor + ":6:codigo_siagie,foto_ref:>",
		}
		if len(aud) != len(want) {
			t.Fatalf("auditoría: %v", aud)
		}
		for i := range want {
			if !strings.HasPrefix(aud[i], want[i]) {
				t.Fatalf("auditoría[%d] = %q, se esperaba prefijo %q", i, aud[i], want[i])
			}
		}
	})

	t.Run("Update_SinCambios_NoVersionNiAuditoria", func(t *testing.T) {
		e := crearOK(t, basico("Igual", "Que Antes"))
		u, err := update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str(" Igual ")})
		if err != nil || u.Version != 1 {
			t.Fatalf("sin cambios reales no debe crear versión: %+v err=%v", u, err)
		}
		if aud := auditorias(t, e.Id); len(aud) != 1 {
			t.Fatalf("sin cambios no debe auditarse: %v", aud)
		}
	})

	t.Run("Update_Documento_Reglas", func(t *testing.T) {
		e := crearOK(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Doc", Apellidos: "Reglas", TipoDocumento: "DNI", NumeroDocumento: dni(4)})
		// Pasar a SIN_DOCUMENTO sin vaciar el número: no se corrige en silencio.
		_, err := update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, TipoDocumento: str("SIN_DOCUMENTO")})
		wantErr(t, err, codes.InvalidArgument, "invalid_document", "SIN_DOCUMENTO conservando número")
		u, err := update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, TipoDocumento: str("SIN_DOCUMENTO"), NumeroDocumento: str("")})
		if err != nil || u.TipoDocumento != "SIN_DOCUMENTO" || u.NumeroDocumento != "" {
			t.Fatalf("pasar a SIN_DOCUMENTO: %+v err=%v", u, err)
		}
		_, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 2, TipoDocumento: str("DNI"), NumeroDocumento: str("99")})
		wantErr(t, err, codes.InvalidArgument, "invalid_document", "DNI inválido en edición")
		_, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 2, Nombres: str("")})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "nombres vacío")
		_, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 2, ApellidoMaterno: str("SOLO")})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "materno sin paterno")
	})

	t.Run("Update_Duplicados_EInexistente", func(t *testing.T) {
		a := crearOK(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "A", Apellidos: "Dup", TipoDocumento: "DNI", NumeroDocumento: dni(5), CodigoSiagie: siagie(5)})
		b := crearOK(t, basico("B", "Dup"))
		antes := contarAuditoria(t)
		_, err := update(actor, &estudiantes.UpdateEstudianteRequest{Id: b.Id, ExpectedVersion: 1, TipoDocumento: str("DNI"), NumeroDocumento: str(dni(5))})
		wantErr(t, err, codes.AlreadyExists, "duplicate_document", "DNI duplicado en edición")
		_, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: b.Id, ExpectedVersion: 1, CodigoSiagie: str(a.CodigoSiagie)})
		wantErr(t, err, codes.AlreadyExists, "duplicate_codigo_siagie", "SIAGIE duplicado en edición")
		_, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: "00000000-0000-4000-8000-00000000dead", ExpectedVersion: 1, Nombres: str("X")})
		wantErr(t, err, codes.NotFound, "student_not_found", "estudiante inexistente")
		_, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: "no-es-uuid", ExpectedVersion: 1, Nombres: str("X")})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "id inválido")
		_, err = update(actor, &estudiantes.UpdateEstudianteRequest{Id: b.Id, Nombres: str("X")})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "sin expected_version")
		if despues := contarAuditoria(t); despues != antes {
			t.Fatalf("ediciones fallidas generaron auditoría (%d -> %d)", antes, despues)
		}
		got, err := h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: b.Id})
		if err != nil || got.Estudiante.Version != 1 {
			t.Fatalf("una edición fallida no debe cambiar la versión: %+v err=%v", got, err)
		}
	})

	t.Run("Update_RegistroLegado_NoTocaDatosAcademicos", func(t *testing.T) {
		// Registro "como el existente": sin apellidos separados y con datos
		// académicos; la edición no debe alterar nivel/grado/sección/turno.
		id := h.seed(seedInput{Codigo: codigo(), Nombres: "Legado", Apellidos: "Previo Registro", Nivel: "PRIMARIA", Grado: "2", Seccion: "B", Turno: "TARDE"})
		u, err := update(actor, &estudiantes.UpdateEstudianteRequest{Id: id, ExpectedVersion: 1, Nombres: str("Legado Editado"), Apellidos: str("Previo  Registro Nuevo")})
		if err != nil || u.Apellidos != "Previo Registro Nuevo" || u.Nivel != "PRIMARIA" || u.Grado != "2" || u.Seccion != "B" || u.Turno != "TARDE" || u.EstadoMatricula != "ACTIVO" || u.Codigo == "" {
			t.Fatalf("edición de un registro legado: %+v err=%v", u, err)
		}
	})

	// ================= CONCURRENCIA =================
	t.Run("Concurrencia_VersionObsoleta_Conflicto", func(t *testing.T) {
		e := crearOK(t, basico("Concurrente", "Secuencial"))
		// A y B leen la versión 1; A guarda primero.
		if _, err := update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str("Cambio de A")}); err != nil {
			t.Fatalf("A: %v", err)
		}
		_, err := update(otro, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str("Cambio de B")})
		wantErr(t, err, codes.Aborted, "version_conflict", "B con versión obsoleta")
		got, err := h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: e.Id})
		if err != nil || got.Estudiante.Nombres != "Cambio de A" || got.Estudiante.Version != 2 {
			t.Fatalf("el cambio de A no debe sobrescribirse: %+v err=%v", got, err)
		}
		if aud := auditorias(t, e.Id); len(aud) != 2 {
			t.Fatalf("el conflicto no debe auditarse: %v", aud)
		}
	})

	t.Run("Concurrencia_Simultanea_UnSoloGanador", func(t *testing.T) {
		e := crearOK(t, basico("Carrera", "Simultanea"))
		const clientes = 6
		var wg sync.WaitGroup
		errs := make([]error, clientes)
		for i := 0; i < clientes; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, errs[i] = update(actor, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str(fmt.Sprintf("Cliente %d", i))})
			}(i)
		}
		wg.Wait()
		ok, conflictos := 0, 0
		for _, err := range errs {
			switch status.Code(err) {
			case codes.OK:
				ok++
			case codes.Aborted:
				conflictos++
			default:
				t.Fatalf("error inesperado: %v", err)
			}
		}
		if ok != 1 || conflictos != clientes-1 {
			t.Fatalf("se esperaba 1 éxito y %d conflictos; hubo %d y %d", clientes-1, ok, conflictos)
		}
		if aud := auditorias(t, e.Id); len(aud) != 2 {
			t.Fatalf("solo el ganador se audita: %v", aud)
		}
	})

	// ================= ESTADO =================
	t.Run("ChangeEstado_Transiciones_YRechazos", func(t *testing.T) {
		e := crearOK(t, basico("Estado", "Cambiante"))
		u, err := cambiarEstado(actor, e.Id, 1, "inactivo")
		if err != nil || u.Estado != "INACTIVO" || u.Version != 2 {
			t.Fatalf("ACTIVO -> INACTIVO: %+v err=%v", u, err)
		}
		u, err = cambiarEstado(otro, e.Id, 2, "ACTIVO")
		if err != nil || u.Estado != "ACTIVO" || u.Version != 3 {
			t.Fatalf("INACTIVO -> ACTIVO: %+v err=%v", u, err)
		}
		_, err = cambiarEstado(actor, e.Id, 3, "ACTIVO")
		wantErr(t, err, codes.FailedPrecondition, "invalid_state", "mismo estado")
		for _, malo := range []string{"RETIRADO", "TRASLADADO", "CONCLUIDO", ""} {
			_, err = cambiarEstado(actor, e.Id, 3, malo)
			wantErr(t, err, codes.InvalidArgument, "invalid_state", "estado "+malo)
		}
		_, err = cambiarEstado(actor, e.Id, 1, "INACTIVO")
		wantErr(t, err, codes.Aborted, "version_conflict", "estado con versión obsoleta")
		_, err = cambiarEstado(actor, "00000000-0000-4000-8000-00000000dead", 1, "INACTIVO")
		wantErr(t, err, codes.NotFound, "student_not_found", "estado de inexistente")
		aud := auditorias(t, e.Id)
		want := []string{
			"CREATE:" + actor + ":1:",
			"CHANGE_ESTADO:" + actor + ":2:estado:ACTIVO>INACTIVO",
			"CHANGE_ESTADO:" + otro + ":3:estado:INACTIVO>ACTIVO",
		}
		if len(aud) != len(want) {
			t.Fatalf("auditoría de estado: %v", aud)
		}
		for i := range want {
			if aud[i] != want[i] && !strings.HasPrefix(aud[i], want[i]) {
				t.Fatalf("auditoría[%d] = %q, se esperaba %q", i, aud[i], want[i])
			}
		}
		// El listado refleja el estado y la versión.
		busq, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: e.Codigo})
		if err != nil || busq.Total != 1 || busq.Estudiantes[0].Estado != "ACTIVO" || busq.Estudiantes[0].Version != 3 {
			t.Fatalf("SearchEstudiantes tras cambios de estado: %+v err=%v", busq, err)
		}
	})

	// ================= AUDITORÍA =================
	t.Run("Auditoria_AppendOnly", func(t *testing.T) {
		e := crearOK(t, basico("Audit", "Inmutable"))
		if _, err := h.app.Exec(h.ctx, `UPDATE auditoria_estudiantes SET accion = 'UPDATE' WHERE estudiante_id = $1`, e.Id); err == nil {
			t.Fatal("la aplicación no debe poder modificar la auditoría")
		}
		if _, err := h.app.Exec(h.ctx, `DELETE FROM auditoria_estudiantes WHERE estudiante_id = $1`, e.Id); err == nil {
			t.Fatal("la aplicación no debe poder borrar la auditoría")
		}
		if _, err := h.app.Exec(h.ctx, `TRUNCATE auditoria_estudiantes`); err == nil {
			t.Fatal("la aplicación no debe poder vaciar la auditoría")
		}
		// Incluso con privilegios (rol de pruebas), el trigger lo impide.
		if _, err := h.admin.Exec(h.ctx, `UPDATE auditoria_estudiantes SET accion = 'UPDATE' WHERE estudiante_id = $1`, e.Id); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("el trigger append-only debe rechazar UPDATE: %v", err)
		}
		if _, err := h.admin.Exec(h.ctx, `DELETE FROM auditoria_estudiantes WHERE estudiante_id = $1`, e.Id); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Fatalf("el trigger append-only debe rechazar DELETE: %v", err)
		}
		// Un valor no puede terminar en "campos" (CHECK de nombres de columna).
		if _, err := h.app.Exec(h.ctx, `INSERT INTO auditoria_estudiantes (usuario_id, accion, estudiante_id, version_resultante, campos)
			VALUES ($1, 'UPDATE', $2, 1, ARRAY['12345678'])`, actor, e.Id); err == nil {
			t.Fatal("campos solo debe admitir nombres de columnas")
		}
	})

	// ================= PERMISOS =================
	t.Run("Permisos_SinTokenYSinPermiso", func(t *testing.T) {
		e := crearOK(t, basico("Permisos", "Protegido"))
		_, err := h.client.CreateEstudiante(h.ctx, basico("X", "Y"))
		wantCode(t, err, codes.Unauthenticated, "Create sin token")
		_, err = h.client.UpdateEstudiante(h.ctx, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str("X")})
		wantCode(t, err, codes.Unauthenticated, "Update sin token")
		_, err = h.client.ChangeEstadoEstudiante(h.ctx, &estudiantes.ChangeEstadoEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Estado: "INACTIVO"})
		wantCode(t, err, codes.Unauthenticated, "ChangeEstado sin token")

		_, err = crear(soloLectura, basico("X", "Y"))
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "Create solo con view")
		_, err = update(soloLectura, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str("X")})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "Update solo con view")
		_, err = cambiarEstado(soloLectura, e.Id, 1, "INACTIVO")
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "ChangeEstado solo con view")

		// Cada permiso habilita solo su operación.
		soloCrear := "00000000-0000-4000-8000-0000000000e8"
		h.grant(soloCrear, "estudiantes.create")
		_, err = update(soloCrear, &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str("X")})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "Update con solo create")
		_, err = cambiarEstado(soloCrear, e.Id, 1, "INACTIVO")
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "ChangeEstado con solo create")

		// Sin permiso se deniega antes de validar: no revela si el id existe.
		_, err = update(soloLectura, &estudiantes.UpdateEstudianteRequest{Id: "00000000-0000-4000-8000-00000000dead", ExpectedVersion: 1})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "Update sin permiso sobre inexistente")

		got, err := h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: e.Id})
		if err != nil || got.Estudiante.Version != 1 || got.Estudiante.Nombres != "Permisos" {
			t.Fatalf("las operaciones denegadas no deben modificar nada: %+v err=%v", got, err)
		}
		if aud := auditorias(t, e.Id); len(aud) != 1 {
			t.Fatalf("las operaciones denegadas no deben auditarse: %v", aud)
		}
	})

	// ================= COMPATIBILIDAD =================
	t.Run("Compatibilidad_RPCLectura_ConEstudianteCreado", func(t *testing.T) {
		e := crearOK(t, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Compat", ApellidoPaterno: "Lectura", TipoDocumento: "DNI", NumeroDocumento: dni(9)})
		got, err := h.client.GetEstudiante(h.as(soloLectura), &estudiantes.GetEstudianteRequest{Id: e.Id})
		if err != nil || got.Estudiante.Apellidos != "Lectura" || got.Estudiante.Version != 1 {
			t.Fatalf("GetEstudiante: %+v err=%v", got, err)
		}
		basic, err := h.client.GetEstudiantesBasic(h.as(soloLectura), &estudiantes.GetEstudiantesBasicRequest{Ids: []string{e.Id}})
		if err != nil || len(basic.Estudiantes) != 1 || basic.Estudiantes[0].NombreCompleto != "Compat Lectura" {
			t.Fatalf("GetEstudiantesBasic: %+v err=%v", basic, err)
		}
		busq, err := h.client.SearchEstudiantes(h.as(soloLectura), &estudiantes.SearchEstudiantesRequest{Q: dni(9)})
		if err != nil || busq.Total != 1 {
			t.Fatalf("SearchEstudiantes por DNI del estudiante creado: %+v err=%v", busq, err)
		}
	})
}
