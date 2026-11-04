package integration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"estudiantes/estudiantes"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BuscarEstudiantesSeleccion contra PostgreSQL. Requiere una base AISLADA
// (INTEGRATION_TEST=true + TEST_ESTUDIANTES_DB_URL): siembra estudiantes con
// un sufijo aleatorio y los elimina al terminar. Nunca ejecutar contra la base
// real.
func TestBuscarEstudiantesSeleccion(t *testing.T) {
	h := newHarness(t)
	sfx := sufijoAleatorio(t)
	// Sistema consumidor con el scope del cliente Servicio Psicología.
	sistema := h.asService("estudiantes.basic.view estudiantes.search.basic")

	dni, dniCE := dniAleatorio(t), dniAleatorio(t)
	siagie := dniAleatorio(t) + dniAleatorio(t)[:6] // 14 dígitos
	activo := h.seed(seedInput{
		Codigo: "SEL" + sfx, Nombres: "Zóila" + sfx, Apellidos: "Quispe" + sfx + " Huamán" + sfx,
		ApellidoPaterno: "Quispe" + sfx, ApellidoMaterno: "Huamán" + sfx,
		NumeroDocumento: dni, CodigoSiagie: siagie, Grado: "3", Seccion: "B",
	})
	h.seed(seedInput{Codigo: "SEI" + sfx, Nombres: "Zóila" + sfx, Apellidos: "Inactiva" + sfx, Estado: "INACTIVO", Grado: "4", Seccion: "A"})
	h.seed(seedInput{Codigo: "SEC" + sfx, Nombres: "Extranjera" + sfx, Apellidos: "Carnet" + sfx, TipoDocumento: "CE", NumeroDocumento: dniCE})

	buscar := func(q string, page, size int32) (*estudiantes.BuscarEstudiantesSeleccionResponse, error) {
		return h.client.BuscarEstudiantesSeleccion(sistema, &estudiantes.BuscarEstudiantesSeleccionRequest{Q: q, Page: page, PageSize: size})
	}
	unico := func(t *testing.T, q string) {
		t.Helper()
		r, err := buscar(q, 1, 20)
		if err != nil || r.Total != 1 || len(r.Estudiantes) != 1 || r.Estudiantes[0].Id != activo {
			t.Fatalf("%q debe encontrar solo al estudiante activo: %v %v", q, r, err)
		}
		e := r.Estudiantes[0]
		if e.NombreCompleto != "Zóila"+sfx+" Quispe"+sfx+" Huamán"+sfx || e.Grado != "3" || e.Seccion != "B" {
			t.Fatalf("datos mínimos inesperados: %+v", e)
		}
	}
	vacio := func(t *testing.T, q string) {
		t.Helper()
		r, err := buscar(q, 1, 20)
		if err != nil || r.Total != 0 || len(r.Estudiantes) != 0 {
			t.Fatalf("%q no debe encontrar nada: %v %v", q, r, err)
		}
	}

	t.Run("nombres sin tildes ni mayúsculas (inactivo excluido)", func(t *testing.T) { unico(t, "zoila"+sfx) })
	t.Run("apellido paterno", func(t *testing.T) { unico(t, "QUISPE"+sfx) })
	t.Run("apellido materno", func(t *testing.T) { unico(t, "huaman"+sfx) })
	t.Run("términos en cualquier orden", func(t *testing.T) { unico(t, "Huaman"+sfx+" zoila"+sfx) })
	t.Run("DNI completo", func(t *testing.T) { unico(t, dni) })
	t.Run("DNI + apellido", func(t *testing.T) { unico(t, "quispe"+sfx+" "+dni) })
	t.Run("DNI parcial", func(t *testing.T) { vacio(t, dni[:6]) })
	t.Run("código SIAGIE", func(t *testing.T) { vacio(t, siagie) })
	t.Run("código interno", func(t *testing.T) { vacio(t, "SEL"+sfx) })
	t.Run("CE de 8 dígitos no se busca como DNI", func(t *testing.T) { vacio(t, dniCE) })
	t.Run("solo inactivo", func(t *testing.T) { vacio(t, "Inactiva"+sfx) })

	t.Run("paginación", func(t *testing.T) {
		otro := h.seed(seedInput{Codigo: "SEP" + sfx, Nombres: "Zóila" + sfx, Apellidos: "Ramos" + sfx, Grado: "5", Seccion: "C"})
		p1, err := buscar("zoila"+sfx, 1, 1)
		if err != nil || p1.Total != 2 || len(p1.Estudiantes) != 1 {
			t.Fatalf("página 1: %v %v", p1, err)
		}
		p2, _ := buscar("zoila"+sfx, 2, 1)
		if len(p2.Estudiantes) != 1 || p2.Estudiantes[0].Id == p1.Estudiantes[0].Id {
			t.Fatalf("página 2 debe traer el otro estudiante: %v", p2)
		}
		if p1.Estudiantes[0].Id != activo && p1.Estudiantes[0].Id != otro {
			t.Fatal("resultado inesperado")
		}
	})

	t.Run("sistema sin estudiantes.search.basic", func(t *testing.T) {
		_, err := h.client.BuscarEstudiantesSeleccion(h.asService("estudiantes.basic.view"), &estudiantes.BuscarEstudiantesSeleccionRequest{Q: "zoila" + sfx})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("basic.view no autoriza la búsqueda de selección: %v", err)
		}
	})

	t.Run("un JWT humano no sustituye al token de servicio", func(t *testing.T) {
		usuario := "6f1c2d3e-0000-4000-8000-00000000c002"
		h.grant(usuario, "estudiantes.view")
		h.grant(usuario, "estudiantes.search.basic")
		_, err := h.client.BuscarEstudiantesSeleccion(h.as(usuario), &estudiantes.BuscarEstudiantesSeleccionRequest{Q: "zoila" + sfx})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("un usuario no debe usar la búsqueda de sistemas: %v", err)
		}
	})

	// FASE 1: respuesta mínima para sistemas; los usuarios la conservan completa.
	t.Run("GetEstudiantesBasic mínimo para sistemas", func(t *testing.T) {
		r, err := h.client.GetEstudiantesBasic(sistema, &estudiantes.GetEstudiantesBasicRequest{Ids: []string{activo}})
		if err != nil || len(r.Estudiantes) != 1 {
			t.Fatalf("basic: %v %v", r, err)
		}
		e := r.Estudiantes[0]
		if e.NombreCompleto == "" || e.Grado != "3" || e.Seccion != "B" || e.NumeroDocumento != "" || e.CodigoSiagie != "" || e.Codigo != "" || e.Nombres != "" {
			t.Fatalf("respuesta no mínima: %+v", e)
		}
		usuario := "6f1c2d3e-0000-4000-8000-00000000c003"
		h.grant(usuario, "estudiantes.view")
		u, err := h.client.GetEstudiantesBasic(h.as(usuario), &estudiantes.GetEstudiantesBasicRequest{Ids: []string{activo}})
		if err != nil || len(u.Estudiantes) != 1 || u.Estudiantes[0].NumeroDocumento != dni || u.Estudiantes[0].CodigoSiagie == "" {
			t.Fatalf("el usuario debe conservar la respuesta completa: %v", err)
		}
	})

	t.Run("ResolveEstudiante por DNI completo, sin devolverlo", func(t *testing.T) {
		r, err := h.client.ResolveEstudiante(sistema, &estudiantes.ResolveEstudianteRequest{TipoDocumento: "DNI", NumeroDocumento: dni})
		if err != nil || r.Estudiante.Id != activo || r.Estudiante.NumeroDocumento != "" || r.Estudiante.CodigoSiagie != "" {
			t.Fatalf("resolver mínimo: %v %v", r, err)
		}
		for nombre, req := range map[string]*estudiantes.ResolveEstudianteRequest{
			"DNI parcial": {TipoDocumento: "DNI", NumeroDocumento: dni[:6]},
			"SIAGIE":      {CodigoSiagie: siagie},
			"código":      {Codigo: "SEL" + sfx},
		} {
			if _, err := h.client.ResolveEstudiante(sistema, req); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("%s: se esperaba InvalidArgument: %v", nombre, err)
			}
		}
	})

	// FASE 3: solo lectura, aunque el token tuviera permisos de escritura.
	t.Run("sistemas no escriben ni leen datos personales", func(t *testing.T) {
		todo := h.asService("estudiantes.view estudiantes.basic.view estudiantes.personal.view estudiantes.create estudiantes.update estudiantes.matricula.update")
		_, e1 := h.client.CreateEstudiante(todo, &estudiantes.CreateEstudianteRequest{})
		_, e2 := h.client.UpdateEstudiante(todo, &estudiantes.UpdateEstudianteRequest{Id: activo})
		_, e3 := h.client.MatricularEstudiante(todo, &estudiantes.MatricularEstudianteRequest{EstudianteId: activo})
		_, e4 := h.client.GetEstudiantePersonal(todo, &estudiantes.GetEstudiantePersonalRequest{Id: activo})
		for i, err := range []error{e1, e2, e3, e4} {
			if status.Code(err) != codes.PermissionDenied {
				t.Fatalf("operación %d: se esperaba PermissionDenied: %v", i+1, err)
			}
		}
	})
}

func sufijoAleatorio(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func dniAleatorio(t *testing.T) string {
	t.Helper()
	n, err := rand.Int(rand.Reader, big.NewInt(90000000))
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%08d", n.Int64()+10000000)
}
