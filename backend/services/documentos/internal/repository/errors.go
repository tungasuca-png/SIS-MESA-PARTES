package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrDocumentoNotFound = errors.New("documento not found")

func classifyNotFound(err error, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}
