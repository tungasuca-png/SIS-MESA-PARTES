package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Persona struct {
	ID              string
	TipoDocumento   string
	NumeroDocumento string
	Nombres         string
	ApellidoPaterno string
	ApellidoMaterno string
	FechaNacimiento *time.Time
	Sexo            *string
	CorreoPersonal  *string
	Telefono        *string
	Activo          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PersonaRepository struct {
	db *pgxpool.Pool
}

func NewPersonaRepository(db *pgxpool.Pool) *PersonaRepository {
	return &PersonaRepository{db: db}
}

const personaColumns = `
	id,
	tipo_documento,
	numero_documento,
	nombres,
	apellido_paterno,
	apellido_materno,
	fecha_nacimiento,
	sexo,
	correo_personal,
	telefono,
	activo,
	created_at,
	updated_at
`

func scanPersona(row pgx.Row) (*Persona, error) {
	p := &Persona{}

	err := row.Scan(
		&p.ID,
		&p.TipoDocumento,
		&p.NumeroDocumento,
		&p.Nombres,
		&p.ApellidoPaterno,
		&p.ApellidoMaterno,
		&p.FechaNacimiento,
		&p.Sexo,
		&p.CorreoPersonal,
		&p.Telefono,
		&p.Activo,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return p, nil
}

func (r *PersonaRepository) FindByID(
	ctx context.Context,
	id string,
) (*Persona, error) {

	return scanPersona(
		r.db.QueryRow(
			ctx,
			`SELECT `+personaColumns+`
			 FROM personas
			 WHERE id = $1`,
			id,
		),
	)
}