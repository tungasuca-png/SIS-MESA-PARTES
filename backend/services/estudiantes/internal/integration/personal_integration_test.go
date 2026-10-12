package integration

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"estudiantes/estudiantes"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestDatosPersonalesE08 verifica en PostgreSQL real el dominio de DATOS
// PERSONALES (FASE E-08): permisos separados, lectura auditada, edición con
// control de versión, cierre del bypass de UpdateEstudiante/CreateEstudiante y
// ausencia de valores personales en auditoría y logs.
func TestDatosPersonalesE08(t *testing.T) {
	h := newHarness(t)

	// Captura TODO lo que el servicio registre durante la prueba: al final se
	// verifica que ningún valor personal llegó a los logs.
	var logBuf syncBuffer
	logx.SetWriter(logx.NewWriter(&logBuf))
	t.Cleanup(func() { logx.Reset() })

	base := time.Now().UnixNano()
	tag := strconv.FormatInt(base%100_000_000, 36)
	n := 0
	codigo := func() string { n++; return fmt.Sprintf("E08-%s-%d", tag, n) }

	const (
		lector          = "00000000-0000-4000-8000-0000000000a1" // view + personal.view
		editor          = "00000000-0000-4000-8000-0000000000a2" // personal.update
		basico          = "00000000-0000-4000-8000-0000000000a3" // view + update (+ personal.update, para el bypass)
		creador         = "00000000-0000-4000-8000-0000000000a4" // create
		creadorPersonal = "00000000-0000-4000-8000-0000000000a5" // create + personal.update
		soloVista       = "00000000-0000-4000-8000-0000000000a6" // view
	)
	h.grant(lector, "estudiantes.view")
	h.grant(lector, "estudiantes.personal.view")
	h.grant(editor, "estudiantes.personal.update")
	h.grant(basico, "estudiantes.view")
	h.grant(basico, "estudiantes.update")
	h.grant(basico, "estudiantes.personal.update")
	h.grant(creador, "estudiantes.create")
	h.grant(creadorPersonal, "estudiantes.create")
	h.grant(creadorPersonal, "estudiantes.personal.update")
	h.grant(soloVista, "estudiantes.view")

	// Valores personales con marcas reconocibles para buscarlas en logs.
	const (
		fecha   = "2013-07-21"
		ubigeo  = "080101"
		lengua1 = "AWAJUN"
		lengua2 = "ASHANINKA"
	)
	valoresPersonales := []string{fecha, ubigeo, lengua1, lengua2, "awajun", "ashaninka"}

	str := func(s string) *string { return &s }
	wantErr := func(t *testing.T, err error, code codes.Code, prefijo, que string) {
		t.Helper()
		st := status.Convert(err)
		if err == nil || st.Code() != code || !strings.HasPrefix(st.Message(), prefijo+":") {
			t.Fatalf("%s: se esperaba %v %q, se obtuvo %v", que, code, prefijo, err)
		}
	}
	crear := func(t *testing.T, as string, req *estudiantes.CreateEstudianteRequest) (*estudiantes.EstudianteBasico, error) {
		t.Helper()
		resp, err := h.client.CreateEstudiante(h.as(as), req)
		if err != nil {
			return nil, err
		}
		h.created = append(h.created, resp.Estudiante.Id)
		return resp.Estudiante, nil
	}
	conPersonales := func() *estudiantes.CreateEstudianteRequest {
		return &estudiantes.CreateEstudianteRequest{
			Codigo: codigo(), Nombres: "Personal", Apellidos: "Prueba",
			FechaNacimiento: fecha, Sexo: "f", PaisNacimiento: "pe", UbigeoNacimiento: ubigeo,
			LenguaMaterna: "awajun", SegundaLengua: "ashaninka",
		}
	}
	nuevo := func(t *testing.T) *estudiantes.EstudianteBasico {
		t.Helper()
		e, err := crear(t, creadorPersonal, conPersonales())
		if err != nil {
			t.Fatalf("crear con datos personales: %v", err)
		}
		return e
	}
	// huella de la fila completa (incluye version y updated_at).
	huella := func(t *testing.T, id string) string {
		t.Helper()
		var s string
		if err := h.admin.QueryRow(h.ctx, `SELECT md5(row_to_json(e)::text) FROM estudiantes e WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	// huella de lo NO personal (para demostrar que la edición personal no
	// toca identidad, estado ni datos académicos).
	huellaNoPersonal := func(t *testing.T, id string) string {
		t.Helper()
		var s string
		if err := h.admin.QueryRow(h.ctx, `SELECT md5(concat_ws('|', codigo, codigo_siagie, tipo_documento, numero_documento,
			nombres, apellido_paterno, apellido_materno, apellidos, foto_ref, estado, nivel, grado, seccion, turno,
			estado_matricula, created_at)) FROM estudiantes WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	auditorias := func(t *testing.T, id string) []string {
		t.Helper()
		rows, err := h.admin.Query(h.ctx, `SELECT accion || ':' || usuario_id || ':' || version_resultante || ':' ||
			array_to_string(campos, ',') FROM auditoria_estudiantes WHERE estudiante_id = $1 ORDER BY fecha_hora, version_resultante`, id)
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
	auditoriaSinValores := func(t *testing.T, id string) {
		t.Helper()
		var filas string
		if err := h.admin.QueryRow(h.ctx, `SELECT COALESCE(string_agg(row_to_json(a)::text, ' '), '') FROM auditoria_estudiantes a WHERE estudiante_id = $1`, id).Scan(&filas); err != nil {
			t.Fatal(err)
		}
		for _, v := range valoresPersonales {
			if strings.Contains(filas, v) {
				t.Fatalf("la auditoría contiene un valor personal (%s)", v)
			}
		}
	}
	leerPersonalesDB := func(t *testing.T, id string) string {
		t.Helper()
		var s string
		if err := h.admin.QueryRow(h.ctx, `SELECT concat_ws('|', fecha_nacimiento, sexo, pais_nacimiento, ubigeo_nacimiento,
			lengua_materna, segunda_lengua) FROM estudiantes WHERE id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// ================= CONTRATOS =================
	t.Run("Contratos_SoloCamposPermitidos", func(t *testing.T) {
		campos := func(m protoreflect.ProtoMessage) []string {
			fields := m.ProtoReflect().Descriptor().Fields()
			out := make([]string, 0, fields.Len())
			for i := 0; i < fields.Len(); i++ {
				out = append(out, string(fields.Get(i).Name()))
			}
			sort.Strings(out)
			return out
		}
		got := strings.Join(campos(&estudiantes.EstudiantePersonal{}), ",")
		// Modelo maestro (etapa 2): + domicilio, contacto y edad calculada.
		want := "correo,domicilio_departamento,domicilio_direccion,domicilio_distrito,domicilio_provincia,domicilio_ubigeo," +
			"edad,fecha_nacimiento,id,lengua_materna,pais_nacimiento,segunda_lengua,sexo,telefono,ubigeo_nacimiento,version"
		if got != want {
			t.Fatalf("EstudiantePersonal expone %s, se esperaba exactamente %s", got, want)
		}
		// GetEstudiante y EstudianteBasico (listado, búsqueda, basic-batch)
		// siguen sin datos personales ni foto_ref.
		personales := map[string]bool{"fecha_nacimiento": true, "sexo": true, "pais_nacimiento": true,
			"ubigeo_nacimiento": true, "lengua_materna": true, "segunda_lengua": true, "foto_ref": true,
			"domicilio_direccion": true, "domicilio_departamento": true, "domicilio_provincia": true,
			"domicilio_distrito": true, "domicilio_ubigeo": true, "correo": true, "telefono": true, "edad": true}
		for nombre, msg := range map[string]protoreflect.ProtoMessage{
			"Estudiante":       &estudiantes.Estudiante{},
			"EstudianteBasico": &estudiantes.EstudianteBasico{},
		} {
			for _, c := range campos(msg) {
				if personales[c] {
					t.Fatalf("%s expone el campo personal %s", nombre, c)
				}
			}
		}
	})

	// ================= LECTURA =================
	t.Run("Lectura_ConPermiso_Auditada_SinCambiarVersion", func(t *testing.T) {
		e := nuevo(t)
		antes := huella(t, e.Id)
		resp, err := h.client.GetEstudiantePersonal(h.as(lector), &estudiantes.GetEstudiantePersonalRequest{Id: e.Id})
		if err != nil {
			t.Fatalf("GetEstudiantePersonal: %v", err)
		}
		p := resp.Personal
		if p.Id != e.Id || p.Version != 1 || p.FechaNacimiento != fecha || p.Sexo != "F" || p.PaisNacimiento != "PE" ||
			p.UbigeoNacimiento != ubigeo || p.LenguaMaterna != lengua1 || p.SegundaLengua != lengua2 {
			t.Fatalf("datos personales: %+v", p)
		}
		if huella(t, e.Id) != antes {
			t.Fatal("la lectura no debe modificar el registro ni su versión")
		}
		aud := auditorias(t, e.Id)
		want := "PERSONAL_VIEW:" + lector + ":1:fecha_nacimiento,sexo,pais_nacimiento,ubigeo_nacimiento,lengua_materna,segunda_lengua," +
			"domicilio_direccion,domicilio_departamento,domicilio_provincia,domicilio_distrito,domicilio_ubigeo,correo,telefono"
		if len(aud) != 2 || aud[1] != want {
			t.Fatalf("auditoría de lectura: %v", aud)
		}
		auditoriaSinValores(t, e.Id)
	})

	t.Run("Lectura_SinPermiso_Inexistente_IdInvalido", func(t *testing.T) {
		e := nuevo(t)
		for _, quien := range []string{soloVista, editor, basico, creadorPersonal} {
			_, err := h.client.GetEstudiantePersonal(h.as(quien), &estudiantes.GetEstudiantePersonalRequest{Id: e.Id})
			wantErr(t, err, codes.PermissionDenied, "permission_denied", "lectura personal sin personal.view ("+quien+")")
		}
		_, err := h.client.GetEstudiantePersonal(h.ctx, &estudiantes.GetEstudiantePersonalRequest{Id: e.Id})
		wantCode(t, err, codes.Unauthenticated, "lectura personal sin token")
		_, err = h.client.GetEstudiantePersonal(h.as(lector), &estudiantes.GetEstudiantePersonalRequest{Id: "00000000-0000-4000-8000-00000000dead"})
		wantErr(t, err, codes.NotFound, "student_not_found", "lectura personal de inexistente")
		_, err = h.client.GetEstudiantePersonal(h.as(lector), &estudiantes.GetEstudiantePersonalRequest{Id: "no-es-uuid"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "lectura personal con id inválido")
		if aud := auditorias(t, e.Id); len(aud) != 1 {
			t.Fatalf("las lecturas denegadas no deben auditarse: %v", aud)
		}
	})

	// ================= ACTUALIZACIÓN =================
	t.Run("Update_Parcial_Auditado_SoloCamposPersonales", func(t *testing.T) {
		e := nuevo(t)
		noPersonal := huellaNoPersonal(t, e.Id)
		u, err := h.client.UpdateEstudiantePersonal(h.as(editor), &estudiantes.UpdateEstudiantePersonalRequest{
			Id: e.Id, ExpectedVersion: 1, Sexo: str("m"), SegundaLengua: str(" quechua "),
		})
		if err != nil || u.Estudiante.Version != 2 {
			t.Fatalf("UpdateEstudiantePersonal: %+v err=%v", u, err)
		}
		if got := leerPersonalesDB(t, e.Id); got != fecha+"|M|PE|"+ubigeo+"|"+lengua1+"|QUECHUA" {
			t.Fatalf("solo deben cambiar los campos enviados: %s", got)
		}
		if huellaNoPersonal(t, e.Id) != noPersonal {
			t.Fatal("la edición personal no debe tocar identidad, estado ni datos académicos")
		}
		// Vaciar un campo ("" = NULL) y quitar el ubigeo junto con el país.
		u, err = h.client.UpdateEstudiantePersonal(h.as(editor), &estudiantes.UpdateEstudiantePersonalRequest{
			Id: e.Id, ExpectedVersion: 2, SegundaLengua: str(""), UbigeoNacimiento: str(""), PaisNacimiento: str(""),
		})
		if err != nil || u.Estudiante.Version != 3 {
			t.Fatalf("vaciar campos personales: %+v err=%v", u, err)
		}
		if got := leerPersonalesDB(t, e.Id); got != fecha+"|M|"+lengua1 {
			t.Fatalf("campos vaciados: %s", got)
		}
		aud := auditorias(t, e.Id)
		want := []string{
			"CREATE:" + creadorPersonal + ":1:",
			"PERSONAL_UPDATE:" + editor + ":2:sexo,segunda_lengua",
			"PERSONAL_UPDATE:" + editor + ":3:pais_nacimiento,ubigeo_nacimiento,segunda_lengua",
		}
		if len(aud) != 3 || !strings.HasPrefix(aud[0], want[0]) || aud[1] != want[1] || aud[2] != want[2] {
			t.Fatalf("auditoría de edición personal: %v", aud)
		}
		auditoriaSinValores(t, e.Id)

		// Sin cambios reales: sin versión nueva ni auditoría.
		u, err = h.client.UpdateEstudiantePersonal(h.as(editor), &estudiantes.UpdateEstudiantePersonalRequest{Id: e.Id, ExpectedVersion: 3, Sexo: str("M")})
		if err != nil || u.Estudiante.Version != 3 || len(auditorias(t, e.Id)) != 3 {
			t.Fatalf("edición sin cambios: %+v err=%v", u, err)
		}
	})

	t.Run("Update_Validaciones_Permisos_Version", func(t *testing.T) {
		e := nuevo(t)
		antes := huella(t, e.Id)
		casos := []struct {
			que string
			req *estudiantes.UpdateEstudiantePersonalRequest
		}{
			{"fecha futura", &estudiantes.UpdateEstudiantePersonalRequest{FechaNacimiento: str(time.Now().AddDate(1, 0, 0).Format("2006-01-02"))}},
			{"fecha imposible", &estudiantes.UpdateEstudiantePersonalRequest{FechaNacimiento: str("2013-02-30")}},
			{"sexo inválido", &estudiantes.UpdateEstudiantePersonalRequest{Sexo: str("X")}},
			{"país distinto de PE con ubigeo guardado", &estudiantes.UpdateEstudiantePersonalRequest{PaisNacimiento: str("CL")}},
			{"lenguas iguales", &estudiantes.UpdateEstudiantePersonalRequest{SegundaLengua: str("awajun")}},
		}
		for _, c := range casos {
			c.req.Id, c.req.ExpectedVersion = e.Id, 1
			_, err := h.client.UpdateEstudiantePersonal(h.as(editor), c.req)
			wantErr(t, err, codes.InvalidArgument, "validation_error", c.que)
		}
		_, err := h.client.UpdateEstudiantePersonal(h.as(editor), &estudiantes.UpdateEstudiantePersonalRequest{Id: e.Id, Sexo: str("M")})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "sin expected_version")
		for _, quien := range []string{lector, soloVista, creador} {
			_, err = h.client.UpdateEstudiantePersonal(h.as(quien), &estudiantes.UpdateEstudiantePersonalRequest{Id: e.Id, ExpectedVersion: 1, Sexo: str("M")})
			wantErr(t, err, codes.PermissionDenied, "permission_denied", "edición personal sin personal.update ("+quien+")")
		}
		_, err = h.client.UpdateEstudiantePersonal(h.as(editor), &estudiantes.UpdateEstudiantePersonalRequest{Id: e.Id, ExpectedVersion: 7, Sexo: str("M")})
		wantErr(t, err, codes.Aborted, "version_conflict", "versión incorrecta")
		_, err = h.client.UpdateEstudiantePersonal(h.as(editor), &estudiantes.UpdateEstudiantePersonalRequest{Id: "00000000-0000-4000-8000-00000000dead", ExpectedVersion: 1, Sexo: str("M")})
		wantErr(t, err, codes.NotFound, "student_not_found", "edición personal de inexistente")
		if huella(t, e.Id) != antes || len(auditorias(t, e.Id)) != 1 {
			t.Fatal("las ediciones fallidas no deben modificar ni auditar nada")
		}
	})

	t.Run("Concurrencia_ConEdicionBasica", func(t *testing.T) {
		e := nuevo(t)
		// Otro usuario edita la identidad (versión 1 -> 2); la edición personal
		// preparada sobre la versión 1 debe fallar, no sobrescribir.
		if _, err := h.client.UpdateEstudiante(h.as(basico), &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str("Otro Nombre")}); err != nil {
			t.Fatalf("edición básica: %v", err)
		}
		_, err := h.client.UpdateEstudiantePersonal(h.as(editor), &estudiantes.UpdateEstudiantePersonalRequest{Id: e.Id, ExpectedVersion: 1, Sexo: str("M")})
		wantErr(t, err, codes.Aborted, "version_conflict", "edición personal con versión obsoleta")
		// Carrera: varias ediciones personales simultáneas con la misma versión.
		var wg sync.WaitGroup
		errs := make([]error, 5)
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, errs[i] = h.client.UpdateEstudiantePersonal(h.as(editor), &estudiantes.UpdateEstudiantePersonalRequest{
					Id: e.Id, ExpectedVersion: 2, LenguaMaterna: str(fmt.Sprintf("L%d", i)),
				})
			}(i)
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
			} else if status.Code(err) != codes.Aborted {
				t.Fatalf("error inesperado: %v", err)
			}
		}
		if ok != 1 {
			t.Fatalf("se esperaba un solo ganador, hubo %d", ok)
		}
	})

	// ================= BYPASS =================
	t.Run("Bypass_UpdateEstudiante_RechazaCadaCampoPersonal", func(t *testing.T) {
		e := nuevo(t)
		antes := huella(t, e.Id)
		audAntes := len(auditorias(t, e.Id))
		casos := map[string]func(r *estudiantes.UpdateEstudianteRequest, v string){
			"fecha_nacimiento":  func(r *estudiantes.UpdateEstudianteRequest, v string) { r.FechaNacimiento = &v },
			"sexo":              func(r *estudiantes.UpdateEstudianteRequest, v string) { r.Sexo = &v },
			"pais_nacimiento":   func(r *estudiantes.UpdateEstudianteRequest, v string) { r.PaisNacimiento = &v },
			"ubigeo_nacimiento": func(r *estudiantes.UpdateEstudianteRequest, v string) { r.UbigeoNacimiento = &v },
			"lengua_materna":    func(r *estudiantes.UpdateEstudianteRequest, v string) { r.LenguaMaterna = &v },
			"segunda_lengua":    func(r *estudiantes.UpdateEstudianteRequest, v string) { r.SegundaLengua = &v },
		}
		valores := map[string]string{"fecha_nacimiento": "2014-01-01", "sexo": "M", "pais_nacimiento": "CL",
			"ubigeo_nacimiento": "150101", "lengua_materna": "AIMARA", "segunda_lengua": "INGLES"}
		for campo, set := range casos {
			for _, v := range []string{valores[campo], ""} {
				// Junto a un cambio legítimo (nombres): la solicitud completa
				// debe rechazarse, sin aplicar tampoco ese cambio.
				req := &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, Nombres: str("Cambio Colado")}
				set(req, v)
				_, err := h.client.UpdateEstudiante(h.as(basico), req)
				wantErr(t, err, codes.InvalidArgument, "validation_error", "UpdateEstudiante con "+campo+"="+strconv.Quote(v))
				if !strings.Contains(status.Convert(err).Message(), "UpdateEstudiantePersonal") {
					t.Fatalf("el error debe indicar el RPC correcto: %v", err)
				}
			}
		}
		if huella(t, e.Id) != antes {
			t.Fatal("el bypass rechazado no debe modificar la fila ni la versión")
		}
		if len(auditorias(t, e.Id)) != audAntes {
			t.Fatal("el bypass rechazado no debe generar auditoría")
		}
		// foto_ref NO es dato personal en E-08: sigue editándose por UpdateEstudiante.
		if _, err := h.client.UpdateEstudiante(h.as(basico), &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: 1, FotoRef: str("fotos/x.jpg")}); err != nil {
			t.Fatalf("foto_ref por UpdateEstudiante (sin cambios en E-08): %v", err)
		}
	})

	// ================= CREATE =================
	t.Run("Create_ConYSinPermisoPersonal", func(t *testing.T) {
		e, err := crear(t, creadorPersonal, conPersonales())
		if err != nil {
			t.Fatalf("create + personal.update con datos personales: %v", err)
		}
		if got := leerPersonalesDB(t, e.Id); got != fecha+"|F|PE|"+ubigeo+"|"+lengua1+"|"+lengua2 {
			t.Fatalf("datos personales del alta: %s", got)
		}

		var antes int
		if err := h.admin.QueryRow(h.ctx, `SELECT count(*) FROM estudiantes WHERE codigo LIKE $1`, "E08-"+tag+"-%").Scan(&antes); err != nil {
			t.Fatal(err)
		}
		for campo, req := range map[string]*estudiantes.CreateEstudianteRequest{
			"fecha_nacimiento":  {FechaNacimiento: fecha},
			"sexo":              {Sexo: "F"},
			"pais_nacimiento":   {PaisNacimiento: "PE"},
			"ubigeo_nacimiento": {PaisNacimiento: "PE", UbigeoNacimiento: ubigeo},
			"lengua_materna":    {LenguaMaterna: lengua1},
			"segunda_lengua":    {SegundaLengua: lengua2},
		} {
			req.Codigo, req.Nombres, req.Apellidos = codigo(), "Sin", "Permiso"
			_, err := crear(t, creador, req)
			wantErr(t, err, codes.PermissionDenied, "permission_denied", "create sin personal.update con "+campo)
		}
		var despues int
		if err := h.admin.QueryRow(h.ctx, `SELECT count(*) FROM estudiantes WHERE codigo LIKE $1`, "E08-"+tag+"-%").Scan(&despues); err != nil {
			t.Fatal(err)
		}
		if despues != antes {
			t.Fatalf("un alta rechazada no debe crear nada (%d -> %d)", antes, despues)
		}

		// Sin datos personales, estudiantes.create sigue funcionando igual
		// (incluida foto_ref, que no es dato personal en E-08).
		s, err := crear(t, creador, &estudiantes.CreateEstudianteRequest{Codigo: codigo(), Nombres: "Solo", Apellidos: "Identidad", FotoRef: "fotos/y.jpg", Sexo: "   "})
		if err != nil || s.Version != 1 {
			t.Fatalf("create sin datos personales: %+v err=%v", s, err)
		}
		if got := leerPersonalesDB(t, s.Id); got != "" {
			t.Fatalf("un alta sin datos personales no debe guardar ninguno: %q", got)
		}
	})

	// ================= LOGS =================
	t.Run("Logs_SinValoresPersonales", func(t *testing.T) {
		// Sonda: demuestra que la captura funciona (si no, la verificación
		// de abajo pasaría en vacío).
		logx.Error("sonda-captura-e08")
		logs := logBuf.String()
		if !strings.Contains(logs, "sonda-captura-e08") {
			t.Fatal("la captura de logs no está funcionando")
		}
		for _, v := range valoresPersonales {
			if strings.Contains(logs, v) {
				t.Fatalf("los logs del servicio contienen un valor personal (%s)", v)
			}
		}
	})
}

// syncBuffer: bytes.Buffer seguro para escrituras concurrentes de logx.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
