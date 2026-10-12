package logic

import (
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// dniPattern: DNI peruano = exactamente 8 dígitos. Es un identificador, no una
// cantidad: siempre texto (conserva ceros a la izquierda).
var dniPattern = regexp.MustCompile(`^[0-9]{8}$`)

// normalizeDNI recorta espacios alrededor y valida el formato. No modifica
// nada más (un DNI con espacios internos o letras es inválido).
func normalizeDNI(raw string) (string, error) {
	dni := strings.TrimSpace(raw)
	if dni == "" {
		return "", status.Error(codes.InvalidArgument, "el DNI es obligatorio")
	}
	if !dniPattern.MatchString(dni) {
		return "", status.Error(codes.InvalidArgument, "el DNI debe tener 8 dígitos numéricos")
	}
	return dni, nil
}
