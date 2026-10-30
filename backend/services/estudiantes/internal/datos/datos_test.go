package datos

import (
	"context"
	"strings"
	"testing"

	"tabular"
)

// Maestro simulado (datos ficticios): sin base de datos.
type maestroFalso struct {
	registrados []Registrado
	nombres     map[string]int
	secciones   []SeccionCatalogo
}

func (m maestroFalso) Identificadores(_ context.Context, docs [][2]string, codigos, siagies []string) ([]Registrado, error) {
	var out []Registrado
	for _, r := range m.registrados {
		hit := false
		for _, d := range docs {
			hit = hit || (d[0] == r.TipoDocumento && d[1] == r.NumeroDocumento)
		}
		for _, c := range codigos {
			hit = hit || c == r.Codigo
		}
		for _, s := range siagies {
			hit = hit || (s != "" && s == r.CodigoSiagie)
		}
		if hit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m maestroFalso) CoincidenciasNombre(_ context.Context, nombres []string) (map[string]int, error) {
	out := map[string]int{}
	for _, n := range nombres {
		if c := m.nombres[strings.ToUpper(n)]; c > 0 {
			out[n] = c
		}
	}
	return out, nil
}

func (m maestroFalso) Secciones(context.Context) ([]SeccionCatalogo, error) { return m.secciones, nil }

var maestro = maestroFalso{
	registrados: []Registrado{
		{ID: "e1", Codigo: "DNI11111111", TipoDocumento: "DNI", NumeroDocumento: "11111111", Apellidos: "Rojas", Nombres: "Ana"},
		{ID: "e2", Codigo: "EST-0002", TipoDocumento: "DNI", NumeroDocumento: "22222222", Apellidos: "Quispe", Nombres: "Luis"},
		{ID: "e3", Codigo: "SDABCDEF123456", TipoDocumento: "SIN_DOCUMENTO", Apellidos: "Mamani", Nombres: "Rosa"},
	},
	nombres:   map[string]int{"MAMANI ROSA": 1, "PEREZ JUAN": 2},
	secciones: []SeccionCatalogo{{"SECUNDARIA", "1", "A"}, {"SECUNDARIA", "2", "B"}},
}

func analizar(t *testing.T, enc []string, filas ...[]string) (*Resultado, tabular.Mapeo) {
	t.Helper()
	tab := &tabular.Tabla{Encabezados: enc}
	for i, f := range filas {
		tab.Filas = append(tab.Filas, tabular.Fila{Numero: i + 2, Valores: f})
	}
	m := Esquema.Sugerir(tab, nil)
	res, err := Analizar(context.Background(), tab, m, "huella-prueba", maestro)
	if err != nil {
		t.Fatal(err)
	}
	return res, m
}

func porFila(res *Resultado) map[int]*FilaResultado {
	out := map[int]*FilaResultado{}
	for _, f := range res.Filas {
		out[f.Fila] = f
	}
	return out
}

func TestEsquema_ReconoceFormatoDelColegioYDeLaTabla(t *testing.T) {
	colegio := &tabular.Tabla{Encabezados: []string{"Nro", "TIPO.DOC", "DNI", "APELLIDO PATERNO", "APELLIDO MATERNO", "NOMBRES", "CONDICION", "ESTADO", "NIVEL"}}
	m := Esquema.Sugerir(colegio, nil)
	want := []string{"", "tipo_documento", "numero_documento", "apellido_paterno", "apellido_materno", "nombres", "condicion", "estado", "nivel"}
	for i, w := range want {
		if a := m.Asignaciones[i]; a.Campo != w || (w != "" && a.Confianza != tabular.ConfianzaAlta) {
			t.Errorf("colegio col %d: %+v, se esperaba %q", i, a, w)
		}
	}
	tabla := &tabular.Tabla{Encabezados: []string{"Código", "Estudiante", "Documento", "Grado", "Sección", "Estado", "Acciones"}}
	m = Esquema.Sugerir(tabla, nil)
	want = []string{"codigo", "nombre_completo", "documento", "grado", "seccion", "estado", ""}
	for i, w := range want {
		if a := m.Asignaciones[i]; a.Campo != w {
			t.Errorf("tabla col %d: %+v, se esperaba %q", i, a, w)
		}
	}
	// Conceptos distintos no se confunden.
	otros := &tabular.Tabla{Encabezados: []string{"Estado de matrícula", "Nivel", "Grado académico", "Aula"}}
	m = Esquema.Sugerir(otros, nil)
	if m.Asignaciones[0].Campo != "estado_matricula" || m.Asignaciones[1].Campo != "nivel" {
		t.Fatalf("estado de matrícula y nivel: %+v", m.Asignaciones[:2])
	}
	for _, i := range []int{2, 3} {
		if m.Asignaciones[i].Confianza == tabular.ConfianzaAlta {
			t.Fatalf("GRADO ACADÉMICO y AULA siempre se confirman: %+v", m.Asignaciones[i])
		}
	}
}

func TestAnalizar_Categorias(t *testing.T) {
	enc := []string{"TIPO.DOC", "DNI", "APELLIDO PATERNO", "APELLIDO MATERNO", "NOMBRES", "CONDICION", "ESTADO", "NIVEL"}
	res, _ := analizar(t, enc,
		[]string{"DNI", "01234567", "Núñez", "Peña", "José", "ESTUDIANTE", "ACTIVO", "SECUNDARIA"},     // 2 nuevo
		[]string{"DNI", "11111111", "Rojas", "", "Ana", "ESTUDIANTE", "ACTIVO", "SECUNDARIA"},          // 3 existente
		[]string{"DNI", "22222222", "Quispe", "", "Pedro", "ESTUDIANTE", "ACTIVO", "SECUNDARIA"},       // 4 existente, nombre distinto
		[]string{"DNI", "01234567", "Otro", "", "Repetido", "ESTUDIANTE", "ACTIVO", "SECUNDARIA"},      // 5 duplicado de 2
		[]string{"", "", "Perez", "", "Juan", "ESTUDIANTE", "ACTIVO", "SECUNDARIA"},                    // 6 por confirmar
		[]string{"DNI", "1234567", "Lopez", "", "Ana", "ESTUDIANTE", "ACTIVO", "SECUNDARIA"},           // 7 inválido (7 dígitos)
		[]string{"DNI", "33333333", "Diaz", "", "Eva", "DOCENTE NOMBRADO", "ACTIVO", "SECUNDARIA"},     // 8 inválido (no estudiante)
		[]string{"CARNE EXT.", "AB-12.34", "Silva", "", "Kim", "ESTUDIANTE", "INACTIVO", "SECUNDARIA"}, // 9 nuevo con correcciones
		[]string{"", "", "Torres", "", "Leo", "ESTUDIANTE", "", "UNIVERSIDAD"},                         // 10 inválido (nivel)
	)
	f := porFila(res)
	want := map[int]string{2: Nuevo, 3: Existente, 4: Existente, 5: Duplicado, 6: PorConfirmar, 7: Invalido, 8: Invalido, 9: Nuevo, 10: Invalido}
	for fila, cat := range want {
		if f[fila].Categoria != cat {
			t.Errorf("fila %d: %s, se esperaba %s (%v %v)", fila, f[fila].Categoria, cat, f[fila].Errores, f[fila].Avisos)
		}
	}
	if f[2].Registro.Codigo != "DNI01234567" || *f[2].Registro.NumeroDocumento != "01234567" || f[2].Registro.Apellidos != "Núñez Peña" {
		t.Fatalf("registro nuevo: %+v", f[2].Registro)
	}
	if !strings.Contains(strings.Join(f[4].Avisos, " "), "nombres difieren") {
		t.Fatalf("documento igual con nombre distinto: aviso de revisión: %v", f[4].Avisos)
	}
	if !strings.Contains(strings.Join(f[5].Avisos, " "), "fila 2") {
		t.Fatalf("duplicado señala la fila original: %v", f[5].Avisos)
	}
	if !strings.Contains(strings.Join(f[7].Errores, " "), "texto") || !strings.Contains(strings.Join(f[8].Errores, " "), "solo se importan estudiantes") {
		t.Fatalf("motivos: %v | %v", f[7].Errores, f[8].Errores)
	}
	if f[9].Registro.TipoDocumento != "CE" || *f[9].Registro.NumeroDocumento != "AB1234" || len(f[9].Correcciones) == 0 {
		t.Fatalf("alias de tipo y separadores quitados con corrección visible: %+v %v", f[9].Registro, f[9].Correcciones)
	}
}

func TestAnalizar_ConflictosDeIdentificadores(t *testing.T) {
	enc := []string{"Código", "DNI", "Apellidos", "Nombres"}
	res, _ := analizar(t, enc,
		[]string{"EST-0002", "11111111", "Rojas", "Ana"},   // 2: código de e2, documento de e1
		[]string{"EST-9999", "22222222", "Quispe", "Luis"}, // 3: documento de e2, otro código
		[]string{"DNI11111111", "", "Rojas", "Ana"},        // 4: código provisto de e1, sin documento -> existente
	)
	f := porFila(res)
	if f[2].Categoria != Conflicto || !strings.Contains(strings.Join(f[2].Errores, " "), "distintos") {
		t.Fatalf("identificadores de dos estudiantes: %+v", f[2])
	}
	if f[3].Categoria != Conflicto || !strings.Contains(strings.Join(f[3].Errores, " "), "EST-9999") {
		t.Fatalf("código que contradice al registrado: %+v", f[3])
	}
	if f[4].Categoria != Existente || f[4].Coincidencia.ID != "e1" {
		t.Fatalf("existente por código: %+v", f[4])
	}
}

func TestAnalizar_DocumentoCombinadoYNombreCompleto(t *testing.T) {
	enc := []string{"Código", "Estudiante", "Documento", "Grado", "Sección", "Estado"}
	res, m := analizar(t, enc,
		[]string{"", "Gómez Ruiz, María José", "DNI: 01234567", "1", "A", "ACTIVO"},
		[]string{"", "Sin Coma María", "87654321", "9", "Z", "ACTIVO"},
	)
	if m.RequiereConfirmacion() {
		// "Documento" y "Estudiante" son sinónimos exactos en este esquema.
		for _, a := range m.Asignaciones {
			if a.Campo != "" && a.Confianza != tabular.ConfianzaAlta {
				t.Fatalf("mapeo de la tabla: %+v", a)
			}
		}
	}
	f := porFila(res)
	r := f[2].Registro
	if f[2].Categoria != Nuevo || r.TipoDocumento != "DNI" || *r.NumeroDocumento != "01234567" || r.Apellidos != "Gómez Ruiz" || r.Nombres != "María José" {
		t.Fatalf("documento combinado y «Apellidos, Nombres»: %+v %v", r, f[2].Errores)
	}
	if f[3].Categoria != Invalido || !strings.Contains(strings.Join(f[3].Errores, " "), "APELLIDOS, NOMBRES") {
		t.Fatalf("nombre completo sin coma: %+v", f[3])
	}
	// Grado/sección fuera del catálogo: solo aviso (columnas heredadas).
	res, _ = analizar(t, []string{"DNI", "Apellidos", "Nombres", "Nivel", "Grado", "Sección"},
		[]string{"76543210", "Paz", "Leo", "SECUNDARIA", "9", "Z"})
	if fr := res.Filas[0]; fr.Categoria != Nuevo || !strings.Contains(strings.Join(fr.Avisos, " "), "no coincide con ninguna sección") {
		t.Fatalf("catálogo de secciones: %+v", fr)
	}
}

func TestAnalizar_SinDocumentoReimportadoYMapeoIncompleto(t *testing.T) {
	// Fila sin documento cuyo código derivado (huella+fila) ya existe.
	enc := []string{"Apellidos", "Nombres"}
	tab := &tabular.Tabla{Encabezados: enc, Filas: []tabular.Fila{{Numero: 2, Valores: []string{"Mamani", "Rosa"}}}}
	m := Esquema.Sugerir(tab, nil)
	res, err := Analizar(context.Background(), tab, m, "h", maestro)
	if err != nil {
		t.Fatal(err)
	}
	if fr := res.Filas[0]; fr.Categoria != PorConfirmar {
		t.Fatalf("sin documento y mismo nombre que un registrado: %+v", fr)
	}
	sinNombres := &tabular.Tabla{Encabezados: []string{"DNI", "Apellidos"}, Filas: []tabular.Fila{{Numero: 2, Valores: []string{"1", "x"}}}}
	res, err = Analizar(context.Background(), sinNombres, Esquema.Sugerir(sinNombres, nil), "h", maestro)
	if err != nil || len(res.Bloqueantes) == 0 || len(res.Filas) != 0 {
		t.Fatalf("sin columna de nombres el análisis se bloquea: %+v %v", res, err)
	}
}

func TestExportacion_ValoresYColumnas(t *testing.T) {
	f := FilaMaestro{Codigo: "0007", TipoDocumento: "DNI", NumeroDocumento: "01234567", Apellidos: "Peña Ruiz", Nombres: "Ana", Grado: "1", Estado: "ACTIVO"}
	for campo, want := range map[string]string{
		"codigo": "0007", "documento": "DNI: 01234567", "nombre_completo": "Peña Ruiz, Ana", "condicion": "ESTUDIANTE",
		"matricula_grado": "", "matricula_periodo": "", "acciones": "",
	} {
		if got := f.ValorExport(campo); got != want {
			t.Errorf("%s = %q, se esperaba %q", campo, got, want)
		}
	}
	if (FilaMaestro{}).ValorExport("documento") != "Sin documento" {
		t.Fatal("sin documento")
	}
	cols, err := ColumnasExport([]tabular.ColumnaExport{{Clave: "codigo"}, {Clave: "acciones"}, {Clave: "estado", Encabezado: "Situación"}})
	if err != nil || cols[0].Encabezado != "Código" || cols[1].Encabezado != "Acciones" || cols[2].Encabezado != "Situación" {
		t.Fatalf("columnas: %+v %v", cols, err)
	}
	for _, malas := range [][]tabular.ColumnaExport{nil, {{Clave: "password"}}, {{Clave: "codigo"}, {Clave: "codigo"}}} {
		if _, err := ColumnasExport(malas); tabular.CodigoDe(err) != tabular.ErrOpcion {
			t.Fatalf("columnas inválidas %+v: %v", malas, err)
		}
	}
}
