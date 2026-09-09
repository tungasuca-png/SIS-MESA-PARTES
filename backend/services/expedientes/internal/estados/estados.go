package estados

// Valores controlados del dominio de Expedientes. La lista de tipos es
// deliberadamente corta y pensada para ampliarse sin reescribir el servicio.
const (
	TipoSolicitud = "SOLICITUD"
	TipoTramite   = "TRAMITE"
	TipoOficio    = "OFICIO"
	TipoOtro      = "OTRO"
)

const (
	PrioridadNormal  = "NORMAL"
	PrioridadUrgente = "URGENTE"
)

const (
	EstadoPendiente = "PENDIENTE"
	EstadoEnProceso = "EN_PROCESO"
	EstadoAtendido  = "ATENDIDO"
	EstadoObservado = "OBSERVADO"
)

// Áreas internas responsables de un expediente (ver migración
// 002_add_area_actual.sql). Los códigos coinciden con los roles reales del
// JWT — ADMIN se incluye por si alguna vez se deriva ahí explícitamente,
// aunque en la práctica ya ve todo sin importar el área (CanViewAll).
const (
	AreaSecretaria  = "SECRETARIA"
	AreaDirector    = "DIRECTOR"
	AreaSubdirector = "SUBDIRECTOR"
	AreaDocente     = "DOCENTE"
	AreaAuxiliar    = "AUXILIAR"
	AreaAdmin       = "ADMIN"
)

var areasValidas = map[string]bool{
	AreaSecretaria:  true,
	AreaDirector:    true,
	AreaSubdirector: true,
	AreaDocente:     true,
	AreaAuxiliar:    true,
	AreaAdmin:       true,
}

func IsValidArea(area string) bool {
	return areasValidas[area]
}

var tiposValidos = map[string]bool{
	TipoSolicitud: true,
	TipoTramite:   true,
	TipoOficio:    true,
	TipoOtro:      true,
}

var prioridadesValidas = map[string]bool{
	PrioridadNormal:  true,
	PrioridadUrgente: true,
}

// transiciones define, para cada estado, a qué estados puede pasar.
// ATENDIDO y OBSERVADO son terminales en esta etapa a propósito (así lo
// especifica el diseño inicial); ampliar el flujo más adelante (por ejemplo
// permitir OBSERVADO -> EN_PROCESO) solo requiere agregar una entrada aquí.
var transiciones = map[string][]string{
	EstadoPendiente: {EstadoEnProceso, EstadoObservado},
	EstadoEnProceso: {EstadoAtendido},
	EstadoAtendido:  {},
	EstadoObservado: {},
}

func IsValidTipo(tipo string) bool {
	return tiposValidos[tipo]
}

func IsValidPrioridad(prioridad string) bool {
	return prioridadesValidas[prioridad]
}

func IsValidEstado(estado string) bool {
	_, ok := transiciones[estado]
	return ok
}

func CanTransition(from, to string) bool {
	for _, allowed := range transiciones[from] {
		if allowed == to {
			return true
		}
	}
	return false
}
