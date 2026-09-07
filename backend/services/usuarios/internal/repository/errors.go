package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUsuarioNotFound = errors.New("usuario not found")
	ErrDniExists       = errors.New("dni already exists")
)

func classifyUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}

	if pgErr.ConstraintName == "uq_usuarios_dni" {
		return ErrDniExists
	}
	return err
}

func classifyNotFound(err error, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}
