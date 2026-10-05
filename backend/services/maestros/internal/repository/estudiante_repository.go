package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Estudiante struct {
	ID                  string
	PersonaID           string
	CodigoInstitucional string
	CodigoSiagie        *string
	EstadoAcademico     string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type EstudianteRepository struct {
	db *pgxpool.Pool
}

func NewEstudianteRepository(
	db *pgxpool.Pool,
) *EstudianteRepository {
	return &EstudianteRepository{
		db: db,
	}
}

const estudianteColumns = `
	id,
	persona_id,
	codigo_institucional,
	codigo_siagie,
	estado_academico,
	created_at,
	updated_at
`

func scanEstudiante(row pgx.Row) (*Estudiante, error) {
	e := &Estudiante{}

	err := row.Scan(
		&e.ID,
		&e.PersonaID,
		&e.CodigoInstitucional,
		&e.CodigoSiagie,
		&e.EstadoAcademico,
		&e.CreatedAt,
		&e.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return e, nil
}

func (r *EstudianteRepository) FindByID(
	ctx context.Context,
	id string,
) (*Estudiante, error) {

	return scanEstudiante(
		r.db.QueryRow(
			ctx,
			`SELECT `+estudianteColumns+`
			 FROM estudiantes
			 WHERE id = $1`,
			id,
		),
	)
}
