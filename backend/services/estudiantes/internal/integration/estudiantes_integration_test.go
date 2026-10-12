package integration

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"estudiantes/estudiantes"
	"estudiantes/internal/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func wantCode(t *testing.T, err error, want codes.Code, what string) {
	t.Helper()
	if status.Code(err) != want {
		t.Fatalf("%s: se esperaba %v, got %v", what, want, err)
	}
}

func TestEstudiantesService(t *testing.T) {
	h := newHarness(t)
	base := time.Now().UnixNano()
	suf := func(n int) string { return strconv.FormatInt(base%1_000_000, 36) + string(rune('A'+n)) }

	// actor tiene "estudiantes.view" concedido explícitamente por userID (nunca
	// por rol): es el usuario que usan todos los subtests funcionales de abajo.
	// Las pruebas de autorización (JWT_ValidoSinPermiso, etc.) usan sus propias
	// identidades, sin ese permiso.
	actor := "00000000-0000-4000-8000-000000000001"
	h.grant(actor, "estudiantes.view")

	// ---- JWT ----
	t.Run("JWT_Ausente_Invalido_Valido", func(t *testing.T) {
		_, err := h.client.SearchEstudiantes(h.ctx, &estudiantes.SearchEstudiantesRequest{Q: "xx"})
		wantCode(t, err, codes.Unauthenticated, "sin token")
		bad := metadata.AppendToOutgoingContext(h.ctx, "authorization", "Bearer no.es.un.jwt")
		_, err = h.client.SearchEstudiantes(bad, &estudiantes.SearchEstudiantesRequest{Q: "xx"})
		wantCode(t, err, codes.Unauthenticated, "token inválido")
		if _, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "xx"}); err != nil {
			t.Fatalf("token válido: %v", err)
		}
	})

	// ---- Autorización (estudiantes.view) ----
	t.Run("Autorizacion_SinPermiso_ConPermiso_AuthCaido", func(t *testing.T) {
		sinPermiso := "00000000-0000-4000-8000-000000000002"
		id := h.seed(seedInput{Codigo: "AUTHZ-" + suf(9), Nombres: "Rita", Apellidos: "Nunez"})

		// JWT válido, SIN estudiantes.view -> PermissionDenied en los 3 RPC.
		_, err := h.client.GetEstudiante(h.as(sinPermiso), &estudiantes.GetEstudianteRequest{Id: id})
		wantCode(t, err, codes.PermissionDenied, "GetEstudiante sin permiso")
		_, err = h.client.SearchEstudiantes(h.as(sinPermiso), &estudiantes.SearchEstudiantesRequest{Q: "Rita"})
		wantCode(t, err, codes.PermissionDenied, "SearchEstudiantes sin permiso")
		_, err = h.client.GetEstudiantesBasic(h.as(sinPermiso), &estudiantes.GetEstudiantesBasicRequest{Ids: []string{id}})
		wantCode(t, err, codes.PermissionDenied, "GetEstudiantesBasic sin permiso")

		// JWT válido, CON estudiantes.view -> permitido (ya probado en el resto
		// de subtests vía "actor", pero se confirma aquí explícitamente con una
		// identidad dedicada).
		conPermiso := "00000000-0000-4000-8000-000000000003"
		h.grant(conPermiso, "estudiantes.view")
		if _, err := h.client.GetEstudiante(h.as(conPermiso), &estudiantes.GetEstudianteRequest{Id: id}); err != nil {
			t.Fatalf("GetEstudiante con permiso: %v", err)
		}

		// Auth caído (error de comunicación, no allowed=false) -> DENEGADO en
		// los 3 RPC, fail-closed: nunca se traduce en acceso permitido.
		h.auth.failing = true
		defer func() { h.auth.failing = false }()
		_, err = h.client.GetEstudiante(h.as(conPermiso), &estudiantes.GetEstudianteRequest{Id: id})
		if status.Code(err) == codes.OK {
			t.Fatalf("con Auth caído no debe permitirse GetEstudiante, got %v", err)
		}
		_, err = h.client.SearchEstudiantes(h.as(conPermiso), &estudiantes.SearchEstudiantesRequest{Q: "Rita"})
		if status.Code(err) == codes.OK {
			t.Fatalf("con Auth caído no debe permitirse SearchEstudiantes, got %v", err)
		}
		_, err = h.client.GetEstudiantesBasic(h.as(conPermiso), &estudiantes.GetEstudiantesBasicRequest{Ids: []string{id}})
		if status.Code(err) == codes.OK {
			t.Fatalf("con Auth caído no debe permitirse GetEstudiantesBasic, got %v", err)
		}
	})

	// ---- GetEstudiante ----
	t.Run("GetEstudiante_Existente_OK", func(t *testing.T) {
		id := h.seed(seedInput{Codigo: "EST-G-" + suf(1), Nombres: "Ana", Apellidos: "Torres", Nivel: "SECUNDARIA", Grado: "3", Seccion: "A", Turno: "MANANA"})
		resp, err := h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: id})
		if err != nil {
			t.Fatalf("GetEstudiante: %v", err)
		}
		e := resp.Estudiante
		if e.Id != id || e.Codigo != "EST-G-"+suf(1) || e.Nombres != "Ana" || e.Apellidos != "Torres" ||
			e.Nivel != "SECUNDARIA" || e.Grado != "3" || e.Seccion != "A" || e.Turno != "MANANA" || e.EstadoMatricula != "ACTIVO" {
			t.Fatalf("campos inesperados: %+v", e)
		}
	})

	t.Run("GetEstudiante_Inexistente_NotFound", func(t *testing.T) {
		_, err := h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: "00000000-0000-4000-8000-000000009999"})
		wantCode(t, err, codes.NotFound, "estudiante inexistente")
	})

	t.Run("GetEstudiante_UUIDInvalido_InvalidArgument", func(t *testing.T) {
		_, err := h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: "no-es-un-uuid"})
		wantCode(t, err, codes.InvalidArgument, "UUID inválido")
		_, err = h.client.GetEstudiante(h.as(actor), &estudiantes.GetEstudianteRequest{Id: ""})
		wantCode(t, err, codes.InvalidArgument, "UUID vacío")
	})

	// ---- SearchEstudiantes ----
	t.Run("Search_PorCodigoNombreApellido_SinResultados_Paginacion", func(t *testing.T) {
		tag := suf(2)
		id1 := h.seed(seedInput{Codigo: "SRCH-" + tag + "-1", Nombres: "Juan", Apellidos: "Perez" + tag, Nivel: "PRIMARIA", Grado: "5", Seccion: "B"})
		id2 := h.seed(seedInput{Codigo: "SRCH-" + tag + "-2", Nombres: "Maria" + tag, Apellidos: "Lopez", Nivel: "PRIMARIA", Grado: "6", Seccion: "A"})

		byCodigo, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "SRCH-" + tag})
		if err != nil || byCodigo.Total != 2 {
			t.Fatalf("búsqueda por código: total=%d err=%v", byCodigo.GetTotal(), err)
		}

		byApellido, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "Perez" + tag})
		if err != nil || byApellido.Total != 1 || byApellido.Estudiantes[0].Id != id1 {
			t.Fatalf("búsqueda por apellido: %+v err=%v", byApellido, err)
		}

		byNombre, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "Maria" + tag})
		if err != nil || byNombre.Total != 1 || byNombre.Estudiantes[0].Id != id2 {
			t.Fatalf("búsqueda por nombre: %+v err=%v", byNombre, err)
		}
		if byNombre.Estudiantes[0].NombreCompleto != "Maria"+tag+" Lopez" {
			t.Fatalf("nombre_completo inesperado: %q", byNombre.Estudiantes[0].NombreCompleto)
		}

		sinResultados, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "NoExisteZZZ" + tag})
		if err != nil || sinResultados.Total != 0 || len(sinResultados.Estudiantes) != 0 {
			t.Fatalf("se esperaban 0 resultados: %+v err=%v", sinResultados, err)
		}

		p1, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "SRCH-" + tag, Page: 1, PageSize: 1})
		if err != nil || len(p1.Estudiantes) != 1 || p1.Total != 2 {
			t.Fatalf("página 1: %+v err=%v", p1, err)
		}
		p2, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "SRCH-" + tag, Page: 2, PageSize: 1})
		if err != nil || len(p2.Estudiantes) != 1 || p2.Estudiantes[0].Id == p1.Estudiantes[0].Id {
			t.Fatalf("página 2 debe traer el otro registro: p1=%+v p2=%+v err=%v", p1, p2, err)
		}
	})

	// FASE 5D: Search ahora también busca por numero_documento (DNI), con
	// coincidencia parcial igual que código/nombres/apellidos.
	t.Run("Search_PorNumeroDocumento_ExactoYParcial", func(t *testing.T) {
		tag := suf(7)
		// DNI real de 8 dígitos (FASE E-03 valida el formato), único por corrida.
		dni := fmt.Sprintf("7%07d", base%10_000_000)
		id := h.seed(seedInput{Codigo: "DNI-" + tag, Nombres: "Karla", Apellidos: "Rojas" + tag, NumeroDocumento: dni})

		exacto, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: dni})
		if err != nil || exacto.Total != 1 || exacto.Estudiantes[0].Id != id {
			t.Fatalf("búsqueda por DNI exacto: %+v err=%v", exacto, err)
		}
		if exacto.Estudiantes[0].NumeroDocumento != dni {
			t.Fatalf("numero_documento no viene en el resultado: %+v", exacto.Estudiantes[0])
		}

		parcial, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: dni[:5]})
		if err != nil || parcial.Total != 1 || parcial.Estudiantes[0].Id != id {
			t.Fatalf("búsqueda por DNI parcial: %+v err=%v", parcial, err)
		}
	})

	t.Run("Search_QueryCorta_InvalidArgument", func(t *testing.T) {
		_, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "a"})
		wantCode(t, err, codes.InvalidArgument, "búsqueda de 1 carácter")
		// FASE E-04: q en blanco ya no es error, es el listado general.
		if _, err = h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "  "}); err != nil {
			t.Fatalf("búsqueda en blanco debe ser el listado general: %v", err)
		}
	})

	t.Run("Search_PageSizeFueraDeRango_SeRecortaAlDefault", func(t *testing.T) {
		tag := suf(3)
		h.seed(seedInput{Codigo: "PGSZ-" + tag, Nombres: "Pedro", Apellidos: "Ramirez" + tag})
		resp, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "PGSZ-" + tag, PageSize: 999})
		if err != nil {
			t.Fatalf("page_size fuera de rango no debe rechazarse: %v", err)
		}
		if len(resp.Estudiantes) != 1 {
			t.Fatalf("se esperaba encontrar el registro sembrado: %+v", resp)
		}
		negativo, err := h.client.SearchEstudiantes(h.as(actor), &estudiantes.SearchEstudiantesRequest{Q: "PGSZ-" + tag, Page: -5, PageSize: -5})
		if err != nil || len(negativo.Estudiantes) != 1 {
			t.Fatalf("page/page_size negativos deben recortarse a valores válidos: %+v err=%v", negativo, err)
		}
	})

	// ---- GetEstudiantesBasic ----
	t.Run("Basic_Existentes_Inexistente_Mezclados_Duplicados_Vacio", func(t *testing.T) {
		tag := suf(4)
		id1 := h.seed(seedInput{Codigo: "BAS-" + tag + "-1", Nombres: "Luis", Apellidos: "Gomez", Nivel: "INICIAL", Grado: "4", Seccion: "Unica"})
		id2 := h.seed(seedInput{Codigo: "BAS-" + tag + "-2", Nombres: "Rosa", Apellidos: "Diaz"})
		inexistente := "00000000-0000-4000-8000-000000008888"

		vacio, err := h.client.GetEstudiantesBasic(h.as(actor), &estudiantes.GetEstudiantesBasicRequest{})
		if err != nil || len(vacio.Estudiantes) != 0 {
			t.Fatalf("lista vacía: %+v err=%v", vacio, err)
		}

		resp, err := h.client.GetEstudiantesBasic(h.as(actor), &estudiantes.GetEstudiantesBasicRequest{
			Ids: []string{id1, id2, id1, inexistente}, // duplicado (id1) + inexistente mezclados
		})
		if err != nil {
			t.Fatalf("GetEstudiantesBasic: %v", err)
		}
		if len(resp.Estudiantes) != 2 {
			t.Fatalf("se esperaban 2 (el inexistente no viene, el duplicado no se repite): %+v", resp.Estudiantes)
		}
		got := map[string]bool{}
		for _, e := range resp.Estudiantes {
			got[e.Id] = true
			if e.NombreCompleto == "" {
				t.Fatalf("nombre_completo vacío: %+v", e)
			}
		}
		if !got[id1] || !got[id2] {
			t.Fatalf("faltan ids esperados: %+v", resp.Estudiantes)
		}

		_, err = h.client.GetEstudiantesBasic(h.as(actor), &estudiantes.GetEstudiantesBasicRequest{Ids: []string{"no-es-uuid"}})
		wantCode(t, err, codes.InvalidArgument, "id con formato inválido")
	})

	// ---- Integridad ----
	t.Run("Integridad_CodigoDuplicado_Conflicto", func(t *testing.T) {
		tag := suf(5)
		h.seed(seedInput{Codigo: "DUP-" + tag, Nombres: "Carla", Apellidos: "Vega"})
		_, err := h.app.Exec(h.ctx, `
			INSERT INTO estudiantes (codigo, nombres, apellidos) VALUES ($1, 'Otro', 'Nombre')
		`, "DUP-"+tag)
		if err == nil {
			t.Fatal("se esperaba que el código duplicado violara UNIQUE(codigo)")
		}
		if classified := repository.ClassifyUniqueViolation(err); classified != repository.ErrCodigoExists {
			t.Fatalf("se esperaba ErrCodigoExists, got %v", classified)
		}
	})

	t.Run("EstadoMatricula_CheckConstraint", func(t *testing.T) {
		tag := suf(6)
		_, err := h.app.Exec(h.ctx, `
			INSERT INTO estudiantes (codigo, nombres, apellidos, estado_matricula) VALUES ($1, 'X', 'Y', 'INVENTADO')
		`, "CHK-"+tag)
		if err == nil {
			t.Fatal("un estado_matricula fuera del catálogo debe violar el CHECK")
		}
	})
}
