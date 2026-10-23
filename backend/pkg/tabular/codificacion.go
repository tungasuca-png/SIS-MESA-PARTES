package tabular

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// Codificaciones admitidas para texto (CSV/TXT/HTML).
const (
	CodUTF8        = "utf-8"
	CodUTF16       = "utf-16"
	CodWindows1252 = "windows-1252"
	CodLatin1      = "iso-8859-1"
)

var codificaciones = map[string]encoding.Encoding{
	CodUTF16:       unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
	CodWindows1252: charmap.Windows1252,
	CodLatin1:      charmap.ISO8859_1,
}

// decodificar convierte a UTF-8. Sin codificación indicada: BOM UTF-8/UTF-16
// o UTF-8 válido; si no es UTF-8 válido se asume Windows-1252 (Excel en
// Windows) y se avisa, para que el usuario pueda elegir otra.
func decodificar(b []byte, pedida string) (string, string, []string, error) {
	pedida = strings.ToLower(strings.TrimSpace(pedida))
	switch {
	case pedida == "" || pedida == "auto":
		switch {
		case bytes.HasPrefix(b, bomUTF8):
			return string(b[len(bomUTF8):]), CodUTF8, nil, nil
		case bytes.HasPrefix(b, []byte("\xFF\xFE")) || bytes.HasPrefix(b, []byte("\xFE\xFF")):
			return decodificarCon(b, CodUTF16)
		case utf8.Valid(b):
			return string(b), CodUTF8, nil, nil
		}
		s, cod, _, err := decodificarCon(b, CodWindows1252)
		return s, cod, []string{"el texto no es UTF-8: se leyó como Windows-1252; si ve caracteres extraños, elija otra codificación"}, err
	case pedida == CodUTF8 || pedida == "utf8":
		b = bytes.TrimPrefix(b, bomUTF8)
		if !utf8.Valid(b) {
			return "", "", nil, errorf(ErrOpcion, "el archivo no es UTF-8 válido; elija otra codificación")
		}
		return string(b), CodUTF8, nil, nil
	}
	return decodificarCon(b, pedida)
}

func decodificarCon(b []byte, cod string) (string, string, []string, error) {
	enc, ok := codificaciones[cod]
	if !ok {
		return "", "", nil, errorf(ErrOpcion, "codificación no admitida: use utf-8, utf-16, windows-1252 o iso-8859-1")
	}
	out, err := enc.NewDecoder().Bytes(b)
	if err != nil {
		return "", "", nil, errorf(ErrDanado, "el texto no se pudo decodificar como %s", cod)
	}
	return strings.TrimPrefix(string(out), string(rune(0xFEFF))), cod, nil, nil
}

// codificar convierte UTF-8 a la codificación de salida (exportación).
func codificar(s string, cod string) ([]byte, error) {
	switch strings.ToLower(cod) {
	case "", CodUTF8:
		return []byte(s), nil
	case "utf-8-bom":
		return append(append([]byte{}, bomUTF8...), s...), nil
	case CodWindows1252, CodLatin1:
		enc := codificaciones[strings.ToLower(cod)]
		out, err := encoding.ReplaceUnsupported(enc.NewEncoder()).Bytes([]byte(s))
		if err != nil {
			return nil, errorf(ErrOpcion, "no se pudo convertir a %s", cod)
		}
		return out, nil
	}
	return nil, errorf(ErrOpcion, "codificación de salida no admitida: use utf-8, utf-8-bom, windows-1252 o iso-8859-1")
}
