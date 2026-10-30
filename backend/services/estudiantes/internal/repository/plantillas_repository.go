package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Plantillas del motor de datos (migración 010).

var (
	ErrPlantillaNotFound  = errors.New("plantilla no encontrada")
	ErrPlantillaDuplicada = errors.New("plantilla duplicada")
	ErrPlantillaVersion   = errors.New("plantilla modificada")
	ErrPlantillasSinTabla = errors.New("plantillas no instaladas")
)

type Plantilla struct {
	ID             string
	Modulo         string
	Tipo           string // IMPORTACION | EXPORTACION
	Nombre         string
	Formato        string
	Configuracion  []byte // JSON
	Version        int
	CreadoPor      string
	ActualizadoPor string
	CreatedAt      string
	UpdatedAt      string
}

const plantillaColumnas = `id, modulo, tipo, nombre, COALESCE(formato, ''), configuracion, version, creado_por,
	actualizado_por, to_char(created_at AT TIME ZONE 'America/Lima', 'YYYY-MM-DD HH24:MI'),
	to_char(updated_at AT TIME ZONE 'America/Lima', 'YYYY-MM-DD HH24:MI')`

func scanPlantilla(row pgx.Row) (*Plantilla, error) {
	p := &Plantilla{}
	err := row.Scan(&p.ID, &p.Modulo, &p.Tipo, &p.Nombre, &p.Formato, &p.Configuracion, &p.Version,
		&p.CreadoPor, &p.ActualizadoPor, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, clasificarPlantilla(err)
	}
	return p, nil
}

// clasificarPlantilla traduce errores conocidos (tabla aún no creada,
// nombre repetido, inexistente).
func clasificarPlantilla(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPlantillaNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42P01": // undefined_table: falta aplicar la migración 010
			return ErrPlantillasSinTabla
		case "23505":
			return ErrPlantillaDuplicada
		case "23514":
			return ErrDatosInvalidos
		}
	}
	return err
}

func (r *EstudianteRepository) ListPlantillas(ctx context.Context, modulo, tipo string) ([]*Plantilla, error) {
	rows, err := r.db.Query(ctx, `SELECT `+plantillaColumnas+` FROM plantillas_datos
		WHERE modulo = $1 AND ($2 = '' OR tipo = $2) ORDER BY tipo, lower(nombre)`, modulo, tipo)
	if err != nil {
		return nil, clasificarPlantilla(err)
	}
	defer rows.Close()
	var out []*Plantilla
	for rows.Next() {
		p, err := scanPlantilla(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, clasificarPlantilla(rows.Err())
}

func (r *EstudianteRepository) GetPlantilla(ctx context.Context, id, modulo string) (*Plantilla, error) {
	return scanPlantilla(r.db.QueryRow(ctx, `SELECT `+plantillaColumnas+` FROM plantillas_datos WHERE id = $1 AND modulo = $2`, id, modulo))
}

func (r *EstudianteRepository) CrearPlantilla(ctx context.Context, p Plantilla) (*Plantilla, error) {
	return scanPlantilla(r.db.QueryRow(ctx, `
		INSERT INTO plantillas_datos (modulo, tipo, nombre, formato, configuracion, creado_por, actualizado_por)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $6)
		RETURNING `+plantillaColumnas, p.Modulo, p.Tipo, p.Nombre, p.Formato, p.Configuracion, p.CreadoPor))
}

// ActualizarPlantilla con control de versión (otra persona pudo cambiarla).
func (r *EstudianteRepository) ActualizarPlantilla(ctx context.Context, p Plantilla, versionEsperada int) (*Plantilla, error) {
	out, err := scanPlantilla(r.db.QueryRow(ctx, `
		UPDATE plantillas_datos SET nombre = $3, formato = NULLIF($4, ''), configuracion = $5,
			actualizado_por = $6, version = version + 1
		WHERE id = $1 AND modulo = $2 AND tipo = $7 AND version = $8
		RETURNING `+plantillaColumnas, p.ID, p.Modulo, p.Nombre, p.Formato, p.Configuracion, p.ActualizadoPor, p.Tipo, versionEsperada))
	if errors.Is(err, ErrPlantillaNotFound) {
		if _, e := r.GetPlantilla(ctx, p.ID, p.Modulo); e == nil {
			return nil, ErrPlantillaVersion
		}
	}
	return out, err
}

func (r *EstudianteRepository) EliminarPlantilla(ctx context.Context, id, modulo, tipo string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM plantillas_datos WHERE id = $1 AND modulo = $2 AND tipo = $3`, id, modulo, tipo)
	if err != nil {
		return clasificarPlantilla(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPlantillaNotFound
	}
	return nil
}
