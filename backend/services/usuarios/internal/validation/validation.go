package validation

import (
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsValidUUID valida el formato del identificador que Auth Service asigna al
// usuario. Usuarios Service nunca genera este UUID: solo lo recibe.
func IsValidUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}

// TiposUsuario refleja los tipos manejados hoy por el sistema. NO reemplaza a
// los roles de Auth: describe la identidad/perfil del usuario, mientras que la
// autorización sigue viviendo en Auth Service.
var tiposUsuario = map[string]bool{
	"ADMIN":       true,
	"DIRECTOR":    true,
	"SUBDIRECTOR": true,
	"SECRETARIA":  true,
	"DOCENTE":     true,
	"AUXILIAR":    true,
	"SOLICITANTE": true,
}

func IsValidTipoUsuario(value string) bool {
	return tiposUsuario[value]
}
