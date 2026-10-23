package tabular

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// leerJSON acepta un arreglo de objetos o un objeto que contenga arreglos de
// objetos en alguna ruta ("datos.estudiantes"). Si hay varias rutas
// posibles, se usa la indicada o la de más registros, y se informan todas.
func leerJSON(contenido []byte, op Opciones) (*Tabla, error) {
	texto, _, _, err := decodificar(contenido, op.Codificacion)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(texto))
	dec.UseNumber() // conserva el número tal como está escrito
	raiz, err := decodificarOrdenado(dec, 0, op.Limites.MaxProfundidad)
	if err != nil {
		return nil, err
	}
	if err := verificarProfundidad(raiz, 0, op.Limites.MaxProfundidad); err != nil {
		return nil, err
	}
	rutas := map[string][]any{}
	buscarArreglos(raiz, "", rutas, op.Limites.MaxProfundidad)
	if len(rutas) == 0 {
		return nil, errorf(ErrEstructura, "el JSON no contiene un arreglo de objetos (registros)")
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
	t, err := tablaDeObjetos(rutas[ruta], op.Limites, func(i int) int { return i + 1 })
	if err != nil {
		return nil, err
	}
	t.Origen.Rutas, t.Origen.Ruta = nombres, ruta
	return t, nil
}

// leerJSONL: un objeto por línea (.jsonl/.ndjson); la fila es el número de línea.
func leerJSONL(contenido []byte, op Opciones) (*Tabla, error) {
	texto, _, _, err := decodificar(contenido, op.Codificacion)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(strings.NewReader(texto))
	sc.Buffer(make([]byte, 64<<10), maxXMLDescomprimido)
	var objetos []any
	var lineas []int
	n := 0
	for sc.Scan() {
		n++
		l := bytes.TrimSpace(sc.Bytes())
		if len(l) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(l))
		dec.UseNumber()
		obj, err := decodificarOrdenado(dec, 0, op.Limites.MaxProfundidad)
		if err != nil {
			if CodigoDe(err) == ErrEstructura {
				return nil, err
			}
			return nil, errorf(ErrDanado, "la línea %d no es JSON válido", n)
		}
		if _, ok := obj.(*objeto); !ok {
			return nil, errorf(ErrEstructura, "la línea %d no es un objeto JSON", n)
		}
		if err := verificarProfundidad(obj, 0, op.Limites.MaxProfundidad); err != nil {
			return nil, err
		}
		objetos = append(objetos, obj)
		lineas = append(lineas, n)
		if len(objetos) > op.Limites.MaxFilas {
			return nil, errorf(ErrDemasiadas, "el archivo tiene más de %d registros", op.Limites.MaxFilas)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, errorf(ErrDanado, "no se pudo leer el archivo línea por línea")
	}
	return tablaDeObjetos(objetos, op.Limites, func(i int) int { return lineas[i] })
}

// objeto JSON con el orden original de sus claves.
type objeto struct {
	claves  []string
	valores map[string]any
}

// decodificarOrdenado decodifica con tokens (conserva el orden de las claves
// y corta el anidamiento excesivo ANTES de construirlo en memoria).
func decodificarOrdenado(dec *json.Decoder, nivel, max int) (any, error) {
	if nivel > max {
		return nil, errorf(ErrEstructura, "el anidamiento supera %d niveles", max)
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, errorf(ErrDanado, "el JSON no es válido: %s", resumirErrorJSON(err))
	}
	switch d := tok.(type) {
	case json.Delim:
		switch d {
		case '{':
			o := &objeto{valores: map[string]any{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, errorf(ErrDanado, "el JSON no es válido: %s", resumirErrorJSON(err))
				}
				k, _ := kt.(string)
				v, err := decodificarOrdenado(dec, nivel+1, max)
				if err != nil {
					return nil, err
				}
				if _, ok := o.valores[k]; !ok {
					o.claves = append(o.claves, k)
				}
				o.valores[k] = v
			}
			_, err := dec.Token() // }
			if err != nil {
				return nil, errorf(ErrDanado, "el JSON no es válido: objeto sin cerrar")
			}
			return o, nil
		case '[':
			var arr []any
			for dec.More() {
				v, err := decodificarOrdenado(dec, nivel+1, max)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, errorf(ErrDanado, "el JSON no es válido: arreglo sin cerrar")
			}
			return arr, nil
		}
		return nil, errorf(ErrDanado, "el JSON no es válido")
	}
	return tok, nil
}

func resumirErrorJSON(err error) string {
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return fmt.Sprintf("error de sintaxis cerca del byte %d", se.Offset)
	}
	return "estructura inesperada"
}

func verificarProfundidad(v any, nivel, max int) error {
	if nivel > max {
		return errorf(ErrEstructura, "el anidamiento supera %d niveles", max)
	}
	switch x := v.(type) {
	case *objeto:
		for _, h := range x.valores {
			if err := verificarProfundidad(h, nivel+1, max); err != nil {
				return err
			}
		}
	case []any:
		for _, h := range x {
			if err := verificarProfundidad(h, nivel+1, max); err != nil {
				return err
			}
		}
	}
	return nil
}

// buscarArreglos registra cada ruta con un arreglo cuyos elementos son
// objetos ("$" = la raíz).
func buscarArreglos(v any, ruta string, out map[string][]any, max int) {
	if max < 0 {
		return
	}
	switch x := v.(type) {
	case []any:
		objetos := 0
		for _, e := range x {
			if _, ok := e.(*objeto); ok {
				objetos++
			}
		}
		if objetos > 0 && objetos*2 >= len(x) {
			r := ruta
			if r == "" {
				r = "$"
			}
			out[r] = x
		}
	case *objeto:
		for _, k := range x.claves {
			hijo := k
			if ruta != "" {
				hijo = ruta + "." + k
			}
			buscarArreglos(x.valores[k], hijo, out, max-1)
		}
	}
}

// tablaDeObjetos aplana cada objeto (rutas "a.b"; arreglos de valores simples
// unidos con "; "; arreglos de objetos con índice "a.0.b") y une las claves
// en el orden en que aparecen.
func tablaDeObjetos(objetos []any, lim Limites, numero func(int) int) (*Tabla, error) {
	indice := map[string]int{}
	var encabezados []string
	planos := make([]map[string]string, 0, len(objetos))
	for _, o := range objetos {
		plano := map[string]string{}
		var orden []string
		aplanar(o, "", plano, &orden)
		for _, k := range orden {
			if _, ok := indice[k]; !ok {
				indice[k] = len(encabezados)
				encabezados = append(encabezados, k)
				if len(encabezados) > lim.MaxColumnas {
					return nil, errorf(ErrDemasiadasCols, "los registros tienen más de %d campos distintos", lim.MaxColumnas)
				}
			}
		}
		planos = append(planos, plano)
	}
	t := &Tabla{Encabezados: encabezados}
	for i, p := range planos {
		valores := make([]string, len(encabezados))
		for k, v := range p {
			valores[indice[k]] = strings.TrimSpace(v)
		}
		if vacia(valores) {
			continue
		}
		t.Filas = append(t.Filas, Fila{Numero: numero(i), Valores: valores})
	}
	return t, nil
}

// aplanar recorre el valor en el orden del documento y anota cada ruta.
func aplanar(v any, ruta string, out map[string]string, orden *[]string) {
	poner := func(k, val string) {
		if _, ok := out[k]; !ok {
			*orden = append(*orden, k)
		}
		out[k] = val
	}
	switch x := v.(type) {
	case *objeto:
		for _, k := range x.claves {
			hijo := k
			if ruta != "" {
				hijo = ruta + "." + k
			}
			aplanar(x.valores[k], hijo, out, orden)
		}
	case []any:
		simples := true
		for _, e := range x {
			switch e.(type) {
			case *objeto, []any:
				simples = false
			}
		}
		if simples {
			partes := make([]string, 0, len(x))
			for _, e := range x {
				partes = append(partes, valorJSON(e))
			}
			poner(ruta, strings.Join(partes, "; "))
			return
		}
		for i, e := range x {
			aplanar(e, fmt.Sprintf("%s.%d", ruta, i), out, orden)
		}
	default:
		poner(ruta, valorJSON(x))
	}
}

func valorJSON(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	}
	return fmt.Sprint(v)
}
