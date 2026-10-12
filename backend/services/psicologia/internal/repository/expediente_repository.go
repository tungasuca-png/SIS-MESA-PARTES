package repository

import (
	"context"
	"fmt"
	"time"

	"psicologia/internal/clinicalcrypto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// resumen_caso y motivo_cierre se guardan cifrados (FASE 6N-3, C1).
const expedienteColumns = `id::text, estudiante_id::text, codigo, estado, responsable_id::text, abierto_por::text,
	fecha_apertura, fecha_cierre, motivo_cierre_cifrado, resumen_caso_cifrado, cifrado_key_id, created_at, updated_at`

type Expediente struct {
	ID            string
	EstudianteID  string
	Codigo        string
	Estado        string
	ResponsableID string
	AbiertoPor    string
	FechaApertura time.Time
	FechaCierre   *time.Time
	// Descifrados por el repositorio solo en lecturas de detalle; nil en el
	// listado (que no debe exponerlos).
	MotivoCierre *string
	ResumenCaso  *string
	CreatedAt    time.Time
	UpdatedAt    time.Time

	motivoCierreCifrado []byte
	resumenCifrado      []byte
	cifradoKeyID        *string
}

func scanExpediente(row pgx.Row) (*Expediente, error) {
	e := &Expediente{}
	err := row.Scan(&e.ID, &e.EstudianteID, &e.Codigo, &e.Estado, &e.ResponsableID, &e.AbiertoPor,
		&e.FechaApertura, &e.FechaCierre, &e.motivoCierreCifrado, &e.resumenCifrado, &e.cifradoKeyID,
		&e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, classifyNotFound(err)
	}
	return e, nil
}

type ExpedienteRepository struct {
	db       *pgxpool.Pool
	cifrador Cifrador
}

func NewExpedienteRepository(db *pgxpool.Pool, cifrador Cifrador) *ExpedienteRepository {
	return &ExpedienteRepository{db: db, cifrador: cifrador}
}

// abrir descifra resumen del caso y motivo de cierre de un expediente leído.
func (r *ExpedienteRepository) abrir(e *Expediente, err error) (*Expediente, error) {
	if err != nil || e == nil {
		return e, err
	}
	if e.ResumenCaso, err = descifrarTexto(r.cifrador, e.resumenCifrado, e.cifradoKeyID, clinicalcrypto.AADResumenCaso(e.ID)); err != nil {
		return nil, err
	}
	if e.MotivoCierre, err = descifrarTexto(r.cifrador, e.motivoCierreCifrado, e.cifradoKeyID, clinicalcrypto.AADMotivoCierre(e.ID)); err != nil {
		return nil, err
	}
	return e, nil
}

// textosCifrados: resumen y motivo de cierre cifrados con la clave ACTIVA. La
// fila tiene un solo cifrado_key_id, así que al escribir cualquiera de los dos
// campos se vuelven a cifrar ambos (el que no cambia se descifra primero).
// nuevoResumen / nuevoMotivo nil = conservar el valor actual.
func (r *ExpedienteRepository) textosCifrados(ctx context.Context, tx pgx.Tx, id string, nuevoResumen, nuevoMotivo *string) (resumen, motivo []byte, keyID *string, err error) {
	actual, err := r.abrir(scanExpediente(tx.QueryRow(ctx, `SELECT `+expedienteColumns+` FROM expedientes_psicologicos WHERE id = $1::uuid FOR UPDATE`, id)))
	if err != nil {
		return nil, nil, nil, err
	}
	valor := func(nuevo, actual *string) string {
		if nuevo != nil {
			return *nuevo
		}
		if actual != nil {
			return *actual
		}
		return ""
	}
	resumen, k1, err := cifrarTexto(r.cifrador, valor(nuevoResumen, actual.ResumenCaso), clinicalcrypto.AADResumenCaso(id))
	if err != nil {
		return nil, nil, nil, err
	}
	motivo, k2, err := cifrarTexto(r.cifrador, valor(nuevoMotivo, actual.MotivoCierre), clinicalcrypto.AADMotivoCierre(id))
	if err != nil {
		return nil, nil, nil, err
	}
	if k1 != nil {
		keyID = k1
	} else {
		keyID = k2
	}
	return resumen, motivo, keyID, nil
}

// withTx ejecuta fn dentro de una transacción: BEGIN -> fn -> COMMIT, con
// ROLLBACK ante cualquier error. Mismo patrón que Auth/Expedientes.
func (r *ExpedienteRepository) withTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type OpenInput struct {
	EstudianteID  string
	ResponsableID string
	AbiertoPor    string
	ResumenCaso   string
}

// Open crea el expediente, su código correlativo y el evento CREATE de
// auditoría en UNA transacción. La garantía final de "1 estudiante = 1
// expediente" es UNIQUE(estudiante_id): ante dos solicitudes concurrentes solo
// una gana y la otra recibe ErrEstudianteYaTieneExpediente (su transacción,
// incluido el incremento del contador, se revierte: no quedan huecos).
func (r *ExpedienteRepository) Open(ctx context.Context, in OpenInput) (*Expediente, error) {
	var created *Expediente
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		var anio, ultimo int
		if err := tx.QueryRow(ctx, `
			INSERT INTO expediente_codigo_counters (anio, ultimo)
			VALUES (EXTRACT(YEAR FROM CURRENT_TIMESTAMP)::int, 1)
			ON CONFLICT (anio) DO UPDATE SET ultimo = expediente_codigo_counters.ultimo + 1
			RETURNING anio, ultimo
		`).Scan(&anio, &ultimo); err != nil {
			return err
		}
		codigo := fmt.Sprintf("PSI-%04d-%06d", anio, ultimo)

		// El id se genera antes del INSERT: el AAD del resumen cifrado lo usa.
		var id string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
			return err
		}
		resumen, keyID, err := cifrarTexto(r.cifrador, in.ResumenCaso, clinicalcrypto.AADResumenCaso(id))
		if err != nil {
			return err
		}
		exp, err := r.abrir(scanExpediente(tx.QueryRow(ctx, `
			INSERT INTO expedientes_psicologicos (id, estudiante_id, codigo, estado, responsable_id, abierto_por,
				resumen_caso_cifrado, cifrado_key_id)
			VALUES ($1::uuid, $2::uuid, $3, 'ABIERTO', $4::uuid, $5::uuid, $6, $7)
			RETURNING `+expedienteColumns,
			id, in.EstudianteID, codigo, in.ResponsableID, in.AbiertoPor, resumen, keyID,
		)))
		if err != nil {
			return classifyUniqueViolation(err)
		}
		created = exp

		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: in.AbiertoPor, Accion: AccionCreate, Recurso: RecursoExpediente,
			RecursoID: exp.ID, EstudianteID: exp.EstudianteID, Resultado: ResultadoPermitido,
		})
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *ExpedienteRepository) GetByID(ctx context.Context, id string) (*Expediente, error) {
	return r.abrir(scanExpediente(r.db.QueryRow(ctx, `SELECT `+expedienteColumns+` FROM expedientes_psicologicos WHERE id = $1::uuid`, id)))
}

func (r *ExpedienteRepository) GetByEstudiante(ctx context.Context, estudianteID string) (*Expediente, error) {
	return r.abrir(scanExpediente(r.db.QueryRow(ctx, `SELECT `+expedienteColumns+` FROM expedientes_psicologicos WHERE estudiante_id = $1::uuid`, estudianteID)))
}

type ListFilter struct {
	Estado string
	// ResponsableID no vacío = alcance propio: solo expedientes donde
	// responsable_id coincide. Se aplica en SQL, nunca solo en la lógica.
	ResponsableID string
	Page          int
	PageSize      int
}

func (r *ExpedienteRepository) List(ctx context.Context, f ListFilter) ([]*Expediente, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM expedientes_psicologicos
		WHERE ($1::text = '' OR estado = $1) AND ($2::text = '' OR responsable_id = NULLIF($2::text, '')::uuid)
	`, f.Estado, f.ResponsableID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+expedienteColumns+` FROM expedientes_psicologicos
		WHERE ($1::text = '' OR estado = $1) AND ($2::text = '' OR responsable_id = NULLIF($2::text, '')::uuid)
		ORDER BY fecha_apertura DESC, id
		LIMIT $3 OFFSET $4
	`, f.Estado, f.ResponsableID, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// El listado NO descifra: ni el resumen del caso ni el motivo de cierre
	// salen de aquí (FASE 6N-3, bloque 13).
	items := make([]*Expediente, 0, f.PageSize)
	for rows.Next() {
		e, err := scanExpediente(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	return items, total, rows.Err()
}

// lockScoped bloquea la fila (FOR UPDATE) verificando el alcance dentro de la
// misma transacción: con scopeResponsableID no vacío, un expediente de otro
// responsable es indistinguible de uno inexistente (ErrExpedienteNotFound).
func lockScoped(ctx context.Context, tx pgx.Tx, id, scopeResponsableID string) (estado, estudianteID string, err error) {
	var responsable string
	err = tx.QueryRow(ctx, `
		SELECT estado, estudiante_id::text, responsable_id::text FROM expedientes_psicologicos
		WHERE id = $1::uuid FOR UPDATE
	`, id).Scan(&estado, &estudianteID, &responsable)
	if err != nil {
		return "", "", classifyNotFound(err)
	}
	if scopeResponsableID != "" && responsable != scopeResponsableID {
		return "", "", ErrExpedienteNotFound
	}
	return estado, estudianteID, nil
}

type UpdateInput struct {
	ID                 string
	ActorID            string
	ScopeResponsableID string
	ResumenCaso        *string
	ResponsableID      *string
	Estado             *string
	// Campos: nombres de los campos modificados, para el evento de auditoría
	// (nunca sus valores).
	Campos string
}

func (r *ExpedienteRepository) Update(ctx context.Context, in UpdateInput) (*Expediente, error) {
	var updated *Expediente
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		estado, estudianteID, err := lockScoped(ctx, tx, in.ID, in.ScopeResponsableID)
		if err != nil {
			return err
		}
		if estado == "CERRADO" {
			return ErrEstadoInvalido
		}

		setResumen := in.ResumenCaso != nil
		var resumen, motivo []byte
		var keyID *string
		if setResumen {
			if resumen, motivo, keyID, err = r.textosCifrados(ctx, tx, in.ID, in.ResumenCaso, nil); err != nil {
				return err
			}
		}
		setResp, resp := in.ResponsableID != nil, ""
		if setResp {
			resp = *in.ResponsableID
		}
		setEstado, nuevoEstado := in.Estado != nil, ""
		if setEstado {
			nuevoEstado = *in.Estado
		}

		updated, err = r.abrir(scanExpediente(tx.QueryRow(ctx, `
			UPDATE expedientes_psicologicos SET
				resumen_caso_cifrado  = CASE WHEN $2::boolean THEN $3::bytea ELSE resumen_caso_cifrado END,
				motivo_cierre_cifrado = CASE WHEN $2::boolean THEN $4::bytea ELSE motivo_cierre_cifrado END,
				cifrado_key_id        = CASE WHEN $2::boolean THEN $5::text ELSE cifrado_key_id END,
				responsable_id = CASE WHEN $6::boolean THEN NULLIF($7::text, '')::uuid ELSE responsable_id END,
				estado         = CASE WHEN $8::boolean THEN $9::text ELSE estado END,
				updated_at     = CURRENT_TIMESTAMP
			WHERE id = $1::uuid
			RETURNING `+expedienteColumns,
			in.ID, setResumen, resumen, motivo, keyID, setResp, resp, setEstado, nuevoEstado,
		)))
		if err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: in.ActorID, Accion: AccionUpdate, Recurso: RecursoExpediente,
			RecursoID: in.ID, EstudianteID: estudianteID, Resultado: ResultadoPermitido, Motivo: in.Campos,
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *ExpedienteRepository) Close(ctx context.Context, id, actorID, scopeResponsableID, motivo string) (*Expediente, error) {
	var closed *Expediente
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		estado, estudianteID, err := lockScoped(ctx, tx, id, scopeResponsableID)
		if err != nil {
			return err
		}
		if estado == "CERRADO" {
			return ErrEstadoInvalido
		}
		resumenCifrado, motivoCifrado, keyID, err := r.textosCifrados(ctx, tx, id, nil, &motivo)
		if err != nil {
			return err
		}
		closed, err = r.abrir(scanExpediente(tx.QueryRow(ctx, `
			UPDATE expedientes_psicologicos
			SET estado = 'CERRADO', fecha_cierre = CURRENT_TIMESTAMP, motivo_cierre_cifrado = $2,
				resumen_caso_cifrado = $3, cifrado_key_id = $4, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1::uuid RETURNING `+expedienteColumns, id, motivoCifrado, resumenCifrado, keyID)))
		if err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: actorID, Accion: AccionClose, Recurso: RecursoExpediente,
			RecursoID: id, EstudianteID: estudianteID, Resultado: ResultadoPermitido,
		})
	})
	if err != nil {
		return nil, err
	}
	return closed, nil
}

// Reopen: CERRADO -> ABIERTO o EN_SEGUIMIENTO (nuevoEstado ya validado).
func (r *ExpedienteRepository) Reopen(ctx context.Context, id, actorID, scopeResponsableID, nuevoEstado string) (*Expediente, error) {
	var reopened *Expediente
	err := r.withTx(ctx, func(tx pgx.Tx) error {
		estado, estudianteID, err := lockScoped(ctx, tx, id, scopeResponsableID)
		if err != nil {
			return err
		}
		if estado != "CERRADO" {
			return ErrEstadoInvalido
		}
		reopened, err = r.abrir(scanExpediente(tx.QueryRow(ctx, `
			UPDATE expedientes_psicologicos SET estado = $2, updated_at = CURRENT_TIMESTAMP
			WHERE id = $1::uuid RETURNING `+expedienteColumns, id, nuevoEstado)))
		if err != nil {
			return err
		}
		return insertAuditoria(ctx, tx, AuditEntry{
			UsuarioID: actorID, Accion: AccionReopen, Recurso: RecursoExpediente,
			RecursoID: id, EstudianteID: estudianteID, Resultado: ResultadoPermitido, Motivo: nuevoEstado,
		})
	})
	if err != nil {
		return nil, err
	}
	return reopened, nil
}
