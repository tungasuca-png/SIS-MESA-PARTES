package authorization

// AlcanceGlobalExpedientes replica authorization.CanViewAll de Expedientes
// Service: solo ADMIN y SECRETARIA ven todos los expedientes; DIRECTOR,
// SUBDIRECTOR, DOCENTE y AUXILIAR ven lo derivado a su área y un SOLICITANTE
// lo suyo. No decide PERMISOS (eso es Auth.HasPermission): solo si el listado
// general de derivaciones puede omitir el filtro por los expedientes que el
// usuario puede ver. El rol sale del JWT ya validado (firmado por Auth).
func AlcanceGlobalExpedientes(role string) bool {
	return role == "ADMIN" || role == "SECRETARIA"
}
