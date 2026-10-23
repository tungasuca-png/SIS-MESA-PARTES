package tabular

import (
	"archive/zip"
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Firmas de archivo.
var (
	firmaZIP  = []byte("PK\x03\x04")
	firmaOLE2 = []byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1") // .xls/.doc binarios (Excel 97-2003)
	bomUTF8   = []byte("\xEF\xBB\xBF")
)

// Detectar identifica el formato por el CONTENIDO (firma y estructura). La
// extensión solo sirve para avisar discrepancias o desempatar entre texto
// delimitado y otros textos. Devuelve el formato y avisos.
func Detectar(nombre string, contenido []byte) (string, []string, error) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(nombre)), ".")
	if len(contenido) == 0 {
		return "", nil, errorf(ErrVacio, "el archivo está vacío")
	}
	var formato string
	switch {
	case bytes.HasPrefix(contenido, firmaZIP):
		f, err := formatoZip(contenido)
		if err != nil {
			return "", nil, err
		}
		formato = f
	case bytes.HasPrefix(contenido, firmaOLE2):
		return "", nil, errorf(ErrNoSoportado, "es un Excel 97-2003 binario (.xls); ábralo en Excel o LibreOffice y guárdelo como .xlsx o .csv")
	default:
		texto := bytes.TrimPrefix(contenido, bomUTF8)
		if pareceBinario(texto) {
			if ext == "xlsb" {
				return "", nil, errorf(ErrNoSoportado, "el formato .xlsb no está soportado; guárdelo como .xlsx o .csv")
			}
			return "", nil, errorf(ErrNoSoportado, "el archivo no es una tabla de datos reconocible")
		}
		formato = formatoTexto(texto, ext)
	}
	var avisos []string
	if esperado, ok := extensionEsperada[ext]; ok && esperado != formato {
		avisos = append(avisos, "la extensión ."+ext+" no corresponde al contenido; se leyó como "+nombreFormato[formato])
	}
	return formato, avisos, nil
}

var extensionEsperada = map[string]string{
	"xlsx": FormatoXLSX, "xlsm": FormatoXLSX, "xltx": FormatoXLSX, "ods": FormatoODS,
	// .xls binario (OLE2) se rechaza antes; si llega aquí, el contenido es otro
	// (típicamente HTML exportado por un sistema web) y se avisa.
	"xls": "xls-binario",
	"htm": FormatoHTML, "html": FormatoHTML, "csv": FormatoCSV, "tsv": FormatoCSV, "txt": FormatoCSV,
	"json": FormatoJSON, "jsonl": FormatoJSONL, "ndjson": FormatoJSONL, "xml": FormatoXML,
}

var nombreFormato = map[string]string{
	FormatoXLSX: "Excel (.xlsx)", FormatoODS: "OpenDocument (.ods)", FormatoHTML: "tabla HTML",
	FormatoCSV: "texto delimitado", FormatoJSON: "JSON", FormatoJSONL: "JSON Lines", FormatoXML: "XML",
}

// formatoZip distingue OOXML (Excel) de OpenDocument por su contenido.
func formatoZip(contenido []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(contenido), int64(len(contenido)))
	if err != nil {
		return "", errorf(ErrDanado, "el archivo comprimido está dañado")
	}
	for _, f := range zr.File {
		switch {
		case f.Name == "xl/workbook.xml" || f.Name == "xl/workbook.bin":
			if f.Name == "xl/workbook.bin" {
				return "", errorf(ErrNoSoportado, "el formato .xlsb no está soportado; guárdelo como .xlsx o .csv")
			}
			return FormatoXLSX, nil
		case f.Name == "mimetype":
			rc, err := f.Open()
			if err != nil {
				return "", errorf(ErrDanado, "el archivo comprimido está dañado")
			}
			mime, _ := io.ReadAll(io.LimitReader(rc, 100))
			_ = rc.Close()
			if strings.TrimSpace(string(mime)) == "application/vnd.oasis.opendocument.spreadsheet" {
				return FormatoODS, nil
			}
		}
	}
	return "", errorf(ErrNoSoportado, "el archivo comprimido no es una hoja de cálculo (.xlsx u .ods)")
}

// pareceBinario: bytes NUL o mucho texto no imprimible en el inicio. UTF-16
// (con BOM) se acepta: lo decodifica el lector de texto.
func pareceBinario(b []byte) bool {
	if bytes.HasPrefix(b, []byte("\xFF\xFE")) || bytes.HasPrefix(b, []byte("\xFE\xFF")) {
		return false
	}
	muestra := b
	if len(muestra) > 4096 {
		muestra = muestra[:4096]
	}
	if bytes.IndexByte(muestra, 0) >= 0 {
		return true
	}
	raros := 0
	for _, c := range muestra {
		if c < 0x09 || (c > 0x0D && c < 0x20) {
			raros++
		}
	}
	return raros*20 > len(muestra)
}

// formatoTexto: JSON, JSON Lines, XML, HTML o texto delimitado.
func formatoTexto(b []byte, ext string) string {
	t := bytes.TrimLeft(b, " \t\r\n")
	minus := strings.ToLower(string(t[:min(len(t), 2048)]))
	switch {
	case len(t) > 0 && (t[0] == '{' || t[0] == '['):
		if t[0] == '{' && (ext == "jsonl" || ext == "ndjson" || esJSONL(t)) {
			return FormatoJSONL
		}
		return FormatoJSON
	case strings.HasPrefix(minus, "<?xml") && !strings.Contains(minus, "<html"):
		return FormatoXML
	case strings.Contains(minus, "<table") || strings.Contains(minus, "<html") || strings.HasPrefix(minus, "<!doctype html"):
		return FormatoHTML
	case strings.HasPrefix(minus, "<") && utf8.Valid(t):
		return FormatoXML
	}
	return FormatoCSV
}

// esJSONL: dos o más líneas que empiezan con "{" y la primera cierra en la misma línea.
func esJSONL(t []byte) bool {
	lineas := bytes.Split(t, []byte("\n"))
	n := 0
	for _, l := range lineas {
		l = bytes.TrimSpace(l)
		if len(l) == 0 {
			continue
		}
		if l[0] != '{' || l[len(l)-1] != '}' {
			return false
		}
		n++
		if n >= 2 {
			return true
		}
	}
	return false
}

// Leer detecta el formato y devuelve la tabla común.
func Leer(nombre string, contenido []byte, op Opciones) (*Tabla, error) {
	op.Limites = op.Limites.conDefectos()
	if int64(len(contenido)) > op.Limites.MaxBytes {
		return nil, errorf(ErrGrande, "el archivo supera el máximo de %d KB", op.Limites.MaxBytes/1024)
	}
	formato, avisos, err := Detectar(nombre, contenido)
	if err != nil {
		return nil, err
	}
	var t *Tabla
	switch formato {
	case FormatoXLSX:
		t, err = leerXLSX(contenido, op)
	case FormatoODS:
		t, err = leerODS(contenido, op)
	case FormatoHTML:
		t, err = leerHTML(contenido, op)
	case FormatoCSV:
		t, err = leerCSV(contenido, op)
	case FormatoJSON:
		t, err = leerJSON(contenido, op)
	case FormatoJSONL:
		t, err = leerJSONL(contenido, op)
	case FormatoXML:
		t, err = leerXML(contenido, op)
	}
	if err != nil {
		return nil, err
	}
	t.Origen.Formato = formato
	t.Origen.ExtensionDeclarada = strings.TrimPrefix(strings.ToLower(filepath.Ext(nombre)), ".")
	t.Origen.Avisos = append(avisos, t.Origen.Avisos...)
	if err := t.validarLimites(op.Limites); err != nil {
		return nil, err
	}
	if len(t.Filas) == 0 {
		return t, errorf(ErrVacio, "no se encontraron registros con datos")
	}
	return t, nil
}

func (t *Tabla) validarLimites(l Limites) error {
	if len(t.Encabezados) > l.MaxColumnas {
		return errorf(ErrDemasiadasCols, "el archivo tiene %d columnas; el máximo es %d", len(t.Encabezados), l.MaxColumnas)
	}
	if len(t.Filas) > l.MaxFilas {
		return errorf(ErrDemasiadas, "el archivo tiene más de %d registros", l.MaxFilas)
	}
	for _, f := range t.Filas {
		for _, v := range f.Valores {
			if utf8.RuneCountInString(v) > l.MaxRunesCelda {
				return errorf(ErrEstructura, "la fila %d tiene un valor de más de %d caracteres", f.Numero, l.MaxRunesCelda)
			}
		}
	}
	return nil
}
