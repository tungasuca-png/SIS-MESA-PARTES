package estados

// Valores controlados del dominio de Expedientes. La lista de tipos es
// deliberadamente corta y pensada para ampliarse sin reescribir el servicio.
//
// Genéricos (compatibilidad con expedientes existentes y con el registro
// interno de "Nuevo expediente" que no corresponde a ninguno de los 5
// trámites institucionales de abajo):
const (
	TipoSolicitud = "SOLICITUD"
	TipoTramite   = "TRAMITE"
	TipoOficio    = "OFICIO"
	TipoOtro      = "OTRO"
)

// Trámites reales de Mesa de Partes (Etapa 2, ver docs/etapa-2-tipos-tramite.md).
// Vienen de docs/analisis-5-flujos-completo.md — FLUJO 2 (Certificado /
// Constancia de Estudios) y FLUJO 4 (Permiso / Justificación de Falta /
// Justificación de Tardanza) — no se inventan nombres nuevos. Se mantienen
// CERTIFICADO y CONSTANCIA como tipos independientes: la diferencia exacta
// de requisitos entre ambos sigue sin definir (ver pregunta F2-01 en
// docs/etapa-12-especificacion-funcional.md), no se asume ninguna.
const (
	TipoCertificado           = "CERTIFICADO"
	TipoConstancia            = "CONSTANCIA"
	TipoPermiso               = "PERMISO"
	TipoJustificacionFalta    = "JUSTIFICACION_FALTA"
	TipoJustificacionTardanza = "JUSTIFICACION_TARDANZA"
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

	TipoCertificado:           true,
	TipoConstancia:            true,
	TipoPermiso:               true,
	TipoJustificacionFalta:    true,
	TipoJustificacionTardanza: true,
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

// derivacionesPermitidas: transiciones de área REALMENTE confirmadas por la
// institución para cada tipo de trámite (Etapa 3, ver
// docs/workflow-definitivo-mesa-de-partes.md y
// docs/analisis-5-flujos-completo.md). No se inventa ninguna combinación
// que no esté documentada:
//   - F2 (CERTIFICADO, CONSTANCIA): SECRETARIA -> DIRECTOR es la única
//     derivación que describe el análisis (Secretaría revisa y deriva a
//     Dirección). El análisis NO describe una derivación posterior desde
//     Dirección para F2 (el cierre de F2 es REGLA PENDIENTE — pregunta
//     F2-02) — por eso no hay ninguna entrada DIRECTOR -> * para estos tipos.
//   - F4 (PERMISO, JUSTIFICACION_FALTA, JUSTIFICACION_TARDANZA):
//     SECRETARIA -> DIRECTOR (igual que F2) y además DIRECTOR ->
//     SUBDIRECTOR ("derivación aprobatoria", ya confirmada como dato en la
//     matriz de derivaciones de la Etapa 12, sección 15) — el resultado
//     exacto de esa aprobación (Conflicto F4-01) NO se implementa acá, solo
//     el movimiento de área que ya está confirmado como parte del flujo.
//
// Etapa 4 (ver docs/etapa-4-resolucion-cierre-f2-f4.md): se agregan las
// transiciones de cierre ya confirmadas por la institución —
// DIRECTOR -> SECRETARIA para F2 (Dirección devuelve el expediente con el
// documento final) y SUBDIRECTOR -> DOCENTE para F4 (Subdirección envía el
// proveído ya formalizado) — ambas citadas explícitamente en la regla
// institucional de esta etapa, no inventadas.
var derivacionesPermitidas = map[string]map[string][]string{
	TipoCertificado: {
		AreaSecretaria: {AreaDirector},
		AreaDirector:   {AreaSecretaria},
	},
	TipoConstancia: {
		AreaSecretaria: {AreaDirector},
		AreaDirector:   {AreaSecretaria},
	},

	TipoPermiso: {
		AreaSecretaria:  {AreaDirector},
		AreaDirector:    {AreaSubdirector},
		AreaSubdirector: {AreaDocente},
	},
	TipoJustificacionFalta: {
		AreaSecretaria:  {AreaDirector},
		AreaDirector:    {AreaSubdirector},
		AreaSubdirector: {AreaDocente},
	},
	TipoJustificacionTardanza: {
		AreaSecretaria:  {AreaDirector},
		AreaDirector:    {AreaSubdirector},
		AreaSubdirector: {AreaDocente},
	},
}

// CanDerivar responde si existe una regla institucional CONFIRMADA que
// permita derivar un expediente de este tipo desde areaActual hacia
// areaDestino. Para los 4 tipos genéricos (SOLICITUD/TRAMITE/OFICIO/OTRO)
// no hay ninguna regla institucional que los abarque (no son un trámite
// real de Mesa de Partes, son el registro interno preexistente) — se
// mantienen tan permisivos como ya lo eran antes de esta etapa: cualquier
// área válida es un destino aceptable, para no romper ese uso existente.
func CanDerivar(tipoTramite, areaActual, areaDestino string) bool {
	if !IsValidArea(areaDestino) {
		return false
	}
	reglas, tieneReglaEspecifica := derivacionesPermitidas[tipoTramite]
	if !tieneReglaEspecifica {
		// Tipo genérico: sin regla institucional definida, se preserva el
		// comportamiento previo a la Etapa 3 (cualquier área válida vale).
		return true
	}
	for _, destino := range reglas[areaActual] {
		if destino == areaDestino {
			return true
		}
	}
	return false
}

var tiposF2 = map[string]bool{TipoCertificado: true, TipoConstancia: true}

var tiposF4 = map[string]bool{
	TipoPermiso:               true,
	TipoJustificacionFalta:    true,
	TipoJustificacionTardanza: true,
}

// EsF2 / EsF4 clasifican un tipo de trámite según a cuál de los 2 flujos
// institucionales con cierre confirmado pertenece (Etapa 4). Se usan para
// restringir operaciones que solo tienen sentido para uno de los dos
// flujos (RechazarExpediente es exclusivo de F4: F2 no tiene una regla de
// rechazo definida por la institución; ResolverExpediente es exclusivo de
// F2). No se aplican a los tipos genéricos ni a F1/F3/F5 (fuera de
// alcance).
func EsF2(tipo string) bool {
	return tiposF2[tipo]
}

func EsF4(tipo string) bool {
	return tiposF4[tipo]
}

// TipoDocumentoProveido: mismo valor ya definido en Documentos Service
// (internal/validation, chk_documentos_tipo) — se reutiliza para el
// "documento final" de F2 y el "proveído" de F4 (Etapa 4 pide reutilizar
// el mecanismo de documentos existente, no inventar un tipo nuevo).
const TipoDocumentoProveido = "PROVEIDO"
