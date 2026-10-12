package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrExpedienteNotFound          = errors.New("expediente psicologico not found")
	ErrEstudianteYaTieneExpediente = errors.New("el estudiante ya tiene un expediente psicologico")
	ErrEstadoInvalido              = errors.New("transicion de estado invalida")
)

// classifyUniqueViolation traduce la violación de UNIQUE(estudiante_id) —la
// garantía final de "1 estudiante = 1 expediente"— a un error de dominio, sin
// exponer nunca el mensaje de PostgreSQL al cliente.
func classifyUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	if pgErr.ConstraintName == "uq_expedientes_psicologicos_estudiante" {
		return ErrEstudianteYaTieneExpediente
	}
	return err
}

func classifyNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrExpedienteNotFound
	}
	return err
}
