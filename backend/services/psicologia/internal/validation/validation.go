package validation

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}

// Estados del expediente psicológico (VARCHAR + CHECK en la BD, no ENUM).
const (
	EstadoAbierto       = "ABIERTO"
	EstadoEnSeguimiento = "EN_SEGUIMIENTO"
	EstadoCerrado       = "CERRADO"
)

// IsEstadoActivo: estados a los que se puede llevar un expediente con
// UpdateExpediente/ReopenExpediente. CERRADO solo se alcanza con CloseExpediente.
func IsEstadoActivo(estado string) bool {
	return estado == EstadoAbierto || estado == EstadoEnSeguimiento
}

func IsEstado(estado string) bool {
	return IsEstadoActivo(estado) || estado == EstadoCerrado
}

// Límites de texto (evitan cuerpos desproporcionados; no son reglas clínicas).
const (
	MaxResumenCaso = 10000
	MaxMotivo      = 2000
)

func TextoDentroDeLimite(value string, max int) bool {
	return utf8.RuneCountInString(value) <= max
}

// ---------------------------------------------------------------------------
// Atenciones (Hoja de Atención)
// ---------------------------------------------------------------------------

// Límites de la Hoja de Atención (cuerpos razonables; no son reglas clínicas).
const (
	MaxTextoAtencion = 10000 // descripción, análisis, estrategias, seguimiento, acuerdos, compromisos
	MaxTextoOtros    = 500   // "Otros", "otros:", "Otro:"
)

// Estados de atención admitidos como filtro de listado.
func IsEstadoAtencionListable(estado string) bool {
	return estado == "BORRADOR" || estado == "REGISTRADA" || estado == "ANULADA"
}

// "Atención realizada por": valores fijos de la hoja institucional.
var realizadaPorValidos = map[string]bool{"DOCENTE": true, "AUXILIAR": true, "DIRECTIVO": true, "PSICOLOGO": true}

func IsRealizadaPor(codigo string) bool {
	return realizadaPorValidos[codigo]
}

var codigoCatalogoPattern = regexp.MustCompile(`^[A-Z_]{1,40}$`)

// IsCodigoCatalogo: formato de un código de catálogo (su existencia y que
// esté activo lo verifica el repositorio contra la tabla).
func IsCodigoCatalogo(codigo string) bool {
	return codigoCatalogoPattern.MatchString(codigo)
}

// IsFecha: YYYY-MM-DD real (vacío no es válido aquí).
func IsFecha(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

// IsHora: HH:MM de 24 horas (vacío no es válido aquí).
func IsHora(value string) bool {
	_, err := time.Parse("15:04", value)
	return err == nil
}
