// Package authorization implementa un modelo de autorización PROPIO de
// Expedientes Service, basado únicamente en el rol que viaja en el claim
// "role" del JWT emitido por Auth Service.
//
// Esto NO es una integración con las tablas permissions/role_permissions
// de Auth Service: hoy no existe un mecanismo cross-service para que otro
// microservicio consulte los permisos reales de un usuario, y el proyecto
// no tiene dados de alta permisos "expedientes.*" en Auth. Este paquete es
// una propuesta explícita y aislada para esta primera etapa, pensada para
// reemplazarse el día que exista un mecanismo real de autorización
// compartido entre microservicios.
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

// IsInternal indica si el rol pertenece al personal de la institución
// (en contraposición al SOLICITANTE externo).
func IsInternal(role string) bool {
	return internalRoles[role]
}

// CanCreate: cualquier usuario autenticado (solicitante o personal interno)
// puede registrar un expediente.
func CanCreate(role string) bool {
	return role == RoleSolicitante || IsInternal(role)
}

// CanUpdate: solo el personal interno puede editar asunto/descripcion/prioridad.
func CanUpdate(role string) bool {
	return IsInternal(role)
}

// CanChangeEstado: solo el personal interno puede mover el expediente entre estados.
func CanChangeEstado(role string) bool {
	return IsInternal(role)
}

// CanViewAll: el personal interno ve todos los expedientes; un SOLICITANTE
// solo debe ver los suyos (el llamador debe filtrar por solicitante_id
// cuando esta función devuelve false).
func CanViewAll(role string) bool {
	return IsInternal(role)
}
