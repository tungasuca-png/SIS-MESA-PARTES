package logic

import (
	"bytes"
	"fmt"
	"image/png"
	"regexp"
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
// lo que ya vive en columnas propias del expediente (asunto/código/fecha) y
// lo que se trae de Documentos Service (lista de adjuntos reales + firma).
type futCargoData struct {
	datosSolicitanteFut
	Sumilla    string
	Codigo     string
	Fecha      string
	Documentos []string
	FirmaPNG   []byte // nil si no se subió firma
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
func embedFirma(pdf *fpdf.Fpdf, firmaPNG []byte, firmaX, firmaBoxWidth, y float64) (embebida bool) {
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
	maxH := 16.0
	scale := firmaBoxWidth / info.Width()
	if info.Height()*scale > maxH {
		scale = maxH / info.Height()
	}
	drawW := info.Width() * scale
	drawH := info.Height() * scale
	pdf.ImageOptions("firma-fut", firmaX+(firmaBoxWidth-drawW)/2, y, drawW, drawH, false, imgOpt, 0, "")
	if !pdf.Ok() {
		pdf.ClearError()
		return false
	}
	return true
}

var (
	colLabel = [3]int{104, 118, 159} // #68769F
	colValue = [3]int{27, 37, 89}    // #1B2559
	colLine  = [3]int{233, 237, 247} // #E9EDF7
	colMuted = [3]int{160, 168, 192} // #A0A8C0
)

// buildFutCargoPdf dibuja el mismo cargo que ya se ve en pantalla
// (FutDigital.jsx), pero con texto vectorial real (glifos de fuente, no una
// imagen rasterizada) — por eso se ve nítido sin importar el zoom o la
// impresión, a diferencia del PDF anterior generado con canvas+JPEG en el
// navegador. Usa fuentes núcleo (Helvetica) con traducción a cp1252 para que
// tildes/eñes se vean bien sin tener que empaquetar una fuente TrueType.
func buildFutCargoPdf(datos futCargoData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.SetMargins(15, 15, 15)
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	contentWidth := 210.0 - 30.0

	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(colLabel[0], colLabel[1], colLabel[2])
	pdf.CellFormat(0, 5, tr("I.E. TUNGASUCA"), "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "B", 15)
	pdf.SetTextColor(colValue[0], colValue[1], colValue[2])
	pdf.CellFormat(0, 8, tr("Formulario Único de Trámite (FUT)"), "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(0, 7, tr(`SEÑORA DIRECTORA DE LA I.E. "TUNGASUCA":`), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	sectionTitle := func(text string) {
		pdf.SetFont("Helvetica", "B", 8.5)
		pdf.SetTextColor(colLabel[0], colLabel[1], colLabel[2])
		pdf.CellFormat(0, 6, tr(strings.ToUpper(text)), "", 1, "L", false, 0, "")
	}

	fieldGrid := func(fields [][2]string) {
		colGap := 6.0
		colWidth := (contentWidth - colGap) / 2
		for i := 0; i < len(fields); i += 2 {
			row := fields[i:min(i+2, len(fields))]
			startY := pdf.GetY()
			for col, field := range row {
				x := 15 + float64(col)*(colWidth+colGap)
				pdf.SetXY(x, startY)
				pdf.SetFont("Helvetica", "B", 7)
				pdf.SetTextColor(colLabel[0], colLabel[1], colLabel[2])
				pdf.CellFormat(colWidth, 4, tr(strings.ToUpper(field[0])), "", 0, "L", false, 0, "")
				pdf.SetXY(x, startY+4.5)
				pdf.SetFont("Helvetica", "", 10)
				pdf.SetTextColor(colValue[0], colValue[1], colValue[2])
				value := field[1]
				if value == "" {
					value = "—"
				}
				pdf.CellFormat(colWidth, 5, tr(value), "", 0, "L", false, 0, "")
				pdf.SetDrawColor(colLine[0], colLine[1], colLine[2])
				pdf.Line(x, startY+10.5, x+colWidth, startY+10.5)
			}
			pdf.SetXY(15, startY+13)
		}
	}

	sectionTitle("Datos del solicitante")
	fieldGrid([][2]string{
		{"Nombres y apellidos", datos.Nombres},
		{"DNI", datos.Dni},
		{"Teléfono", datos.Telefono},
		{"Domicilio actual", datos.Domicilio},
		{"Distrito", datos.Distrito},
		{"Correo electrónico", datos.Correo},
	})
	pdf.Ln(2)

	sectionTitle("Asunto")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(colValue[0], colValue[1], colValue[2])
	pdf.MultiCell(contentWidth, 5, tr(datos.Sumilla), "", "L", false)
	pdf.Ln(1)

	sectionTitle("Fundamentación de lo que solicita")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(colValue[0], colValue[1], colValue[2])
	if datos.Fundamentacion != "" {
		pdf.MultiCell(contentWidth, 5.5, tr(datos.Fundamentacion), "", "L", false)
	} else {
		pdf.SetFont("Helvetica", "I", 10)
		pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
		pdf.CellFormat(0, 6, "—", "", 1, "L", false, 0, "")
	}
	pdf.Ln(1)

	sectionTitle("Documento que se adjunta (sustentatorio de su solicitud)")
	pdf.SetFont("Helvetica", "", 9.5)
	pdf.SetTextColor(colValue[0], colValue[1], colValue[2])
	if len(datos.Documentos) == 0 {
		pdf.SetFont("Helvetica", "I", 9.5)
		pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
		pdf.CellFormat(0, 5, "Ninguno", "", 1, "L", false, 0, "")
	} else {
		for _, nombre := range datos.Documentos {
			pdf.SetFont("Helvetica", "", 9.5)
			pdf.SetTextColor(colValue[0], colValue[1], colValue[2])
			pdf.MultiCell(contentWidth, 5, tr(fmt.Sprintf("•  %s", nombre)), "", "L", false)
		}
	}
	pdf.SetFont("Helvetica", "", 8.5)
	pdf.SetTextColor(colLabel[0], colLabel[1], colLabel[2])
	pdf.CellFormat(0, 5, tr(fmt.Sprintf("N° de folios: %s", datos.Folios)), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	fechaFirmaY := pdf.GetY()
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(colValue[0], colValue[1], colValue[2])
	pdf.SetXY(15, fechaFirmaY)
	pdf.CellFormat(contentWidth/2, 5, tr(fmt.Sprintf("Fecha: Carabayllo, %s", datos.Fecha)), "", 0, "L", false, 0, "")

	firmaBoxWidth := 55.0
	firmaX := 15 + contentWidth - firmaBoxWidth
	firmaEmbebida := embedFirma(pdf, normalizeFirmaPNG(datos.FirmaPNG), firmaX, firmaBoxWidth, fechaFirmaY)
	if !firmaEmbebida {
		pdf.SetXY(firmaX, fechaFirmaY+8)
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(colMuted[0], colMuted[1], colMuted[2])
		pdf.CellFormat(firmaBoxWidth, 5, tr("Firma no capturada"), "", 0, "L", false, 0, "")
	}
	pdf.SetDrawColor(colLine[0], colLine[1], colLine[2])
	pdf.Line(firmaX, fechaFirmaY+17, firmaX+firmaBoxWidth, fechaFirmaY+17)
	pdf.SetXY(firmaX, fechaFirmaY+18)
	pdf.SetFont("Helvetica", "B", 6.5)
	pdf.SetTextColor(colLabel[0], colLabel[1], colLabel[2])
	pdf.CellFormat(firmaBoxWidth, 4, tr("FIRMA DEL SOLICITANTE"), "", 0, "L", false, 0, "")

	pdf.SetXY(15, fechaFirmaY+26)
	pdf.SetDrawColor(colLine[0], colLine[1], colLine[2])
	pdf.Line(15, fechaFirmaY+26, 15+contentWidth, fechaFirmaY+26)
	pdf.SetXY(15, fechaFirmaY+30)

	sectionTitle("Cargo de recepción")
	resumenRow := func(label, value string) {
		y := pdf.GetY()
		labelWidth := contentWidth * 0.32
		pdf.SetDrawColor(colLine[0], colLine[1], colLine[2])
		pdf.Rect(15, y, labelWidth, 8, "D")
		pdf.Rect(15+labelWidth, y, contentWidth-labelWidth, 8, "D")
		pdf.SetXY(15+2, y+1.5)
		pdf.SetFont("Helvetica", "B", 7.5)
		pdf.SetTextColor(colLabel[0], colLabel[1], colLabel[2])
		pdf.CellFormat(labelWidth-4, 5, tr(strings.ToUpper(label)), "", 0, "L", false, 0, "")
		pdf.SetXY(15+labelWidth+2, y+1.5)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(colValue[0], colValue[1], colValue[2])
		pdf.CellFormat(contentWidth-labelWidth-4, 5, tr(value), "", 0, "L", false, 0, "")
		pdf.SetXY(15, y+8)
	}
	resumenRow("Apellidos y nombres", datos.Nombres)
	resumenRow("Asunto", datos.Sumilla)
	resumenRow("Fecha", fmt.Sprintf("Carabayllo, %s", datos.Fecha))
	resumenRow("N° expediente", datos.Codigo)
	resumenRow("N° folios", datos.Folios)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
