package svc

import (
	"errors"
	"strings"

	"github.com/zeromicro/go-zero/core/service"
)

// Política de autenticación de Psicología frente a Estudiantes.
//
//   - Por defecto (y siempre fuera de Mode dev/test): client credentials
//     OBLIGATORIAS. Sin PSICOLOGIA_CLIENT_ID y PSICOLOGIA_CLIENT_SECRET el
//     servicio no inicia; nunca se reenvía el JWT del usuario.
//   - Solo en desarrollo (Mode dev o test) y SOLO con
//     PSICOLOGIA_ALLOW_LEGACY_AUTH=true explícito, se permite el modo anterior
//     (reenviar el JWT del usuario; exige estudiantes.view al usuario).
//   - Una sola de las dos credenciales es siempre un error.

type ModoEstudiantes int

const (
	ModoServicio ModoEstudiantes = iota + 1
	ModoUsuarioLegado
)

func (m ModoEstudiantes) String() string {
	switch m {
	case ModoServicio:
		return "sistema consumidor (client credentials)"
	case ModoUsuarioLegado:
		return "modo anterior: JWT del usuario (solo desarrollo)"
	}
	return "desconocido"
}

// ResolverModoEstudiantes decide el modo a partir del Mode de go-zero y de la
// configuración. Los mensajes de error nunca incluyen valores.
func ResolverModoEstudiantes(mode, clientID, clientSecret, allowLegacy string) (ModoEstudiantes, error) {
	permitirLegado, err := parseBooleano(allowLegacy)
	if err != nil {
		return 0, err
	}
	tieneID, tieneSecreto := strings.TrimSpace(clientID) != "", strings.TrimSpace(clientSecret) != ""
	switch {
	case tieneID && tieneSecreto:
		return ModoServicio, nil
	case tieneID != tieneSecreto:
		return 0, errors.New("PSICOLOGIA_CLIENT_ID y PSICOLOGIA_CLIENT_SECRET deben configurarse juntos")
	case !permitirLegado:
		return 0, errors.New("faltan PSICOLOGIA_CLIENT_ID y PSICOLOGIA_CLIENT_SECRET (credenciales de Psicología para Estudiantes); " +
			"solo en desarrollo (Mode dev o test) puede usarse PSICOLOGIA_ALLOW_LEGACY_AUTH=true")
	case mode != service.DevMode && mode != service.TestMode:
		return 0, errors.New("PSICOLOGIA_ALLOW_LEGACY_AUTH solo se admite con Mode dev o test; en este entorno las credenciales de servicio son obligatorias")
	}
	return ModoUsuarioLegado, nil
}

func parseBooleano(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	}
	return false, errors.New("PSICOLOGIA_ALLOW_LEGACY_AUTH debe ser true o false")
}
