package validation

import (
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}

// Tipos de movimiento identificados en la sección 15 del análisis funcional
// (docs/etapa-12-especificacion-funcional.md): se distingue explícitamente
// entre una derivación real (cambia el área/responsable), una notificación
// (informa, no transfiere responsabilidad), una asignación (habilita a
// alguien para actuar), una recepción (alguien recibe formalmente el
// trámite) y una aprobación (evento de decisión, no de transporte).
const (
	TipoDerivacion   = "DERIVACION"
	TipoNotificacion = "NOTIFICACION"
	TipoAsignacion   = "ASIGNACION"
	TipoRecepcion    = "RECEPCION"
	TipoAprobacion   = "APROBACION"
)

var tiposDerivacion = map[string]bool{
	TipoDerivacion:   true,
	TipoNotificacion: true,
	TipoAsignacion:   true,
	TipoRecepcion:    true,
	TipoAprobacion:   true,
}

func IsValidTipo(value string) bool {
	return tiposDerivacion[value]
}
