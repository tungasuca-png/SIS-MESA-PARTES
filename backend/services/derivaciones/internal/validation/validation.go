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

// areasValidas refleja el mismo catálogo de áreas de Expedientes Service
// (internal/estados.IsValidArea) — no importable directamente (paquete
// "internal" de otro módulo Go), así que se duplica el valor, no la
// lógica, igual que "estadoPendienteExpediente" en Documentos Service.
// Cierra un hueco real: antes "origen"/"destino" solo se validaban por
// longitud, nunca contra el catálogo real de áreas.
var areasValidas = map[string]bool{
	"SECRETARIA":  true,
	"DIRECTOR":    true,
	"SUBDIRECTOR": true,
	"DOCENTE":     true,
	"AUXILIAR":    true,
	"ADMIN":       true,
}

func IsValidArea(value string) bool {
	return areasValidas[value]
}
