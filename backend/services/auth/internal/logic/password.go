package logic

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	minPasswordLength = 8
	// maxPasswordBytes: bcrypt solo procesa 72 bytes y GenerateFromPassword
	// rechaza más con un error interno. Antes eso llegaba al cliente como
	// 500; ahora es un 400 explícito antes de calcular ningún hash.
	maxPasswordBytes = 72
)

// validarPassword aplica las reglas de longitud de una contraseña nueva.
func validarPassword(password string) error {
	if len(password) < minPasswordLength {
		return status.Error(codes.InvalidArgument, "la contraseña debe tener al menos 8 caracteres")
	}
	if len(password) > maxPasswordBytes {
		return status.Error(codes.InvalidArgument, "la contraseña no debe superar los 72 bytes")
	}
	return nil
}
