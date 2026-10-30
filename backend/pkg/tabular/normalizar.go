package tabular

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var quitarMarcas = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// SinTildes quita los diacríticos ("Sección" -> "Seccion", "Ñ" -> "N").
func SinTildes(s string) string {
	out, _, err := transform.String(quitarMarcas, s)
	if err != nil {
		return s
	}
	return out
}

// NormalizarEncabezado lleva un encabezado a una forma comparable:
// mayúsculas, sin tildes, sin "(S)"/"(AS)" de plural, con cualquier signo
// (punto, guion, barra, dos puntos, °) convertido en espacio y los espacios
// colapsados. "Nro. Doc", "NRO-DOC" y "nro  doc" dan "NRO DOC".
func NormalizarEncabezado(s string) string {
	s = strings.ToUpper(SinTildes(s))
	for _, plural := range []string{"(S)", "(ES)", "(AS)", "(OS)"} {
		s = strings.ReplaceAll(s, plural, "")
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// palabrasVacias no aportan significado al comparar encabezados.
var palabrasVacias = map[string]bool{"DE": true, "DEL": true, "LA": true, "EL": true, "LOS": true, "LAS": true, "Y": true, "EN": true}

// Tokens: palabras significativas de un encabezado normalizado.
func Tokens(normalizado string) []string {
	var out []string
	for _, t := range strings.Fields(normalizado) {
		if !palabrasVacias[t] {
			out = append(out, t)
		}
	}
	return out
}
