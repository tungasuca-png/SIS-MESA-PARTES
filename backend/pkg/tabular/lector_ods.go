package tabular

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
)

// leerODS lee hojas OpenDocument (content.xml dentro del zip) en streaming.
// Respeta las celdas/filas repetidas (table:number-*-repeated) con topes para
// que una fila "repetida un millón de veces" no agote memoria.
func leerODS(contenido []byte, op Opciones) (*Tabla, error) {
	zr, err := zip.NewReader(bytes.NewReader(contenido), int64(len(contenido)))
	if err != nil {
		return nil, errorf(ErrDanado, "el archivo .ods está dañado")
	}
	var content *zip.File
	for _, f := range zr.File {
		if f.Name == "content.xml" {
			content = f
		}
	}
	if content == nil {
		return nil, errorf(ErrDanado, "el archivo .ods no tiene contenido")
	}
	if content.UncompressedSize64 > maxXMLDescomprimido {
		return nil, errorf(ErrGrande, "el contenido del .ods es demasiado grande")
	}
	rc, err := content.Open()
	if err != nil {
		return nil, errorf(ErrDanado, "el archivo .ods está dañado")
	}
	defer func() { _ = rc.Close() }()

	hojas, filasPorHoja, err := parsearODS(io.LimitReader(rc, maxXMLDescomprimido), op.Limites)
	if err != nil {
		return nil, err
	}
	if len(hojas) == 0 {
		return nil, errorf(ErrEstructura, "el archivo no tiene hojas")
	}
	hoja, err := elegirHoja(hojas, op.Hoja)
	if err != nil {
		return nil, err
	}
	return armarTabla(filasPorHoja[hoja], op, Origen{Hojas: hojas, Hoja: hoja})
}

func atributo(e xml.StartElement, local string) string {
	for _, a := range e.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func repeticion(e xml.StartElement, local string, tope int) int {
	n, err := strconv.Atoi(atributo(e, local))
	if err != nil || n < 1 {
		return 1
	}
	return min(n, tope)
}

func parsearODS(r io.Reader, lim Limites) ([]string, map[string][]filaCruda, error) {
	dec := xml.NewDecoder(r)
	var hojas []string
	filas := map[string][]filaCruda{}
	var hoja string
	var fila []string
	var texto strings.Builder
	enCelda, enParrafo := false, false
	repCelda, repFila, numero := 1, 1, 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, errorf(ErrDanado, "el contenido del .ods no es XML válido")
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "table":
				hoja = atributo(e, "name")
				hojas = append(hojas, hoja)
				numero = 0
			case "table-row":
				fila = fila[:0]
				repFila = repeticion(e, "number-rows-repeated", lim.MaxFilas)
			case "table-cell", "covered-table-cell":
				enCelda = true
				texto.Reset()
				repCelda = repeticion(e, "number-columns-repeated", lim.MaxColumnas)
			case "p":
				if enCelda && texto.Len() > 0 {
					texto.WriteString("\n")
				}
				enParrafo = true
			case "s": // espacios comprimidos
				if enParrafo {
					texto.WriteString(strings.Repeat(" ", repeticion(e, "c", 100)))
				}
			}
		case xml.CharData:
			if enCelda && enParrafo {
				texto.Write(e)
			}
		case xml.EndElement:
			switch e.Name.Local {
			case "p":
				enParrafo = false
			case "table-cell", "covered-table-cell":
				enCelda = false
				for i := 0; i < repCelda && len(fila) < lim.MaxColumnas; i++ {
					fila = append(fila, texto.String())
				}
			case "table-row":
				celdas := recortarFinal(fila)
				for i := 0; i < repFila; i++ {
					numero++
					if len(celdas) > 0 {
						filas[hoja] = append(filas[hoja], filaCruda{numero: numero, celdas: append([]string(nil), celdas...)})
						if len(filas[hoja]) > lim.MaxFilas+filasBuscarEncabezado {
							return nil, nil, errorf(ErrDemasiadas, "la hoja %q tiene más de %d registros", hoja, lim.MaxFilas)
						}
					}
				}
			}
		}
	}
	return hojas, filas, nil
}
