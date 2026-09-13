package logic

import (
	"bytes"
	"fmt"
	"image/png"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
)

// El FUT no tiene columnas propias en Expedientes Service para nombres/DNI/
// teléfono/domicilio/distrito/correo del solicitante (ver comentario en
// frontend/src/services/futService.js) — quedan empaquetados como texto
// estructurado dentro de "descripcion". Este parser es el equivalente en Go
// de parseDatosSolicitante() del frontend: MISMOS patrones, para que el PDF
// generado acá reconstruya exactamente los mismos datos que ya se ven en el
// "Cargo digital" en pantalla.
type datosSolicitanteFut struct {
	Nombres        string
	Dni            string
	Telefono       string
	Domicilio      string
	Distrito       string
	Correo         string
	Fundamentacion string
	Folios         string
}

var (
	reNombres        = regexp.MustCompile(`Nombres y apellidos:\s*(.+)`)
	reFolios         = regexp.MustCompile(`N° de folios adjuntos:\s*(\d+)`)
	reDni            = regexp.MustCompile(`DNI:\s*(.+)`)
	reTelefono       = regexp.MustCompile(`Teléfono:\s*(.+)`)
	reDomicilio      = regexp.MustCompile(`Domicilio:\s*(.+?)\s*—\s*Distrito:\s*(.+)`)
	reCorreo         = regexp.MustCompile(`Correo:\s*(.+)`)
	reFundamentacion = regexp.MustCompile(`(?s)FUNDAMENTACIÓN DE LO QUE SOLICITA\n(.*?)\n\nN° de folios adjuntos:`)
)

// parseDatosSolicitanteFut devuelve nil si "descripcion" no vino del FUT
// Digital (no tiene los dos campos mínimos que buildDescripcion() del
// frontend siempre incluye: nombres y folios) — mismo criterio que
// tieneFutExportable()/parseDatosSolicitante() en el frontend.
func parseDatosSolicitanteFut(descripcion string) *datosSolicitanteFut {
	nombresMatch := reNombres.FindStringSubmatch(descripcion)
	foliosMatch := reFolios.FindStringSubmatch(descripcion)
	if nombresMatch == nil || foliosMatch == nil {
		return nil
	}

	datos := &datosSolicitanteFut{
		Nombres: strings.TrimSpace(nombresMatch[1]),
		Folios:  strings.TrimSpace(foliosMatch[1]),
	}
	if m := reDni.FindStringSubmatch(descripcion); m != nil {
		datos.Dni = strings.TrimSpace(m[1])
	}
	if m := reTelefono.FindStringSubmatch(descripcion); m != nil {
		datos.Telefono = strings.TrimSpace(m[1])
	}
	if m := reDomicilio.FindStringSubmatch(descripcion); m != nil {
		if d := strings.TrimSpace(m[1]); d != "-" {
			datos.Domicilio = d
		}
		if d := strings.TrimSpace(m[2]); d != "-" {
			datos.Distrito = d
		}
	}
	if m := reCorreo.FindStringSubmatch(descripcion); m != nil {
		datos.Correo = strings.TrimSpace(m[1])
	}
	if m := reFundamentacion.FindStringSubmatch(descripcion); m != nil {
		datos.Fundamentacion = strings.TrimSpace(m[1])
	}
	return datos
}

// futCargoData junta los datos parseados de la descripción con el resto de
// lo que ya vive en columnas propias del expediente (asunto/código/fecha_
// registro tal cual, sin formatear) y lo que se trae de Documentos Service
// (lista de adjuntos reales + firma).
type futCargoData struct {
	datosSolicitanteFut
	Sumilla       string
	Codigo        string
	FechaRegistro string // ISO tal cual viene de Expedientes Service
	Documentos    []string
	FirmaPNG      []byte // nil si no se subió firma
}

// normalizeFirmaPNG valida el PNG decodificándolo con el paquete estándar
// de Go (mucho más tolerante que el parser propio de fpdf, que tiene un bug
// conocido: puede entrar en pánico con ciertos PNG válidos pero atípicos —
// p. ej. paletizados con transparencia) y lo vuelve a codificar en un PNG
// canónico (RGBA, 8 bits, sin entrelazado) — la misma forma que ya produce
// un navegador real al capturar la firma con canvas.toDataURL("image/png").
// Devuelve nil si los bytes no son un PNG válido, para que el cargo se
// genere igual sin firma en vez de fallar.
func normalizeFirmaPNG(raw []byte) []byte {
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// embedFirma intenta dibujar la firma ya normalizada en el PDF y devuelve
// si quedó embebida. El propio parser PNG de fpdf ya demostró (en pruebas
// con un PNG paletizado con transparencia) que puede entrar en pánico en
// vez de solo devolver un error — por eso, además de revisar pdf.Ok(), se
// envuelve en un recover(): una firma con un formato que ni siquiera
// normalizeFirmaPNG detectó como problemático no debe poder tumbar la
// generación de todo el cargo.
func embedFirma(pdf *fpdf.Fpdf, firmaPNG []byte, x, maxWidth, y, maxHeight float64) (embebida bool) {
	if len(firmaPNG) == 0 {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			pdf.ClearError()
			embebida = false
		}
	}()

	imgOpt := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	info := pdf.RegisterImageOptionsReader("firma-fut", imgOpt, bytes.NewReader(firmaPNG))
	if info == nil || !pdf.Ok() {
		pdf.ClearError()
		return false
	}
	scale := maxWidth / info.Width()
	if info.Height()*scale > maxHeight {
		scale = maxHeight / info.Height()
	}
	drawW := info.Width() * scale
	drawH := info.Height() * scale
	pdf.ImageOptions("firma-fut", x+(maxWidth-drawW)/2, y, drawW, drawH, false, imgOpt, 0, "")
	if !pdf.Ok() {
		pdf.ClearError()
		return false
	}
	return true
}

var mesesEs = [...]string{
	"enero", "febrero", "marzo", "abril", "mayo", "junio",
	"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre",
}

// formatFechaLarga convierte el timestamp ISO que entrega Expedientes
// Service (p. ej. "2026-09-13T14:07:38-05:00") a la misma forma en que un
// FUT en papel escribe la fecha: "13 de septiembre del 2026". Si el
// formato no calza (no debería pasar, pero un timestamp es dato de otro
// servicio), se devuelve tal cual en vez de fallar.
func formatFechaLarga(fechaISO string) string {
	fecha := strings.SplitN(fechaISO, "T", 2)[0]
	partes := strings.Split(fecha, "-")
	if len(partes) != 3 {
		return fechaISO
	}
	mes, err := strconv.Atoi(partes[1])
	if err != nil || mes < 1 || mes > 12 {
		return fechaISO
	}
	dia := strings.TrimPrefix(partes[2], "0")
	return fmt.Sprintf("%s de %s del %s", dia, mesesEs[mes-1], partes[0])
}

// wrapTextMM envuelve texto al ancho disponible (en mm, según la fuente ya
// seteada en pdf) respetando saltos de línea existentes — el equivalente en
// Go de wrapText() del frontend, pero midiendo con pdf.GetStringWidth en vez
// de ctx.measureText.
func wrapTextMM(pdf *fpdf.Fpdf, text string, maxWidth float64) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		var current string
		for _, word := range strings.Fields(paragraph) {
			attempt := word
			if current != "" {
				attempt = current + " " + word
			}
			if current != "" && pdf.GetStringWidth(attempt) > maxWidth {
				lines = append(lines, current)
				current = word
			} else {
				current = attempt
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	return lines
}

// buildFutCargoPdf reproduce el formato físico oficial del FUT de la
// institución (el mismo que ya se usa impreso, con el membrete, la tabla de
// derivación interna y el resumen final) con texto vectorial real — no una
// imagen rasterizada, a diferencia del PDF anterior generado con canvas+JPEG
// en el navegador. Usa fuentes núcleo (Helvetica) con traducción a cp1252
// para que tildes/eñes se vean bien sin tener que empaquetar una fuente
// TrueType.
func buildFutCargoPdf(datos futCargoData) ([]byte, error) {
	const marginL = 12.0
	const marginR = 12.0
	const marginTop = 14.0
	const pageW = 210.0
	contentW := pageW - marginL - marginR

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 14)
	pdf.SetMargins(marginL, marginTop, marginR)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetDrawColor(0, 0, 0)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()

	centered := func(x, y, w float64, text string) {
		pdf.SetXY(x, y)
		pdf.CellFormat(w, 4.2, tr(text), "", 0, "C", false, 0, "")
	}

	// ---- Membrete (institución / sumilla+título / N° de expediente) ----
	colInstX, colInstW := marginL, 70.0
	colTitleX, colTitleW := marginL+72, 70.0
	colRightX := marginL + 144
	colRightW := contentW - 144

	y := marginTop
	pdf.SetFont("Helvetica", "B", 7.5)
	centered(colInstX, y, colInstW, "INSTITUCIÓN EDUCATIVA")
	pdf.SetFont("Helvetica", "B", 18)
	centered(colInstX, y+5, colInstW, "TUNGASUCA")
	pdf.SetFont("Helvetica", "B", 7.5)
	centered(colInstX, y+13, colInstW, "UGEL 04 – CARABAYLLO")
	pdf.SetFont("Helvetica", "", 6.5)
	centered(colInstX, y+18, colInstW, "Av. Mariano Condorcanqui S/N")
	centered(colInstX, y+22, colInstW, "Urb. Tungasuca – Carabayllo")
	centered(colInstX, y+26, colInstW, "5441531")

	pdf.SetFont("Helvetica", "BU", 7.5)
	centered(colTitleX, y, colTitleW, "SUMILLA:")
	pdf.SetFont("Helvetica", "B", 8)
	centered(colTitleX, y+6, colTitleW, `"Año de la`)
	centered(colTitleX, y+10, colTitleW, "Esperanza y el")
	centered(colTitleX, y+14, colTitleW, "Fortalecimiento de")
	centered(colTitleX, y+18, colTitleW, `la Democracia"`)
	pdf.SetFont("Helvetica", "B", 10.5)
	centered(colTitleX, y+25, colTitleW, "FORMULARIO ÚNICO")
	centered(colTitleX, y+29.5, colTitleW, "DE TRÁMITE")

	pdf.SetDrawColor(0, 0, 0)
	officeLineW := colRightW * 0.75
	for _, ly := range []float64{y + 1, y + 6, y + 11} {
		pdf.Line(colRightX+colRightW-officeLineW, ly, colRightX+colRightW, ly)
	}
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetXY(colRightX, y+16)
	pdf.CellFormat(colRightW, 5, tr(fmt.Sprintf("N° %s", datos.Codigo)), "", 0, "R", false, 0, "")

	y += 38

	// ---- Destinatario ----
	pdf.SetFont("Helvetica", "BI", 9.5)
	pdf.SetXY(marginL, y)
	pdf.CellFormat(contentW, 5, tr(`SEÑORA DIRECTORA DE LA I.E "TUNGASUCA":`), "", 1, "L", false, 0, "")
	y += 8

	// ---- Filas de campos (línea + valor encima + etiqueta debajo, como
	// un formulario físico llenado a máquina) ----
	fieldsRow := func(y float64, ratios []float64, fields [][2]string) float64 {
		gap := 4.0
		totalRatio := 0.0
		for _, r := range ratios {
			totalRatio += r
		}
		usable := contentW - gap*float64(len(ratios)-1)
		x := marginL
		for i, field := range fields {
			w := usable * ratios[i] / totalRatio
			lineY := y + 5.5
			pdf.SetDrawColor(0, 0, 0)
			pdf.Line(x, lineY, x+w, lineY)

			pdf.SetFont("Helvetica", "", 10)
			pdf.SetXY(x, lineY-5)
			pdf.CellFormat(w, 5, tr(field[1]), "", 0, "L", false, 0, "")

			pdf.SetFont("Helvetica", "", 7.5)
			pdf.SetXY(x, lineY+1)
			pdf.CellFormat(w, 4, tr(field[0]), "", 0, "C", false, 0, "")

			x += w + gap
		}
		return y + 14
	}

	y = fieldsRow(y, []float64{1.8, 0.8, 0.8}, [][2]string{
		{"NOMBRES Y APELLIDOS", datos.Nombres},
		{"DNI", datos.Dni},
		{"TELEFONO", datos.Telefono},
	})
	y = fieldsRow(y, []float64{1.2, 0.65, 1.35}, [][2]string{
		{"DOMICILIO ACTUAL", datos.Domicilio},
		{"DISTRITO", datos.Distrito},
		{"CORREO ELECTRÓNICO", datos.Correo},
	})

	// ---- Fundamentación (párrafo sobre líneas rayadas, mínimo 4) ----
	pdf.SetFont("Helvetica", "BI", 9)
	pdf.SetXY(marginL, y)
	pdf.CellFormat(contentW, 5, tr("FUNDAMENTACIÓN DE LO QUE SOLICITA:"), "", 1, "L", false, 0, "")
	y += 5

	ruledParagraph := func(y float64, text string, minLines int) float64 {
		pdf.SetFont("Helvetica", "", 9.5)
		lines := wrapTextMM(pdf, text, contentW-2)
		if len(lines) < minLines {
			lines = append(lines, make([]string, minLines-len(lines))...)
		}
		for _, line := range lines {
			lineY := y + 5.2
			if line != "" {
				pdf.SetXY(marginL+1, y)
				pdf.CellFormat(contentW-2, 5, tr(line), "", 0, "L", false, 0, "")
			}
			pdf.Line(marginL, lineY, marginL+contentW, lineY)
			y = lineY + 1.8
		}
		return y + 3
	}
	y = ruledParagraph(y, datos.Fundamentacion, 4)

	// ---- Documento que se adjunta ----
	pdf.SetFont("Helvetica", "BI", 9)
	pdf.SetXY(marginL, y)
	pdf.CellFormat(contentW, 5, tr("DOCUMENTO QUE SE ADJUNTA:(SUSTENTATORIO DE SU SOLICITUD)"), "", 1, "L", false, 0, "")
	y += 5

	documentosTexto := strings.Join(datos.Documentos, "; ")
	y = ruledParagraph(y, documentosTexto, 2)

	// ---- Fecha y firma ----
	pdf.SetFont("Helvetica", "I", 9.5)
	pdf.SetXY(marginL, y+2)
	pdf.CellFormat(contentW*0.62, 5, tr(fmt.Sprintf("FECHA: Carabayllo, %s", formatFechaLarga(datos.FechaRegistro))), "", 0, "L", false, 0, "")

	sigX := marginL + contentW*0.65
	sigW := contentW - contentW*0.65
	sigLineY := y + 16
	if !embedFirma(pdf, normalizeFirmaPNG(datos.FirmaPNG), sigX, sigW, y, 14) {
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetXY(sigX, y+6)
		pdf.CellFormat(sigW, 4, tr("(sin firma capturada)"), "", 0, "C", false, 0, "")
	}
	pdf.SetDrawColor(0, 0, 0)
	pdf.Line(sigX, sigLineY, sigX+sigW, sigLineY)
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.SetXY(sigX, sigLineY+1)
	pdf.CellFormat(sigW, 4, tr("FIRMA"), "", 0, "C", false, 0, "")
	y = sigLineY + 8

	// ---- Tabla de derivación interna (uso de la institución) ----
	routingHeaders := []string{"N° EXPEDIENTE", "DIRECCION", "SUBDIRECCIÓN", "RECURSOS\nFINANCIEROS", "SECRETARIA U\nOTROS"}
	routingWidths := make([]float64, len(routingHeaders))
	for i := range routingWidths {
		routingWidths[i] = contentW / float64(len(routingHeaders))
	}
	headerH := 10.0
	dataH := 12.0

	x := marginL
	pdf.SetFont("Helvetica", "", 6.5)
	for i, header := range routingHeaders {
		pdf.Rect(x, y, routingWidths[i], headerH, "D")
		pdf.SetXY(x, y+1)
		pdf.MultiCell(routingWidths[i], 3, tr(header), "", "C", false)
		x += routingWidths[i]
	}
	y += headerH

	x = marginL
	pdf.SetFont("Helvetica", "", 9)
	for i, w := range routingWidths {
		pdf.Rect(x, y, w, dataH, "D")
		if i == 0 {
			pdf.SetXY(x, y+dataH/2-2)
			pdf.CellFormat(w, 4, tr(datos.Codigo), "", 0, "C", false, 0, "")
		}
		x += w
	}
	y += dataH

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetXY(marginL, y+1)
	pdf.CellFormat(contentW, 5, tr("FECHA: ________________________________________________"), "", 1, "L", false, 0, "")
	y += 8

	// ---- Separador punteado ----
	pdf.SetDashPattern([]float64{1, 1}, 0)
	pdf.Line(marginL, y, marginL+contentW, y)
	pdf.SetDashPattern(nil, 0)
	y += 6

	// ---- Título + resumen final (igual al que sella/verifica Secretaría) ----
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetXY(marginL, y)
	pdf.CellFormat(contentW, 6, tr("FORMULARIO ÚNICO DE TRÁMITE (FUT)"), "", 1, "C", false, 0, "")
	y += 8

	summaryRows := [][2]string{
		{"APELLIDOS Y NOMBRES", datos.Nombres},
		{"ASUNTO", datos.Sumilla},
		{"FECHA", fmt.Sprintf("Carabayllo, %s", formatFechaLarga(datos.FechaRegistro))},
		{"N° EXPEDIENTE", datos.Codigo},
		{"N° FOLIOS", datos.Folios},
	}
	labelW := contentW * 0.3
	rowH := 8.0
	for _, row := range summaryRows {
		pdf.Rect(marginL, y, labelW, rowH, "D")
		pdf.Rect(marginL+labelW, y, contentW-labelW, rowH, "D")
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetXY(marginL+2, y+rowH/2-2.5)
		pdf.CellFormat(labelW-4, 5, tr(row[0]), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9.5)
		pdf.SetXY(marginL+labelW+2, y+rowH/2-2.5)
		pdf.CellFormat(contentW-labelW-4, 5, tr(row[1]), "", 0, "L", false, 0, "")
		y += rowH
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
