package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Maestro (FASE E-05) es el registro maestro básico que la escritura puede
// fijar: identidad y estado del registro. Los opcionales son *string (nil =
// NULL). No incluye nivel/grado/seccion/turno/estado_matricula: esos campos
// no se escriben desde el maestro (pasarán al dominio Matrícula).
type Maestro struct {
	Codigo           string
	CodigoSiagie     *string
	TipoDocumento    string
	NumeroDocumento  *string
	Nombres          string
	ApellidoPaterno  *string
	ApellidoMaterno  *string
	Apellidos        string
	FechaNacimiento  *string // AAAA-MM-DD
	Sexo             *string
	PaisNacimiento   *string
	UbigeoNacimiento *string
	LenguaMaterna    *string
	SegundaLengua    *string
	FotoRef          *string
	Estado           string
	Version          int
}

// Auditoria es lo que se registra en auditoria_estudiantes: nunca valores,
// solo nombres de campos (ver migración 004).
type Auditoria struct {
	UsuarioID      string
	Accion         string // CREATE | UPDATE | CHANGE_ESTADO
	Campos         []string
	EstadoAnterior string
	EstadoNuevo    string
}

const (
	AccionCreate       = "CREATE"
	AccionUpdate       = "UPDATE"
	AccionChangeEstado = "CHANGE_ESTADO"
)

const maestroColumns = `
	codigo, codigo_siagie, tipo_documento, numero_documento, nombres, apellido_paterno,
	apellido_materno, apellidos, to_char(fecha_nacimiento, 'YYYY-MM-DD'), sexo, pais_nacimiento,
	ubigeo_nacimiento, lengua_materna, segunda_lengua, foto_ref, estado, version
`

func scanMaestro(row pgx.Row) (*Maestro, error) {
	m := &Maestro{}
	err := row.Scan(&m.Codigo, &m.CodigoSiagie, &m.TipoDocumento, &m.NumeroDocumento, &m.Nombres,
		&m.ApellidoPaterno, &m.ApellidoMaterno, &m.Apellidos, &m.FechaNacimiento, &m.Sexo,
		&m.PaisNacimiento, &m.UbigeoNacimiento, &m.LenguaMaterna, &m.SegundaLengua, &m.FotoRef,
		&m.Estado, &m.Version)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Create inserta el estudiante y su registro de auditoría en UNA transacción:
// o quedan ambos o ninguno. id y version los asigna la base.
func (r *EstudianteRepository) Create(ctx context.Context, m Maestro, aud Auditoria) (*Estudiante, error) {
	return r.inTx(ctx, func(tx pgx.Tx) (string, error) {
		var id string
		var version int
		err := tx.QueryRow(ctx, `
			INSERT INTO estudiantes (codigo, codigo_siagie, tipo_documento, numero_documento, nombres,
				apellido_paterno, apellido_materno, apellidos, fecha_nacimiento, sexo, pais_nacimiento,
				ubigeo_nacimiento, lengua_materna, segunda_lengua, foto_ref, estado)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::date, $10, $11, $12, $13, $14, $15, $16)
			RETURNING id, version
		`, m.Codigo, m.CodigoSiagie, m.TipoDocumento, m.NumeroDocumento, m.Nombres,
			m.ApellidoPaterno, m.ApellidoMaterno, m.Apellidos, m.FechaNacimiento, m.Sexo, m.PaisNacimiento,
			m.UbigeoNacimiento, m.LenguaMaterna, m.SegundaLengua, m.FotoRef, m.Estado).Scan(&id, &version)
		if err != nil {
			return "", classifyWriteError(err)
		}
		return id, insertAuditoria(ctx, tx, id, version, aud)
	})
}

// Update aplica un cambio parcial con control de concurrencia optimista:
//  1. bloquea la fila (SELECT ... FOR UPDATE): distingue "no existe"
//     (ErrEstudianteNotFound) de "cambió desde que el cliente la leyó"
//     (ErrVersionConflict) sin revelar nada más;
//  2. apply (la lógica) combina y valida el estado final y devuelve los
//     nombres de los campos que cambian;
//  3. UPDATE ... WHERE id = $1 AND version = $2 (defensa adicional: 0 filas
//     afectadas = conflicto) — version la incrementa el trigger, nunca Go;
//  4. auditoría en la misma transacción.
//
// Si apply no cambia nada no se escribe ni se audita (no hay versión nueva).
func (r *EstudianteRepository) Update(ctx context.Context, id string, expectedVersion int, aud Auditoria,
	apply func(cur *Maestro) ([]string, error)) (*Estudiante, error) {
	return r.inTx(ctx, func(tx pgx.Tx) (string, error) {
		cur, err := lockMaestro(ctx, tx, id, expectedVersion)
		if err != nil {
			return "", err
		}
		campos, err := apply(cur)
		if err != nil {
			return "", err
		}
		if len(campos) == 0 {
			return id, nil
		}
		version, err := updateMaestro(ctx, tx, id, expectedVersion, cur)
		if err != nil {
			return "", err
		}
		aud.Campos = campos
		return id, insertAuditoria(ctx, tx, id, version, aud)
	})
}

// ChangeEstado cambia solo el estado del registro (ACTIVO/INACTIVO), con el
// mismo control de versión y auditoría que Update. Pedir el estado que ya
// tiene devuelve ErrMismoEstado (nada se escribe ni se audita).
func (r *EstudianteRepository) ChangeEstado(ctx context.Context, id string, expectedVersion int, estado string, aud Auditoria) (*Estudiante, error) {
	return r.inTx(ctx, func(tx pgx.Tx) (string, error) {
		cur, err := lockMaestro(ctx, tx, id, expectedVersion)
		if err != nil {
			return "", err
		}
		if cur.Estado == estado {
			return "", ErrMismoEstado
		}
		aud.EstadoAnterior, aud.EstadoNuevo = cur.Estado, estado
		cur.Estado = estado
		version, err := updateMaestro(ctx, tx, id, expectedVersion, cur)
		if err != nil {
			return "", err
		}
		aud.Campos = []string{"estado"}
		return id, insertAuditoria(ctx, tx, id, version, aud)
	})
}

// inTx ejecuta fn en una transacción y, tras el COMMIT, devuelve el registro
// básico resultante. Cualquier error hace ROLLBACK: nunca queda un estudiante
// modificado sin su auditoría ni una auditoría sin cambio.
func (r *EstudianteRepository) inTx(ctx context.Context, fn func(tx pgx.Tx) (string, error)) (*Estudiante, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	id, err := fn(tx)
	if err != nil {
		return nil, err
	}
	found, err := scanEstudiante(tx.QueryRow(ctx, `SELECT `+estudianteColumns+` FROM estudiantes WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, classifyWriteError(err)
	}
	return found, nil
}

func lockMaestro(ctx context.Context, tx pgx.Tx, id string, expectedVersion int) (*Maestro, error) {
	cur, err := scanMaestro(tx.QueryRow(ctx, `SELECT `+maestroColumns+` FROM estudiantes WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return nil, classifyNotFound(err)
	}
	if cur.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	return cur, nil
}

// updateMaestro escribe SOLO columnas del maestro (el código institucional y
// las columnas académicas no se tocan) y devuelve la versión nueva.
func updateMaestro(ctx context.Context, tx pgx.Tx, id string, expectedVersion int, m *Maestro) (int, error) {
	var version int
	err := tx.QueryRow(ctx, `
		UPDATE estudiantes SET
			codigo_siagie = $3, tipo_documento = $4, numero_documento = $5, nombres = $6,
			apellido_paterno = $7, apellido_materno = $8, apellidos = $9, fecha_nacimiento = $10::date,
			sexo = $11, pais_nacimiento = $12, ubigeo_nacimiento = $13, lengua_materna = $14,
			segunda_lengua = $15, foto_ref = $16, estado = $17
		WHERE id = $1 AND version = $2
		RETURNING version
	`, id, expectedVersion, m.CodigoSiagie, m.TipoDocumento, m.NumeroDocumento, m.Nombres,
		m.ApellidoPaterno, m.ApellidoMaterno, m.Apellidos, m.FechaNacimiento, m.Sexo, m.PaisNacimiento,
		m.UbigeoNacimiento, m.LenguaMaterna, m.SegundaLengua, m.FotoRef, m.Estado).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrVersionConflict
	}
	if err != nil {
		return 0, classifyWriteError(err)
	}
	return version, nil
}

func insertAuditoria(ctx context.Context, tx pgx.Tx, estudianteID string, version int, aud Auditoria) error {
	campos := aud.Campos
	if campos == nil {
		campos = []string{}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO auditoria_estudiantes (usuario_id, accion, estudiante_id, version_resultante, campos, estado_anterior, estado_nuevo)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''))
	`, aud.UsuarioID, aud.Accion, estudianteID, version, campos, aud.EstadoAnterior, aud.EstadoNuevo)
	return err
}
