// hashgen: genera el hash bcrypt de una contraseña para cargarlo a mano en
// auth_db. La contraseña NUNCA va en el código ni como argumento (quedaría en
// el historial de la consola): se pide por teclado sin mostrarla, o se lee de
// la entrada estándar si viene por tubería.
//
//	go run ./cmd/hashgen
//	Get-Content clave.txt | go run ./cmd/hashgen
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

// Mismos límites que exige Auth al registrar contraseñas: bcrypt ignora lo
// que pase de 72 bytes.
const (
	minLongitud = 8
	maxBytes    = 72
)

func main() {
	password, err := leerContrasena()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Println(string(hash))
}

func leerContrasena() (string, error) {
	fd := int(os.Stdin.Fd())
	var password string
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Contraseña: ")
		primera, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		fmt.Fprint(os.Stderr, "Repetir contraseña: ")
		segunda, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if string(primera) != string(segunda) {
			return "", errors.New("las contraseñas no coinciden")
		}
		password = string(primera)
	} else {
		linea, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && linea == "" {
			return "", errors.New("no se recibió ninguna contraseña por la entrada estándar")
		}
		password = strings.TrimRight(linea, "\r\n")
	}

	if len(password) < minLongitud {
		return "", fmt.Errorf("la contraseña debe tener al menos %d caracteres", minLongitud)
	}
	if len(password) > maxBytes {
		return "", fmt.Errorf("la contraseña no puede superar %d bytes", maxBytes)
	}
	return password, nil
}
