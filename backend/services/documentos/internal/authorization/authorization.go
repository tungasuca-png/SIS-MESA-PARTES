// Package authorization implementa un modelo de autorizacion PROPIO de
// Documentos Service, basado en el claim "role" del JWT emitido por Auth.
//
// PENDIENTE DE INTEGRACION CON AUTH: no existe todavia un mecanismo
// cross-service para consultar permisos reales "documentos.*" en Auth
// Service. Estas reglas son locales y deben reemplazarse cuando exista ese
// mecanismo compartido (igual que en Expedientes y Usuarios Service).
//
// LIMITACION CONOCIDA (v1): este servicio no verifica si el expediente_id
// indicado realmente pertenece al solicitante autenticado -- eso requeriria
// una llamada cross-service a Expedientes Service que no se implementa en
// esta etapa. Un SOLICITANTE autenticado puede listar/ver/descargar
// documentos de cualquier expediente si conoce su id. Documentado como
// pendiente para una proxima etapa (ver README).
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

// CanViewAll: el personal interno puede listar documentos de cualquier
// expediente sin filtro adicional. Un SOLICITANTE que liste SIN indicar un
// expediente_id especifico se acota a "subido_por = el mismo" (ver
// ListFilter.SubidoPor en el repositorio), ya que este servicio no puede
// verificar de quien es cada expediente (limitacion ya documentada arriba).
func CanViewAll(role string) bool {
	return IsInternal(role)
}
