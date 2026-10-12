package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// FASE E-08 — dominio de DATOS PERSONALES del estudiante. Separado de la
// identidad básica por permiso ("estudiantes.personal.view/.update"). No
// incluye foto_ref (fuera de este dominio en E-08).

const (
	AccionPersonalView   = "PERSONAL_VIEW"
	AccionPersonalUpdate = "PERSONAL_UPDATE"
)

// CamposPersonales son los nombres de columna del dominio (van a la
// auditoría como nombres, nunca como valores).
// Modelo maestro (etapa 2): el dominio incluye domicilio y contacto (E-02).
var CamposPersonales = []string{
	"fecha_nacimiento", "sexo", "pais_nacimiento", "ubigeo_nacimiento", "lengua_materna", "segunda_lengua",
	"domicilio_direccion", "domicilio_departamento", "domicilio_provincia", "domicilio_distrito",
	"domicilio_ubigeo", "correo", "telefono",
}

// Personal: nil = no registrado (NULL).
type Personal struct {
	Version          int
	FechaNacimiento  *string // AAAA-MM-DD
	Sexo             *string
	PaisNacimiento   *string
	UbigeoNacimiento *string
	LenguaMaterna    *string
	SegundaLengua    *string
	// Domicilio vigente y contacto (etapa 2).
	DomicilioDireccion    *string
	DomicilioDepartamento *string
	DomicilioProvincia    *string
	DomicilioDistrito     *string
	DomicilioUbigeo       *string
	Correo                *string
	Telefono              *string
}

// personalColumns / scanPersonal: una sola definición del orden de columnas
// para la lectura y para el bloqueo previo a la edición.
const personalColumns = `
	version, to_char(fecha_nacimiento, 'YYYY-MM-DD'), sexo, pais_nacimiento, ubigeo_nacimiento,
	lengua_materna, segunda_lengua, domicilio_direccion, domicilio_departamento, domicilio_provincia,
	domicilio_distrito, domicilio_ubigeo, correo, telefono
`

func scanPersonal(row pgx.Row) (*Personal, error) {
	p := &Personal{}
	err := row.Scan(&p.Version, &p.FechaNacimiento, &p.Sexo, &p.PaisNacimiento, &p.UbigeoNacimiento,
		&p.LenguaMaterna, &p.SegundaLengua, &p.DomicilioDireccion, &p.DomicilioDepartamento,
		&p.DomicilioProvincia, &p.DomicilioDistrito, &p.DomicilioUbigeo, &p.Correo, &p.Telefono)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetPersonal lee los datos personales y registra la lectura (PERSONAL_VIEW)
// en la MISMA transacción: si la auditoría no se puede escribir, no se
// devuelve nada (fail-closed). version_resultante = versión observada; la
// lectura no modifica el registro ni su versión.
func (r *EstudianteRepository) GetPersonal(ctx context.Context, id string, aud Auditoria) (*Personal, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	p, err := scanPersonal(tx.QueryRow(ctx, `SELECT `+personalColumns+` FROM estudiantes WHERE id = $1`, id))
	if err != nil {
		return nil, classifyNotFound(err)
	}
	aud.Accion = AccionPersonalView
	aud.Campos = CamposPersonales
	if err := insertAuditoria(ctx, tx, id, p.Version, aud); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePersonal: edición parcial de los datos personales con el mismo
// control optimista que Update (FOR UPDATE + comparación de versión + UPDATE
// ... WHERE version = $2) y auditoría PERSONAL_UPDATE en la misma
// transacción. El UPDATE escribe EXCLUSIVAMENTE las columnas del dominio
// personal (CamposPersonales); la versión la incrementa el trigger. Sin
// cambios reales no se escribe ni se audita.
func (r *EstudianteRepository) UpdatePersonal(ctx context.Context, id string, expectedVersion int, aud Auditoria,
	apply func(p *Personal) ([]string, error)) (*Estudiante, error) {
	return r.inTx(ctx, func(tx pgx.Tx) (string, error) {
		p, err := scanPersonal(tx.QueryRow(ctx, `SELECT `+personalColumns+` FROM estudiantes WHERE id = $1 FOR UPDATE`, id))
		if err != nil {
			return "", classifyNotFound(err)
		}
		if p.Version != expectedVersion {
			return "", ErrVersionConflict
		}
		campos, err := apply(p)
		if err != nil {
			return "", err
		}
		if len(campos) == 0 {
			return id, nil
		}
		var version int
		err = tx.QueryRow(ctx, `
			UPDATE estudiantes SET
				fecha_nacimiento = $3::date, sexo = $4, pais_nacimiento = $5,
				ubigeo_nacimiento = $6, lengua_materna = $7, segunda_lengua = $8,
				domicilio_direccion = $9, domicilio_departamento = $10, domicilio_provincia = $11,
				domicilio_distrito = $12, domicilio_ubigeo = $13, correo = $14, telefono = $15
			WHERE id = $1 AND version = $2
			RETURNING version
		`, id, expectedVersion, p.FechaNacimiento, p.Sexo, p.PaisNacimiento,
			p.UbigeoNacimiento, p.LenguaMaterna, p.SegundaLengua,
			p.DomicilioDireccion, p.DomicilioDepartamento, p.DomicilioProvincia,
			p.DomicilioDistrito, p.DomicilioUbigeo, p.Correo, p.Telefono).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrVersionConflict
		}
		if err != nil {
			return "", classifyWriteError(err)
		}
		aud.Accion = AccionPersonalUpdate
		aud.Campos = campos
		return id, insertAuditoria(ctx, tx, id, version, aud)
	})
}
