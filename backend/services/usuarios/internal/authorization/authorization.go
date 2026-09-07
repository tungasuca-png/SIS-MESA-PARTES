// Package authorization implementa un modelo de autorización PROPIO de
// Usuarios Service, basado en el claim "role" del JWT emitido por Auth.
//
// PENDIENTE DE INTEGRACIÓN CON AUTH: hoy no existe un mecanismo cross-service
// para consultar las tablas permissions/role_permissions de Auth Service, ni
// existen permisos "usuarios.*" dados de alta ahí. Estas reglas son locales y
// deben reemplazarse cuando exista ese mecanismo compartido.
package authorization

const RoleAdmin = "ADMIN"

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

// CanViewBasic: cualquier usuario autenticado puede resolver nombre y tipo de
// otro usuario. Es el dato mínimo necesario para mostrar "Solicitante: Juan
// Pérez" en expedientes, derivaciones o seguimiento; no incluye DNI, teléfono,
// correo ni dirección.
func CanViewBasic() bool {
	return true
}

// CanViewFullProfile: el perfil completo (DNI, teléfono, correo, dirección)
// solo lo ve el personal interno o el propio usuario sobre sí mismo.
func CanViewFullProfile(role, requesterID, targetID string) bool {
	return IsInternal(role) || requesterID == targetID
}

// CanListOrSearch: el listado/búsqueda administrativa de usuarios queda
// restringido a ADMIN para no exponer datos personales de forma masiva.
func CanListOrSearch(role string) bool {
	return role == RoleAdmin
}

// CanUpsert: solo ADMIN administra perfiles en esta etapa. La edición del
// perfil propio por parte del usuario no está definida todavía.
func CanUpsert(role string) bool {
	return role == RoleAdmin
}
