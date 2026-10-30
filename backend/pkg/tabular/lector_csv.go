package tabular

import (
	"encoding/csv"
	"errors"
	"io"
	"strings"
)

// Separadores admitidos (nombre legible -> rune).
var separadores = map[string]rune{",": ',', ";": ';', "\t": '\t', "tab": '\t', "|": '|'}

// leerCSV lee texto delimitado (.csv/.tsv/.txt). Detecta el separador por
// consistencia entre líneas si no se indica.
func leerCSV(contenido []byte, op Opciones) (*Tabla, error) {
	texto, cod, avisos, err := decodificar(contenido, op.Codificacion)
	if err != nil {
		return nil, err
	}
	sep := ','
	nombreSep := op.Separador
	if nombreSep != "" {
		r, ok := separadores[strings.ToLower(nombreSep)]
		if !ok {
			return nil, errorf(ErrOpcion, "separador no admitido: use coma, punto y coma, tabulador o barra")
		}
		sep = r
	} else {
		sep = detectarSeparador(texto)
	}
	if sep == '\t' {
		nombreSep = "tab"
	} else {
		nombreSep = string(sep)
	}

	r := csv.NewReader(strings.NewReader(texto))
	r.Comma = sep
	r.FieldsPerRecord = -1 // filas de distinto largo se informan, no se rechazan
	r.LazyQuotes = true
	r.ReuseRecord = false
	var crudas []filaCruda
	tope := op.Limites.MaxFilas + filasBuscarEncabezado + 1
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			line, _ := r.FieldPos(0)
			return nil, errorf(ErrDanado, "el texto delimitado está mal formado cerca de la línea %d", line)
		}
		line, _ := r.FieldPos(0)
		if len(rec) > op.Limites.MaxColumnas {
			return nil, errorf(ErrDemasiadasCols, "la línea %d tiene más de %d columnas", line, op.Limites.MaxColumnas)
		}
		crudas = append(crudas, filaCruda{numero: line, celdas: rec})
		if !vacia(rec) {
			tope--
			if tope == 0 {
				break
			}
		}
	}
	return armarTabla(crudas, op, Origen{Separador: nombreSep, Codificacion: cod, Avisos: avisos})
}

// detectarSeparador elige, entre los candidatos, el que divide las primeras
// líneas en la misma cantidad (> 1) de campos con más frecuencia.
func detectarSeparador(texto string) rune {
	lineas := strings.Split(texto, "\n")
	if len(lineas) > 30 {
		lineas = lineas[:30]
	}
	mejor, mejorPuntaje := ',', -1
	for _, cand := range []rune{';', ',', '\t', '|'} {
		conteos := map[int]int{}
		for _, l := range lineas {
			if strings.TrimSpace(l) == "" {
				continue
			}
			if n := strings.Count(l, string(cand)); n > 0 {
				conteos[n+1]++
			}
		}
		for campos, veces := range conteos {
			if puntaje := veces*100 + campos; campos > 1 && puntaje > mejorPuntaje {
				mejor, mejorPuntaje = cand, puntaje
			}
		}
	}
	return mejor
}
