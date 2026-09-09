// Package authorization implementa un modelo de autorización PROPIO de
// Derivaciones Service, basado únicamente en el rol que viaja en el claim
// "role" del JWT emitido por Auth Service.
//
// PENDIENTE DE INTEGRACION CON AUTH: no existe todavía un mecanismo
// cross-service para consultar permisos reales "derivaciones.*" en Auth
// Service. Estas reglas son locales y deben reemplazarse cuando exista ese
// mecanismo compartido (igual que en Expedientes, Usuarios y Documentos).
//
// LIMITACION CONOCIDA (v1): este servicio no verifica si el expediente_id
// indicado realmente pertenece al solicitante autenticado -- eso requeriria
// una llamada cross-service a Expedientes Service que no se implementa en
// esta etapa (mismo patron ya documentado en Documentos Service).
package authorization

const RoleSolicitante = "SOLICITANTE"

var internalRoles = map[string]bool{
	"ADMIN":       true,
	"DIRECTOR":    true,
	"SUBDIRECTOR": true,
	"SECRETARIA":  true,
	"DOCENTE":     true,
	"AUXILIAR":    true,
}

func IsInternal(role string) bool {
	return internalRoles[role]
}

// CanCreate: registrar una derivación es una acción interna (mover un
// expediente de un área a otra, notificar, aprobar, etc.) — un SOLICITANTE
// no decide el ruteo interno de su propio trámite.
func CanCreate(role string) bool {
	return IsInternal(role)
}

// CanViewAll: el personal interno puede listar derivaciones de cualquier
// expediente sin filtro adicional. Un SOLICITANTE debe indicar siempre un
// expediente_id puntual (ver la limitación documentada arriba) — no puede
// pedir el listado global.
func CanViewAll(role string) bool {
	return IsInternal(role)
}
