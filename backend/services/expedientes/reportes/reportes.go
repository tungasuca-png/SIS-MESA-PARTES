// Package reportes reúne las reglas COMUNES de los reportes (Fase 2) que
// comparten Expedientes, Derivaciones, Documentos y el Gateway: periodo,
// zona horaria, granularidad (día, semana, mes), series continuas y
// normalización de áreas históricas. Es público (no internal) porque esos
// módulos ya dependen de "expedientes"; así no se duplica lógica.
//
// Solo funciones puras: no consulta datos.
package reportes

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Zona: Perú no tiene horario de verano (UTC-5 fijo). Las consultas SQL usan
// la misma zona con su nombre (ZonaSQL).
var Zona = time.FixedZone("America/Lima", -5*60*60)

const (
	ZonaSQL      = "America/Lima"
	FormatoFecha = "2006-01-02"
	FormatoMes   = "2006-01"

	GranularidadDia    = "DIA"
	GranularidadSemana = "SEMANA"
	GranularidadMes    = "MES"

	// SinClasificar: área histórica irreconocible (decisión aprobada).
	SinClasificar = "SIN_CLASIFICAR"

	// MinimoComparacion: registros mínimos en CADA periodo para mostrar una
	// variación (decisión aprobada).
	MinimoComparacion = 10
)

// Puntos máximos por serie (evita respuestas desmedidas).
var maxPuntos = map[string]int{GranularidadDia: 366, GranularidadSemana: 260, GranularidadMes: 240}

var (
	ErrGranularidad = errors.New("la granularidad debe ser DIA, SEMANA o MES")
	ErrSerieLarga   = errors.New("el periodo es demasiado largo para la granularidad elegida: use una granularidad mayor o acote las fechas")
	ErrFormatoDesde = errors.New("la fecha desde debe tener el formato AAAA-MM-DD")
	ErrFormatoHasta = errors.New("la fecha hasta debe tener el formato AAAA-MM-DD")
	ErrRangoDesde   = errors.New("la fecha desde debe estar entre 2000-01-01 y hoy")
	ErrRangoHasta   = errors.New("la fecha hasta no puede ser anterior a 2000-01-01")
	ErrOrden        = errors.New("la fecha desde no puede ser posterior a la fecha hasta")
	ErrDiezAnios    = errors.New("el periodo no puede superar 10 años")
)

// Conteo: clave y cantidad.
type Conteo struct {
	Clave    string
	Cantidad int
}

// Granularidad normaliza el valor ("" = MES, el comportamiento de la Fase 1).
func Granularidad(g string) (string, error) {
	g = strings.ToUpper(strings.TrimSpace(g))
	if g == "" {
		return GranularidadMes, nil
	}
	if _, ok := maxPuntos[g]; !ok {
		return "", ErrGranularidad
	}
	return g, nil
}

// ValidarPeriodo aplica las mismas reglas que la Fase 1 (fechas AAAA-MM-DD,
// desde entre 2000-01-01 y hoy, desde <= hasta, máximo 10 años). Los errores
// no repiten los valores recibidos.
func ValidarPeriodo(desde, hasta string, ahora time.Time) error {
	ahora = ahora.In(Zona)
	var d, h time.Time
	var err error
	if desde != "" {
		if d, err = time.ParseInLocation(FormatoFecha, desde, Zona); err != nil {
			return ErrFormatoDesde
		}
	}
	if hasta != "" {
		if h, err = time.ParseInLocation(FormatoFecha, hasta, Zona); err != nil {
			return ErrFormatoHasta
		}
	}
	minimo := time.Date(2000, 1, 1, 0, 0, 0, 0, Zona)
	hoy := time.Date(ahora.Year(), ahora.Month(), ahora.Day(), 0, 0, 0, 0, Zona)
	if desde != "" && (d.Before(minimo) || d.After(hoy)) {
		return ErrRangoDesde
	}
	if hasta != "" && h.Before(minimo) {
		return ErrRangoHasta
	}
	if desde != "" && hasta != "" {
		if h.Before(d) {
			return ErrOrden
		}
		if h.After(d.AddDate(10, 0, 0)) {
			return ErrDiezAnios
		}
	}
	return nil
}

// inicio devuelve el inicio del periodo que contiene t (lunes para SEMANA).
func inicio(t time.Time, g string) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Zona)
	switch g {
	case GranularidadSemana:
		desplazamiento := (int(t.Weekday()) + 6) % 7 // lunes = 0
		return t.AddDate(0, 0, -desplazamiento)
	case GranularidadMes:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, Zona)
	}
	return t
}

func siguiente(t time.Time, g string) time.Time {
	switch g {
	case GranularidadSemana:
		return t.AddDate(0, 0, 7)
	case GranularidadMes:
		return t.AddDate(0, 1, 0)
	}
	return t.AddDate(0, 0, 1)
}

func formato(g string) string {
	if g == GranularidadMes {
		return FormatoMes
	}
	return FormatoFecha
}

// Clave del periodo de una FECHA (AAAA-MM-DD, hora de Lima): el día, el lunes
// de su semana o el mes.
func Clave(fecha string, g string) (string, bool) {
	t, err := time.ParseInLocation(FormatoFecha, fecha, Zona)
	if err != nil {
		return "", false
	}
	return inicio(t, g).Format(formato(g)), true
}

// Serie construye una serie CONTINUA (con ceros) por granularidad a partir de
// conteos por FECHA (AAAA-MM-DD). Va del inicio del periodo (o de la primera
// fecha con datos) al fin del periodo (o a hoy). Sin datos ni "desde",
// devuelve una serie vacía. Un periodo demasiado largo devuelve ErrSerieLarga.
func Serie(porFecha map[string]int, g, desde, hasta string, ahora time.Time) ([]Conteo, error) {
	agrupado := map[string]int{}
	primera, ultima := "", ""
	for fecha, n := range porFecha {
		clave, ok := Clave(fecha, g)
		if !ok {
			continue
		}
		agrupado[clave] += n
		if primera == "" || fecha < primera {
			primera = fecha
		}
		if fecha > ultima {
			ultima = fecha
		}
	}
	desdeEf := desde
	if desdeEf == "" {
		desdeEf = primera
	}
	if desdeEf == "" {
		return []Conteo{}, nil
	}
	hastaEf := hasta
	if hastaEf == "" {
		hastaEf = ahora.In(Zona).Format(FormatoFecha)
		if ultima > hastaEf { // sin "hasta", un dato posterior a hoy no se pierde
			hastaEf = ultima
		}
	}
	a, err1 := time.ParseInLocation(FormatoFecha, desdeEf, Zona)
	b, err2 := time.ParseInLocation(FormatoFecha, hastaEf, Zona)
	if err1 != nil || err2 != nil || b.Before(a) {
		return []Conteo{}, nil
	}
	out := []Conteo{}
	for t := inicio(a, g); !t.After(b); t = siguiente(t, g) {
		if len(out) >= maxPuntos[g] {
			return nil, ErrSerieLarga
		}
		clave := t.Format(formato(g))
		out = append(out, Conteo{Clave: clave, Cantidad: agrupado[clave]})
	}
	return out, nil
}

// Áreas válidas (mismos códigos que area_actual) y etiquetas históricas.
var codigosArea = map[string]string{
	"SECRETARIA": "SECRETARIA", "DIRECTOR": "DIRECTOR", "SUBDIRECTOR": "SUBDIRECTOR",
	"DOCENTE": "DOCENTE", "AUXILIAR": "AUXILIAR", "ADMIN": "ADMIN",
	// Etiquetas en español usadas por versiones anteriores de la interfaz.
	"DIRECCION": "DIRECTOR", "SUBDIRECCION": "SUBDIRECTOR", "ADMINISTRACION": "ADMIN",
}

// OrdenAreas: orden de presentación (+ Sin clasificar al final).
var OrdenAreas = []string{"SECRETARIA", "DIRECTOR", "SUBDIRECTOR", "DOCENTE", "AUXILIAR", "ADMIN", SinClasificar}

// NormalizarArea convierte un origen/destino histórico (código, etiqueta con o
// sin tilde, o "usuario (Etiqueta)") en un código de área. Lo irreconocible
// (texto dañado, "Sistema", etc.) es SinClasificar. NUNCA devuelve el texto
// original: puede contener nombres de usuario.
func NormalizarArea(texto string) string {
	t := strings.TrimSpace(texto)
	// "usuario (Rol)": solo se usa lo que está entre paréntesis.
	if i := strings.LastIndex(t, "("); i >= 0 && strings.HasSuffix(t, ")") {
		t = t[i+1 : len(t)-1]
	}
	if codigo, ok := codigosArea[simplificar(t)]; ok {
		return codigo
	}
	return SinClasificar
}

var sinTildes = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N")

// simplificar: mayúsculas, sin tildes y solo letras A-Z (un carácter dañado
// se descarta, así que la palabra deja de coincidir: Sin clasificar).
func simplificar(s string) string {
	var b strings.Builder
	for _, r := range sinTildes.Replace(s) {
		r = unicode.ToUpper(r)
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Ordenar devuelve los conteos del mapa en el orden dado (con ceros) y, al
// final, cualquier clave no prevista ordenada alfabéticamente.
func Ordenar(orden []string, valores map[string]int) []Conteo {
	out := make([]Conteo, 0, len(orden))
	vistos := map[string]bool{}
	for _, k := range orden {
		out = append(out, Conteo{Clave: k, Cantidad: valores[k]})
		vistos[k] = true
	}
	extra := []string{}
	for k := range valores {
		if !vistos[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		out = append(out, Conteo{Clave: k, Cantidad: valores[k]})
	}
	return out
}
