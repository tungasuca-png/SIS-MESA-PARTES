package logic

import (
	"strings"
	"testing"

	"estudiantes/estudiantes"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestParsearConsultaSeleccion(t *testing.T) {
	casos := []struct {
		q             string
		nombres, dnis string
		sinResultados bool
		errorEsperado bool
	}{
		{q: "Zoila", nombres: "Zoila"},
		{q: "  quispe   huaman ", nombres: "quispe,huaman"},
		{q: "45127823", dnis: "45127823"},
		{q: "quispe 45127823", nombres: "quispe", dnis: "45127823"},
		{q: "451278", sinResultados: true},     // DNI parcial
		{q: "4512782345", sinResultados: true}, // más de 8 dígitos
		{q: "quispe 1234", sinResultados: true},
		{q: "S12345", nombres: "S12345"}, // alfanumérico: solo contra nombres
		{q: "a", errorEsperado: true},
		{q: "   ", errorEsperado: true},
		{q: strings.Repeat("a", 101), errorEsperado: true},
		{q: "uno dos tres cuatro cinco seis", errorEsperado: true},
		{q: "uno dos tres cuatro cinco", nombres: "uno,dos,tres,cuatro,cinco"},
	}
	for _, c := range casos {
		f, sin, err := parsearConsultaSeleccion(c.q)
		if c.errorEsperado {
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("%q: se esperaba InvalidArgument, %v", c.q, err)
			}
			// Solo con consultas distintivas: una letra suelta aparece en cualquier mensaje.
			if q := strings.TrimSpace(c.q); len(q) >= 5 && strings.Contains(err.Error(), q) {
				t.Fatalf("%q: el error no debe repetir la consulta", c.q)
			}
			continue
		}
		if err != nil || sin != c.sinResultados {
			t.Fatalf("%q: sin=%v err=%v", c.q, sin, err)
		}
		if sin {
			continue
		}
		if strings.Join(f.Nombres, ",") != c.nombres || strings.Join(f.DNIs, ",") != c.dnis {
			t.Fatalf("%q: nombres=%v dnis=%v", c.q, f.Nombres, f.DNIs)
		}
	}
}

func TestPaginaSeleccion(t *testing.T) {
	for _, c := range []struct{ p, s, ep, es int32 }{
		{0, 0, 1, 20}, {1, 5, 1, 5}, {3, 20, 3, 20}, {1, 21, 1, 20}, {-1, -5, 1, 20}, {9999, 10, 500, 10},
	} {
		p, s := paginaSeleccion(c.p, c.s)
		if p != int(c.ep) || s != int(c.es) {
			t.Fatalf("pagina(%d,%d) = (%d,%d), se esperaba (%d,%d)", c.p, c.s, p, s, c.ep, c.es)
		}
	}
}

// El contrato de respuesta tiene EXACTAMENTE id, nombre_completo, grado y
// sección: ningún documento, código ni estado puede viajar.
func TestEstudianteSeleccionSoloCamposMinimos(t *testing.T) {
	campos := (&estudiantes.EstudianteSeleccion{}).ProtoReflect().Descriptor().Fields()
	var nombres []string
	for i := 0; i < campos.Len(); i++ {
		nombres = append(nombres, string(campos.Get(i).Name()))
	}
	if strings.Join(nombres, ",") != "id,nombre_completo,grado,seccion" {
		t.Fatalf("campos inesperados en EstudianteSeleccion: %v", nombres)
	}
	resp := (&estudiantes.BuscarEstudiantesSeleccionResponse{}).ProtoReflect().Descriptor().Fields()
	if resp.Len() != 2 || resp.ByName(protoreflect.Name("total")) == nil || resp.ByName("estudiantes") == nil {
		t.Fatal("la respuesta solo debe tener estudiantes y total")
	}
}
