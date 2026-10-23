package tabular

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Formatos de exportación implementados (y probados).
const (
	SalidaXLSX  = "xlsx"
	SalidaCSV   = "csv"
	SalidaTSV   = "tsv"
	SalidaJSON  = "json"
	SalidaJSONL = "jsonl"
	SalidaXML   = "xml"
)

// SalidasSoportadas, en el orden en que se ofrecen.
var SalidasSoportadas = []string{SalidaXLSX, SalidaCSV, SalidaTSV, SalidaJSON, SalidaJSONL, SalidaXML}

// MimeDe devuelve el tipo MIME de un formato de salida.
func MimeDe(formato string) string {
	return map[string]string{
		SalidaXLSX:  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		SalidaCSV:   "text/csv",
		SalidaTSV:   "text/tab-separated-values",
		SalidaJSON:  "application/json",
		SalidaJSONL: "application/x-ndjson",
		SalidaXML:   "application/xml",
	}[formato]
}

// ColumnaExport: clave interna + encabezado elegido por el usuario.
type ColumnaExport struct {
	Clave      string
	Encabezado string
}

// OpcionesExport. ProtegerFormulas aplica solo a CSV/TSV (ver protegerFormula).
type OpcionesExport struct {
	Formato          string
	Separador        string // csv: "," (defecto), ";", "|"
	Codificacion     string // csv/tsv: utf-8, utf-8-bom (defecto, Excel), windows-1252, iso-8859-1
	ProtegerFormulas bool
	Modulo           string    // json/xml: nombre del conjunto ("estudiantes")
	Generado         time.Time // json/xml: fecha de generación
}

// Exportador recibe registros (valores alineados con las columnas).
type Exportador struct {
	cols       []ColumnaExport
	op         OpcionesExport
	filas      int
	protegidas int
	// xlsx
	libro *excelize.File
	sw    *excelize.StreamWriter
	texto int
	// demás formatos
	buf     bytes.Buffer
	csvw    *csv.Writer
	primero bool
}

// NuevoExportador valida columnas y opciones y prepara la salida.
func NuevoExportador(cols []ColumnaExport, op OpcionesExport) (*Exportador, error) {
	if len(cols) == 0 {
		return nil, errorf(ErrOpcion, "elija al menos un campo para exportar")
	}
	vistos := map[string]bool{}
	for _, c := range cols {
		h := strings.TrimSpace(c.Encabezado)
		if h == "" {
			return nil, errorf(ErrOpcion, "el campo %q no tiene encabezado", c.Clave)
		}
		if vistos[strings.ToUpper(h)] {
			return nil, errorf(ErrOpcion, "el encabezado %q está repetido", h)
		}
		vistos[strings.ToUpper(h)] = true
	}
	if op.Generado.IsZero() {
		op.Generado = time.Now()
	}
	e := &Exportador{cols: cols, op: op, primero: true}
	switch op.Formato {
	case SalidaXLSX:
		return e, e.iniciarXLSX()
	case SalidaCSV, SalidaTSV:
		sep := ','
		if op.Formato == SalidaTSV {
			sep = '\t'
		} else if op.Separador != "" {
			r, ok := map[string]rune{",": ',', ";": ';', "|": '|'}[op.Separador]
			if !ok {
				return nil, errorf(ErrOpcion, "separador no admitido para CSV: use coma, punto y coma o barra")
			}
			sep = r
		}
		if op.Codificacion == "" {
			e.op.Codificacion = "utf-8-bom" // Excel abre bien tildes y ñ
		}
		if _, err := codificar("", e.op.Codificacion); err != nil {
			return nil, err
		}
		e.csvw = csv.NewWriter(&e.buf)
		e.csvw.Comma = sep
		e.csvw.UseCRLF = true
		return e, e.csvw.Write(encabezados(cols))
	case SalidaJSON:
		e.buf.WriteString(`{"modulo":` + jsonTexto(op.Modulo) + `,"generado":` + jsonTexto(op.Generado.Format(time.RFC3339)) + `,"campos":[`)
		for i, c := range cols {
			if i > 0 {
				e.buf.WriteByte(',')
			}
			e.buf.WriteString(`{"clave":` + jsonTexto(c.Clave) + `,"encabezado":` + jsonTexto(c.Encabezado) + `}`)
		}
		e.buf.WriteString(`],"registros":[`)
		return e, nil
	case SalidaJSONL:
		return e, nil
	case SalidaXML:
		e.buf.WriteString(xml.Header)
		return e, nil
	}
	return nil, errorf(ErrOpcion, "formato de salida no admitido: use %s", strings.Join(SalidasSoportadas, ", "))
}

func encabezados(cols []ColumnaExport) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Encabezado
	}
	return out
}

func (e *Exportador) iniciarXLSX() error {
	e.libro = excelize.NewFile()
	hoja := "Datos"
	if e.op.Modulo != "" {
		hoja = strings.ToUpper(e.op.Modulo[:1]) + e.op.Modulo[1:]
	}
	if err := e.libro.SetSheetName("Sheet1", hoja); err != nil {
		return err
	}
	negrita, err := e.libro.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"164194"}}})
	if err != nil {
		return err
	}
	if e.texto, err = e.libro.NewStyle(&excelize.Style{NumFmt: 49}); err != nil { // "@" texto
		return err
	}
	if e.sw, err = e.libro.NewStreamWriter(hoja); err != nil {
		return err
	}
	for i, c := range e.cols {
		_ = e.sw.SetColWidth(i+1, i+1, float64(min(max(len(c.Encabezado)+4, 12), 45)))
	}
	_ = e.sw.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	fila := make([]any, len(e.cols))
	for i, c := range e.cols {
		fila[i] = excelize.Cell{StyleID: negrita, Value: c.Encabezado}
	}
	return e.sw.SetRow("A1", fila)
}

// Agregar escribe un registro (los valores faltantes quedan vacíos).
func (e *Exportador) Agregar(valores []string) error {
	v := make([]string, len(e.cols))
	copy(v, valores)
	e.filas++
	switch e.op.Formato {
	case SalidaXLSX:
		// Celdas de TEXTO: Excel nunca las evalúa como fórmula (sin alterar el dato).
		fila := make([]any, len(v))
		for i, x := range v {
			fila[i] = excelize.Cell{StyleID: e.texto, Value: x}
		}
		celda, _ := excelize.CoordinatesToCellName(1, e.filas+1)
		return e.sw.SetRow(celda, fila)
	case SalidaCSV, SalidaTSV:
		if e.op.ProtegerFormulas {
			for i := range v {
				if p, ok := protegerFormula(v[i]); ok {
					v[i] = p
					e.protegidas++
				}
			}
		}
		return e.csvw.Write(v)
	case SalidaJSON:
		if !e.primero {
			e.buf.WriteByte(',')
		}
		e.primero = false
		e.buf.WriteString(objetoJSON(e.cols, v))
	case SalidaJSONL:
		e.buf.WriteString(objetoJSON(e.cols, v))
		e.buf.WriteByte('\n')
	case SalidaXML:
		e.buf.WriteString("<registro>")
		for i, c := range e.cols {
			e.buf.WriteString(`<campo clave="` + xmlEscape(c.Clave) + `" nombre="` + xmlEscape(c.Encabezado) + `">` + xmlEscape(v[i]) + `</campo>`)
		}
		e.buf.WriteString("</registro>")
	}
	return nil
}

// Filas exportadas y celdas protegidas contra fórmulas (CSV/TSV).
func (e *Exportador) Filas() int      { return e.filas }
func (e *Exportador) Protegidas() int { return e.protegidas }

// Cerrar termina el archivo y devuelve sus bytes.
func (e *Exportador) Cerrar() ([]byte, error) {
	switch e.op.Formato {
	case SalidaXLSX:
		defer func() { _ = e.libro.Close() }()
		if err := e.sw.Flush(); err != nil {
			return nil, err
		}
		ultima, _ := excelize.ColumnNumberToName(len(e.cols))
		if err := e.libro.AutoFilter(e.libro.GetSheetName(0), "A1:"+ultima+"1", nil); err != nil {
			return nil, err
		}
		var out bytes.Buffer
		if err := e.libro.Write(&out); err != nil {
			return nil, err
		}
		return out.Bytes(), nil
	case SalidaCSV, SalidaTSV:
		e.csvw.Flush()
		if err := e.csvw.Error(); err != nil {
			return nil, err
		}
		return codificar(e.buf.String(), e.op.Codificacion)
	case SalidaJSON:
		e.buf.WriteString(`],"total":` + strconv.Itoa(e.filas) + `}`)
	case SalidaXML:
		cuerpo := e.buf.String()
		i := strings.Index(cuerpo, "?>") + 2
		apertura := `<registros modulo="` + xmlEscape(e.op.Modulo) + `" generado="` + e.op.Generado.Format(time.RFC3339) +
			`" total="` + strconv.Itoa(e.filas) + `">`
		return []byte(cuerpo[:i] + "\n" + apertura + cuerpo[i:] + "</registros>\n"), nil
	}
	return e.buf.Bytes(), nil
}

// protegerFormula: en CSV, un valor que empieza con = + @ (o tabulador/retorno)
// lo ejecutaría Excel como fórmula al abrirlo. Se antepone un apóstrofo
// (recomendación OWASP), que Excel muestra como texto. Un "-" solo se protege
// si no es un número negativo. JSON y XML no se alteran (no son hojas de
// cálculo) y en XLSX se usan celdas de texto.
func protegerFormula(v string) (string, bool) {
	if v == "" {
		return v, false
	}
	switch v[0] {
	case '=', '+', '@', '\t', '\r':
		return "'" + v, true
	case '-':
		if len(v) > 1 && (v[1] >= '0' && v[1] <= '9') && esNumero(v) {
			return v, false
		}
		return "'" + v, true
	}
	return v, false
}

func jsonTexto(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// objetoJSON con las claves en el orden elegido (los encabezados del usuario).
func objetoJSON(cols []ColumnaExport, v []string) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, c := range cols {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(jsonTexto(c.Encabezado) + ":" + jsonTexto(v[i]))
	}
	b.WriteByte('}')
	return b.String()
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
