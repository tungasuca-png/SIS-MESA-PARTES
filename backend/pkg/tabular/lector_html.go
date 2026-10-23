package tabular

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// leerHTML lee las tablas <table> de un HTML (muchos sistemas exportan "Excel"
// como HTML con extensión .xls). Solo se toma el TEXTO de las celdas: no se
// ejecutan scripts ni se siguen enlaces o recursos externos. Cada tabla es una
// "hoja" seleccionable; por defecto, la de más filas.
func leerHTML(contenido []byte, op Opciones) (*Tabla, error) {
	texto, cod, avisos, err := decodificar(contenido, op.Codificacion)
	if err != nil {
		return nil, err
	}
	tablas, err := tablasHTML(texto, op.Limites)
	if err != nil {
		return nil, err
	}
	if len(tablas) == 0 {
		return nil, errorf(ErrEstructura, "el HTML no contiene tablas de datos")
	}
	nombres := make([]string, len(tablas))
	elegida := 0
	for i, t := range tablas {
		nombres[i] = fmt.Sprintf("Tabla %d", i+1)
		if len(t) > len(tablas[elegida]) {
			elegida = i
		}
	}
	if op.Hoja != "" {
		hoja, err := elegirHoja(nombres, op.Hoja)
		if err != nil {
			return nil, err
		}
		elegida, _ = strconv.Atoi(strings.TrimPrefix(hoja, "Tabla "))
		elegida--
	}
	return armarTabla(tablas[elegida], op, Origen{Hojas: nombres, Hoja: nombres[elegida], Codificacion: cod, Avisos: avisos})
}

// tablasHTML extrae las filas de cada tabla de primer nivel. Las tablas
// anidadas se aplanan como texto de la celda que las contiene; colspan agrega
// celdas vacías para no desalinear las columnas siguientes.
func tablasHTML(texto string, lim Limites) ([][]filaCruda, error) {
	z := html.NewTokenizer(strings.NewReader(texto))
	z.SetMaxBuf(maxXMLDescomprimido)
	var tablas [][]filaCruda
	var fila []string
	var celda strings.Builder
	profundidad, enCelda, ignorar, colspan := 0, false, 0, 1
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return tablas, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			nombre, conAttr := z.TagName()
			tag := string(nombre)
			switch tag {
			case "script", "style":
				if tt == html.StartTagToken {
					ignorar++
				}
			case "table":
				profundidad++
				if profundidad == 1 {
					tablas = append(tablas, nil)
				}
			case "tr":
				if profundidad == 1 {
					fila = nil
				}
			case "td", "th":
				if profundidad == 1 {
					enCelda = true
					celda.Reset()
					colspan = 1
					for conAttr {
						var k, v []byte
						k, v, conAttr = z.TagAttr()
						if string(k) == "colspan" {
							if n, err := strconv.Atoi(string(v)); err == nil && n > 1 {
								colspan = min(n, lim.MaxColumnas)
							}
						}
					}
				}
			case "br":
				if enCelda {
					celda.WriteString(" ")
				}
			}
		case html.TextToken:
			if enCelda && ignorar == 0 {
				celda.Write(z.Text())
			}
		case html.EndTagToken:
			nombre, _ := z.TagName()
			switch string(nombre) {
			case "script", "style":
				if ignorar > 0 {
					ignorar--
				}
			case "td", "th":
				if profundidad == 1 && enCelda {
					enCelda = false
					fila = append(fila, strings.Join(strings.Fields(celda.String()), " "))
					for i := 1; i < colspan; i++ {
						fila = append(fila, "")
					}
				}
			case "tr":
				if profundidad == 1 && len(tablas) > 0 {
					t := &tablas[len(tablas)-1]
					*t = append(*t, filaCruda{numero: len(*t) + 1, celdas: fila})
					if len(*t) > lim.MaxFilas+filasBuscarEncabezado {
						return nil, errorf(ErrDemasiadas, "la tabla tiene más de %d registros", lim.MaxFilas)
					}
				}
			case "table":
				if profundidad > 0 {
					profundidad--
				}
			}
		}
	}
}
