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

// MaxTamanoDocumento se mantiene deliberadamente por debajo del limite por
// defecto de gRPC (4 MiB) mas el margen que este servicio habilita via
// grpc.MaxRecvMsgSize/MaxSendMsgSize (ver documentos.go y el cliente del
// Gateway). Subir este limite requiere subir ambos en conjunto.
const MaxTamanoDocumento = 8 * 1024 * 1024 // 8 MiB

const EstadoActivo = "ACTIVO"
const EstadoEliminado = "ELIMINADO"
