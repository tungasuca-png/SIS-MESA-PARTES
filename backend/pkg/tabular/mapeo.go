package tabular

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Campo de destino de un módulo.
type Campo struct {
	Clave    string
	Etiqueta string
	Grupo    string // p. ej. "Estudiante" / "Matrícula vigente"
	// Sinonimos: encabezados equivalentes SIN duda (se comparan normalizados).
	Sinonimos []string
	// SinonimosDudosos: suelen significar este campo, pero no siempre (p. ej.
	// "AULA" por "SECCIÓN"): nunca se asignan sin confirmación.
	SinonimosDudosos []string
	Importable       bool
	Exportable       bool
	// Detección por contenido (opcional): valores admitidos o patrón.
	Catalogo []string
	Patron   *regexp.Regexp
}

// Esquema de destino de un módulo (Estudiantes hoy; otros en el futuro).
type Esquema struct {
	Modulo string
	Campos []Campo
}

func (e Esquema) Campo(clave string) (Campo, bool) {
	for _, c := range e.Campos {
		if c.Clave == clave {
			return c, true
		}
	}
	return Campo{}, false
}

// Confianza de una asignación. Solo ALTA se aplica sin confirmar.
const (
	ConfianzaAlta  = "ALTA"
	ConfianzaMedia = "MEDIA"
	ConfianzaBaja  = "BAJA"
)

// Asignacion: columna del archivo -> campo de destino ("" = sin asignar).
type Asignacion struct {
	Columna      int
	Encabezado   string
	Normalizado  string
	Campo        string
	Confianza    string
	Motivo       string
	Alternativas []string
	Muestras     []string
	Confirmada   bool // por plantilla o por el usuario
}

// Mapeo completo con los problemas que impiden usarlo sin revisión.
type Mapeo struct {
	Asignaciones []Asignacion
	Problemas    []string
}

// RequiereConfirmacion: alguna asignación no es de confianza alta ni confirmada.
func (m Mapeo) RequiereConfirmacion() bool {
	for _, a := range m.Asignaciones {
		if a.Campo != "" && a.Confianza != ConfianzaAlta && !a.Confirmada {
			return true
		}
	}
	return false
}

// Campos asignados: clave de campo -> índice de columna.
func (m Mapeo) Indices() map[string]int {
	out := map[string]int{}
	for _, a := range m.Asignaciones {
		if a.Campo != "" {
			out[a.Campo] = a.Columna
		}
	}
	return out
}

// PuntuarEncabezado cuenta cuántas celdas reconoce el esquema (para detectar
// la fila de encabezados en archivos con títulos encima).
func (e Esquema) PuntuarEncabezado(celdas []string) int {
	n := 0
	for _, c := range celdas {
		if h := NormalizarEncabezado(c); h != "" && len(e.exactos(h, nil)) > 0 {
			n++
		}
	}
	return n
}

func normalizarTodos(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = NormalizarEncabezado(x)
	}
	return out
}

// exactos: campos cuyo nombre, clave o sinónimo (normalizados) es h.
func (e Esquema) exactos(h string, plantilla map[string]string) []string {
	if c, ok := plantilla[h]; ok {
		return []string{c}
	}
	var out []string
	for _, c := range e.Campos {
		nombres := append([]string{c.Etiqueta, strings.ReplaceAll(c.Clave, "_", " ")}, c.Sinonimos...)
		for _, n := range normalizarTodos(nombres) {
			if n == h {
				out = append(out, c.Clave)
				break
			}
		}
	}
	return out
}

func (e Esquema) dudosos(h string) []string {
	var out []string
	for _, c := range e.Campos {
		for _, n := range normalizarTodos(c.SinonimosDudosos) {
			if n == h {
				out = append(out, c.Clave)
			}
		}
	}
	return out
}

// parecidos: los tokens de algún nombre del campo están todos en el
// encabezado y este agrega a lo sumo una palabra ("CODIGO DEL ESTUDIANTE"
// ~ "CODIGO ESTUDIANTE"). Es solo un INDICIO.
func (e Esquema) parecidos(h string) []string {
	ht := Tokens(h)
	set := map[string]bool{}
	for _, t := range ht {
		set[t] = true
	}
	var out []string
	for _, c := range e.Campos {
		nombres := append([]string{c.Etiqueta}, c.Sinonimos...)
		for _, n := range normalizarTodos(nombres) {
			nt := Tokens(n)
			if len(nt) == 0 || len(ht) > len(nt)+1 {
				continue
			}
			todos := true
			for _, t := range nt {
				if !set[t] {
					todos = false
				}
			}
			if todos {
				out = append(out, c.Clave)
				break
			}
		}
	}
	return out
}

// coincidenciaContenido: proporción de valores no vacíos que cumplen el
// catálogo o patrón del campo (-1 si el campo no define detector).
func coincidenciaContenido(c Campo, valores []string) float64 {
	if len(c.Catalogo) == 0 && c.Patron == nil {
		return -1
	}
	cat := map[string]bool{}
	for _, v := range c.Catalogo {
		cat[NormalizarEncabezado(v)] = true
	}
	total, ok := 0, 0
	for _, v := range valores {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		total++
		if (len(cat) > 0 && cat[NormalizarEncabezado(v)]) || (c.Patron != nil && c.Patron.MatchString(v)) {
			ok++
		}
	}
	if total == 0 {
		return -1
	}
	return float64(ok) / float64(total)
}

func valoresColumna(t *Tabla, col, max int) []string {
	var out []string
	for _, f := range t.Filas {
		if len(out) >= max {
			break
		}
		out = append(out, f.Valor(col))
	}
	return out
}

func muestras(valores []string, n int) []string {
	var out []string
	for _, v := range valores {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
			if len(out) == n {
				break
			}
		}
	}
	return out
}

// Sugerir propone el mapeo de cada columna. plantilla (opcional) trae
// asociaciones CONFIRMADAS: encabezado normalizado -> campo.
func (e Esquema) Sugerir(t *Tabla, plantilla map[string]string) Mapeo {
	var m Mapeo
	for i, enc := range t.Encabezados {
		h := NormalizarEncabezado(enc)
		valores := valoresColumna(t, i, 200)
		a := Asignacion{Columna: i, Encabezado: enc, Normalizado: h, Muestras: muestras(valores, 3)}
		_, porPlantilla := plantilla[h]
		switch exactos := e.exactos(h, plantilla); {
		case len(exactos) == 1:
			a.Campo, a.Confianza = exactos[0], ConfianzaAlta
			a.Motivo = "el encabezado corresponde a «" + e.etiqueta(exactos[0]) + "»"
			if porPlantilla {
				a.Motivo, a.Confirmada = "asociación confirmada en la plantilla", true
			}
		case len(exactos) > 1:
			a.Campo, a.Confianza, a.Alternativas = exactos[0], ConfianzaMedia, exactos[1:]
			a.Motivo = "el encabezado puede ser varios campos: confirme cuál"
		default:
			if d := e.dudosos(h); len(d) > 0 {
				a.Campo, a.Confianza, a.Alternativas = d[0], ConfianzaMedia, d[1:]
				a.Motivo = "«" + enc + "» suele significar «" + e.etiqueta(d[0]) + "», pero no siempre: confirme"
			} else if p := e.parecidos(h); len(p) > 0 {
				c, _ := e.Campo(p[0])
				prop := coincidenciaContenido(c, valores)
				a.Campo, a.Alternativas = p[0], p[1:]
				if prop >= 0.8 {
					a.Confianza, a.Motivo = ConfianzaMedia, "encabezado parecido y valores compatibles con «"+c.Etiqueta+"»: confirme"
				} else {
					a.Confianza, a.Motivo = ConfianzaBaja, "encabezado parecido a «"+c.Etiqueta+"»: confirme"
				}
			}
		}
		// El contenido puede contradecir al encabezado.
		if a.Campo != "" && !a.Confirmada {
			c, _ := e.Campo(a.Campo)
			if prop := coincidenciaContenido(c, valores); prop >= 0 && prop < 0.5 {
				a.Confianza = ConfianzaMedia
				a.Motivo = fmt.Sprintf("el encabezado sugiere «%s», pero solo el %d%% de los valores son compatibles: revise", c.Etiqueta, int(prop*100))
			}
		}
		m.Asignaciones = append(m.Asignaciones, a)
	}
	e.sugerirPorContenido(t, &m)
	e.resolverDuplicados(&m)
	return m
}

// sugerirPorContenido: columnas sin asignar cuyos valores calzan casi todos
// con un único campo aún libre -> sugerencia BAJA (siempre por confirmar).
func (e Esquema) sugerirPorContenido(t *Tabla, m *Mapeo) {
	usados := map[string]bool{}
	for _, a := range m.Asignaciones {
		usados[a.Campo] = true
	}
	for i := range m.Asignaciones {
		a := &m.Asignaciones[i]
		if a.Campo != "" {
			continue
		}
		valores := valoresColumna(t, a.Columna, 200)
		var candidatos []string
		for _, c := range e.Campos {
			if !c.Importable || usados[c.Clave] {
				continue
			}
			if prop := coincidenciaContenido(c, valores); prop >= 0.9 {
				candidatos = append(candidatos, c.Clave)
			}
		}
		if len(candidatos) == 1 {
			a.Campo, a.Confianza = candidatos[0], ConfianzaBaja
			a.Motivo = "por sus valores parece «" + e.etiqueta(candidatos[0]) + "»: confirme"
			usados[candidatos[0]] = true
		}
	}
}

// resolverDuplicados: un campo asignado a varias columnas queda en la de
// mayor confianza; si empatan, ninguna se aplica sin confirmación.
func (e Esquema) resolverDuplicados(m *Mapeo) {
	porCampo := map[string][]int{}
	for i, a := range m.Asignaciones {
		if a.Campo != "" {
			porCampo[a.Campo] = append(porCampo[a.Campo], i)
		}
	}
	rango := map[string]int{ConfianzaAlta: 3, ConfianzaMedia: 2, ConfianzaBaja: 1}
	campos := make([]string, 0, len(porCampo))
	for c := range porCampo {
		campos = append(campos, c)
	}
	sort.Strings(campos)
	for _, campo := range campos {
		idx := porCampo[campo]
		if len(idx) < 2 {
			continue
		}
		mejor := idx[0]
		empate := false
		for _, i := range idx[1:] {
			ri, rm := rango[m.Asignaciones[i].Confianza], rango[m.Asignaciones[mejor].Confianza]
			if ri > rm {
				mejor, empate = i, false
			} else if ri == rm {
				empate = true
			}
		}
		var encabezados []string
		for _, i := range idx {
			encabezados = append(encabezados, "«"+m.Asignaciones[i].Encabezado+"»")
			if i != mejor || empate {
				m.Asignaciones[i].Alternativas = append([]string{campo}, m.Asignaciones[i].Alternativas...)
				if i != mejor {
					m.Asignaciones[i].Campo = ""
					m.Asignaciones[i].Confianza = ""
					m.Asignaciones[i].Motivo = "otra columna parece ser «" + e.etiqueta(campo) + "»"
				} else {
					m.Asignaciones[i].Confianza = ConfianzaMedia
				}
			}
		}
		m.Problemas = append(m.Problemas, fmt.Sprintf("varias columnas (%s) parecen «%s»: confirme cuál usar", strings.Join(encabezados, ", "), e.etiqueta(campo)))
	}
}

func (e Esquema) etiqueta(clave string) string {
	if c, ok := e.Campo(clave); ok {
		return c.Etiqueta
	}
	return clave
}

// Aplicar valida un mapeo enviado por el usuario (columna -> campo) contra el
// esquema y la tabla: campos existentes e importables, columnas válidas y
// sin un mismo campo en dos columnas. Las asignaciones quedan confirmadas.
func (e Esquema) Aplicar(t *Tabla, columnas map[int]string) (Mapeo, error) {
	var m Mapeo
	usado := map[string]int{}
	for i, enc := range t.Encabezados {
		a := Asignacion{Columna: i, Encabezado: enc, Normalizado: NormalizarEncabezado(enc), Muestras: muestras(valoresColumna(t, i, 50), 3)}
		if campo := columnas[i]; campo != "" {
			c, ok := e.Campo(campo)
			if !ok || !c.Importable {
				return m, errorf(ErrOpcion, "el campo %q no existe o no se puede importar", campo)
			}
			if prev, ok := usado[campo]; ok {
				return m, errorf(ErrOpcion, "«%s» está asignado a dos columnas (%s y %s)", c.Etiqueta, t.Encabezados[prev], enc)
			}
			usado[campo] = i
			a.Campo, a.Confianza, a.Confirmada, a.Motivo = campo, ConfianzaAlta, true, "asignado por el usuario"
		}
		m.Asignaciones = append(m.Asignaciones, a)
	}
	for col := range columnas {
		if col < 0 || col >= len(t.Encabezados) {
			return m, errorf(ErrOpcion, "la columna %d no existe en el archivo", col+1)
		}
	}
	return m, nil
}
