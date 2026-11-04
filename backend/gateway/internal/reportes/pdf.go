package reportes

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

// PDF del Documento genérico: el MISMO contenido que el CSV y el XLSX de cada
// pestaña (título, metadatos con filtros y alcance, tablas con notas y
// totales). Usa go-pdf/fpdf, que el Gateway ya usa para el FUT, con las
// fuentes estándar y la traducción a cp1252 (tildes, ñ, «», –, ×).
//
// - Horizontal si alguna tabla tiene más de 5 columnas.
// - Las tablas largas continúan en la página siguiente repitiendo el título
//   ("continuación") y los encabezados de columna.
// - Pie institucional con «Página N de M».

const (
	pdfMargen     = 14.0
	pdfAltoLinea  = 4.6
	pdfPadCelda   = 1.6
	pdfAltoPie    = 12.0
	pdfColsVertic = 5
)

// Colores del sistema (azul institucional y grises de la interfaz).
var (
	pdfAzul       = [3]int{0x16, 0x41, 0x94}
	pdfTextoSec   = [3]int{0x47, 0x54, 0x67}
	pdfBorde      = [3]int{0xD0, 0xD5, 0xDD}
	pdfFondoAlt   = [3]int{0xF5, 0xF7, 0xFA}
	pdfFondoTotal = [3]int{0xE8, 0xEF, 0xFA}
)

// Caracteres sin equivalente en cp1252 (fuentes estándar del PDF).
var pdfSustituir = strings.NewReplacer("→", "->", "−", "-", "≥", ">=", "≤", "<=", "▲", "+", "▼", "-")

func documentoPDF(d Documento) ([]byte, error) {
	orientacion := "P"
	for _, t := range d.Tablas {
		if len(t.Columnas) > pdfColsVertic {
			orientacion = "L"
		}
	}
	pdf := fpdf.New(orientacion, "mm", "A4", "")
	pdf.SetMargins(pdfMargen, pdfMargen, pdfMargen)
	pdf.SetAutoPageBreak(false, pdfMargen+pdfAltoPie) // los saltos los decide la tabla
	pdf.AliasNbPages("{nb}")
	pdf.SetTitle(d.Titulo, true)
	pdf.SetAuthor("SIS-MESA-PARTES", true)
	pdf.SetCreator("SIS-MESA-PARTES", true)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	txt := func(s string) string { return tr(pdfSustituir.Replace(s)) }
	ancho, alto := pdf.GetPageSize()
	util := ancho - 2*pdfMargen
	limite := alto - pdfMargen - pdfAltoPie

	pdf.SetFooterFunc(func() {
		pdf.SetY(alto - pdfMargen - 4)
		pdf.SetDrawColor(pdfBorde[0], pdfBorde[1], pdfBorde[2])
		pdf.Line(pdfMargen, pdf.GetY()-1.5, ancho-pdfMargen, pdf.GetY()-1.5)
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetTextColor(pdfTextoSec[0], pdfTextoSec[1], pdfTextoSec[2])
		pdf.CellFormat(util/2, 4, txt(Institucion+" · Documento generado por el sistema"), "", 0, "L", false, 0, "")
		pdf.CellFormat(util/2, 4, txt(fmt.Sprintf("Página %d de {nb}", pdf.PageNo())), "", 0, "R", false, 0, "")
	})

	// ---- Encabezado del documento ----
	pdf.AddPage()
	pdf.SetFillColor(pdfAzul[0], pdfAzul[1], pdfAzul[2])
	pdf.Rect(pdfMargen, pdfMargen, util, 1.2, "F")
	pdf.SetY(pdfMargen + 4)
	pdf.SetFont("Helvetica", "B", 8.5)
	pdf.SetTextColor(pdfAzul[0], pdfAzul[1], pdfAzul[2])
	pdf.CellFormat(util, 4.5, txt("COLEGIO TUNGASUCA · MESA DE PARTES"), "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 15)
	pdf.SetTextColor(0x10, 0x18, 0x28)
	pdf.MultiCell(util, 7, txt(d.Titulo), "", "L", false)
	pdf.Ln(2)

	// Metadatos (institución, generación en hora de Lima, periodo, alcance y
	// filtros) en dos columnas: campo | valor. Ambos se ajustan en varias
	// líneas; la fila toma la altura mayor.
	anchoCampo := 62.0
	for i, m := range d.Meta {
		if i == 0 {
			continue // ("Reporte", Titulo) ya es el título
		}
		pdf.SetFont("Helvetica", "", 8.5)
		lineas := len(pdf.SplitText(medible(txt(m[1])), util-anchoCampo))
		pdf.SetFont("Helvetica", "B", 8.5)
		lineas = max(1, lineas, len(pdf.SplitText(medible(txt(m[0])), anchoCampo-3)))
		h := float64(lineas) * 4.4
		if pdf.GetY()+h > limite {
			pdf.AddPage()
		}
		y := pdf.GetY()
		pdf.SetTextColor(pdfTextoSec[0], pdfTextoSec[1], pdfTextoSec[2])
		pdf.MultiCell(anchoCampo-3, 4.4, txt(m[0]), "", "L", false)
		pdf.SetFont("Helvetica", "", 8.5)
		pdf.SetTextColor(0x10, 0x18, 0x28)
		pdf.SetXY(pdfMargen+anchoCampo, y)
		pdf.MultiCell(util-anchoCampo, 4.4, txt(m[1]), "", "L", false)
		pdf.SetXY(pdfMargen, y+h)
	}
	pdf.Ln(2)

	// ---- Tablas ----
	for _, t := range d.Tablas {
		pdfTabla(pdf, t, txt, util, limite)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// anchosColumnas: la primera (etiqueta) se lleva el espacio restante tras dar
// a cada columna numérica un ancho fijo acotado.
func anchosColumnas(n int, util float64) []float64 {
	anchos := make([]float64, n)
	if n == 1 {
		anchos[0] = util
		return anchos
	}
	resto := min(30.0, util*0.55/float64(n-1))
	anchos[0] = util - resto*float64(n-1)
	for i := 1; i < n; i++ {
		anchos[i] = resto
	}
	return anchos
}

func pdfTabla(pdf *fpdf.Fpdf, t Tabla, txt func(string) string, util, limite float64) {
	anchos := anchosColumnas(len(t.Columnas), util)

	alturaFila := func(celdas []string, negrita bool) float64 {
		estilo := ""
		if negrita {
			estilo = "B"
		}
		pdf.SetFont("Helvetica", estilo, 8)
		lineas := 1
		for i, c := range celdas {
			lineas = max(lineas, len(pdf.SplitText(medible(txt(c)), anchos[i]-2*pdfPadCelda)))
		}
		return float64(lineas)*pdfAltoLinea + 2*pdfPadCelda - 1
	}
	dibujarFila := func(celdas []string, alineacion []string, alto float64, relleno *[3]int, negrita, blanco bool) {
		x0, y0 := pdfMargen, pdf.GetY()
		x := x0
		for i, c := range celdas {
			if relleno != nil {
				pdf.SetFillColor(relleno[0], relleno[1], relleno[2])
				pdf.Rect(x, y0, anchos[i], alto, "F")
			}
			estilo := ""
			if negrita {
				estilo = "B"
			}
			pdf.SetFont("Helvetica", estilo, 8)
			if blanco {
				pdf.SetTextColor(0xFF, 0xFF, 0xFF)
			} else {
				pdf.SetTextColor(0x10, 0x18, 0x28)
			}
			pdf.SetXY(x+pdfPadCelda, y0+pdfPadCelda-0.5)
			pdf.MultiCell(anchos[i]-2*pdfPadCelda, pdfAltoLinea, txt(c), "", alineacion[i], false)
			x += anchos[i]
		}
		pdf.SetDrawColor(pdfBorde[0], pdfBorde[1], pdfBorde[2])
		pdf.Line(x0, y0+alto, x0+util, y0+alto)
		pdf.SetXY(x0, y0+alto)
	}
	titulo := func(continuacion bool) {
		texto := t.Titulo
		if continuacion {
			texto += " (continuación)"
		}
		pdf.SetFont("Helvetica", "B", 10.5)
		pdf.SetTextColor(pdfAzul[0], pdfAzul[1], pdfAzul[2])
		pdf.MultiCell(util, 5.5, txt(texto), "", "L", false)
		if !continuacion && t.Nota != "" {
			pdf.SetFont("Helvetica", "I", 7.8)
			pdf.SetTextColor(pdfTextoSec[0], pdfTextoSec[1], pdfTextoSec[2])
			pdf.MultiCell(util, 3.9, txt(t.Nota), "", "L", false)
		}
		pdf.Ln(1)
	}
	izquierda := make([]string, len(t.Columnas))
	for i := range izquierda {
		izquierda[i] = "R"
	}
	izquierda[0] = "L"
	altoCab := alturaFila(t.Columnas, true)
	cabecera := func() {
		dibujarFila(t.Columnas, izquierda, altoCab, &pdfAzul, true, true)
	}

	// Título + nota + encabezado + primera fila deben caber juntos.
	pdf.SetFont("Helvetica", "I", 7.8)
	altoNota := 0.0
	if t.Nota != "" {
		altoNota = float64(len(pdf.SplitText(medible(txt(t.Nota)), util))) * 3.9
	}
	primera := 0.0
	if len(t.Filas) > 0 {
		primera = alturaFila(ajustar(celdasDe(t.Filas[0]), len(t.Columnas)), t.Filas[0].Total)
	}
	if pdf.GetY()+7+altoNota+altoCab+primera > limite {
		pdf.AddPage()
	}
	titulo(false)
	cabecera()
	for i, f := range t.Filas {
		celdas := ajustar(celdasDe(f), len(t.Columnas))
		alto := alturaFila(celdas, f.Total)
		if pdf.GetY()+alto > limite {
			pdf.AddPage()
			titulo(true)
			cabecera()
		}
		var relleno *[3]int
		switch {
		case f.Total:
			relleno = &pdfFondoTotal
		case i%2 == 1:
			relleno = &pdfFondoAlt
		}
		dibujarFila(celdas, izquierda, alto, relleno, f.Total, false)
	}
	pdf.Ln(5)
}

// ajustar: exactamente n celdas, para que cada valor quede bajo su encabezado.
func ajustar(celdas []string, n int) []string {
	for len(celdas) < n {
		celdas = append(celdas, "")
	}
	return celdas[:n]
}

// medible: SplitText mide runas, pero el texto ya está en cp1252 (un byte por
// carácter): cada byte se pasa a la runa del mismo código para medirlo.
func medible(cp1252 string) string {
	r := make([]rune, len(cp1252))
	for i := 0; i < len(cp1252); i++ {
		r[i] = rune(cp1252[i])
	}
	return string(r)
}

// celdasDe: etiqueta + valores como texto (mismo formato que el CSV).
func celdasDe(f FilaTabla) []string {
	celdas := []string{f.Etiqueta}
	for _, v := range f.Valores {
		celdas = append(celdas, v.texto())
	}
	return celdas
}
