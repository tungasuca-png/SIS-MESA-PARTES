package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrExpedienteNotFound = errors.New("expediente not found")
	ErrCodigoExists       = errors.New("codigo already exists")
)

func classifyUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}

	if pgErr.ConstraintName == "uq_expedientes_codigo" {
		return ErrCodigoExists
	}
	return err
}

func classifyNotFound(err error, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}
