package validation

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}

// Estados de matrícula (VARCHAR + CHECK en la BD, no ENUM): mínimo pedido
// para esta fase. No se inventan estados académicos adicionales
// (TRASLADADO, EGRESADO, etc.) sin una regla institucional confirmada.
const (
	EstadoActivo   = "ACTIVO"
	EstadoRetirado = "RETIRADO"
)

func IsValidEstadoMatricula(value string) bool {
	return value == EstadoActivo || value == EstadoRetirado
}

// MinQueryLength: por debajo de esto, una búsqueda "LIKE %q%" sobre nombres/
// apellidos/código es demasiado costosa e inespecífica para ser útil.
const MinQueryLength = 2

func QueryLenOK(q string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(q)) >= MinQueryLength
}

// FASE E-04 — búsqueda y filtros del listado.

// Estado del REGISTRO del estudiante (columna estado, migración 002). No es
// el estado de matrícula (estado_matricula): son conceptos distintos.
const (
	EstadoRegistroActivo   = "ACTIVO"
	EstadoRegistroInactivo = "INACTIVO"
)

func IsValidEstadoRegistro(value string) bool {
	return value == EstadoRegistroActivo || value == EstadoRegistroInactivo
}

// Límites defensivos: evitan consultas con decenas de condiciones LIKE o
// valores de filtro absurdos. Un nombre completo real cabe holgadamente.
const (
	MaxQueryLength  = 100
	MaxQueryTerms   = 6
	MaxFilterLength = 30
)

// QueryTerms separa la búsqueda en palabras (cualquier espacio en blanco,
// repetido o no). Permite buscar nombres y apellidos en cualquier orden.
func QueryTerms(q string) []string {
	return strings.Fields(q)
}

func FilterLenOK(value string) bool {
	return utf8.RuneCountInString(value) <= MaxFilterLength
}
