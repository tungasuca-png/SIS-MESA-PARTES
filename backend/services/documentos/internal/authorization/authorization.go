// Package authorization implementa un modelo de autorizacion PROPIO de
// Documentos Service, basado en el claim "role" del JWT emitido por Auth.
//
// PENDIENTE DE INTEGRACION CON AUTH: no existe todavia un mecanismo
// cross-service para consultar permisos reales "documentos.*" en Auth
// Service. Estas reglas son locales y deben reemplazarse cuando exista ese
// mecanismo compartido (igual que en Expedientes y Usuarios Service).
//
// El acceso a cada expediente (propietario, área, alcance global) lo decide
// Expedientes Service: este servicio lo consulta con Expedientes.GetExpediente
// / ListExpedientes reenviando el mismo JWT (ver DebeVerificarExpediente).
package authorization

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

// CanUpload: cualquier usuario autenticado puede subir documentos, pero un
// SOLICITANTE solo puede subir tipo ADJUNTO (sustentos); el personal interno
// puede subir cualquier tipo (proveidos, actas, informes, formatos).
func CanUpload(role, tipoDocumento string) bool {
	if IsInternal(role) {
		return true
	}
	return tipoDocumento == "ADJUNTO"
}

// CanDelete: solo el personal interno puede eliminar (baja logica) un
// documento ya cargado.
func CanDelete(role string) bool {
	return IsInternal(role)
}

// CanViewAll: rol del personal interno. NO decide qué documentos se ven
// (ver DebeVerificarExpediente); se conserva por compatibilidad.
func CanViewAll(role string) bool {
	return IsInternal(role)
}

// AlcanceGlobalExpedientes replica authorization.CanViewAll de Expedientes
// Service: solo ADMIN y SECRETARIA ven todos los expedientes. DIRECTOR,
// SUBDIRECTOR, DOCENTE y AUXILIAR ven solo lo derivado a su área, y un
// SOLICITANTE solo lo suyo. El rol sale del JWT ya validado (firmado por Auth).
func AlcanceGlobalExpedientes(role string) bool {
	return role == "ADMIN" || role == "SECRETARIA"
}

// DebeVerificarExpediente: "documentos.view" permite ver los documentos de
// los expedientes que el usuario PUEDE ver, no los de toda la institución.
// Solo con alcance global se omite la consulta a Expedientes; en cualquier
// otro caso el acceso se confirma allí (fail-closed). Antes, cualquier rol
// con "documentos.view" (p. ej. DOCENTE) listaba y descargaba documentos de
// expedientes ajenos a su área.
func DebeVerificarExpediente(hasViewAll bool, role string) bool {
	return !hasViewAll || !AlcanceGlobalExpedientes(role)
}
