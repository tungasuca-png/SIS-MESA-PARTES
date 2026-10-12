package integration

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"estudiantes/estudiantes"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestListadoMaestroE04 verifica en PostgreSQL real el listado del maestro
// (FASE E-04): búsqueda normalizada por palabras, filtros, paginación, orden
// determinista y que el listado no exponga datos personales ni sensibles.
//
// Aislamiento: la base es compartida, así que cada búsqueda incluye el tag
// único de esta ejecución (presente en el código de cada fixture) y cada
// consulta por filtros incluye la sección única "sec".
func TestListadoMaestroE04(t *testing.T) {
	h := newHarness(t)
	base := time.Now().UnixNano()
	tag := strconv.FormatInt(base%100_000_000, 36) // ~6 caracteres
	sec := "S" + tag                               // seccion es VARCHAR(10)
	dniA := fmt.Sprintf("5%07d", base%10_000_000)
	siagieA := fmt.Sprintf("4%013d", base%10_000_000_000_000)

	actor := "00000000-0000-4000-8000-000000000004"
	h.grant(actor, "estudiantes.view")

	ids := map[string]string{}
	ids["A"] = h.seed(seedInput{Codigo: "E04-" + tag + "-A", Nombres: "José Luis", Apellidos: "Pérez Ñúñez",
		ApellidoPaterno: "Pérez", ApellidoMaterno: "Ñúñez", NumeroDocumento: dniA, CodigoSiagie: siagieA,
		Nivel: "PRIMARIA", Grado: "3", Seccion: sec, Turno: "MAÑANA"})
	ids["B"] = h.seed(seedInput{Codigo: "E04-" + tag + "-B", Nombres: "María", Apellidos: "Quispe Mamani",
		ApellidoPaterno: "Quispe", ApellidoMaterno: "Mamani",
		Nivel: "SECUNDARIA", Grado: "1", Seccion: sec, Turno: "TARDE", Estado: "INACTIVO"})
	ids["C"] = h.seed(seedInput{Codigo: "E04-" + tag + "-C", Nombres: "Ana", Apellidos: "Pérez Quispe",
		ApellidoPaterno: "Pérez", ApellidoMaterno: "Quispe",
		Nivel: "PRIMARIA", Grado: "3", Seccion: sec, Turno: "TARDE"})
	ids["D"] = h.seed(seedInput{Codigo: "E04-" + tag + "-D", Nombres: "Luis", Apellidos: "Ñúñez Rojas",
		Nivel: "PRIMARIA", Grado: "4", Seccion: sec, Turno: "MAÑANA"})
	// E y F: mismos apellidos y nombres, para probar el desempate por id.
	ids["E"] = h.seed(seedInput{Codigo: "E04-" + tag + "-E", Nombres: "Gemelo", Apellidos: "Iguales",
		Nivel: "SECUNDARIA", Grado: "2", Seccion: sec, Turno: "MAÑANA"})
	ids["F"] = h.seed(seedInput{Codigo: "E04-" + tag + "-F", Nombres: "Gemelo", Apellidos: "Iguales",
		Nivel: "SECUNDARIA", Grado: "2", Seccion: sec, Turno: "MAÑANA"})
	nombreDe := map[string]string{}
	for k, v := range ids {
		nombreDe[v] = k
	}

	// A recibe también datos personales y sensibles: el listado NO debe
	// devolverlos (ver subtest Listado_SinCamposSensibles).
	if _, err := h.app.Exec(h.ctx, `UPDATE estudiantes SET fecha_nacimiento = '2015-03-04', sexo = 'M',
		pais_nacimiento = 'PE', ubigeo_nacimiento = '150101', lengua_materna = 'QUECHUA',
		segunda_lengua = 'AIMARA', foto_ref = 'fotos/e04-sensible.jpg' WHERE id = $1`, ids["A"]); err != nil {
		t.Fatalf("datos personales de A: %v", err)
	}

	search := func(t *testing.T, req *estudiantes.SearchEstudiantesRequest) *estudiantes.SearchEstudiantesResponse {
		t.Helper()
		resp, err := h.client.SearchEstudiantes(h.as(actor), req)
		if err != nil {
			t.Fatalf("SearchEstudiantes(%+v): %v", req, err)
		}
		return resp
	}
	// letras devuelve las claves (A..F) de los resultados, ordenadas.
	letras := func(resp *estudiantes.SearchEstudiantesResponse) string {
		out := []string{}
		for _, e := range resp.Estudiantes {
			if k, ok := nombreDe[e.Id]; ok {
				out = append(out, k)
			} else {
				out = append(out, "?")
			}
		}
		sort.Strings(out)
		return strings.Join(out, "")
	}
	esperar := func(t *testing.T, que string, resp *estudiantes.SearchEstudiantesResponse, want string) {
		t.Helper()
		if got := letras(resp); got != want || int(resp.Total) != len(want) {
			t.Fatalf("%s: se esperaba %q (total %d), se obtuvo %q (total %d)", que, want, len(want), got, resp.Total)
		}
	}

	t.Run("Busqueda_PorCadaCampo_MayusculasYTildes", func(t *testing.T) {
		casos := []struct{ que, q, want string }{
			{"código exacto", "E04-" + tag + "-A", "A"},
			{"código en minúsculas", strings.ToLower("E04-" + tag + "-B"), "B"},
			{"código SIAGIE", siagieA, "A"},
			{"código SIAGIE parcial", siagieA[:9] + " " + tag, "A"},
			{"número de documento", dniA, "A"},
			{"nombres sin tilde y en mayúsculas", "JOSE " + tag, "A"},
			{"nombres con tilde", "maría " + tag, "B"},
			{"apellido paterno", "PEREZ " + tag, "AC"},
			{"apellido materno (ñ sin tilde)", "nunez " + tag, "AD"},
			{"apellidos completos", "Quispe Mamani " + tag, "B"},
			{"nombre completo", "jose luis perez nunez " + tag, "A"},
			{"apellido + nombre (orden inverso)", "Pérez José " + tag, "A"},
			{"espacios repetidos", "   josé     PÉREZ   " + tag + "  ", "A"},
			{"término inexistente", "zzqqxx " + tag, ""},
		}
		for _, c := range casos {
			esperar(t, c.que, search(t, &estudiantes.SearchEstudiantesRequest{Q: c.q}), c.want)
		}
	})

	t.Run("Busqueda_ComodinesLIKE_SonTextoLiteral", func(t *testing.T) {
		// Si "_" actuara como comodín, "e04_<tag>" coincidiría con "e04-<tag>".
		esperar(t, "guion bajo", search(t, &estudiantes.SearchEstudiantesRequest{Q: "E04_" + tag}), "")
		esperar(t, "porcentaje", search(t, &estudiantes.SearchEstudiantesRequest{Q: "%% " + tag}), "")
	})

	t.Run("Busqueda_Validaciones", func(t *testing.T) {
		_, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "a"})
		wantCode(t, err, codes.InvalidArgument, "q de 1 carácter")
		_, err = h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "a b c d e f g"})
		wantCode(t, err, codes.InvalidArgument, "demasiadas palabras")
		_, err = h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: strings.Repeat("x", 101)})
		wantCode(t, err, codes.InvalidArgument, "q demasiado largo")
		// estado es el del REGISTRO (ACTIVO/INACTIVO); RETIRADO pertenece a
		// estado_matricula y no se mezcla.
		_, err = h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Estado: "RETIRADO"})
		wantCode(t, err, codes.InvalidArgument, "estado de matrícula usado como estado")
		_, err = h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Grado: strings.Repeat("9", 31)})
		wantCode(t, err, codes.InvalidArgument, "filtro demasiado largo")
		sinPermiso := "00000000-0000-4000-8000-000000000005"
		_, err = h.client.SearchEstudiantes(h.as(sinPermiso), &estudiantes.SearchEstudiantesRequest{Seccion: sec})
		wantCode(t, err, codes.PermissionDenied, "listado sin estudiantes.view")
	})

	t.Run("Filtros_CadaUno_YCombinados", func(t *testing.T) {
		casos := []struct {
			que  string
			req  *estudiantes.SearchEstudiantesRequest
			want string
		}{
			{"solo sección", &estudiantes.SearchEstudiantesRequest{Seccion: sec}, "ABCDEF"},
			{"estado INACTIVO", &estudiantes.SearchEstudiantesRequest{Seccion: sec, Estado: "INACTIVO"}, "B"},
			{"estado en minúsculas", &estudiantes.SearchEstudiantesRequest{Seccion: sec, Estado: "activo"}, "ACDEF"},
			{"nivel", &estudiantes.SearchEstudiantesRequest{Seccion: sec, Nivel: "PRIMARIA"}, "ACD"},
			{"grado", &estudiantes.SearchEstudiantesRequest{Seccion: sec, Grado: "3"}, "AC"},
			{"turno", &estudiantes.SearchEstudiantesRequest{Seccion: sec, Turno: "TARDE"}, "BC"},
			{"combinados", &estudiantes.SearchEstudiantesRequest{Seccion: sec, Nivel: "PRIMARIA", Grado: "3", Turno: "TARDE", Estado: "ACTIVO"}, "C"},
			{"búsqueda + filtro", &estudiantes.SearchEstudiantesRequest{Q: "perez " + tag, Turno: "MAÑANA"}, "A"},
			{"filtros sin coincidencias", &estudiantes.SearchEstudiantesRequest{Seccion: sec, Nivel: "INICIAL"}, ""},
		}
		for _, c := range casos {
			esperar(t, c.que, search(t, c.req), c.want)
		}
	})

	t.Run("Paginacion", func(t *testing.T) {
		todos := search(t, &estudiantes.SearchEstudiantesRequest{Seccion: sec, PageSize: 100})
		if len(todos.Estudiantes) != 6 || todos.Total != 6 {
			t.Fatalf("page_size 100 (máximo permitido): %d resultados, total %d", len(todos.Estudiantes), todos.Total)
		}
		var concatenado []string
		for page := int32(1); page <= 3; page++ {
			p := search(t, &estudiantes.SearchEstudiantesRequest{Seccion: sec, Page: page, PageSize: 2})
			if len(p.Estudiantes) != 2 || p.Total != 6 {
				t.Fatalf("página %d: %d resultados, total %d", page, len(p.Estudiantes), p.Total)
			}
			for _, e := range p.Estudiantes {
				concatenado = append(concatenado, e.Id)
			}
		}
		for i, e := range todos.Estudiantes {
			if concatenado[i] != e.Id {
				t.Fatalf("las páginas no reproducen el listado completo en la posición %d", i)
			}
		}
		vacia := search(t, &estudiantes.SearchEstudiantesRequest{Seccion: sec, Page: 4, PageSize: 2})
		if len(vacia.Estudiantes) != 0 || vacia.Total != 6 {
			t.Fatalf("página fuera de rango debe venir vacía con el total real: %+v", vacia)
		}
		// Por encima del máximo se recorta al valor por defecto (20).
		general := search(t, &estudiantes.SearchEstudiantesRequest{PageSize: 101})
		if len(general.Estudiantes) > 20 {
			t.Fatalf("page_size 101 debe recortarse a 20, devolvió %d", len(general.Estudiantes))
		}
		if max := search(t, &estudiantes.SearchEstudiantesRequest{PageSize: 100}); len(max.Estudiantes) > 100 {
			t.Fatalf("page_size 100 devolvió %d", len(max.Estudiantes))
		}
	})

	t.Run("Orden_Determinista", func(t *testing.T) {
		primera := search(t, &estudiantes.SearchEstudiantesRequest{Seccion: sec, PageSize: 100})
		for i := 0; i < 5; i++ {
			otra := search(t, &estudiantes.SearchEstudiantesRequest{Seccion: sec, PageSize: 100})
			for j := range primera.Estudiantes {
				if otra.Estudiantes[j].Id != primera.Estudiantes[j].Id {
					t.Fatalf("el orden cambió entre llamadas idénticas (intento %d, posición %d)", i, j)
				}
			}
		}
		// Orden por apellidos, nombres (collation de la base) y, en empate, id.
		var pos = map[string]int{}
		for i, e := range primera.Estudiantes {
			pos[nombreDe[e.Id]] = i
		}
		menor, mayor := "E", "F"
		if ids["F"] < ids["E"] {
			menor, mayor = "F", "E"
		}
		if pos[menor] > pos[mayor] {
			t.Fatalf("empate de apellidos y nombres debe resolverse por id ascendente")
		}
		var consistente bool
		if err := h.app.QueryRow(h.ctx, `SELECT $1::text < $2::text`, "Pérez Quispe", "Quispe Mamani").Scan(&consistente); err != nil {
			t.Fatal(err)
		}
		if consistente != (pos["C"] < pos["B"]) {
			t.Fatalf("el listado no respeta el orden por apellidos de la base")
		}
	})

	t.Run("Listado_SinCamposSensibles", func(t *testing.T) {
		permitidos := map[string]bool{
			"id": true, "codigo": true, "nombre_completo": true, "nivel": true, "grado": true, "seccion": true,
			"estado_matricula": true, "numero_documento": true, "nombres": true, "apellidos": true,
			"apellido_paterno": true, "apellido_materno": true, "codigo_siagie": true, "tipo_documento": true,
			"estado": true, "turno": true,
			// FASE E-05: versión del registro (concurrencia); no es un dato personal.
			"version": true,
		}
		campos := (&estudiantes.EstudianteBasico{}).ProtoReflect().Descriptor().Fields()
		for i := 0; i < campos.Len(); i++ {
			if n := string(campos.Get(i).Name()); !permitidos[n] {
				t.Fatalf("EstudianteBasico expone un campo no previsto para el listado: %s", n)
			}
		}
		resp := search(t, &estudiantes.SearchEstudiantesRequest{Q: dniA})
		if len(resp.Estudiantes) != 1 {
			t.Fatalf("se esperaba A: %+v", resp)
		}
		a := resp.Estudiantes[0]
		a.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
			if !permitidos[string(fd.Name())] {
				t.Errorf("campo no permitido con valor: %s", fd.Name())
			}
			return true
		})
		js, err := protojson.Marshal(resp)
		if err != nil {
			t.Fatal(err)
		}
		for _, sensible := range []string{"2015-03-04", "150101", "QUECHUA", "AIMARA", "fotos/", "fecha_nacimiento", "sexo", "ubigeo", "lengua", "foto"} {
			if strings.Contains(strings.ToLower(string(js)), strings.ToLower(sensible)) {
				t.Fatalf("la respuesta del listado contiene un dato sensible (%s)", sensible)
			}
		}
		// Y sí trae la identificación básica completa.
		if a.Nombres != "José Luis" || a.Apellidos != "Pérez Ñúñez" || a.ApellidoPaterno != "Pérez" ||
			a.ApellidoMaterno != "Ñúñez" || a.CodigoSiagie != siagieA || a.TipoDocumento != "DNI" ||
			a.NumeroDocumento != dniA || a.Estado != "ACTIVO" || a.Turno != "MAÑANA" ||
			a.NombreCompleto != "José Luis Pérez Ñúñez" || a.EstadoMatricula != "ACTIVO" {
			t.Fatalf("identificación básica incompleta: %+v", a)
		}
	})

	t.Run("Compatibilidad_3RPC", func(t *testing.T) {
		got, err := h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: ids["A"]})
		if err != nil || got.Estudiante.Codigo != "E04-"+tag+"-A" || got.Estudiante.Apellidos != "Pérez Ñúñez" ||
			got.Estudiante.Turno != "MAÑANA" || got.Estudiante.EstadoMatricula != "ACTIVO" {
			t.Fatalf("GetEstudiante: %+v err=%v", got, err)
		}
		basic, err := h.client.GetEstudiantesBasic(h.as(actor), &estudiantes.GetEstudiantesBasicRequest{Ids: []string{ids["D"], ids["A"]}})
		if err != nil || len(basic.Estudiantes) != 2 {
			t.Fatalf("GetEstudiantesBasic: %+v err=%v", basic, err)
		}
		for _, e := range basic.Estudiantes {
			if e.Id == ids["D"] && (e.TipoDocumento != "SIN_DOCUMENTO" || e.NumeroDocumento != "" || e.ApellidoPaterno != "") {
				t.Fatalf("GetEstudiantesBasic D (sin documento ni apellidos separados): %+v", e)
			}
			if e.Id == ids["A"] && (e.NombreCompleto != "José Luis Pérez Ñúñez" || e.CodigoSiagie != siagieA) {
				t.Fatalf("GetEstudiantesBasic A: %+v", e)
			}
		}
		// Semántica previa de Search: una sola palabra parcial sigue funcionando.
		esperar(t, "DNI parcial", search(t, &estudiantes.SearchEstudiantesRequest{Q: dniA[:6] + " " + tag}), "A")
	})
}
