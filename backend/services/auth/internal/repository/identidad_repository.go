package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Identidad de los solicitantes del registro público (migración 025). El
// registro público solo DECLARA el DNI; usuarios.dni se escribe recién
// cuando Secretaría verifica la identidad. Un DNI que ya pertenece a otra
// cuenta nunca se sobrescribe: el índice único uq_usuarios_dni rechaza la
// verificación y la transacción se revierte completa.

const (
	OrigenIdentidadCorreo = "correo"
	OrigenIdentidadGoogle = "google"

	IdentidadPendiente  = "PENDIENTE"
	IdentidadVerificada = "VERIFICADA"
	IdentidadRechazada  = "RECHAZADA"

	// Sincronización con Usuarios Service (migración 025).
	SincronizacionPerfilPendiente = "PERFIL_PENDIENTE"
	SincronizacionCompleta        = "COMPLETA"
	// ErrorConflictoPerfil: Usuarios encontró el DNI en OTRO perfil y Auth
	// revirtió su verificación (queda visible como conflicto para el personal).
	ErrorConflictoPerfil = "conflicto_perfil"
)

var (
	ErrIdentidadSolicitanteNoEncontrada = errors.New("identidad de solicitante no encontrada")
	ErrIdentidadConflicto               = errors.New("el DNI declarado ya pertenece a otra cuenta")
	ErrIdentidadDNIDistinto             = errors.New("el DNI declarado cambió durante la verificación")
	ErrIdentidadYaVerificada            = errors.New("la identidad ya fue verificada")
	ErrIdentidadNoSolicitante           = errors.New("la cuenta no es de un solicitante")
	ErrIdentidadNoSincronizable         = errors.New("la identidad no tiene una sincronización pendiente con ese DNI")
)

type IdentidadSolicitante struct {
	UsuarioID     string
	DNIDeclarado  string
	Origen        string
	Estado        string
	Metodo        string
	VerificadoAt  *time.Time
	MotivoRechazo string
	CreatedAt     time.Time
	// Conflicto: el DNI declarado ya está asignado a OTRA cuenta en Auth, o
	// Usuarios lo encontró en otro perfil (solo lo ve el personal con el
	// permiso de verificación).
	Conflicto bool
	// Sincronización con Usuarios: "", PERFIL_PENDIENTE o COMPLETA, con la
	// traza del último intento (código de error, nunca el DNI).
	Sincronizacion         string
	SincronizacionIntentos int
	SincronizacionError    string
}

// declararIdentidad: fila PENDIENTE del registro público, en la misma
// transacción que crea la cuenta.
func declararIdentidad(ctx context.Context, tx pgx.Tx, usuarioID, dni, origen string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO identidad_solicitantes (usuario_id, dni_declarado, origen) VALUES ($1, $2, $3)
	`, usuarioID, dni, origen)
	return err
}

type IdentidadRepository struct {
	db *pgxpool.Pool
}

func NewIdentidadRepository(db *pgxpool.Pool) *IdentidadRepository {
	return &IdentidadRepository{db: db}
}

const columnasIdentidad = `
	i.usuario_id::text, i.dni_declarado, i.origen, i.estado, coalesce(i.metodo, ''), i.verificado_at,
	coalesce(i.motivo_rechazo, ''), i.created_at,
	(EXISTS (SELECT 1 FROM usuarios o WHERE o.dni = i.dni_declarado AND o.id <> i.usuario_id)
	 OR (i.estado = 'PENDIENTE' AND coalesce(i.sincronizacion_error, '') = 'conflicto_perfil')),
	coalesce(i.sincronizacion, ''), i.sincronizacion_intentos, coalesce(i.sincronizacion_error, '')
`

func escanearIdentidad(row pgx.Row) (*IdentidadSolicitante, error) {
	i := &IdentidadSolicitante{}
	err := row.Scan(&i.UsuarioID, &i.DNIDeclarado, &i.Origen, &i.Estado, &i.Metodo, &i.VerificadoAt,
		&i.MotivoRechazo, &i.CreatedAt, &i.Conflicto, &i.Sincronizacion, &i.SincronizacionIntentos, &i.SincronizacionError)
	return i, err
}

// Listar: bandeja de Secretaría por estado (vacío = todos), más antiguos
// primero. sincronizacionPendiente: solo las verificadas cuyo perfil falta
// sincronizar (reconciliación).
func (r *IdentidadRepository) Listar(ctx context.Context, estado string, sincronizacionPendiente bool, limite, desplazamiento int) ([]*IdentidadSolicitante, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM identidad_solicitantes
		WHERE ($1 = '' OR estado = $1) AND (NOT $2 OR sincronizacion = 'PERFIL_PENDIENTE')
	`, estado, sincronizacionPendiente).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT `+columnasIdentidad+`
		FROM identidad_solicitantes i
		WHERE ($1 = '' OR i.estado = $1) AND (NOT $4 OR i.sincronizacion = 'PERFIL_PENDIENTE')
		ORDER BY i.created_at, i.usuario_id
		LIMIT $2 OFFSET $3
	`, estado, limite, desplazamiento, sincronizacionPendiente)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var lista []*IdentidadSolicitante
	for rows.Next() {
		i, err := escanearIdentidad(rows)
		if err != nil {
			return nil, 0, err
		}
		lista = append(lista, i)
	}
	return lista, total, rows.Err()
}

func (r *IdentidadRepository) Obtener(ctx context.Context, usuarioID string) (*IdentidadSolicitante, error) {
	i, err := escanearIdentidad(r.db.QueryRow(ctx, `
		SELECT `+columnasIdentidad+` FROM identidad_solicitantes i WHERE i.usuario_id = $1
	`, usuarioID))
	if err != nil {
		return nil, classifyNotFound(err, ErrIdentidadSolicitanteNoEncontrada)
	}
	return i, nil
}

type identidadBloqueada struct {
	dni    string
	estado string
}

// bloquearIdentidad bloquea la fila de identidad y la cuenta (en ese orden:
// dos verificaciones simultáneas de la misma cuenta se serializan) y
// comprueba que la cuenta siga siendo SOLICITANTE.
func bloquearIdentidad(ctx context.Context, tx pgx.Tx, usuarioID string) (*identidadBloqueada, error) {
	b := &identidadBloqueada{}
	err := tx.QueryRow(ctx, `
		SELECT dni_declarado, estado FROM identidad_solicitantes WHERE usuario_id = $1 FOR UPDATE
	`, usuarioID).Scan(&b.dni, &b.estado)
	if err != nil {
		return nil, classifyNotFound(err, ErrIdentidadSolicitanteNoEncontrada)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM usuarios WHERE id = $1 FOR UPDATE`, usuarioID); err != nil {
		return nil, err
	}
	rol, err := rolDe(ctx, tx, usuarioID)
	if err != nil {
		return nil, err
	}
	if rol != RolSolicitante {
		return nil, ErrIdentidadNoSolicitante
	}
	return b, nil
}

// Verificar: escribe el DNI declarado en usuarios.dni y marca la identidad
// VERIFICADA, en una sola transacción. dniEsperado es el DNI que el personal
// cotejó: si la declaración cambió entretanto, no se verifica nada. Si el DNI
// ya pertenece a otra cuenta, el índice único lo rechaza y nada cambia.
// Repetir la verificación de una identidad ya verificada con el mismo DNI es
// idempotente (yaVerificada = true).
func (r *IdentidadRepository) Verificar(ctx context.Context, usuarioID, dniEsperado, metodo, verificadorID string) (yaVerificada bool, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	b, err := bloquearIdentidad(ctx, tx, usuarioID)
	if err != nil {
		return false, err
	}
	if b.dni != dniEsperado {
		return false, ErrIdentidadDNIDistinto
	}
	var actual *string
	if err := tx.QueryRow(ctx, `SELECT dni FROM usuarios WHERE id = $1`, usuarioID).Scan(&actual); err != nil {
		return false, err
	}
	if b.estado == IdentidadVerificada {
		if actual != nil && *actual == b.dni {
			return true, tx.Commit(ctx)
		}
		return false, ErrIdentidadYaVerificada
	}
	if actual != nil && *actual != b.dni {
		return false, ErrIdentidadConflicto
	}
	if _, err := tx.Exec(ctx, `UPDATE usuarios SET dni = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, usuarioID, b.dni); err != nil {
		if errors.Is(classifyUniqueViolation(err), ErrDNIExists) {
			return false, ErrIdentidadConflicto
		}
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE identidad_solicitantes
		SET estado = 'VERIFICADA', metodo = $2, verificado_por = NULLIF($3, '')::uuid,
		    verificado_at = CURRENT_TIMESTAMP, motivo_rechazo = NULL, updated_at = CURRENT_TIMESTAMP,
		    sincronizacion = 'PERFIL_PENDIENTE', sincronizacion_intentos = 0,
		    sincronizacion_error = NULL, sincronizacion_at = CURRENT_TIMESTAMP
		WHERE usuario_id = $1
	`, usuarioID, metodo, verificadorID); err != nil {
		return false, err
	}
	return false, tx.Commit(ctx)
}

// Rechazar: la identidad declarada no se pudo cotejar. La cuenta sigue sin
// DNI (inicia sesión con su correo o Google). Una identidad verificada no se
// rechaza desde aquí.
func (r *IdentidadRepository) Rechazar(ctx context.Context, usuarioID, motivo, verificadorID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	b, err := bloquearIdentidad(ctx, tx, usuarioID)
	if err != nil {
		return err
	}
	if b.estado == IdentidadVerificada {
		return ErrIdentidadYaVerificada
	}
	if _, err := tx.Exec(ctx, `
		UPDATE identidad_solicitantes
		SET estado = 'RECHAZADA', motivo_rechazo = $2, verificado_por = NULLIF($3, '')::uuid,
		    verificado_at = CURRENT_TIMESTAMP, metodo = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE usuario_id = $1
	`, usuarioID, motivo, verificadorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// LogEvent: auditoría de la verificación (sin DNI ni nombres).
func (r *IdentidadRepository) LogEvent(ctx context.Context, usuarioID, evento, resultado, motivo string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth_eventos (usuario_id, evento, proveedor, resultado, motivo)
		VALUES (NULLIF($1, '')::uuid, $2, 'identidad', $3, $4)
	`, usuarioID, recortar(evento, 40), resultado, recortar(motivo, 60))
	return err
}

// ConfirmarSincronizacion: Usuarios ya tiene el DNI en el perfil. Solo para
// una identidad VERIFICADA cuyo DNI definitivo en Auth es exactamente dni.
// Idempotente.
func (r *IdentidadRepository) ConfirmarSincronizacion(ctx context.Context, usuarioID, dni string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE identidad_solicitantes i
		SET sincronizacion = 'COMPLETA', sincronizacion_error = NULL,
		    sincronizacion_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		FROM usuarios u
		WHERE i.usuario_id = $1 AND u.id = i.usuario_id AND i.estado = 'VERIFICADA'
		  AND i.dni_declarado = $2 AND u.dni = $2
	`, usuarioID, dni)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrIdentidadNoSincronizable
	}
	return nil
}

// RegistrarFalloSincronizacion deja la traza de un intento fallido (código
// estable del error, nunca el DNI). La operación sigue PERFIL_PENDIENTE.
func (r *IdentidadRepository) RegistrarFalloSincronizacion(ctx context.Context, usuarioID, codigo string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE identidad_solicitantes
		SET sincronizacion_intentos = sincronizacion_intentos + 1, sincronizacion_error = $2,
		    sincronizacion_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE usuario_id = $1 AND sincronizacion = 'PERFIL_PENDIENTE'
	`, usuarioID, recortar(codigo, 60))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrIdentidadNoSincronizable
	}
	return nil
}

// RevertirVerificacion: compensación cuando Usuarios encuentra el DNI en
// OTRO perfil. Solo para una verificación con el perfil todavía pendiente y
// cuyo DNI definitivo es exactamente dni: quita el DNI de la cuenta (solo
// ese) y la identidad vuelve a PENDIENTE marcada con el conflicto. Nunca
// elimina ni recrea la cuenta. Una verificación COMPLETA no se revierte aquí.
func (r *IdentidadRepository) RevertirVerificacion(ctx context.Context, usuarioID, dni string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	b, err := bloquearIdentidad(ctx, tx, usuarioID)
	if err != nil {
		return err
	}
	var sincronizacion string
	if err := tx.QueryRow(ctx, `SELECT coalesce(sincronizacion, '') FROM identidad_solicitantes WHERE usuario_id = $1`, usuarioID).Scan(&sincronizacion); err != nil {
		return err
	}
	if b.estado != IdentidadVerificada || sincronizacion != SincronizacionPerfilPendiente || b.dni != dni {
		return ErrIdentidadNoSincronizable
	}
	tag, err := tx.Exec(ctx, `UPDATE usuarios SET dni = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND dni = $2`, usuarioID, dni)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrIdentidadNoSincronizable
	}
	if _, err := tx.Exec(ctx, `
		UPDATE identidad_solicitantes
		SET estado = 'PENDIENTE', metodo = NULL, verificado_por = NULL, verificado_at = NULL,
		    sincronizacion = NULL, sincronizacion_error = 'conflicto_perfil',
		    sincronizacion_intentos = sincronizacion_intentos + 1,
		    sincronizacion_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE usuario_id = $1
	`, usuarioID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
