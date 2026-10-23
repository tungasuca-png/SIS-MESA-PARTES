package tabular

import (
	"fmt"
	"regexp"
	"strings"
)

// filasBuscarEncabezado: los encabezados se buscan en las primeras filas con
// datos (los reportes suelen traer títulos y filas vacías encima).
const filasBuscarEncabezado = 15

// filaCruda: una fila leída de una rejilla con su número original.
type filaCruda struct {
	numero int
	celdas []string
}

func vacia(celdas []string) bool {
	for _, c := range celdas {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// armarTabla convierte una rejilla (Excel, ODS, HTML, CSV) en Tabla:
// detecta (o usa) la fila de encabezados, nombra columnas vacías o repetidas
// y avisa de filas con más valores que encabezados (datos desplazados).
func armarTabla(crudas []filaCruda, op Opciones, origen Origen) (*Tabla, error) {
	var conDatos []filaCruda
	for _, f := range crudas {
		if !vacia(f.celdas) {
			conDatos = append(conDatos, f)
		}
	}
	if len(conDatos) == 0 {
		return nil, errorf(ErrVacio, "no se encontraron datos")
	}

	idx := -1
	if op.FilaEncabezado > 0 {
		for i, f := range conDatos {
			if f.numero == op.FilaEncabezado {
				idx = i
			}
		}
		if idx < 0 {
			return nil, errorf(ErrOpcion, "la fila %d no tiene datos para usarla como encabezados", op.FilaEncabezado)
		}
	} else {
		idx = detectarEncabezado(conDatos, op.PuntuarEncabezado)
	}

	cab := conDatos[idx]
	encabezados := recortarFinal(cab.celdas)
	t := &Tabla{Origen: origen}
	t.Origen.FilaEncabezado = cab.numero
	if idx > 0 {
		t.Origen.Avisos = append(t.Origen.Avisos, fmt.Sprintf("se ignoraron %d fila(s) de título antes de los encabezados (fila %d)", idx, cab.numero))
	}

	desplazadas, literales := 0, 0
	for _, f := range conDatos[idx+1:] {
		valores := recortarFinal(f.celdas)
		if len(valores) > len(encabezados) {
			desplazadas++
			for len(encabezados) < len(valores) {
				encabezados = append(encabezados, "")
			}
		}
		limpios, n := limpiar(valores)
		literales += n
		t.Filas = append(t.Filas, Fila{Numero: f.numero, Valores: limpios})
		if len(t.Filas) > op.Limites.MaxFilas {
			break
		}
	}
	if literales > 0 {
		t.Origen.Avisos = append(t.Origen.Avisos, fmt.Sprintf("%d celda(s) venían como =\"texto\" (forma en que algunos sistemas conservan ceros iniciales para Excel): se leyeron como texto, sin ejecutar nada", literales))
	}
	if desplazadas > 0 {
		t.Origen.Avisos = append(t.Origen.Avisos, fmt.Sprintf("%d fila(s) tienen más valores que encabezados: revise si hay datos desplazados", desplazadas))
	}
	t.Encabezados = nombrarEncabezados(encabezados)
	return t, nil
}

// detectarEncabezado elige, entre las primeras filas con datos, la que más
// celdas reconoce el esquema (si se dio un puntuador) o, si no, la primera
// con varias celdas de texto distintas y no numéricas.
func detectarEncabezado(filas []filaCruda, puntuar func([]string) int) int {
	limite := min(len(filas), filasBuscarEncabezado)
	mejor, mejorPuntaje := -1, 0
	if puntuar != nil {
		for i := 0; i < limite; i++ {
			if p := puntuar(filas[i].celdas); p > mejorPuntaje {
				mejor, mejorPuntaje = i, p
			}
		}
		if mejor >= 0 && mejorPuntaje >= 2 {
			return mejor
		}
	}
	// Sin esquema: la primera fila "ancha" de textos (los títulos ocupan una
	// sola celda; los datos suelen tener números).
	ancho := 0
	for i := 0; i < limite; i++ {
		ancho = max(ancho, len(recortarFinal(filas[i].celdas)))
	}
	for i := 0; i < limite; i++ {
		textos := 0
		for _, c := range filas[i].celdas {
			c = strings.TrimSpace(c)
			if c != "" && !esNumero(c) {
				textos++
			}
		}
		if textos >= 2 && textos*2 >= ancho {
			return i
		}
	}
	return 0
}

func recortarFinal(celdas []string) []string {
	n := len(celdas)
	for n > 0 && strings.TrimSpace(celdas[n-1]) == "" {
		n--
	}
	return append([]string(nil), celdas[:n]...)
}

// literalExcel: ="texto" sin nada más (ni operadores, ni funciones, ni
// comillas internas). Es la forma en que muchos sistemas exportan un valor
// para que Excel lo trate como TEXTO (p. ej. un DNI con ceros iniciales).
// Se toma el texto interior; cualquier otra fórmula queda tal cual y la
// rechaza la validación del módulo.
var literalExcel = regexp.MustCompile(`^="([^"]*)"$`)

func limpiar(valores []string) ([]string, int) {
	out := make([]string, len(valores))
	literales := 0
	for i, v := range valores {
		v = strings.TrimSpace(strings.ReplaceAll(v, "\u00a0", " "))
		if m := literalExcel.FindStringSubmatch(v); m != nil {
			v = strings.TrimSpace(m[1])
			literales++
		}
		out[i] = v
	}
	return out, literales
}

// nombrarEncabezados: vacíos -> "Columna N"; repetidos -> "Nombre (2)". El
// texto original se conserva (sin normalizar) para mostrarlo al usuario.
func nombrarEncabezados(cab []string) []string {
	out := make([]string, len(cab))
	vistos := map[string]int{}
	for i, h := range cab {
		h = strings.TrimSpace(strings.ReplaceAll(h, "\u00a0", " "))
		if h == "" {
			h = fmt.Sprintf("Columna %d", i+1)
		}
		clave := NormalizarEncabezado(h)
		vistos[clave]++
		if vistos[clave] > 1 {
			h = fmt.Sprintf("%s (%d)", h, vistos[clave])
		}
		out[i] = h
	}
	return out
}

func esNumero(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	puntos := 0
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case (r == '.' || r == ',') && puntos == 0:
			puntos++
		case r == '-' && i == 0:
		default:
			return false
		}
	}
	return true
}
