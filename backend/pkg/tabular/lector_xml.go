package tabular

import (
	"encoding/xml"
	"io"
	"sort"
	"strings"
)

// nodo XML genérico (solo lo necesario para tabular).
type nodo struct {
	nombre string
	attrs  [][2]string
	texto  strings.Builder
	hijos  []*nodo
}

// leerXML toma como registros los elementos que se REPITEN bajo un mismo
// padre (p. ej. /estudiantes/estudiante). Cada registro aporta sus atributos
// ("@tipo") y el texto de sus elementos hijos (rutas "domicilio.distrito").
// encoding/xml de Go no expande entidades externas ni descarga DTD.
func leerXML(contenido []byte, op Opciones) (*Tabla, error) {
	texto, _, _, err := decodificar(contenido, op.Codificacion)
	if err != nil {
		return nil, err
	}
	raiz, err := parsearXML(strings.NewReader(texto), op.Limites.MaxProfundidad)
	if err != nil {
		return nil, err
	}
	rutas := map[string][]*nodo{}
	buscarRepetidos(raiz, "/"+raiz.nombre, rutas)
	if len(rutas) == 0 {
		return nil, errorf(ErrEstructura, "el XML no tiene elementos repetidos que puedan leerse como registros")
	}
	nombres := make([]string, 0, len(rutas))
	for r := range rutas {
		nombres = append(nombres, r)
	}
	sort.Slice(nombres, func(i, j int) bool {
		if len(rutas[nombres[i]]) != len(rutas[nombres[j]]) {
			return len(rutas[nombres[i]]) > len(rutas[nombres[j]])
		}
		return nombres[i] < nombres[j]
	})
	ruta := nombres[0]
	if op.Ruta != "" {
		if _, ok := rutas[op.Ruta]; !ok {
			return nil, errorf(ErrOpcion, "la ruta %q no contiene registros; rutas disponibles: %s", op.Ruta, strings.Join(nombres, ", "))
		}
		ruta = op.Ruta
	}

	indice := map[string]int{}
	var encabezados []string
	var planos []map[string]string
	for _, n := range rutas[ruta] {
		plano := map[string]string{}
		var orden []string
		aplanarNodo(n, "", plano, &orden)
		for _, k := range orden {
			if _, ok := indice[k]; !ok {
				indice[k] = len(encabezados)
				encabezados = append(encabezados, k)
				if len(encabezados) > op.Limites.MaxColumnas {
					return nil, errorf(ErrDemasiadasCols, "los registros tienen más de %d campos distintos", op.Limites.MaxColumnas)
				}
			}
		}
		planos = append(planos, plano)
	}
	t := &Tabla{Encabezados: encabezados, Origen: Origen{Rutas: nombres, Ruta: ruta}}
	for i, p := range planos {
		valores := make([]string, len(encabezados))
		for k, v := range p {
			valores[indice[k]] = strings.TrimSpace(v)
		}
		if !vacia(valores) {
			t.Filas = append(t.Filas, Fila{Numero: i + 1, Valores: valores})
		}
	}
	return t, nil
}

func parsearXML(r io.Reader, maxProfundidad int) (*nodo, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = true
	var pila []*nodo
	var raiz *nodo
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errorf(ErrDanado, "el XML no es válido")
		}
		switch e := tok.(type) {
		case xml.StartElement:
			if len(pila) >= maxProfundidad {
				return nil, errorf(ErrEstructura, "el anidamiento supera %d niveles", maxProfundidad)
			}
			n := &nodo{nombre: e.Name.Local}
			for _, a := range e.Attr {
				if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
					continue
				}
				n.attrs = append(n.attrs, [2]string{a.Name.Local, a.Value})
			}
			if len(pila) > 0 {
				padre := pila[len(pila)-1]
				padre.hijos = append(padre.hijos, n)
			} else if raiz == nil {
				raiz = n
			}
			pila = append(pila, n)
		case xml.EndElement:
			if len(pila) > 0 {
				pila = pila[:len(pila)-1]
			}
		case xml.CharData:
			if len(pila) > 0 {
				pila[len(pila)-1].texto.Write(e)
			}
		case xml.Directive:
			// <!DOCTYPE ...>: se ignora (no se resuelven entidades externas).
		}
	}
	if raiz == nil {
		return nil, errorf(ErrEstructura, "el XML no tiene elementos")
	}
	return raiz, nil
}

// buscarRepetidos registra rutas cuyos elementos aparecen 2+ veces bajo el
// mismo padre y tienen hijos o atributos (un registro, no un valor suelto).
func buscarRepetidos(n *nodo, ruta string, out map[string][]*nodo) {
	conteo := map[string]int{}
	for _, h := range n.hijos {
		conteo[h.nombre]++
	}
	for _, h := range n.hijos {
		hijo := ruta + "/" + h.nombre
		if conteo[h.nombre] >= 2 && (len(h.hijos) > 0 || len(h.attrs) > 0) {
			out[hijo] = append(out[hijo], h)
			continue
		}
		buscarRepetidos(h, hijo, out)
	}
}

func aplanarNodo(n *nodo, ruta string, out map[string]string, orden *[]string) {
	poner := func(k, v string) {
		if _, ok := out[k]; !ok {
			*orden = append(*orden, k)
			out[k] = v
			return
		}
		out[k] += "; " + v // elementos hoja repetidos: se unen
	}
	prefijo := ruta
	if prefijo != "" {
		prefijo += "."
	}
	for _, a := range n.attrs {
		poner(prefijo+"@"+a[0], a[1])
	}
	if len(n.hijos) == 0 {
		if ruta != "" {
			poner(ruta, strings.TrimSpace(n.texto.String()))
		}
		return
	}
	for _, h := range n.hijos {
		aplanarNodo(h, prefijo+h.nombre, out, orden)
	}
}
