package tabular

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// Datos ficticios. Cada prueba genera su propio archivo en memoria.

func leer(t *testing.T, nombre string, contenido []byte, op Opciones) *Tabla {
	t.Helper()
	tab, err := Leer(nombre, contenido, op)
	if err != nil {
		t.Fatalf("%s: %v", nombre, err)
	}
	return tab
}

func codigo(t *testing.T, nombre string, contenido []byte, op Opciones) string {
	t.Helper()
	_, err := Leer(nombre, contenido, op)
	return CodigoDe(err)
}

func xlsxCon(t *testing.T, hojas map[string][][]any, orden ...string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	for i, h := range orden {
		if i == 0 {
			_ = f.SetSheetName("Sheet1", h)
		} else if _, err := f.NewSheet(h); err != nil {
			t.Fatal(err)
		}
		for r, fila := range hojas[h] {
			celda, _ := excelize.CoordinatesToCellName(1, r+1)
			if err := f.SetSheetRow(h, celda, &fila); err != nil {
				t.Fatal(err)
			}
		}
	}
	var b bytes.Buffer
	if err := f.Write(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestXLSX_VariasHojasTituloYCeros(t *testing.T) {
	b := xlsxCon(t, map[string][][]any{
		"Resumen": {{"nada"}},
		"Alumnos": {{"REPORTE 2026"}, {}, {"Código", "DNI", "Nombres"}, {"0007", "01234567", "Ana"}, {}, {"0008", "76543210", "Luis"}},
	}, "Resumen", "Alumnos")
	tab := leer(t, "a.xlsx", b, Opciones{Hoja: "Alumnos"})
	if tab.Origen.Formato != FormatoXLSX || len(tab.Origen.Hojas) != 2 || tab.Origen.Hoja != "Alumnos" {
		t.Fatalf("origen: %+v", tab.Origen)
	}
	if tab.Origen.FilaEncabezado != 3 || strings.Join(tab.Encabezados, "|") != "Código|DNI|Nombres" {
		t.Fatalf("encabezados tras el título: fila %d %v", tab.Origen.FilaEncabezado, tab.Encabezados)
	}
	if len(tab.Filas) != 2 || tab.Filas[0].Numero != 4 || tab.Filas[1].Numero != 6 {
		t.Fatalf("filas y números reales: %+v", tab.Filas)
	}
	if tab.Filas[0].Valor(0) != "0007" || tab.Filas[0].Valor(1) != "01234567" {
		t.Fatalf("ceros iniciales: %+v", tab.Filas[0])
	}
	if codigo(t, "a.xlsx", b, Opciones{Hoja: "NoExiste"}) != ErrOpcion {
		t.Fatal("hoja inexistente debe ser error de opción")
	}
}

func TestXLSX_FormulasNoSeEjecutanYMacrosNoSeCargan(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetSheetRow("Sheet1", "A1", &[]any{"Nombre", "Calculo"})
	_ = f.SetSheetRow("Sheet1", "A2", &[]any{"Ana", ""})
	_ = f.SetCellFormula("Sheet1", "B2", `=HYPERLINK("http://ejemplo.invalid","x")`)
	var b bytes.Buffer
	_ = f.Write(&b)
	_ = f.Close()
	// .xlsm: mismo libro con un vbaProject.bin agregado al zip.
	xlsm := agregarAlZip(t, b.Bytes(), "xl/vbaProject.bin", []byte("macro ficticia"))
	for nombre, contenido := range map[string][]byte{"f.xlsx": b.Bytes(), "m.xlsm": xlsm} {
		tab := leer(t, nombre, contenido, Opciones{})
		if v := tab.Filas[0].Valor(1); strings.Contains(v, "http") || strings.Contains(v, "HYPERLINK") {
			t.Fatalf("%s: la fórmula no debe evaluarse ni copiarse como dato: %q", nombre, v)
		}
	}
}

func agregarAlZip(t *testing.T, original []byte, nombre string, datos []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		w, _ := zw.Create(f.Name)
		rc, _ := f.Open()
		var b bytes.Buffer
		_, _ = b.ReadFrom(rc)
		_ = rc.Close()
		_, _ = w.Write(b.Bytes())
	}
	w, _ := zw.Create(nombre)
	_, _ = w.Write(datos)
	_ = zw.Close()
	return out.Bytes()
}

func zipCon(t *testing.T, nombre, datos string) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	w, _ := zw.Create(nombre)
	_, _ = w.Write([]byte(datos))
	_ = zw.Close()
	return out.Bytes()
}

func odsCon(t *testing.T, contenido string) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	w, _ := zw.Create("mimetype")
	_, _ = w.Write([]byte("application/vnd.oasis.opendocument.spreadsheet"))
	w, _ = zw.Create("content.xml")
	_, _ = w.Write([]byte(contenido))
	_ = zw.Close()
	return out.Bytes()
}

func TestODS_HojasCeldasRepetidasYFilasVacias(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"><office:body><office:spreadsheet>
<table:table table:name="Otra"><table:table-row><table:table-cell><text:p>x</text:p></table:table-cell></table:table-row></table:table>
<table:table table:name="Lista">
<table:table-row><table:table-cell><text:p>DNI</text:p></table:table-cell><table:table-cell><text:p>Apellidos</text:p></table:table-cell><table:table-cell><text:p>Nombres</text:p></table:table-cell></table:table-row>
<table:table-row><table:table-cell><text:p>01234567</text:p></table:table-cell><table:table-cell table:number-columns-repeated="2"><text:p>Peña</text:p></table:table-cell><table:table-cell table:number-columns-repeated="16000"/></table:table-row>
<table:table-row table:number-rows-repeated="1048000"><table:table-cell table:number-columns-repeated="16000"/></table:table-row>
</table:table></office:spreadsheet></office:body></office:document-content>`
	b := odsCon(t, content)
	tab := leer(t, "a.ods", b, Opciones{Hoja: "Lista"})
	if tab.Origen.Formato != FormatoODS || len(tab.Origen.Hojas) != 2 {
		t.Fatalf("origen: %+v", tab.Origen)
	}
	if len(tab.Filas) != 1 || tab.Filas[0].Valor(0) != "01234567" || tab.Filas[0].Valor(2) != "Peña" || len(tab.Filas[0].Valores) != 3 {
		t.Fatalf("celdas repetidas y vacías: %+v", tab.Filas)
	}
	if codigo(t, "x.ods", odsCon(t, "<no-cierra"), Opciones{}) != ErrDanado {
		t.Fatal("ODS con XML dañado")
	}
}

func TestHTML_ReporteXLSWebConTituloColspanYScript(t *testing.T) {
	html := "\xEF\xBB\xBF<br><br><h2 style='x'>Reporte de Usuarios</h2><script>alert(1)</script>" +
		"<table border=1><tr><th>Nro</th><th>TIPO.DOC</th><th>DNI</th><th>Nombres</th></tr>" +
		"<tr><td>1</td><td>DNI</td><td>01234567</td><td>José&nbsp;Ñuñez</td></tr>" +
		"<tr><td colspan=2>2</td><td>76543210</td><td>Ana<br>María</td></tr></table>" +
		"<table><tr><td>pie</td></tr></table>"
	tab := leer(t, "ReporteUsuarios.xls", []byte(html), Opciones{})
	if tab.Origen.Formato != FormatoHTML || len(tab.Origen.Hojas) != 2 || tab.Origen.Hoja != "Tabla 1" {
		t.Fatalf("origen: %+v", tab.Origen)
	}
	if len(tab.Origen.Avisos) == 0 || !strings.Contains(tab.Origen.Avisos[0], ".xls") {
		t.Fatalf("debe avisar que la extensión .xls no corresponde: %v", tab.Origen.Avisos)
	}
	if tab.Filas[0].Valor(2) != "01234567" || tab.Filas[0].Valor(3) != "José Ñuñez" {
		t.Fatalf("texto de celdas: %+v", tab.Filas[0])
	}
	if tab.Filas[1].Valor(2) != "76543210" || tab.Filas[1].Valor(3) != "Ana María" {
		t.Fatalf("colspan no debe desalinear: %+v", tab.Filas[1])
	}
	latin := []byte("<table><tr><td>Nombre</td><td>Apellido</td></tr><tr><td>Jos\xe9</td><td>Pe\xf1a</td></tr></table>")
	tab = leer(t, "r.html", latin, Opciones{})
	if tab.Filas[0].Valor(0) != "José" || tab.Origen.Codificacion != CodWindows1252 {
		t.Fatalf("HTML en Windows-1252: %+v", tab)
	}
}

// Exportaciones web que escriben ="01234567" para que Excel conserve los
// ceros: se leen como texto (con aviso); una fórmula real queda tal cual.
func TestLiteralExcelDeTexto(t *testing.T) {
	html := `<table><tr><th>DNI</th><th>Nombre</th></tr><tr><td>="01234567"</td><td>=HYPERLINK("x")</td></tr></table>`
	tab := leer(t, "r.xls", []byte(html), Opciones{})
	if tab.Filas[0].Valor(0) != "01234567" || tab.Filas[0].Valor(1) != `=HYPERLINK("x")` {
		t.Fatalf("literal y fórmula: %+v", tab.Filas[0])
	}
	var aviso bool
	for _, a := range tab.Origen.Avisos {
		aviso = aviso || strings.Contains(a, "=\"texto\"")
	}
	if !aviso {
		t.Fatalf("debe avisar la normalización: %v", tab.Origen.Avisos)
	}
	csv := leer(t, "x.csv", []byte("dni,n\n"+`"=""00012345""",Ana`+"\n"), Opciones{})
	if csv.Filas[0].Valor(0) != "00012345" {
		t.Fatalf("CSV con =\"...\": %q", csv.Filas[0].Valor(0))
	}
}

func TestCSV_SeparadoresYCodificaciones(t *testing.T) {
	utf16, _ := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte("dni\tnombre\n01234567\tÁngel\n"))
	w1252, _ := charmap.Windows1252.NewEncoder().Bytes([]byte("dni;nombre\n01234567;Peña\n"))
	casos := []struct {
		nombre string
		datos  []byte
		sep    string
		cod    string
	}{
		{"coma.csv", []byte("dni,nombre\n01234567,\"Pérez, Ana\"\n"), ",", CodUTF8},
		{"pyc.csv", []byte("\xEF\xBB\xBFdni;nombre\r\n01234567;Ana\r\n"), ";", CodUTF8},
		{"tab.tsv", []byte("dni\tnombre\n01234567\tAna\n"), "tab", CodUTF8},
		{"barra.txt", []byte("dni|nombre\n01234567|Ana\n"), "|", CodUTF8},
		{"utf16.csv", utf16, "tab", CodUTF16},
		{"latin.csv", w1252, ";", CodWindows1252},
	}
	for _, c := range casos {
		tab := leer(t, c.nombre, c.datos, Opciones{})
		if tab.Origen.Separador != c.sep || tab.Origen.Codificacion != c.cod {
			t.Errorf("%s: separador %q cod %q", c.nombre, tab.Origen.Separador, tab.Origen.Codificacion)
		}
		if tab.Filas[0].Valor(0) != "01234567" || tab.Encabezados[0] != "dni" {
			t.Errorf("%s: %v %+v", c.nombre, tab.Encabezados, tab.Filas)
		}
	}
	tab := leer(t, "coma.csv", casos[0].datos, Opciones{})
	if tab.Filas[0].Valor(1) != "Pérez, Ana" {
		t.Fatalf("comillas con separador dentro: %q", tab.Filas[0].Valor(1))
	}
	tab = leer(t, "latin.csv", w1252, Opciones{})
	if tab.Filas[0].Valor(1) != "Peña" || len(tab.Origen.Avisos) == 0 {
		t.Fatalf("Windows-1252 se detecta y se avisa: %+v", tab)
	}
	// Separador indicado por el usuario.
	tab = leer(t, "x.csv", []byte("a;b,c\n1;2,3\n"), Opciones{Separador: ","})
	if len(tab.Encabezados) != 2 || tab.Encabezados[0] != "a;b" {
		t.Fatalf("separador forzado: %v", tab.Encabezados)
	}
	if codigo(t, "x.csv", []byte("a;b\n1;2\n"), Opciones{Separador: "#"}) != ErrOpcion {
		t.Fatal("separador no admitido")
	}
}

func TestRejilla_EncabezadosVaciosRepetidosYDesplazados(t *testing.T) {
	tab := leer(t, "x.csv", []byte("Nombre,,Nombre,DNI\nAna,x,Ana2,1\nLuis,y,L2,2,EXTRA\n\n,,,\n"), Opciones{})
	if strings.Join(tab.Encabezados, "|") != "Nombre|Columna 2|Nombre (2)|DNI|Columna 5" {
		t.Fatalf("encabezados: %v", tab.Encabezados)
	}
	if len(tab.Filas) != 2 {
		t.Fatalf("las filas vacías se omiten: %d", len(tab.Filas))
	}
	var aviso bool
	for _, a := range tab.Origen.Avisos {
		aviso = aviso || strings.Contains(a, "desplazados")
	}
	if !aviso {
		t.Fatalf("debe avisar de datos desplazados: %v", tab.Origen.Avisos)
	}
	// Fila de encabezados indicada por el usuario.
	tab = leer(t, "x.csv", []byte("titulo\nA,B\n1,2\n"), Opciones{FilaEncabezado: 2})
	if tab.Encabezados[0] != "A" || tab.Origen.FilaEncabezado != 2 {
		t.Fatalf("fila de encabezado manual: %+v", tab)
	}
}

func TestJSON_RutasAnidadasYOrden(t *testing.T) {
	doc := `{"colegio":"X","datos":{"alumnos":[
		{"codigo":"0007","persona":{"dni":"01234567","nombres":"Ana"},"tags":["a","b"]},
		{"codigo":"0008","persona":{"dni":"76543210","nombres":"Luis"},"activo":true,"edad":12}
	],"docentes":[{"x":1}]}}`
	tab := leer(t, "d.json", []byte(doc), Opciones{})
	if tab.Origen.Ruta != "datos.alumnos" || len(tab.Origen.Rutas) != 2 {
		t.Fatalf("ruta por defecto (la de más registros): %+v", tab.Origen)
	}
	if strings.Join(tab.Encabezados, "|") != "codigo|persona.dni|persona.nombres|tags|activo|edad" {
		t.Fatalf("aplanado en el orden del documento: %v", tab.Encabezados)
	}
	if tab.Filas[0].Valor(0) != "0007" || tab.Filas[0].Valor(3) != "a; b" || tab.Filas[1].Valor(4) != "true" || tab.Filas[1].Valor(5) != "12" {
		t.Fatalf("valores: %+v", tab.Filas)
	}
	tab = leer(t, "d.json", []byte(doc), Opciones{Ruta: "datos.docentes"})
	if len(tab.Filas) != 1 {
		t.Fatalf("ruta elegida: %+v", tab)
	}
	if tab := leer(t, "r.json", []byte(`[{"a":"1"},{"a":"2"}]`), Opciones{}); tab.Origen.Ruta != "$" || len(tab.Filas) != 2 {
		t.Fatalf("arreglo raíz: %+v", tab)
	}
	if codigo(t, "e.json", []byte(`{"a":[{"x":1},`), Opciones{}) != ErrDanado {
		t.Fatal("JSON inválido")
	}
	if codigo(t, "e.json", []byte(`{"a":1}`), Opciones{}) != ErrEstructura {
		t.Fatal("JSON sin registros")
	}
	profundo := strings.Repeat(`{"a":`, 50) + `[{"x":1}]` + strings.Repeat("}", 50)
	if codigo(t, "p.json", []byte(profundo), Opciones{}) != ErrEstructura {
		t.Fatal("anidamiento excesivo")
	}
}

func TestJSONL(t *testing.T) {
	tab := leer(t, "d.ndjson", []byte("{\"dni\":\"01234567\",\"n\":\"Ana\"}\n\n{\"dni\":\"7\",\"n\":\"Luis\",\"x\":{\"y\":1}}\n"), Opciones{})
	if tab.Origen.Formato != FormatoJSONL || len(tab.Filas) != 2 || tab.Filas[1].Numero != 3 || tab.Encabezados[2] != "x.y" {
		t.Fatalf("jsonl: %+v", tab)
	}
	if codigo(t, "d.jsonl", []byte("{\"a\":1}\n{roto\n"), Opciones{}) != ErrDanado {
		t.Fatal("línea inválida")
	}
}

func TestXML_RegistrosRepetidosAtributosYXXE(t *testing.T) {
	doc := `<?xml version="1.0"?><colegio><nombre>X</nombre><alumnos>
		<alumno codigo="0007"><dni>01234567</dni><nombres>Ana</nombres><contacto><telefono>1</telefono><telefono>2</telefono></contacto></alumno>
		<alumno codigo="0008"><dni>76543210</dni><nombres>Luis</nombres></alumno>
	</alumnos></colegio>`
	tab := leer(t, "a.xml", []byte(doc), Opciones{})
	if tab.Origen.Ruta != "/colegio/alumnos/alumno" {
		t.Fatalf("ruta: %+v", tab.Origen)
	}
	if strings.Join(tab.Encabezados, "|") != "@codigo|dni|nombres|contacto.telefono" {
		t.Fatalf("campos: %v", tab.Encabezados)
	}
	if tab.Filas[0].Valor(0) != "0007" || tab.Filas[0].Valor(3) != "1; 2" || tab.Filas[1].Valor(1) != "76543210" {
		t.Fatalf("valores: %+v", tab.Filas)
	}
	xxe := `<?xml version="1.0"?><!DOCTYPE r [<!ENTITY x SYSTEM "file:///etc/passwd">]><r><a><b>&x;</b></a><a><b>2</b></a></r>`
	if codigo(t, "x.xml", []byte(xxe), Opciones{}) != ErrDanado {
		t.Fatal("una entidad externa no debe resolverse (el documento se rechaza)")
	}
	if codigo(t, "x.xml", []byte(`<r><a>1</a></r>`), Opciones{}) != ErrEstructura {
		t.Fatal("XML sin registros repetidos")
	}
}

func TestDeteccion_ArchivosFalsosDanadosYNoSoportados(t *testing.T) {
	csvComoXLSX := leer(t, "datos.xlsx", []byte("dni,nombre\n01234567,Ana\n"), Opciones{})
	if csvComoXLSX.Origen.Formato != FormatoCSV || len(csvComoXLSX.Origen.Avisos) == 0 {
		t.Fatalf("CSV con extensión .xlsx: se lee por contenido y se avisa: %+v", csvComoXLSX.Origen)
	}
	var xlsb bytes.Buffer
	zw := zip.NewWriter(&xlsb)
	w, _ := zw.Create("xl/workbook.bin")
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()
	casos := map[string]struct {
		nombre string
		datos  []byte
		code   string
	}{
		"vacío":       {"a.csv", nil, ErrVacio},
		"xls binario": {"a.xls", []byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1" + strings.Repeat("\x00", 64)), ErrNoSoportado},
		"xlsb":        {"a.xlsb", xlsb.Bytes(), ErrNoSoportado},
		"zip dañado":  {"a.xlsx", []byte("PK\x03\x04basura"), ErrDanado},
		"zip no hoja": {"a.xlsx", zipCon(t, "documento.txt", "hola"), ErrNoSoportado},
		"binario":     {"foto.png", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 100)...), ErrNoSoportado},
		"demasiado":   {"a.csv", bytes.Repeat([]byte("a,b\n"), 2000), ErrGrande},
		"solo título": {"a.csv", []byte("\n\n"), ErrVacio},
	}
	for nombre, c := range casos {
		op := Opciones{}
		if nombre == "demasiado" {
			op.Limites.MaxBytes = 1000
		}
		if got := codigo(t, c.nombre, c.datos, op); got != c.code {
			t.Errorf("%s: se esperaba %s, got %q", nombre, c.code, got)
		}
	}
}

func TestLimites_FilasYColumnas(t *testing.T) {
	var b strings.Builder
	b.WriteString("a,b\n")
	for i := 0; i < 30; i++ {
		b.WriteString("1,2\n")
	}
	if codigo(t, "a.csv", []byte(b.String()), Opciones{Limites: Limites{MaxFilas: 20}}) != ErrDemasiadas {
		t.Fatal("límite de filas")
	}
	ancho := strings.Repeat("c,", 300) + "c\n" + strings.Repeat("1,", 300) + "1\n"
	if codigo(t, "a.csv", []byte(ancho), Opciones{}) != ErrDemasiadasCols {
		t.Fatal("límite de columnas")
	}
	// Un archivo grande dentro de los límites se lee completo.
	var g strings.Builder
	g.WriteString("dni,nombre\n")
	for i := 0; i < 4000; i++ {
		g.WriteString("01234567,Ana\n")
	}
	if tab := leer(t, "g.csv", []byte(g.String()), Opciones{}); len(tab.Filas) != 4000 {
		t.Fatalf("archivo grande: %d filas", len(tab.Filas))
	}
}
