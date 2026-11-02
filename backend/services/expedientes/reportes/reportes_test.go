package reportes

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var ahora = time.Date(2026, 10, 30, 9, 0, 0, 0, Zona) // viernes

func claves(cs []Conteo) string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Clave)
	}
	return strings.Join(out, ",")
}

func TestGranularidad(t *testing.T) {
	for in, esperado := range map[string]string{"": "MES", "dia": "DIA", " Semana ": "SEMANA", "MES": "MES"} {
		if g, err := Granularidad(in); err != nil || g != esperado {
			t.Errorf("%q -> %q %v", in, g, err)
		}
	}
	if _, err := Granularidad("ANIO"); !errors.Is(err, ErrGranularidad) {
		t.Fatal("granularidad inválida")
	}
}

func TestValidarPeriodo(t *testing.T) {
	ok := [][2]string{{"", ""}, {"2026-01-01", "2026-10-30"}, {"2026-10-30", ""}, {"", "2027-01-01"}, {"2010-01-01", "2020-01-01"}}
	for _, p := range ok {
		if err := ValidarPeriodo(p[0], p[1], ahora); err != nil {
			t.Errorf("%v: %v", p, err)
		}
	}
	malos := map[[2]string]error{
		{"30/10/2026", ""}:           ErrFormatoDesde,
		{"", "2026-02-30"}:           ErrFormatoHasta,
		{"2026-10-31", ""}:           ErrRangoDesde,
		{"1999-12-31", ""}:           ErrRangoDesde,
		{"", "1999-12-31"}:           ErrRangoHasta,
		{"2026-10-10", "2026-10-01"}: ErrOrden,
		{"2010-01-01", "2020-01-02"}: ErrDiezAnios,
	}
	for p, e := range malos {
		if err := ValidarPeriodo(p[0], p[1], ahora); !errors.Is(err, e) {
			t.Errorf("%v: %v, se esperaba %v", p, err, e)
		}
	}
}

func TestClave(t *testing.T) {
	casos := map[[2]string]string{
		{"2026-10-30", GranularidadDia}:    "2026-10-30",
		{"2026-10-30", GranularidadSemana}: "2026-10-26", // lunes
		{"2026-10-26", GranularidadSemana}: "2026-10-26",
		{"2026-11-01", GranularidadSemana}: "2026-10-26", // domingo: misma semana
		{"2026-10-30", GranularidadMes}:    "2026-10",
	}
	for in, esperado := range casos {
		if c, ok := Clave(in[0], in[1]); !ok || c != esperado {
			t.Errorf("%v -> %s", in, c)
		}
	}
	if _, ok := Clave("x", GranularidadDia); ok {
		t.Fatal("fecha inválida")
	}
}

func TestSerie(t *testing.T) {
	datos := map[string]int{"2026-10-26": 2, "2026-10-28": 1, "2026-11-02": 4}
	dia, err := Serie(datos, GranularidadDia, "2026-10-26", "2026-10-29", ahora)
	if err != nil || claves(dia) != "2026-10-26,2026-10-27,2026-10-28,2026-10-29" || dia[1].Cantidad != 0 || dia[2].Cantidad != 1 {
		t.Fatalf("día: %v %v", dia, err)
	}
	sem, _ := Serie(datos, GranularidadSemana, "2026-10-28", "2026-11-03", ahora)
	if claves(sem) != "2026-10-26,2026-11-02" || sem[0].Cantidad != 3 || sem[1].Cantidad != 4 {
		t.Fatalf("semana: %v", sem)
	}
	mes, _ := Serie(datos, GranularidadMes, "", "", ahora)
	if claves(mes) != "2026-10,2026-11" || mes[0].Cantidad != 3 || mes[1].Cantidad != 4 {
		t.Fatalf("mes (dato posterior a hoy no se pierde): %v", mes)
	}
	if vacia, _ := Serie(nil, GranularidadDia, "", "", ahora); len(vacia) != 0 {
		t.Fatal("sin datos ni desde: vacía")
	}
	if _, err := Serie(nil, GranularidadDia, "2025-01-01", "2026-10-30", ahora); !errors.Is(err, ErrSerieLarga) {
		t.Fatal("más de 366 días por día debe rechazarse")
	}
	if s, err := Serie(nil, GranularidadMes, "2017-01-01", "2026-10-30", ahora); err != nil || len(s) != 118 {
		t.Fatalf("mes largo admitido: %d %v", len(s), err)
	}
}

func TestNormalizarArea(t *testing.T) {
	casos := map[string]string{
		"DIRECTOR": "DIRECTOR", "SECRETARIA": "SECRETARIA", "Dirección": "DIRECTOR", "Direccion": "DIRECTOR",
		"Secretaría": "SECRETARIA", "Secretaria": "SECRETARIA", "subdirección": "SUBDIRECTOR", "Administración": "ADMIN",
		"Docente": "DOCENTE", "secretaria.demo (Secretaría)": "SECRETARIA", "director.demo (Dirección)": "DIRECTOR",
		"Direcci�n": SinClasificar, "e2esec_1 (Secretar�a)": SinClasificar, "Sistema": SinClasificar,
		"": SinClasificar, "Psicología": SinClasificar, "Solicitante": SinClasificar,
	}
	for in, esperado := range casos {
		if got := NormalizarArea(in); got != esperado {
			t.Errorf("%q -> %s, se esperaba %s", in, got, esperado)
		}
	}
	// Nunca devuelve el texto original (puede contener nombres de usuario).
	if got := NormalizarArea("juan.perez (Dirección)"); strings.Contains(got, "juan") {
		t.Fatal("expone el nombre de usuario")
	}
}

func TestOrdenar(t *testing.T) {
	r := Ordenar([]string{"A", "B"}, map[string]int{"B": 2, "Z": 1})
	if claves(r) != "A,B,Z" || r[0].Cantidad != 0 {
		t.Fatalf("%v", r)
	}
}
