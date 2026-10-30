package excel

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Datos ficticios. Cada prueba arma su propio .xlsx en memoria.

var limites = Limites{MaxBytes: 512 << 10, MaxFilas: 100}

// libro crea un .xlsx con la hoja indicada y las filas dadas (la primera
// suele ser la de encabezados). Las celdas se escriben como texto, salvo los
// valores int, que quedan como número (como al tipear un DNI sin formato).
func libro(t *testing.T, hoja string, filas ...[]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if hoja != "Sheet1" {
		if err := f.SetSheetName("Sheet1", hoja); err != nil {
			t.Fatal(err)
		}
	}
	for i, fila := range filas {
		celda, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(hoja, celda, &fila); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func cab() []any {
	out := make([]any, len(Encabezados))
	for i, h := range Encabezados {
		out[i] = h
	}
	return out
}

func fila(vals ...any) []any { return vals }

func codigoError(err error) string {
	var e *ErrorArchivo
	if errors.As(err, &e) {
		return e.Code
	}
	if err == nil {
		return ""
	}
	return strings.SplitN(err.Error(), ":", 2)[0]
}

func TestLeer_ArchivoValido_ConTildesFilasVaciasYCeros(t *testing.T) {
	contenido := libro(t, HojaEstudiantes,
		cab(),
		fila("1", "DNI", "01234567", "Núñez", "Peña", "José Ángel", "ESTUDIANTE", "ACTIVO", "PRIMARIA"),
		fila(),    // fila vacía intercalada
		fila("3"), // solo Nro: también vacía
		fila("4", "ce", " ab12 34 ", "  De la  Cruz ", "", "María  Fernanda", "estudiante", "inactivo", "secundaria"),
		fila("5", "", "", "Quispe", "Mamani", "Lucía", "ESTUDIANTE", "ACTIVO", "INICIAL"),
	)
	filas, err := Leer(contenido, limites)
	if err != nil {
		t.Fatalf("Leer: %v", err)
	}
	if len(filas) != 3 {
		t.Fatalf("se esperaban 3 filas con datos, hay %d: %+v", len(filas), filas)
	}
	if filas[0].Numero != 2 || filas[1].Numero != 5 || filas[2].Numero != 6 {
		t.Fatalf("los números de fila deben ser los reales de Excel: %d %d %d", filas[0].Numero, filas[1].Numero, filas[2].Numero)
	}

	r, errs := Validar(filas[0])
	if len(errs) != 0 || *r.NumeroDocumento != "01234567" || r.Apellidos != "Núñez Peña" || r.Nombres != "José Ángel" {
		t.Fatalf("DNI con cero inicial y tildes deben conservarse: %+v %v", r, errs)
	}
	r, errs = Validar(filas[1])
	if len(errs) != 0 || r.TipoDocumento != "CE" || *r.NumeroDocumento != "AB1234" || r.Apellidos != "De la Cruz" ||
		r.ApellidoMaterno != nil || r.Nombres != "María Fernanda" || r.Estado != "INACTIVO" || r.Nivel != "SECUNDARIA" {
		t.Fatalf("normalización (espacios, mayúsculas solo en códigos): %+v %v", r, errs)
	}
	r, errs = Validar(filas[2])
	if len(errs) != 0 || r.TipoDocumento != "SIN_DOCUMENTO" || r.NumeroDocumento != nil {
		t.Fatalf("TIPO.DOC y DNI vacíos = SIN_DOCUMENTO: %+v %v", r, errs)
	}
}

func TestLeer_DNINumericoSinFormatoNoSaleEnNotacionCientifica(t *testing.T) {
	contenido := libro(t, HojaEstudiantes, cab(),
		fila("1", "DNI", 87654321, "Rojas", "", "Ana", "ESTUDIANTE", "ACTIVO", "PRIMARIA"),
		fila("2", "DNI", 1234567, "Rojas", "", "Luis", "ESTUDIANTE", "ACTIVO", "PRIMARIA"), // perdió el 0 inicial
	)
	filas, err := Leer(contenido, limites)
	if err != nil {
		t.Fatal(err)
	}
	if filas[0].Documento != "87654321" {
		t.Fatalf("DNI numérico debe leerse tal cual, no %q", filas[0].Documento)
	}
	_, errs := Validar(filas[1])
	if len(errs) != 1 || !strings.Contains(errs[0], "escríbalo como texto") {
		t.Fatalf("un DNI de 7 dígitos debe explicar lo de los ceros: %v", errs)
	}
}

func TestLeer_ErroresDeArchivo(t *testing.T) {
	encabezadoMal := cab()
	encabezadoMal[2] = "DOCUMENTO"
	faltaColumna := cab()[:8]
	sobraColumna := append(cab(), "TELEFONO")
	casos := map[string]struct {
		contenido []byte
		code      string
	}{
		"vacío (0 bytes)":           {nil, "archivo_vacio"},
		"corrupto":                  {[]byte("PK\x03\x04esto no es un zip válido"), "archivo_invalido"},
		"extensión engañosa (CSV)":  {[]byte("Nro,TIPO.DOC,DNI\n1,DNI,12345678\n"), "archivo_invalido"},
		"xls antiguo (OLE)":         {[]byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1" + strings.Repeat("x", 100)), "archivo_invalido"},
		"solo encabezados":          {libro(t, HojaEstudiantes, cab()), "archivo_vacio"},
		"encabezados incorrectos":   {libro(t, HojaEstudiantes, encabezadoMal, fila("1")), "encabezados_invalidos"},
		"columna obligatoria falta": {libro(t, HojaEstudiantes, faltaColumna), "encabezados_invalidos"},
		"columna adicional":         {libro(t, HojaEstudiantes, sobraColumna), "encabezados_invalidos"},
		"hoja sin nada":             {libro(t, HojaEstudiantes), "encabezados_invalidos"},
		"demasiado grande":          {append([]byte("PK\x03\x04"), make([]byte, 600<<10)...), "archivo_grande"},
	}
	for nombre, c := range casos {
		if _, err := Leer(c.contenido, limites); codigoError(err) != c.code {
			t.Errorf("%s: se esperaba %s, got %v", nombre, c.code, err)
		}
	}
}

func TestLeer_EncabezadosToleranEspaciosYMayusculas(t *testing.T) {
	h := []any{" nro ", "tipo.doc", "dni", "Apellido  Paterno", "APELLIDO MATERNO ", "Nombres", "condicion", "Estado", "nivel"}
	if _, err := Leer(libro(t, HojaEstudiantes, h, fila("1", "DNI", "12345678", "A", "", "B", "ESTUDIANTE", "ACTIVO", "INICIAL")), limites); err != nil {
		t.Fatalf("encabezados con espacios/mayúsculas distintos deben aceptarse: %v", err)
	}
}

func TestLeer_UsaPrimeraHojaSiNoExisteEstudiantes(t *testing.T) {
	contenido := libro(t, "Hoja1", cab(), fila("1", "DNI", "12345678", "A", "", "B", "ESTUDIANTE", "ACTIVO", "INICIAL"))
	if filas, err := Leer(contenido, limites); err != nil || len(filas) != 1 {
		t.Fatalf("sin hoja «Estudiantes» se lee la primera: %v %v", filas, err)
	}
}

func TestLeer_LimiteDeFilas(t *testing.T) {
	filas := [][]any{cab()}
	for i := 0; i < 4; i++ {
		filas = append(filas, fila("", "DNI", "1234567"+string(rune('0'+i)), "A", "", "B", "ESTUDIANTE", "ACTIVO", "INICIAL"))
	}
	if _, err := Leer(libro(t, HojaEstudiantes, filas...), Limites{MaxFilas: 3}); codigoError(err) != "demasiadas_filas" {
		t.Fatalf("se esperaba demasiadas_filas, got %v", err)
	}
}

func TestValidar_ReglasPorFila(t *testing.T) {
	base := Fila{TipoDocumento: "DNI", Documento: "12345678", ApellidoPaterno: "Pérez", Nombres: "Ana",
		Condicion: "ESTUDIANTE", Estado: "ACTIVO", Nivel: "PRIMARIA"}
	casos := map[string]struct {
		cambiar  func(*Fila)
		contiene string
	}{
		"condición distinta":       {func(f *Fila) { f.Condicion = "DOCENTE" }, "CONDICION debe ser ESTUDIANTE"},
		"condición vacía":          {func(f *Fila) { f.Condicion = "" }, "CONDICION debe ser ESTUDIANTE"},
		"estado no admitido":       {func(f *Fila) { f.Estado = "RETIRADO" }, "ESTADO inválido"},
		"nivel no admitido":        {func(f *Fila) { f.Nivel = "SUPERIOR" }, "NIVEL inválido"},
		"tipo no admitido":         {func(f *Fila) { f.TipoDocumento = "RUC" }, "TIPO.DOC inválido"},
		"número sin tipo":          {func(f *Fila) { f.TipoDocumento = "" }, "TIPO.DOC inválido"},
		"DNI con letras":           {func(f *Fila) { f.Documento = "1234567A" }, "8 dígitos"},
		"sin documento con número": {func(f *Fila) { f.TipoDocumento = "SIN_DOCUMENTO" }, "no admite número"},
		"apellido paterno ausente": {func(f *Fila) { f.ApellidoPaterno = "  " }, "APELLIDO PATERNO es obligatorio"},
		"nombres ausentes":         {func(f *Fila) { f.Nombres = "" }, "NOMBRES"},
		"apellido demasiado largo": {func(f *Fila) { f.ApellidoPaterno = strings.Repeat("a", 61) }, "APELLIDO PATERNO"},
		"fórmula en nombres":       {func(f *Fila) { f.Nombres = "=HYPERLINK(\"x\")" }, "no se admiten fórmulas"},
		"caracter de control":      {func(f *Fila) { f.Nombres = "Ana\x07" }, "NOMBRES"},
	}
	for nombre, c := range casos {
		f := base
		c.cambiar(&f)
		_, errs := Validar(f)
		if !strings.Contains(strings.Join(errs, " | "), c.contiene) {
			t.Errorf("%s: se esperaba un error con %q, got %v", nombre, c.contiene, errs)
		}
	}
	if _, errs := Validar(base); len(errs) != 0 {
		t.Fatalf("la fila base debe ser válida: %v", errs)
	}
}

func TestCodigoPara(t *testing.T) {
	dni, ce, pas := "01234567", "AB1234", "X12345678901"
	if c := CodigoPara(Registro{TipoDocumento: "DNI", NumeroDocumento: &dni}, "h", 2); c != "DNI01234567" {
		t.Fatalf("DNI: %s", c)
	}
	if c := CodigoPara(Registro{TipoDocumento: "CE", NumeroDocumento: &ce}, "h", 2); c != "CEAB1234" {
		t.Fatalf("CE: %s", c)
	}
	if c := CodigoPara(Registro{TipoDocumento: "PASAPORTE", NumeroDocumento: &pas}, "h", 2); c != "PASX12345678901" || len(c) > 20 {
		t.Fatalf("PASAPORTE (≤ 20): %s", c)
	}
	sd := Registro{TipoDocumento: "SIN_DOCUMENTO"}
	a, b, otraFila, otroArchivo := CodigoPara(sd, "h1", 5), CodigoPara(sd, "h1", 5), CodigoPara(sd, "h1", 6), CodigoPara(sd, "h2", 5)
	if a != b || a == otraFila || a == otroArchivo || len(a) != 14 || !strings.HasPrefix(a, "SD") {
		t.Fatalf("SIN_DOCUMENTO: determinista por archivo+fila: %s %s %s %s", a, b, otraFila, otroArchivo)
	}
}

func TestPlantilla_FormatoOficial(t *testing.T) {
	contenido, err := Plantilla()
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(contenido))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if hojas := f.GetSheetList(); len(hojas) != 2 || hojas[0] != HojaEstudiantes || hojas[1] != HojaInstrucciones {
		t.Fatalf("hojas: %v", hojas)
	}
	rows, _ := f.GetRows(HojaEstudiantes)
	if len(rows) != 1 || strings.Join(rows[0], "|") != strings.Join(Encabezados, "|") {
		t.Fatalf("la plantilla debe traer solo los 9 encabezados exactos: %v", rows)
	}
	dvs, err := f.GetDataValidations(HojaEstudiantes)
	if err != nil {
		t.Fatal(err)
	}
	var condicion bool
	for _, dv := range dvs {
		if strings.HasPrefix(dv.Sqref, "G2:") && strings.Contains(dv.Formula1, Condicion) && !strings.Contains(dv.Formula1, ",") {
			condicion = true
		}
	}
	if !condicion {
		t.Fatalf("CONDICION debe tener lista desplegable solo con %s: %+v", Condicion, dvs)
	}
	style, _ := f.GetCellStyle(HojaEstudiantes, "A1")
	st, _ := f.GetStyle(style)
	if st == nil || st.Font == nil || !st.Font.Bold {
		t.Fatal("encabezados en negrita")
	}
	// La plantilla vacía, al importarse, es un archivo sin estudiantes.
	if _, err := Leer(contenido, limites); codigoError(err) != "archivo_vacio" {
		t.Fatalf("plantilla vacía: %v", err)
	}
}

func TestExportador_NueveColumnasTextoYReimportable(t *testing.T) {
	e, err := NuevoExportador()
	if err != nil {
		t.Fatal(err)
	}
	datos := []FilaExport{
		{"DNI", "01234567", "Núñez", "Peña", "José", "ACTIVO", "PRIMARIA"},
		{"SIN_DOCUMENTO", "", "=CMD()", "", "Ana", "INACTIVO", ""},
	}
	for _, d := range datos {
		if err := e.Agregar(d); err != nil {
			t.Fatal(err)
		}
	}
	contenido, err := e.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(contenido))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows, _ := f.GetRows(HojaEstudiantes)
	if len(rows) != 3 {
		t.Fatalf("encabezado + 2 filas: %v", rows)
	}
	for i, r := range rows {
		if len(r) > len(Encabezados) {
			t.Fatalf("fila %d tiene más de 9 columnas: %v", i+1, r)
		}
	}
	if strings.Join(rows[0], "|") != strings.Join(Encabezados, "|") {
		t.Fatalf("encabezados: %v", rows[0])
	}
	if rows[1][0] != "1" || rows[1][2] != "01234567" || rows[1][6] != Condicion || rows[2][0] != "2" {
		t.Fatalf("Nro correlativo, DNI como texto y CONDICION: %v", rows)
	}
	if formula, _ := f.GetCellFormula(HojaEstudiantes, "D3"); formula != "" {
		t.Fatalf("un dato con «=» no debe quedar como fórmula: %q", formula)
	}
	// El archivo exportado se puede volver a leer con el mismo formato.
	if filas, err := Leer(contenido, limites); err != nil || len(filas) != 2 {
		t.Fatalf("la exportación debe ser reimportable: %v %v", filas, err)
	}
}

func TestExportador_SinRegistros(t *testing.T) {
	e, err := NuevoExportador()
	if err != nil {
		t.Fatal(err)
	}
	contenido, err := e.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(contenido))
	defer func() { _ = f.Close() }()
	if rows, _ := f.GetRows(HojaEstudiantes); len(rows) != 1 || len(rows[0]) != 9 {
		t.Fatalf("sin registros: solo los 9 encabezados, got %v", rows)
	}
}

// Reportes exportados de otros sistemas: título y fila vacía sobre los
// encabezados oficiales (caso real del reporte de usuarios del colegio).
func TestLeer_TituloSobreLosEncabezados(t *testing.T) {
	contenido := libro(t, HojaEstudiantes,
		fila("REPORTE DE USUARIOS"),
		fila(),
		cab(),
		fila("1", "CARNE EXT.", "001234567", "Pérez", "", "Ana", "ESTUDIANTE", "ACTIVO", "SECUNDARIA"),
	)
	filas, err := Leer(contenido, limites)
	if err != nil || len(filas) != 1 || filas[0].Numero != 4 {
		t.Fatalf("encabezados en la fila 3 deben aceptarse: %+v %v", filas, err)
	}
	r, errs := Validar(filas[0])
	if len(errs) != 0 || r.TipoDocumento != "CE" {
		t.Fatalf("«CARNE EXT.» equivale a CE: %+v %v", r, errs)
	}
}

func TestLeer_SinEncabezadosEnLasPrimerasFilas(t *testing.T) {
	filas := [][]any{}
	for i := 0; i < 12; i++ {
		filas = append(filas, fila("título"))
	}
	filas = append(filas, cab())
	if _, err := Leer(libro(t, HojaEstudiantes, filas...), limites); codigoError(err) != "encabezados_invalidos" {
		t.Fatalf("encabezados después de la fila 10: %v", err)
	}
}
