package validation

import (
	"net/mail"
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

// IsValidEmail (Paso 22E) valida el formato del correo de CONTACTO del
// perfil (campo "correo" de Usuarios Service, distinto del email de
// autenticación de Auth Service). Mismo criterio ya usado por
// Auth/RegisterLogic: net/mail.ParseAddress, exigiendo que la dirección
// parseada sea idéntica al valor recibido (rechaza formatos con nombre de
// display, ej. "Juan <juan@test.com>").
func IsValidEmail(value string) bool {
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}
