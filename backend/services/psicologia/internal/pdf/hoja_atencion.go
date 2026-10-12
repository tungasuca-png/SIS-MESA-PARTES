// Package pdf genera el PDF de la Hoja de Atención psicológica en el propio
// Psicología Service (texto vectorial con go-pdf/fpdf, en memoria, sin
// archivos temporales). Mismo enfoque que el cargo del FUT del Gateway
// (fuentes núcleo con traducción cp1252 para tildes/eñes), sin compartir su
// código: cada servicio es un módulo Go independiente.
package pdf

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

//go:embed assets/escudo-tungasuca.png
var escudoPNG []byte

// Opcion es una casilla de la hoja (código + etiqueta congelada).
type Opcion struct {
	Codigo   string
	Etiqueta string
}

// HojaAtencion son los datos YA AUTORIZADOS de una versión de una atención.
type HojaAtencion struct {
	EstudianteNombre string
	ExpedienteCodigo string
	Numero           int
	Version          int
	Estado           string

	FechaAtencion         string // DD/MM/AAAA
	HoraAtencion          string // HH:MM
	RealizadaPor          []string
	RealizadaPorOtros     string
	Motivos               []Opcion
	MotivoOtros           string
	DescripcionMotivo     string
	AnalisisIntegral      string
	Estrategias           string
	Derivaciones          []Opcion
	DerivacionOtro        string
	SeguimientoResultados string
	Acuerdos              string
	CompromisoEstudiante  string
	CompromisoFamilia     string
}

const (
	pageW, pageH       = 210.0, 297.0
	marginX            = 15.0
	marginTop          = 12.0
	marginBottom       = 14.0
	contentW           = pageW - 2*marginX
	lineH              = 4.2
	pad                = 1.6
	minTextLines       = 2
	fontBody           = 8.5
	fontLabel          = 8.5
	fontSmall          = 7.5
	checkbox           = 2.8
	firmaMinH          = 18.0
	maxSideCellHeightM = 70.0 // por encima, la celda lateral se imprime a ancho completo
)

// Anchos de columna medidos sobre la hoja institucional: 33.5% | 17% | 24% | 25.5%.
var colW = [4]float64{contentW * 0.335, contentW * 0.17, contentW * 0.24, contentW * 0.255}

type builder struct {
	pdf *fpdf.Fpdf
	tr  func(string) string
}

// Build devuelve el PDF de UNA atención (una versión). No guarda nada.
func Build(h HojaAtencion) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginX, marginTop, marginX)
	pdf.SetAutoPageBreak(false, marginBottom)
	pdf.AliasNbPages("{nb}")
	b := &builder{pdf: pdf, tr: pdf.UnicodeTranslatorFromDescriptor("")}
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(90, 90, 90)
		pdf.CellFormat(0, 4, b.tr(fmt.Sprintf("%s · Atención %02d · Versión %d · Página %d de {nb}",
			h.ExpedienteCodigo, h.Numero, h.Version, pdf.PageNo())), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})
	pdf.AddPage()

	b.encabezado()
	b.identificacion(h)
	b.bloque(h)

	if err := pdf.Error(); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Encabezado institucional (según la hoja: escudo, texto centrado, logo JEC).
// El logo de Jornada Escolar Completa NO existe como archivo en el sistema:
// se reproduce su texto, sin inventar una imagen.
// ---------------------------------------------------------------------------
func (b *builder) encabezado() {
	p := b.pdf
	top := p.GetY()
	opt := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	p.RegisterImageOptionsReader("escudo", opt, bytes.NewReader(escudoPNG))
	if p.Ok() {
		p.ImageOptions("escudo", marginX+8, top, 0, 19, false, opt, 0, "")
	}

	p.SetXY(marginX, top+1.5)
	p.SetFont("Helvetica", "B", 9)
	p.SetTextColor(192, 0, 0)
	p.CellFormat(contentW, 4.5, b.tr("INSTITUCIÓN EDUCATIVA"), "", 2, "C", false, 0, "")
	p.SetFont("Helvetica", "B", 17)
	p.SetTextColor(31, 56, 100)
	p.CellFormat(contentW, 8, b.tr("TUNGASUCA – UGEL 04"), "", 2, "C", false, 0, "")
	p.SetFont("Helvetica", "B", 6.2)
	p.SetTextColor(0, 0, 0)
	p.CellFormat(contentW, 3.5, b.tr("AV. MARIANO CONDORCANQUI S/N – URB. TUNGASUCA CARABAYLLO"), "", 2, "C", false, 0, "")

	jx, jw := marginX+contentW-38, 30.0
	p.SetXY(jx, top+1)
	p.SetFont("Helvetica", "", 6.5)
	p.SetTextColor(31, 56, 100)
	p.CellFormat(jw, 3, b.tr("Jornada Escolar"), "", 2, "C", false, 0, "")
	p.SetFont("Helvetica", "B", 9)
	p.CellFormat(jw, 4, b.tr("COMPLETA"), "", 2, "C", false, 0, "")
	p.SetFont("Helvetica", "B", 5.5)
	p.CellFormat(jw, 2.8, b.tr("SECUNDARIA"), "", 2, "C", false, 0, "")
	p.SetFont("Helvetica", "", 4.8)
	p.CellFormat(jw, 2.4, b.tr("MINISTERIO DE EDUCACIÓN"), "", 2, "C", false, 0, "")
	p.CellFormat(jw, 2.4, b.tr("UGEL 04-COMAS"), "", 2, "C", false, 0, "")
	p.SetTextColor(0, 0, 0)

	p.SetXY(marginX, top+21)
	p.SetFont("Helvetica", "BI", 11)
	p.CellFormat(contentW, 6, b.tr("FICHA DE ATENCIÓN PERSONALIZADA AL ESTUDIANTE Y LA FAMILIA"), "", 1, "C", false, 0, "")
}

// identificacion: franja FUERA de la tabla (la hoja en papel no tiene campos
// para el estudiante). Sin DNI.
func (b *builder) identificacion(h HojaAtencion) {
	p := b.pdf
	p.SetFont("Helvetica", "", fontSmall)
	txt := fmt.Sprintf("Estudiante: %s   ·   Expediente: %s   ·   Atención: %02d   ·   Versión: %d   ·   Estado: %s",
		h.EstudianteNombre, h.ExpedienteCodigo, h.Numero, h.Version, h.Estado)
	for _, line := range b.wrap(txt, contentW) {
		p.CellFormat(contentW, 3.8, line, "", 2, "L", false, 0, "")
	}
	p.Ln(1.2)
}

// ---------------------------------------------------------------------------
// Bloque de atención: la misma tabla de la hoja institucional.
// ---------------------------------------------------------------------------
func (b *builder) bloque(h HojaAtencion) {
	p := b.pdf
	p.SetLineWidth(0.2)
	tableTop := p.GetY()

	// Fila 1: Fecha | Hora | Atención realizada por
	realizadas := setOf(h.RealizadaPor)
	atencionW := colW[2] + colW[3]
	atencionX := marginX + colW[0] + colW[1]
	atencionItems := []check{{"Directivo", realizadas["DIRECTIVO"]}, {"Psicólogo", realizadas["PSICOLOGO"]}}
	otrosAtencion := b.suffixLines(atencionX+pad, atencionX+atencionW-pad, "", atencionItems, "Otros: "+h.RealizadaPorOtros)
	row1H := float64(2+len(otrosAtencion.rest))*lineH + 2*pad
	b.ensure(row1H)
	y := p.GetY()
	b.box(marginX, y, colW[0], row1H)
	b.box(marginX+colW[0], y, colW[1], row1H)
	b.box(atencionX, y, atencionW, row1H)
	b.labelValue(marginX, y+pad+lineH/2, colW[0], "Fecha: ", h.FechaAtencion)
	b.labelValue(marginX+colW[0], y+pad+lineH/2, colW[1], "Hora: ", h.HoraAtencion)
	b.optionLine(atencionX+pad, y+pad, "Atención realizada por", []check{{"Docente", realizadas["DOCENTE"]}, {"Auxiliar", realizadas["AUXILIAR"]}}, "")
	b.optionLine(atencionX+pad, y+pad+lineH, "", atencionItems, otrosAtencion.first)
	b.lines(atencionX+pad, y+pad+2*lineH, atencionW-2*pad, otrosAtencion.rest)
	p.SetXY(marginX, y+row1H)

	// Fila 2: Motivo | Descripción breve del motivo
	motivos := optionSet(h.Motivos)
	motivoW := colW[0] + colW[1]
	descW := colW[2] + colW[3]
	p.SetFont("Helvetica", "", fontBody)
	descLines := b.wrap(h.DescripcionMotivo, descW-2*pad)
	descH := float64(maxInt(len(descLines), minTextLines)+1)*lineH + 2*pad
	motivoItems := []check{{"Inclusión", motivos["INCLUSION"]}, {"familiar", motivos["FAMILIAR"]}}
	otrosMotivo := b.suffixLines(marginX+pad, marginX+motivoW-pad, "", motivoItems, "otros: "+h.MotivoOtros)
	motivoH := float64(2+len(otrosMotivo.rest))*lineH + 2*pad
	row2H := maxF(motivoH, descH)
	sideBySide := row2H <= maxSideCellHeightM
	rowW := motivoW
	if !sideBySide {
		// Descripción muy larga: la celda de Motivo ocupa todo el ancho y la
		// descripción pasa a su propia fila (paginable). No se recorta nada.
		otrosMotivo = b.suffixLines(marginX+pad, marginX+contentW-pad, "", motivoItems, "otros: "+h.MotivoOtros)
		row2H = float64(2+len(otrosMotivo.rest))*lineH + 2*pad
		rowW = contentW
	}
	b.ensure(row2H)
	y = p.GetY()
	b.box(marginX, y, rowW, row2H)
	b.optionLine(marginX+pad, y+pad, "Motivo", []check{
		{"académico", motivos["ACADEMICO"]}, {"conductual", motivos["CONDUCTUAL"]}, {"socioemocional", motivos["SOCIOEMOCIONAL"]},
	}, "")
	b.optionLine(marginX+pad, y+pad+lineH, "", motivoItems, otrosMotivo.first)
	b.lines(marginX+pad, y+pad+2*lineH, rowW-2*pad, otrosMotivo.rest)
	if sideBySide {
		dx := marginX + motivoW
		b.box(dx, y, descW, row2H)
		b.label(dx+pad, y+pad, "Descripción breve del motivo", "")
		b.lines(dx+pad, y+pad+lineH, descW-2*pad, descLines)
		p.SetXY(marginX, y+row2H)
	} else {
		p.SetXY(marginX, y+row2H)
		b.textRow("Descripción breve del motivo", "", h.DescripcionMotivo, nil)
	}

	b.textRow("Análisis integral del caso", "(Factores personales, familiares, académicos y/o sociales que influyen)", h.AnalisisIntegral, nil)
	b.textRow("Estrategias de atención y apoyo", "(Pedagógicas: adaptaciones, refuerzo, flexibilización, etc. Socioemocionales y de convivencia)",
		h.Estrategias, b.derivacionesExtra(h))
	b.textRow("Seguimiento y resultados", "(registrar avances o dificultades observadas en atenciones posteriores)", h.SeguimientoResultados, nil)
	b.textRow("Acuerdos:", "", h.Acuerdos, nil)

	// Fila final: Compromiso del estudiante | Compromiso y firma de la
	// familia/apoderado | recuadro "Firma". La firma NO se genera: su forma y
	// firmante siguen pendientes de decisión institucional.
	famW := colW[1] + colW[2]
	p.SetFont("Helvetica", "", fontBody)
	estLines := b.wrap(h.CompromisoEstudiante, colW[0]-2*pad)
	famLines := b.wrap(h.CompromisoFamilia, famW-2*pad)
	lastH := maxF(float64(maxInt(maxInt(len(estLines), len(famLines)), 3)+1)*lineH+2*pad, firmaMinH+lineH+pad)
	if lastH > maxSideCellHeightM {
		b.textRow("Compromiso del estudiante", "", h.CompromisoEstudiante, nil)
		b.textRow("Compromiso y firma de la familia/apoderado", "", h.CompromisoFamilia, nil)
		b.firmaRow()
	} else {
		b.ensure(lastH)
		y = p.GetY()
		b.box(marginX, y, colW[0], lastH)
		b.box(marginX+colW[0], y, famW, lastH)
		fx := marginX + colW[0] + famW
		b.box(fx, y, colW[3], lastH-lineH-pad)
		b.box(fx, y+lastH-lineH-pad, colW[3], lineH+pad)
		b.centeredLabel(marginX, y+pad, colW[0], "Compromiso del estudiante")
		b.lines(marginX+pad, y+pad+lineH, colW[0]-2*pad, estLines)
		b.centeredLabel(marginX+colW[0], y+pad, famW, "Compromiso y firma de la familia/apoderado")
		b.lines(marginX+colW[0]+pad, y+pad+lineH, famW-2*pad, famLines)
		b.centeredLabel(fx, y+lastH-lineH-pad/2, colW[3], "Firma")
		p.SetXY(marginX, y+lastH)
	}

	// Borde exterior grueso (como la hoja) sobre la primera página del bloque.
	if p.PageNo() == 1 {
		p.SetLineWidth(0.6)
		p.Rect(marginX, tableTop, contentW, p.GetY()-tableTop, "D")
		p.SetLineWidth(0.2)
	}
}

func (b *builder) firmaRow() {
	p := b.pdf
	h := firmaMinH + lineH + pad
	b.ensure(h)
	y := p.GetY()
	fx := marginX + colW[0] + colW[1] + colW[2]
	b.box(fx, y, colW[3], firmaMinH)
	b.box(fx, y+firmaMinH, colW[3], lineH+pad)
	b.centeredLabel(fx, y+firmaMinH+pad/2, colW[3], "Firma")
	p.SetXY(marginX, y+h)
}

// extraRow: contenido adicional al final de una fila de texto (las
// derivaciones, dentro de la celda de Estrategias como en la hoja).
type extraRow struct {
	height float64
	draw   func(y float64) float64
}

// textRow: fila de ancho completo con rótulo y texto. Si el texto no entra en
// la página, continúa en la siguiente (el borde se cierra por página).
func (b *builder) textRow(label, note, text string, extra *extraRow) {
	p := b.pdf
	p.SetFont("Helvetica", "", fontBody)
	lines := b.wrap(text, contentW-2*pad)
	for len(lines) < minTextLines {
		lines = append(lines, "")
	}
	p.SetFont("Helvetica", "I", fontSmall)
	noteLines := b.wrap(note, contentW-2*pad-b.width("BI", fontLabel, label)-2)
	headerH := lineH * float64(maxInt(1, len(noteLines)))
	extraH := 0.0
	if extra != nil {
		extraH = extra.height + pad
	}

	b.ensure(headerH + lineH*minTextLines + 2*pad + extraH)
	y := p.GetY()
	segTop := y
	b.labelWithNote(marginX+pad, y+pad, label, noteLines)
	y += pad + headerH
	p.SetFont("Helvetica", "", fontBody)
	for i, line := range lines {
		last := i == len(lines)-1
		need := lineH
		if last {
			need += pad + extraH
		}
		if y+need > pageH-marginBottom {
			b.box(marginX, segTop, contentW, y-segTop+pad)
			p.AddPage()
			y = p.GetY()
			segTop = y
			y += pad
		}
		p.SetFont("Helvetica", "", fontBody)
		p.SetXY(marginX+pad, y)
		p.CellFormat(contentW-2*pad, lineH, line, "", 0, "L", false, 0, "")
		y += lineH
	}
	if extra != nil {
		y = extra.draw(y)
	}
	y += pad
	b.box(marginX, segTop, contentW, y-segTop)
	p.SetXY(marginX, y)
}

// derivacionesExtra: "Derivaciones (si corresponde): ☐ Tutoría ☐ …" con la
// casilla ANTES del texto (como en la hoja). El texto de "Otro:" que no entre
// en la línea continúa debajo, dentro de la misma celda.
func (b *builder) derivacionesExtra(h HojaAtencion) *extraRow {
	p := b.pdf
	sel := optionSet(h.Derivaciones)
	items := []check{
		{"Tutoría", sel["TUTORIA"]}, {"Psicología", sel["PSICOLOGIA"]}, {"SAANEE", sel["SAANEE"]},
		{"DEMUNA", sel["DEMUNA"]}, {"Centro de Salud", sel["CENTRO_SALUD"]}, {"Otro:", sel["OTRO"]},
	}
	title := "Derivaciones (si corresponde):"
	x := marginX + pad + b.width("I", fontSmall, title) + 1.5
	for _, item := range items {
		x += checkbox + 0.8 + b.width("I", fontSmall, item.text) + 2
	}
	p.SetFont("Helvetica", "", fontSmall)
	var first string
	var rest []string
	if h.DerivacionOtro != "" {
		right := marginX + contentW - pad
		if b.pdf.GetStringWidth(b.tr(h.DerivacionOtro)) <= right-x {
			first = h.DerivacionOtro
		} else {
			rest = b.wrap(h.DerivacionOtro, contentW-2*pad)
		}
	}
	return &extraRow{
		height: float64(1+len(rest)) * lineH,
		draw: func(y float64) float64 {
			x := marginX + pad
			p.SetFont("Helvetica", "I", fontSmall)
			x = b.textAt(x, y, title) + 1.5
			for _, item := range items {
				x = b.checkboxAt(x, y, item.checked) + 0.8
				p.SetFont("Helvetica", "I", fontSmall)
				x = b.textAt(x, y, item.text) + 2
			}
			if first != "" {
				p.SetFont("Helvetica", "", fontSmall)
				b.textAt(x, y, first)
			}
			for i, line := range rest {
				p.SetFont("Helvetica", "", fontSmall)
				p.SetXY(marginX+pad, y+float64(i+1)*lineH)
				p.CellFormat(contentW-2*pad, lineH, line, "", 0, "L", false, 0, "")
			}
			return y + float64(1+len(rest))*lineH
		},
	}
}

// suffixResult: el texto libre ("Otros: …") que va al final de una línea de
// casillas. Si no entra en esa línea, se imprime completo en líneas debajo.
type suffixResult struct {
	first string
	rest  []string
}

func (b *builder) suffixLines(x, right float64, label string, items []check, suffix string) suffixResult {
	left := x
	b.pdf.SetFont("Helvetica", "", fontBody)
	if label != "" {
		x += b.width("BI", fontLabel, label) + b.width("", fontBody, " : ") + 0.5
	}
	for _, item := range items {
		x += b.width("", fontBody, item.text) + 0.8 + checkbox + 2.5
	}
	b.pdf.SetFont("Helvetica", "", fontBody)
	if b.pdf.GetStringWidth(b.tr(suffix)) <= right-x {
		return suffixResult{first: suffix}
	}
	// El rótulo ("Otros:") queda en la línea; el texto va completo debajo.
	colon := strings.Index(suffix, ":")
	head, body := suffix, ""
	if colon >= 0 {
		head, body = suffix[:colon+1], strings.TrimSpace(suffix[colon+1:])
	}
	b.pdf.SetFont("Helvetica", "", fontBody)
	return suffixResult{first: head, rest: b.wrap(body, right-left)}
}

// ---------------------------------------------------------------------------
// Primitivas
// ---------------------------------------------------------------------------

type check struct {
	text    string
	checked bool
}

func (b *builder) box(x, y, w, h float64) {
	b.pdf.Rect(x, y, w, h, "D")
}

// ensure salta de página si el siguiente bloque de altura h no entra.
func (b *builder) ensure(h float64) {
	if b.pdf.GetY()+h > pageH-marginBottom {
		b.pdf.AddPage()
	}
}

func (b *builder) width(style string, size float64, text string) float64 {
	b.pdf.SetFont("Helvetica", style, size)
	return b.pdf.GetStringWidth(b.tr(text)) + 2*b.pdf.GetCellMargin() // igual que textAt
}

// textAt dibuja el texto en x y devuelve dónde termina. CellFormat deja el
// margen interno de celda a cada lado del texto: se incluye en el ancho para
// que lo siguiente (p. ej. una casilla) no se superponga a la última letra.
func (b *builder) textAt(x, y float64, text string) float64 {
	w := b.pdf.GetStringWidth(b.tr(text)) + 2*b.pdf.GetCellMargin()
	b.pdf.SetXY(x, y)
	b.pdf.CellFormat(w, lineH, b.tr(text), "", 0, "L", false, 0, "")
	return x + w
}

func (b *builder) checkboxAt(x, y float64, checked bool) float64 {
	p := b.pdf
	top := y + (lineH-checkbox)/2
	p.Rect(x, top, checkbox, checkbox, "D")
	if checked {
		p.Line(x+0.5, top+0.5, x+checkbox-0.5, top+checkbox-0.5)
		p.Line(x+checkbox-0.5, top+0.5, x+0.5, top+checkbox-0.5)
	}
	return x + checkbox
}

func (b *builder) label(x, y float64, label, note string) {
	b.pdf.SetFont("Helvetica", "BI", fontLabel)
	x = b.textAt(x, y, label)
	if note != "" {
		b.pdf.SetFont("Helvetica", "I", fontSmall)
		b.textAt(x+1, y, note)
	}
}

func (b *builder) labelWithNote(x, y float64, label string, noteLines []string) {
	p := b.pdf
	p.SetFont("Helvetica", "BI", fontLabel)
	nx := b.textAt(x, y, label) + 1
	p.SetFont("Helvetica", "I", fontSmall)
	for i, line := range noteLines {
		p.SetXY(nx, y+float64(i)*lineH)
		p.CellFormat(0, lineH, line, "", 0, "L", false, 0, "")
	}
}

func (b *builder) centeredLabel(x, y, w float64, label string) {
	b.pdf.SetFont("Helvetica", "BI", fontLabel)
	b.pdf.SetXY(x, y)
	b.pdf.CellFormat(w, lineH, b.tr(label), "", 0, "C", false, 0, "")
}

func (b *builder) labelValue(x, y, w float64, label, value string) {
	b.pdf.SetFont("Helvetica", "", fontBody)
	nx := b.textAt(x+pad, y, label)
	b.pdf.SetFont("Helvetica", "B", fontBody)
	b.pdf.SetXY(nx, y)
	b.pdf.CellFormat(w-(nx-x)-pad, lineH, b.tr(value), "", 0, "L", false, 0, "")
}

// optionLine: "Rótulo : texto ☐ texto ☐ ... sufijo", con la casilla DESPUÉS
// del texto (como en la hoja para "Atención realizada por" y "Motivo").
func (b *builder) optionLine(x, y float64, label string, items []check, suffix string) {
	p := b.pdf
	if label != "" {
		p.SetFont("Helvetica", "BI", fontLabel)
		x = b.textAt(x, y, label)
		p.SetFont("Helvetica", "", fontBody)
		x = b.textAt(x, y, " : ") + 0.5
	}
	p.SetFont("Helvetica", "", fontBody)
	for _, item := range items {
		x = b.textAt(x, y, item.text) + 0.8
		x = b.checkboxAt(x, y, item.checked) + 2.5
	}
	if suffix != "" {
		b.textAt(x, y, suffix)
	}
}

func (b *builder) lines(x, y, w float64, lines []string) {
	b.pdf.SetFont("Helvetica", "", fontBody)
	for i, line := range lines {
		b.pdf.SetXY(x, y+float64(i)*lineH)
		b.pdf.CellFormat(w, lineH, line, "", 0, "L", false, 0, "")
	}
}

// wrap envuelve texto (ya traducido a cp1252) al ancho dado con la fuente
// actual, respetando saltos de línea y cortando palabras más largas que el
// ancho. Devuelve líneas listas para CellFormat.
func (b *builder) wrap(text string, maxWidth float64) []string {
	var out []string
	if strings.TrimSpace(text) == "" {
		return out
	}
	measure := b.pdf.GetStringWidth
	for _, paragraph := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		paragraph = b.tr(paragraph)
		if strings.TrimSpace(paragraph) == "" {
			out = append(out, "")
			continue
		}
		current := ""
		for _, word := range strings.Fields(paragraph) {
			for measure(word) > maxWidth {
				cut := len(word)
				for cut > 1 && measure(word[:cut]) > maxWidth {
					cut--
				}
				if current != "" {
					out = append(out, current)
					current = ""
				}
				out = append(out, word[:cut])
				word = word[cut:]
			}
			attempt := word
			if current != "" {
				attempt = current + " " + word
			}
			if current != "" && measure(attempt) > maxWidth {
				out = append(out, current)
				current = word
			} else {
				current = attempt
			}
		}
		if current != "" {
			out = append(out, current)
		}
	}
	return out
}

func setOf(values []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range values {
		m[v] = true
	}
	return m
}

func optionSet(opciones []Opcion) map[string]bool {
	m := map[string]bool{}
	for _, o := range opciones {
		m[o.Codigo] = true
	}
	return m
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
