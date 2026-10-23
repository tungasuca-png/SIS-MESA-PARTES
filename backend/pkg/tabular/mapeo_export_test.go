package tabular

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
)

func TestNormalizarEncabezado(t *testing.T) {
	for entrada, want := range map[string]string{
		"Nro. Doc":         "NRO DOC",
		"NRO-DOC":          "NRO DOC",
		"  nro   doc ":     "NRO DOC",
		"Sección":          "SECCION",
		"NOMBRE(S)":        "NOMBRE",
		"Condición":        "CONDICION",
		"N° Documento":     "N DOCUMENTO",
		"APELLIDO_PATERNO": "APELLIDO PATERNO",
		"Año/Grado":        "ANO GRADO",
	} {
		if got := NormalizarEncabezado(entrada); got != want {
			t.Errorf("%q -> %q, se esperaba %q", entrada, got, want)
		}
	}
}

// Esquema de prueba (independiente de cualquier módulo real).
var esquemaPrueba = Esquema{Modulo: "prueba", Campos: []Campo{
	{Clave: "numero_documento", Etiqueta: "Número de documento", Importable: true,
		Sinonimos: []string{"DNI", "NRO DOC", "NUMERO DOCUMENTO", "DOCUMENTO IDENTIDAD"}, Patron: regexp.MustCompile(`^[0-9]{8}$`)},
	{Clave: "nombres", Etiqueta: "Nombres", Importable: true, Sinonimos: []string{"NOMBRE", "NOMBRES"}},
	{Clave: "codigo", Etiqueta: "Código", Importable: true, Sinonimos: []string{"CODIGO ESTUDIANTE", "CODIGO"}},
	{Clave: "seccion", Etiqueta: "Sección", Importable: true, Sinonimos: []string{"SECCION"}, SinonimosDudosos: []string{"AULA"}},
	{Clave: "estado", Etiqueta: "Estado", Importable: true, Catalogo: []string{"ACTIVO", "INACTIVO"}},
}}

func tablaDe(enc []string, filas ...[]string) *Tabla {
	t := &Tabla{Encabezados: enc}
	for i, f := range filas {
		t.Filas = append(t.Filas, Fila{Numero: i + 2, Valores: f})
	}
	return t
}

func asignado(m Mapeo, col int) Asignacion { return m.Asignaciones[col] }

func TestSugerir_SinonimosOrdenYColumnasDesconocidas(t *testing.T) {
	tab := tablaDe([]string{"Nombre(s)", "Teléfono", "nro. doc", "CÓDIGO DEL ESTUDIANTE", "Estado"},
		[]string{"Ana", "999", "01234567", "0007", "ACTIVO"},
		[]string{"Luis", "998", "76543210", "0008", "INACTIVO"})
	m := esquemaPrueba.Sugerir(tab, nil)
	if a := asignado(m, 0); a.Campo != "nombres" || a.Confianza != ConfianzaAlta {
		t.Fatalf("NOMBRE(S): %+v", a)
	}
	if a := asignado(m, 1); a.Campo != "" {
		t.Fatalf("columna desconocida no se asigna: %+v", a)
	}
	if a := asignado(m, 2); a.Campo != "numero_documento" || a.Confianza != ConfianzaAlta {
		t.Fatalf("NRO DOC: %+v", a)
	}
	if a := asignado(m, 3); a.Campo != "codigo" || a.Confianza == ConfianzaAlta {
		t.Fatalf("encabezado parecido: se sugiere pero se confirma: %+v", a)
	}
	if !m.RequiereConfirmacion() {
		t.Fatal("con una sugerencia no alta, el mapeo requiere confirmación")
	}
}

func TestSugerir_DudososContenidoContradiccionYDuplicados(t *testing.T) {
	tab := tablaDe([]string{"AULA", "Estado", "Col X", "DNI", "Documento identidad"},
		[]string{"A", "MATRICULADO", "ACTIVO", "01234567", "76543210"},
		[]string{"B", "RETIRADO", "INACTIVO", "11111111", "22222222"})
	m := esquemaPrueba.Sugerir(tab, nil)
	if a := asignado(m, 0); a.Campo != "seccion" || a.Confianza != ConfianzaMedia {
		t.Fatalf("AULA es sinónimo dudoso de Sección: %+v", a)
	}
	if a := asignado(m, 1); a.Campo != "estado" || a.Confianza != ConfianzaMedia || !strings.Contains(a.Motivo, "%") {
		t.Fatalf("valores que contradicen el encabezado: %+v", a)
	}
	if a := asignado(m, 2); a.Campo != "" {
		// estado ya está tomado por la col 1: el contenido no lo reasigna.
		t.Fatalf("por contenido solo se sugieren campos libres: %+v", a)
	}
	// Dos columnas ALTA para el mismo campo: queda una, con problema reportado.
	asignadas := 0
	for _, i := range []int{3, 4} {
		if asignado(m, i).Campo == "numero_documento" {
			asignadas++
		}
	}
	if asignadas != 1 || len(m.Problemas) == 0 {
		t.Fatalf("campo en dos columnas: %+v %v", m.Asignaciones[3:], m.Problemas)
	}
}

func TestSugerir_PorContenidoYPlantilla(t *testing.T) {
	tab := tablaDe([]string{"col1", "situación"},
		[]string{"01234567", "ACTIVO"}, []string{"76543210", "INACTIVO"})
	m := esquemaPrueba.Sugerir(tab, nil)
	if a := asignado(m, 0); a.Campo != "numero_documento" || a.Confianza != ConfianzaBaja {
		t.Fatalf("por contenido: sugerencia baja: %+v", a)
	}
	m = esquemaPrueba.Sugerir(tab, map[string]string{"SITUACION": "estado"})
	if a := asignado(m, 1); a.Campo != "estado" || !a.Confirmada {
		t.Fatalf("asociación de plantilla confirmada: %+v", a)
	}
}

func TestAplicar_ValidaMapeoDelUsuario(t *testing.T) {
	tab := tablaDe([]string{"a", "b"}, []string{"1", "2"})
	if _, err := esquemaPrueba.Aplicar(tab, map[int]string{0: "nombres", 1: "nombres"}); CodigoDe(err) != ErrOpcion {
		t.Fatal("un campo en dos columnas")
	}
	if _, err := esquemaPrueba.Aplicar(tab, map[int]string{0: "inventado"}); CodigoDe(err) != ErrOpcion {
		t.Fatal("campo inexistente")
	}
	if _, err := esquemaPrueba.Aplicar(tab, map[int]string{9: "nombres"}); CodigoDe(err) != ErrOpcion {
		t.Fatal("columna inexistente")
	}
	m, err := esquemaPrueba.Aplicar(tab, map[int]string{1: "codigo"})
	if err != nil || m.Indices()["codigo"] != 1 || m.RequiereConfirmacion() {
		t.Fatalf("mapeo válido: %+v %v", m, err)
	}
}

var colsExport = []ColumnaExport{{"codigo", "Código"}, {"dni", "DNI"}, {"nombre", "Estudiante"}}
var filasExport = [][]string{
	{"0007", "01234567", "Peña Ñuñez, José"},
	{"0008", "00000001", "=HYPERLINK(\"http://x\")"},
	{"0009", "-12", "-ataque"},
}

func exportar(t *testing.T, op OpcionesExport) []byte {
	t.Helper()
	op.Modulo = "estudiantes"
	op.Generado = time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	e, err := NuevoExportador(colsExport, op)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range filasExport {
		if err := e.Agregar(f); err != nil {
			t.Fatal(err)
		}
	}
	b, err := e.Cerrar()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExportar_XLSXTextoSinFormulas(t *testing.T) {
	b := exportar(t, OpcionesExport{Formato: SalidaXLSX})
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows, _ := f.GetRows("Estudiantes")
	if strings.Join(rows[0], "|") != "Código|DNI|Estudiante" || rows[1][0] != "0007" || rows[2][1] != "00000001" {
		t.Fatalf("encabezados/ceros: %v", rows)
	}
	if fm, _ := f.GetCellFormula("Estudiantes", "C3"); fm != "" || rows[2][2] != filasExport[1][2] {
		t.Fatalf("el dato con «=» queda como texto, sin alterar: %q %q", fm, rows[2][2])
	}
	// El archivo exportado se vuelve a leer con el motor.
	if tab := leer(t, "x.xlsx", b, Opciones{}); len(tab.Filas) != 3 || tab.Filas[0].Valor(1) != "01234567" {
		t.Fatalf("reimportable: %+v", tab)
	}
}

func TestExportar_CSVProtegeFormulasYCodificaciones(t *testing.T) {
	b := exportar(t, OpcionesExport{Formato: SalidaCSV, Separador: ";", ProtegerFormulas: true})
	if !bytes.HasPrefix(b, bomUTF8) {
		t.Fatal("CSV por defecto en UTF-8 con BOM (Excel)")
	}
	r := csv.NewReader(bytes.NewReader(b[3:]))
	r.Comma = ';'
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if recs[1][1] != "01234567" || recs[1][2] != "Peña Ñuñez, José" {
		t.Fatalf("datos: %v", recs[1])
	}
	if recs[2][2] != "'"+filasExport[1][2] || recs[3][1] != "-12" || recs[3][2] != "'-ataque" {
		t.Fatalf("protección de fórmulas (números negativos intactos): %v %v", recs[2], recs[3])
	}
	sin := exportar(t, OpcionesExport{Formato: SalidaCSV})
	if !bytes.Contains(sin, []byte(`"=HYPERLINK`)) {
		t.Fatal("sin protección el dato se exporta tal cual")
	}
	w := exportar(t, OpcionesExport{Formato: SalidaTSV, Codificacion: CodWindows1252})
	dec, _ := charmap.Windows1252.NewDecoder().Bytes(w)
	if !strings.Contains(string(dec), "Peña Ñuñez") || !strings.Contains(string(dec), "\t") {
		t.Fatalf("TSV en Windows-1252: %q", dec)
	}
}

func TestExportar_JSONJSONLyXMLValidos(t *testing.T) {
	var doc struct {
		Modulo    string              `json:"modulo"`
		Total     int                 `json:"total"`
		Campos    []map[string]string `json:"campos"`
		Registros []map[string]string `json:"registros"`
	}
	b := exportar(t, OpcionesExport{Formato: SalidaJSON})
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, b)
	}
	if doc.Modulo != "estudiantes" || doc.Total != 3 || doc.Registros[0]["DNI"] != "01234567" || doc.Campos[2]["clave"] != "nombre" {
		t.Fatalf("estructura JSON: %+v", doc)
	}
	if !bytes.Contains(b, []byte(`{"Código":"0007","DNI":"01234567"`)) {
		t.Fatal("las claves siguen el orden elegido")
	}
	lineas := strings.Split(strings.TrimSpace(string(exportar(t, OpcionesExport{Formato: SalidaJSONL}))), "\n")
	if len(lineas) != 3 || !json.Valid([]byte(lineas[1])) {
		t.Fatalf("JSONL: %v", lineas)
	}
	var x struct {
		Total     int `xml:"total,attr"`
		Registros []struct {
			Campos []struct {
				Clave string `xml:"clave,attr"`
				Valor string `xml:",chardata"`
			} `xml:"campo"`
		} `xml:"registro"`
	}
	xb := exportar(t, OpcionesExport{Formato: SalidaXML})
	if err := xml.Unmarshal(xb, &x); err != nil {
		t.Fatalf("XML inválido: %v\n%s", err, xb)
	}
	if x.Total != 3 || x.Registros[1].Campos[2].Valor != filasExport[1][2] || x.Registros[0].Campos[1].Valor != "01234567" {
		t.Fatalf("estructura XML: %+v", x)
	}
}

func TestExportar_OpcionesInvalidasYSinRegistros(t *testing.T) {
	if _, err := NuevoExportador(nil, OpcionesExport{Formato: SalidaCSV}); CodigoDe(err) != ErrOpcion {
		t.Fatal("sin campos")
	}
	if _, err := NuevoExportador([]ColumnaExport{{"a", "X"}, {"b", "x"}}, OpcionesExport{Formato: SalidaCSV}); CodigoDe(err) != ErrOpcion {
		t.Fatal("encabezados repetidos")
	}
	if _, err := NuevoExportador(colsExport, OpcionesExport{Formato: "pdf"}); CodigoDe(err) != ErrOpcion {
		t.Fatal("formato no admitido")
	}
	if _, err := NuevoExportador(colsExport, OpcionesExport{Formato: SalidaCSV, Codificacion: "ebcdic"}); CodigoDe(err) != ErrOpcion {
		t.Fatal("codificación no admitida")
	}
	for _, f := range SalidasSoportadas {
		e, err := NuevoExportador(colsExport, OpcionesExport{Formato: f, Modulo: "estudiantes"})
		if err != nil {
			t.Fatal(err)
		}
		b, err := e.Cerrar()
		if err != nil || (len(b) == 0 && f != SalidaJSONL) || e.Filas() != 0 { // JSONL vacío es válido
			t.Fatalf("%s sin registros: %v", f, err)
		}
	}
}
