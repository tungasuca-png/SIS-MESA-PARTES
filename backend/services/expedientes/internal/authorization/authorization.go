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
const RoleSecretaria = "SECRETARIA"

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

// CanDelete: solo el personal interno puede eliminar (baja logica) un
// expediente.
func CanDelete(role string) bool {
	return IsInternal(role)
}

// CanUpdateArea: solo el personal interno puede cambiar el área responsable
// de un expediente — se llama al registrar una derivación real (ver
// Derivaciones Service, tipo = 'DERIVACION').
func CanUpdateArea(role string) bool {
	return IsInternal(role)
}

// CanViewAll: quien ve TODOS los expedientes sin importar el área actual.
// Secretaría es la mesa de partes (punto de entrada de todo trámite nuevo,
// ver migración 002_add_area_actual.sql) y Admin supervisa el sistema
// completo; el resto del personal interno solo debe ver lo que ya se le
// derivó a su propia área (ver AreaDelRol) — antes veían todo, lo que
// permitía que, por ejemplo, Dirección viera una solicitud recién
// presentada antes de que Secretaría la derivara.
func CanViewAll(role string) bool {
	return role == "ADMIN" || role == RoleSecretaria
}

// AreaDelRol: el área cuyos expedientes puede ver un rol interno que NO
// tiene CanViewAll (Director solo ve lo derivado a Dirección, etc). Los
// valores coinciden exactamente con los que acepta la columna area_actual
// (mismos códigos que el rol). Devuelve "" para un rol sin área propia
// mapeada (no debería llegar a usarse si CanViewAll ya es true).
func AreaDelRol(role string) string {
	switch role {
	case "DIRECTOR", "SUBDIRECTOR", "DOCENTE", "AUXILIAR":
		return role
	default:
		return ""
	}
}
