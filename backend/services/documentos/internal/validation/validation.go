package validation

import (
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}

// Tipos de documento identificados en el analisis funcional de los 5 flujos
// (Formato de Horas, Informe, Acta, Proveido) mas ADJUNTO para los sustentos
// que cargan los solicitantes, y OTRO para lo que no encaje todavia.
const (
	TipoAdjunto  = "ADJUNTO"
	TipoProveido = "PROVEIDO"
	TipoActa     = "ACTA"
	TipoInforme  = "INFORME"
	TipoFormato  = "FORMATO"
	TipoOtro     = "OTRO"
)

var tiposDocumento = map[string]bool{
	TipoAdjunto:  true,
	TipoProveido: true,
	TipoActa:     true,
	TipoInforme:  true,
	TipoFormato:  true,
	TipoOtro:     true,
}

func IsValidTipoDocumento(value string) bool {
	return tiposDocumento[value]
}

var extensionesPermitidas = map[string]bool{
	"pdf":  true,
	"jpg":  true,
	"jpeg": true,
	"png":  true,
	"doc":  true,
	"docx": true,
}

func IsValidExtension(value string) bool {
	return extensionesPermitidas[strings.ToLower(strings.TrimSpace(value))]
}

// MaxTamanoDocumento esta acotado por el Gateway, no por este servicio: el
// contenido viaja del frontend al Gateway como JSON con el archivo en
// base64 (~33% de overhead), y go-zero limita a 8 MiB el body JSON completo
// de cualquier request REST (rest/httpx.maxBodyLen, constante interna no
// configurable por ruta). 5 MiB crudos ~= 6.7 MiB en base64, dejando margen
// dentro de ese tope. Subir este limite requiere resolver primero el limite
// de 8 MiB del Gateway (por ejemplo con upload multipart en vez de JSON).
const MaxTamanoDocumento = 5 * 1024 * 1024 // 5 MiB

const EstadoActivo = "ACTIVO"
const EstadoEliminado = "ELIMINADO"
