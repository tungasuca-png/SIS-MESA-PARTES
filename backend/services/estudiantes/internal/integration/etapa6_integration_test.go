package integration

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"estudiantes/estudiantes"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestEtapa6 verifica en PostgreSQL real lo que necesita la ficha integral
// del frontend: orden del listado (lista blanca), cambio de parentesco
// atómico con historial e historial de cambios sin valores ni lecturas.
func TestEtapa6(t *testing.T) {
	h := newHarness(t)
	base := time.Now().UnixNano()
	tag := strconv.FormatInt(base%100_000_000, 36)
	n := 0
	codigo := func() string { n++; return fmt.Sprintf("E6-%s-%d", tag, n) }

	const (
		admin    = "00000000-0000-4000-8000-0000000000d1"
		lector   = "00000000-0000-4000-8000-0000000000d2"
		familiaU = "00000000-0000-4000-8000-0000000000d3"
	)
	for _, p := range []string{"estudiantes.view", "estudiantes.create", "estudiantes.update", "estudiantes.change_estado",
		"estudiantes.personal.view", "estudiantes.personal.update", "estudiantes.familia.view"} {
		h.grant(admin, p)
	}
	h.grant(lector, "estudiantes.view")
	h.grant(familiaU, "estudiantes.familia.update")

	wantErr := func(t *testing.T, err error, code codes.Code, prefijo, que string) {
		t.Helper()
		st := status.Convert(err)
		if err == nil || st.Code() != code || !strings.HasPrefix(st.Message(), prefijo+":") {
			t.Fatalf("%s: se esperaba %v %q, se obtuvo %v", que, code, prefijo, err)
		}
	}
	crear := func(t *testing.T, nombres, paterno string) *estudiantes.EstudianteBasico {
		t.Helper()
		resp, err := h.client.CreateEstudiante(h.as(admin), &estudiantes.CreateEstudianteRequest{
			Codigo: codigo(), Nombres: nombres, ApellidoPaterno: paterno, ApellidoMaterno: tag})
		if err != nil {
			t.Fatalf("CreateEstudiante: %v", err)
		}
		h.created = append(h.created, resp.Estudiante.Id)
		return resp.Estudiante
	}

	t.Run("Search_Orden_ListaBlanca", func(t *testing.T) {
		a := crear(t, "Zoe", "Alva")
		time.Sleep(10 * time.Millisecond)
		b := crear(t, "Ana", "Zuniga")
		ids := func(orden string) []string {
			resp, err := h.client.SearchEstudiantes(h.as(lector), &estudiantes.SearchEstudiantesRequest{Q: tag, Orden: orden})
			if err != nil {
				t.Fatalf("Search orden=%q: %v", orden, err)
			}
			var out []string
			for _, e := range resp.Estudiantes {
				out = append(out, e.Id)
			}
			return out
		}
		if got := ids(""); len(got) != 2 || got[0] != a.Id {
			t.Fatalf("orden por apellidos: %v", got)
		}
		if got := ids("recientes"); len(got) != 2 || got[0] != b.Id {
			t.Fatalf("orden recientes: %v", got)
		}
		if got := ids("CODIGO"); len(got) != 2 || got[0] != a.Id {
			t.Fatalf("orden por código: %v", got)
		}
		_, err := h.client.SearchEstudiantes(h.as(lector), &estudiantes.SearchEstudiantesRequest{Orden: "nombres; DROP TABLE x"})
		wantErr(t, err, codes.InvalidArgument, "validation_error", "orden fuera de la lista blanca")
	})

	t.Run("CambiarParentesco_AtomicoConHistorial", func(t *testing.T) {
		e := crear(t, "Hijo", "Parentesco")
		v, err := h.client.VincularFamiliar(h.as(familiaU), &estudiantes.VincularFamiliarRequest{EstudianteId: e.Id, Parentesco: "TIO_A",
			EsApoderado: true, Desde: "2020-01-01", Persona: &estudiantes.Persona{Nombres: "Persona", ApellidoPaterno: "Prueba " + tag}})
		if err != nil {
			t.Fatal(err)
		}
		h.personas = append(h.personas, v.Familiar.Persona.Id)
		madre, err := h.client.VincularFamiliar(h.as(familiaU), &estudiantes.VincularFamiliarRequest{EstudianteId: e.Id, Parentesco: "MADRE",
			Persona: &estudiantes.Persona{Nombres: "Madre", ApellidoPaterno: "Prueba " + tag}})
		if err != nil {
			t.Fatal(err)
		}
		h.personas = append(h.personas, madre.Familiar.Persona.Id)

		// Ya hay una MADRE vigente: falla y NO cambia nada (la relación sigue vigente).
		_, err = h.client.CambiarParentescoFamiliar(h.as(familiaU), &estudiantes.CambiarParentescoFamiliarRequest{EstudianteId: e.Id, RelacionId: v.Familiar.RelacionId, Parentesco: "MADRE"})
		wantErr(t, err, codes.AlreadyExists, "duplicate_progenitor", "segunda madre vigente")
		var vigente bool
		if err := h.admin.QueryRow(h.ctx, `SELECT vigente FROM estudiante_familiares WHERE id = $1`, v.Familiar.RelacionId).Scan(&vigente); err != nil || !vigente {
			t.Fatalf("la relación debía seguir vigente tras el fallo: vigente=%v err=%v", vigente, err)
		}
		_, err = h.client.CambiarParentescoFamiliar(h.as(familiaU), &estudiantes.CambiarParentescoFamiliarRequest{EstudianteId: e.Id, RelacionId: v.Familiar.RelacionId, Parentesco: "TIO_A"})
		wantErr(t, err, codes.FailedPrecondition, "no_changes", "mismo parentesco")
		_, err = h.client.CambiarParentescoFamiliar(h.as(familiaU), &estudiantes.CambiarParentescoFamiliarRequest{EstudianteId: e.Id, RelacionId: v.Familiar.RelacionId, Parentesco: "ABUELO_A", Desde: "2019-01-01"})
		wantErr(t, err, codes.InvalidArgument, "invalid_date", "desde anterior al inicio")
		_, err = h.client.CambiarParentescoFamiliar(h.as(lector), &estudiantes.CambiarParentescoFamiliarRequest{EstudianteId: e.Id, RelacionId: v.Familiar.RelacionId, Parentesco: "ABUELO_A"})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "sin familia.update")

		resp, err := h.client.CambiarParentescoFamiliar(h.as(familiaU), &estudiantes.CambiarParentescoFamiliarRequest{EstudianteId: e.Id, RelacionId: v.Familiar.RelacionId, Parentesco: "abuelo_a"})
		if err != nil {
			t.Fatalf("cambiar parentesco: %v", err)
		}
		if resp.Anterior.Vigente || resp.Anterior.Parentesco != "TIO_A" || resp.Anterior.Hasta == "" ||
			!resp.Familiar.Vigente || resp.Familiar.Parentesco != "ABUELO_A" || !resp.Familiar.EsApoderado ||
			resp.Familiar.Persona.Id != v.Familiar.Persona.Id || resp.Familiar.RelacionId == v.Familiar.RelacionId {
			t.Fatalf("resultado: %+v", resp)
		}
		lista, err := h.client.ListFamiliaresEstudiante(h.as(admin), &estudiantes.ListFamiliaresEstudianteRequest{EstudianteId: e.Id, IncluirHistorial: true})
		if err != nil || len(lista.Familiares) != 3 {
			t.Fatalf("historial familiar (madre + nueva + cerrada): %+v err=%v", lista, err)
		}
	})

	// Matriz de permisos acordada: cada operación sensible depende de su
	// permiso propio, no de "ver" ni de "editar" en general.
	t.Run("Permisos_EspecificosPorOperacion", func(t *testing.T) {
		const editor = "00000000-0000-4000-8000-0000000000d4" // view + update (sin change_estado)
		h.grant(editor, "estudiantes.view")
		h.grant(editor, "estudiantes.update")
		e := crear(t, "Permisos", "Especificos")

		// Editar no basta para activar o inactivar: exige estudiantes.change_estado.
		_, err := h.client.ChangeEstadoEstudiante(h.as(editor), &estudiantes.ChangeEstadoEstudianteRequest{Id: e.Id, ExpectedVersion: e.Version, Estado: "INACTIVO"})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "cambiar estado sin change_estado")
		if _, err := h.client.UpdateEstudiante(h.as(editor), &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: e.Version, Nombres: strPtr("Permisos Editado")}); err != nil {
			t.Fatalf("con estudiantes.update sí edita: %v", err)
		}

		// El directorio (estudiantes.view) no da datos personales, familia ni historial.
		_, err = h.client.GetEstudiantePersonal(h.as(lector), &estudiantes.GetEstudiantePersonalRequest{Id: e.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "datos personales con solo estudiantes.view")
		_, err = h.client.ListFamiliaresEstudiante(h.as(lector), &estudiantes.ListFamiliaresEstudianteRequest{EstudianteId: e.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "familia con solo estudiantes.view")
		_, err = h.client.ListHistorialEstudiante(h.as(lector), &estudiantes.ListHistorialEstudianteRequest{EstudianteId: e.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "historial con solo estudiantes.view")
		// Quien mantiene el registro (estudiantes.update) sí ve el historial.
		if _, err := h.client.ListHistorialEstudiante(h.as(editor), &estudiantes.ListHistorialEstudianteRequest{EstudianteId: e.Id}); err != nil {
			t.Fatalf("historial con estudiantes.update: %v", err)
		}
	})

	t.Run("Historial_SoloCambios_SinValores", func(t *testing.T) {
		e := crear(t, "Historia", "Cambios")
		up, err := h.client.UpdateEstudiante(h.as(admin), &estudiantes.UpdateEstudianteRequest{Id: e.Id, ExpectedVersion: e.Version, Nombres: strPtr("Historia Editada")})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.client.ChangeEstadoEstudiante(h.as(admin), &estudiantes.ChangeEstadoEstudianteRequest{Id: e.Id, ExpectedVersion: up.Estudiante.Version, Estado: "INACTIVO"}); err != nil {
			t.Fatal(err)
		}
		// Una lectura auditada que NO debe aparecer en el historial.
		if _, err := h.client.GetEstudiantePersonal(h.as(admin), &estudiantes.GetEstudiantePersonalRequest{Id: e.Id}); err != nil {
			t.Fatal(err)
		}

		// Revela quién editó qué: solo con estudiantes.update, no basta el directorio.
		_, err = h.client.ListHistorialEstudiante(h.as(lector), &estudiantes.ListHistorialEstudianteRequest{EstudianteId: e.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "historial con solo estudiantes.view")
		resp, err := h.client.ListHistorialEstudiante(h.as(admin), &estudiantes.ListHistorialEstudianteRequest{EstudianteId: e.Id})
		if err != nil || resp.Total != 3 || len(resp.Eventos) != 3 {
			t.Fatalf("historial: %+v err=%v", resp, err)
		}
		if ev := resp.Eventos[0]; ev.Accion != "CHANGE_ESTADO" || ev.EstadoAnterior != "ACTIVO" || ev.EstadoNuevo != "INACTIVO" || ev.UsuarioId != admin || ev.FechaHora == "" {
			t.Fatalf("evento más reciente: %+v", ev)
		}
		if ev := resp.Eventos[1]; ev.Accion != "UPDATE" || strings.Join(ev.Campos, ",") != "nombres" {
			t.Fatalf("evento de edición: %+v", ev)
		}
		if resp.Eventos[2].Accion != "CREATE" {
			t.Fatalf("primer evento: %+v", resp.Eventos[2])
		}
		for _, ev := range resp.Eventos {
			if strings.Contains(fmt.Sprint(ev), "Historia") || ev.Accion == "PERSONAL_VIEW" {
				t.Fatalf("el historial no debe tener valores ni lecturas: %+v", ev)
			}
		}
		pag, err := h.client.ListHistorialEstudiante(h.as(admin), &estudiantes.ListHistorialEstudianteRequest{EstudianteId: e.Id, Page: 2, PageSize: 2})
		if err != nil || pag.Total != 3 || len(pag.Eventos) != 1 || pag.Eventos[0].Accion != "CREATE" {
			t.Fatalf("paginación del historial: %+v err=%v", pag, err)
		}
		_, err = h.client.ListHistorialEstudiante(h.as(admin), &estudiantes.ListHistorialEstudianteRequest{EstudianteId: "00000000-0000-4000-8000-00000000dead"})
		wantErr(t, err, codes.NotFound, "student_not_found", "historial de inexistente")
		_, err = h.client.ListHistorialEstudiante(h.as(familiaU), &estudiantes.ListHistorialEstudianteRequest{EstudianteId: e.Id})
		wantErr(t, err, codes.PermissionDenied, "permission_denied", "historial sin estudiantes.update")
	})
}

func strPtr(s string) *string { return &s }
