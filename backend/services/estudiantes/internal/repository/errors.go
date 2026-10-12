package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrEstudianteNotFound = errors.New("estudiante not found")
	ErrCodigoExists       = errors.New("codigo already exists")
	// FASE E-05 (escritura del maestro).
	ErrDocumentoExists    = errors.New("documento already exists")
	ErrCodigoSiagieExists = errors.New("codigo_siagie already exists")
	ErrDocumentoInvalido  = errors.New("invalid documento")
	ErrDatosInvalidos     = errors.New("invalid data")
	ErrVersionConflict    = errors.New("version conflict")
	ErrMismoEstado        = errors.New("estado sin cambio")
)

// ClassifyUniqueViolation traduce la violación de UNIQUE(codigo) a un error
// de dominio (la usan también los fixtures de test).
func ClassifyUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	if pgErr.ConstraintName == "uq_estudiantes_codigo" {
		return ErrCodigoExists
	}
	return err
}

// classifyWriteError (FASE E-05) traduce las violaciones de restricciones a
// errores de dominio, identificando SOLO la restricción: nunca se propaga el
// detalle de PostgreSQL (que incluye el valor, p. ej. el DNI duplicado).
// La lógica valida antes todos los formatos; esto es la última defensa
// (p. ej. dos altas simultáneas con el mismo documento).
func classifyWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		switch pgErr.ConstraintName {
		case "uq_estudiantes_codigo":
			return ErrCodigoExists
		case "uq_estudiantes_documento":
			return ErrDocumentoExists
		case "uq_estudiantes_codigo_siagie":
			return ErrCodigoSiagieExists
		}
	case "23514", "23503":
		if pgErr.ConstraintName == "chk_estudiantes_documento" || pgErr.ConstraintName == "fk_estudiantes_tipo_documento" {
			return ErrDocumentoInvalido
		}
		return ErrDatosInvalidos
	case "22001", "22007", "22008":
		// Texto demasiado largo o fecha inválida que escapó a la validación.
		return ErrDatosInvalidos
	}
	return err
}

// SafeErrorSummary resume un error de base para los logs sin valores de
// usuario: SQLSTATE y nombre de la restricción, nunca Detail ni el mensaje.
func SafeErrorSummary(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return "postgres " + pgErr.Code + " " + pgErr.ConstraintName
	}
	return err.Error()
}

func classifyNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEstudianteNotFound
	}
	return err
}
