package reportes

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Documento genérico exportable: título, metadatos (campo, valor) y tablas.
// Lo usan todas las pestañas de Reportes (Fase 1 y Fase 2) con el MISMO
// formato: XLSX con números reales, CSV UTF-8 con BOM, CRLF, separador ";" y
// protección contra fórmulas, y PDF (pdf.go).
type Documento struct {
	Titulo string
	Meta   [][2]string // el primer par es ("Reporte", Titulo)
	Tablas []Tabla
}

// Tabla: columnas (la primera es la etiqueta de la fila) y filas.
type Tabla struct {
	Titulo   string
	Nota     string
	Columnas []string
	Filas    []FilaTabla
}

type FilaTabla struct {
	Etiqueta string
	Valores  []Valor
	Total    bool
}

// Valor de una celda: entero, porcentaje (0..1) o texto. Vacío si nada.
type Valor struct {
	Entero *int
	Pct    *float64
	Texto  string
}

func Entero(n int) Valor          { return Valor{Entero: &n} }
func Porcentaje(p *float64) Valor { return Valor{Pct: p} }
func Texto(s string) Valor        { return Valor{Texto: s} }

func (v Valor) texto() string {
	switch {
	case v.Entero != nil:
		return fmt.Sprint(*v.Entero)
	case v.Pct != nil:
		return textoPorcentaje(v.Pct)
	}
	return v.Texto
}

// GenerarDocumento devuelve el archivo en el formato pedido.
func GenerarDocumento(d Documento, formato string) ([]byte, error) {
	switch formato {
	case FormatoXLSX:
		return documentoXLSX(d)
	case FormatoCSV:
		return documentoCSV(d)
	case FormatoPDF:
		return documentoPDF(d)
	}
	return nil, fmt.Errorf("formato no admitido")
}

// protegerFormula: misma regla que tabular (un valor que empieza con = + @ o
// "-" no numérico se antepone con "'" para que Excel no lo evalúe).
func protegerFormula(v string) string {
	if v == "" {
		return v
	}
	switch v[0] {
	case '=', '+', '@', '\t', '\r':
		return "'" + v
	case '-':
		if len(v) > 1 && v[1] >= '0' && v[1] <= '9' {
			return v
		}
		return "'" + v
	}
	return v
}

func textoPorcentaje(p *float64) string {
	if p == nil {
		return ""
	}
	return strings.Replace(fmt.Sprintf("%.1f %%", *p*100), ".", ",", 1)
}

func documentoCSV(d Documento) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM UTF-8: Excel abre bien tildes y ñ
	w := csv.NewWriter(&buf)
	// Separador ";": Excel en español usa la coma como separador DECIMAL y el
	// ";" como separador de listas; con "," todo el archivo quedaba en una sola
	// columna (y los porcentajes "12,3 %" obligaban a entrecomillar).
	// encoding/csv entrecomilla los valores con ";", comillas o saltos de línea.
	w.Comma = SeparadorCSV
	w.UseCRLF = true
	escribir := func(campos ...string) {
		for i := range campos {
			campos[i] = protegerFormula(campos[i])
		}
		_ = w.Write(campos)
	}
	for _, m := range d.Meta {
		escribir(m[0], m[1])
	}
	for _, t := range d.Tablas {
		escribir()
		escribir(t.Titulo)
		if t.Nota != "" {
			escribir(t.Nota)
		}
		escribir(append([]string{}, t.Columnas...)...)
		for _, f := range t.Filas {
			campos := []string{f.Etiqueta}
			for _, v := range f.Valores {
				campos = append(campos, v.texto())
			}
			escribir(campos...)
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func documentoXLSX(d Documento) ([]byte, error) {
	libro := excelize.NewFile()
	defer func() { _ = libro.Close() }()
	const hoja = "Reporte"
	if err := libro.SetSheetName("Sheet1", hoja); err != nil {
		return nil, err
	}
	estilo := func(s *excelize.Style) int {
		id, _ := libro.NewStyle(s)
		return id
	}
	titulo := estilo(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	campo := estilo(&excelize.Style{Font: &excelize.Font{Bold: true}})
	seccionE := estilo(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 12}})
	nota := estilo(&excelize.Style{Font: &excelize.Font{Italic: true, Color: "555555"}})
	cabecera := estilo(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"164194"}}})
	// Un decimal, igual que el CSV y la pantalla ("12,3 %").
	formatoPct := "0.0%"
	porcentaje := estilo(&excelize.Style{CustomNumFmt: &formatoPct})
	borde := []excelize.Border{{Type: "top", Color: "000000", Style: 1}}
	totalE := estilo(&excelize.Style{Font: &excelize.Font{Bold: true}, Border: borde})
	totalPct := estilo(&excelize.Style{Font: &excelize.Font{Bold: true}, CustomNumFmt: &formatoPct, Border: borde})

	maxCols := 2
	for _, t := range d.Tablas {
		maxCols = max(maxCols, len(t.Columnas))
	}
	ultimaCol, _ := excelize.ColumnNumberToName(maxCols)
	_ = libro.SetColWidth(hoja, "A", "A", 48)
	_ = libro.SetColWidth(hoja, "B", ultimaCol, 16)
	n := 1
	ref := func(col int) string {
		c, _ := excelize.CoordinatesToCellName(col, n)
		return c
	}

	_ = libro.SetCellValue(hoja, ref(1), d.Titulo)
	_ = libro.SetCellStyle(hoja, ref(1), ref(1), titulo)
	n += 2
	for i, m := range d.Meta {
		if i == 0 {
			continue // ("Reporte", Titulo) ya es el título
		}
		_ = libro.SetCellValue(hoja, ref(1), m[0])
		_ = libro.SetCellStyle(hoja, ref(1), ref(1), campo)
		_ = libro.SetCellStr(hoja, ref(2), m[1])
		n++
	}
	for _, t := range d.Tablas {
		n++
		_ = libro.SetCellValue(hoja, ref(1), t.Titulo)
		_ = libro.SetCellStyle(hoja, ref(1), ref(1), seccionE)
		n++
		if t.Nota != "" {
			_ = libro.SetCellValue(hoja, ref(1), t.Nota)
			_ = libro.SetCellStyle(hoja, ref(1), ref(1), nota)
			n++
		}
		cols := make([]any, len(t.Columnas))
		for i, c := range t.Columnas {
			cols[i] = c
		}
		_ = libro.SetSheetRow(hoja, ref(1), &cols)
		_ = libro.SetCellStyle(hoja, ref(1), ref(len(t.Columnas)), cabecera)
		n++
		for _, f := range t.Filas {
			_ = libro.SetCellStr(hoja, ref(1), f.Etiqueta)
			for i, v := range f.Valores {
				c := ref(i + 2)
				switch {
				case v.Entero != nil:
					_ = libro.SetCellInt(hoja, c, int64(*v.Entero))
				case v.Pct != nil:
					_ = libro.SetCellFloat(hoja, c, *v.Pct, 4, 64)
					_ = libro.SetCellStyle(hoja, c, c, porcentaje)
				case v.Texto != "":
					_ = libro.SetCellStr(hoja, c, v.Texto)
				}
			}
			if f.Total {
				for i := 0; i <= len(f.Valores); i++ {
					c := ref(i + 1)
					if i > 0 && f.Valores[i-1].Pct != nil {
						_ = libro.SetCellStyle(hoja, c, c, totalPct)
					} else {
						_ = libro.SetCellStyle(hoja, c, c, totalE)
					}
				}
			}
			n++
		}
	}
	_ = libro.SetDocProps(&excelize.DocProperties{Title: d.Titulo, Creator: "SIS-MESA-PARTES"})
	var buf bytes.Buffer
	if err := libro.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
