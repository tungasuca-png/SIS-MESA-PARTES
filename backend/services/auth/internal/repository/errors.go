package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUsernameExists    = errors.New("username already exists")
	ErrEmailExists       = errors.New("email already exists")
	ErrDNIExists         = errors.New("dni already exists")
	ErrRoleNotFound      = errors.New("role not found")
	ErrRoleAssignment    = errors.New("role assignment failed")
	ErrPermissionNotFound = errors.New("permission not found")
	ErrUserInactive      = errors.New("user inactive")
	ErrRoleInactive      = errors.New("role inactive")
)

func classifyUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}

	switch pgErr.ConstraintName {
	case "uq_usuarios_username":
		return ErrUsernameExists
	case "uq_usuarios_email":
		return ErrEmailExists
	case "uq_usuarios_dni":
		return ErrDNIExists
	default:
		return err
	}
}

func classifyNotFound(err error, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}

// EsquemaPendiente: la consulta usa una tabla de una migración que todavía no
// se aplicó (PostgreSQL 42P01, undefined_table). Las funciones nuevas del
// registro público (024, 025) se desactivan con un error controlado y el
// login sigue funcionando: nunca un 500 por una migración pendiente.
func EsquemaPendiente(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}
