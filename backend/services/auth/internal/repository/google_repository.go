package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Persistencia del acceso con Google: identidades externas, códigos de un
// solo uso y eventos. Cada operación que toca varias tablas es UNA
// transacción de auth_db; los códigos se bloquean con FOR UPDATE, así un
// doble envío o dos solicitudes simultáneas no crean dos cuentas.

const (
	ProveedorGoogle = "google"

	PropositoLogin       = "LOGIN"
	PropositoRegistro    = "REGISTRO"
	PropositoVinculacion = "VINCULACION"

	RolSolicitante = "SOLICITANTE"
)

var (
	ErrIdentidadNoEncontrada = errors.New("identidad externa no encontrada")
	ErrCodigoInvalido        = errors.New("código inválido, vencido o ya utilizado")
	ErrIdentidadVinculada    = errors.New("la identidad de google ya está vinculada a otra cuenta")
	ErrCuentaYaVinculada     = errors.New("la cuenta ya tiene una identidad de google")
	ErrRolNoPermitido        = errors.New("la cuenta no es de un solicitante")
	ErrCuentaNoDisponible    = errors.New("la cuenta no está activa")
	ErrDNIDistinto           = errors.New("el registro pendiente usa otro dni")
	ErrRegistroNoCancelable  = errors.New("el registro no se puede cancelar")
)

type IdentidadExterna struct {
	ID               string
	UsuarioID        string
	Subject          string
	Email            string
	RegistroCompleto bool
}

type CodigoOAuth struct {
	ID        string
	Purpose   string
	UsuarioID string
	Subject   string
	Email     string
	Nombres   string
	Apellidos string
	ExpiresAt time.Time
}

// NuevoCodigo: datos de un código a crear (Hash = SHA-256 del valor).
type NuevoCodigo struct {
	Hash      []byte
	Purpose   string
	UsuarioID string
	Subject   string
	Email     string
	Nombres   string
	Apellidos string
	ExpiresAt time.Time
}

// RegistroGoogle: resultado del paso 1 del autorregistro.
type RegistroGoogle struct {
	UsuarioID string
	Username  string
	DNI       string
	Email     string
}

type GoogleRepository struct {
	db *pgxpool.Pool
}

func NewGoogleRepository(db *pgxpool.Pool) *GoogleRepository {
	return &GoogleRepository{db: db}
}

func (r *GoogleRepository) FindIdentity(ctx context.Context, subject string) (*IdentidadExterna, error) {
	return scanIdentidad(r.db.QueryRow(ctx, `
		SELECT id, usuario_id, provider_user_id, provider_email, registro_completo
		FROM user_auth_providers
		WHERE provider = $1 AND provider_user_id = $2
	`, ProveedorGoogle, subject))
}

func (r *GoogleRepository) FindIdentityByUser(ctx context.Context, usuarioID string) (*IdentidadExterna, error) {
	return scanIdentidad(r.db.QueryRow(ctx, `
		SELECT id, usuario_id, provider_user_id, provider_email, registro_completo
		FROM user_auth_providers
		WHERE provider = $1 AND usuario_id = $2
	`, ProveedorGoogle, usuarioID))
}

func scanIdentidad(row pgx.Row) (*IdentidadExterna, error) {
	i := &IdentidadExterna{}
	if err := row.Scan(&i.ID, &i.UsuarioID, &i.Subject, &i.Email, &i.RegistroCompleto); err != nil {
		return nil, classifyNotFound(err, ErrIdentidadNoEncontrada)
	}
	return i, nil
}

func (r *GoogleRepository) TouchIdentity(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE user_auth_providers SET last_used_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	return err
}

func (r *GoogleRepository) CreateCode(ctx context.Context, c NuevoCodigo) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO oauth_login_codes
			(code_hash, purpose, usuario_id, provider_user_id, provider_email, nombres_sugeridos, apellidos_sugeridos, expires_at)
		VALUES ($1, $2, NULLIF($3, '')::uuid, NULLIF($4, ''), NULLIF($5, '')::citext, NULLIF($6, ''), NULLIF($7, ''), $8)
	`, c.Hash, c.Purpose, c.UsuarioID, c.Subject, c.Email, recortar(c.Nombres, 100), recortar(c.Apellidos, 100), c.ExpiresAt)
	return err
}

const columnasCodigo = `id, purpose, COALESCE(usuario_id::text, ''), COALESCE(provider_user_id, ''),
	COALESCE(provider_email::text, ''), COALESCE(nombres_sugeridos, ''), COALESCE(apellidos_sugeridos, ''), expires_at`

func scanCodigo(row pgx.Row) (*CodigoOAuth, error) {
	c := &CodigoOAuth{}
	if err := row.Scan(&c.ID, &c.Purpose, &c.UsuarioID, &c.Subject, &c.Email, &c.Nombres, &c.Apellidos, &c.ExpiresAt); err != nil {
		return nil, classifyNotFound(err, ErrCodigoInvalido)
	}
	return c, nil
}

// ConsumeCode marca el código como usado (una sola vez) y lo devuelve.
func (r *GoogleRepository) ConsumeCode(ctx context.Context, hash []byte, purpose string) (*CodigoOAuth, error) {
	return scanCodigo(r.db.QueryRow(ctx, `
		UPDATE oauth_login_codes
		SET used_at = CURRENT_TIMESTAMP
		WHERE code_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > CURRENT_TIMESTAMP
		RETURNING `+columnasCodigo, hash, purpose))
}

// PeekCode lee un código vigente sin consumirlo.
func (r *GoogleRepository) PeekCode(ctx context.Context, hash []byte, purpose string) (*CodigoOAuth, error) {
	return scanCodigo(r.db.QueryRow(ctx, `
		SELECT `+columnasCodigo+`
		FROM oauth_login_codes
		WHERE code_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > CURRENT_TIMESTAMP
	`, hash, purpose))
}

// bloquearCodigo: código vigente bloqueado hasta el fin de la transacción.
func bloquearCodigo(ctx context.Context, tx pgx.Tx, hash []byte, purpose string) (*CodigoOAuth, error) {
	return scanCodigo(tx.QueryRow(ctx, `
		SELECT `+columnasCodigo+`
		FROM oauth_login_codes
		WHERE code_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > CURRENT_TIMESTAMP
		FOR UPDATE
	`, hash, purpose))
}

func rolDe(ctx context.Context, tx pgx.Tx, usuarioID string) (string, error) {
	var rol string
	err := tx.QueryRow(ctx, `
		SELECT r.nombre FROM usuario_roles ur JOIN roles r ON r.id = ur.rol_id WHERE ur.usuario_id = $1
	`, usuarioID).Scan(&rol)
	return rol, classifyNotFound(err, ErrRolNoPermitido)
}

// RegisterGoogleUser: paso 1 del autorregistro. Con un código REGISTRO
// vigente crea, en una transacción, el usuario (rol SOLICITANTE fijo, correo
// verificado por Google), su identidad Google con registro_completo = FALSE
// y su DNI DECLARADO (identidad_solicitantes, migración 025). usuarios.dni
// queda en NULL: el DNI no se reserva hasta que Secretaría verifica la
// identidad. Si el código ya tiene usuario (reintento), retoma ese registro
// sin crear nada. Los UNIQUE de usuarios (email, username) y de
// user_auth_providers son la verificación definitiva frente a carreras.
func (r *GoogleRepository) RegisterGoogleUser(ctx context.Context, hash []byte, dni, username, passwordHash string) (*RegistroGoogle, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	codigo, err := bloquearCodigo(ctx, tx, hash, PropositoRegistro)
	if err != nil {
		return nil, err
	}
	reg := &RegistroGoogle{}
	if codigo.UsuarioID != "" {
		// Reintento de un registro cuya cuenta ya existe y espera su perfil.
		var completo bool
		err := tx.QueryRow(ctx, `
			SELECT u.id, u.username, COALESCE(i.dni_declarado, u.dni, ''), u.email::text, p.registro_completo
			FROM usuarios u JOIN user_auth_providers p ON p.usuario_id = u.id AND p.provider = $2
			LEFT JOIN identidad_solicitantes i ON i.usuario_id = u.id
			WHERE u.id = $1
		`, codigo.UsuarioID, ProveedorGoogle).Scan(&reg.UsuarioID, &reg.Username, &reg.DNI, &reg.Email, &completo)
		if err != nil {
			return nil, classifyNotFound(err, ErrCodigoInvalido)
		}
		if completo {
			return nil, ErrCodigoInvalido
		}
		if reg.DNI != dni {
			return nil, ErrDNIDistinto
		}
		return reg, tx.Commit(ctx)
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO usuarios (username, email, password_hash, email_verified, email_verified_at)
		VALUES ($1, $2, $3, TRUE, CURRENT_TIMESTAMP)
		RETURNING id, username, email::text
	`, username, codigo.Email, passwordHash).Scan(&reg.UsuarioID, &reg.Username, &reg.Email)
	if err != nil {
		return nil, classifyUniqueViolation(err)
	}
	if err := declararIdentidad(ctx, tx, reg.UsuarioID, dni, OrigenIdentidadGoogle); err != nil {
		return nil, err
	}
	reg.DNI = dni
	rol, err := findRoleByName(ctx, tx, RolSolicitante)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)`, reg.UsuarioID, rol.ID); err != nil {
		return nil, errors.Join(ErrRoleAssignment, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_auth_providers (usuario_id, provider, provider_user_id, provider_email, registro_completo)
		VALUES ($1, $2, $3, $4, FALSE)
	`, reg.UsuarioID, ProveedorGoogle, codigo.Subject, codigo.Email); err != nil {
		return nil, clasificarIdentidad(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE oauth_login_codes SET usuario_id = $2 WHERE id = $1`, codigo.ID, reg.UsuarioID); err != nil {
		return nil, err
	}
	return reg, tx.Commit(ctx)
}

// CompleteRegistration: el perfil ya existe en Usuarios Service. Marca la
// identidad como completa y consume el código; devuelve el usuario.
func (r *GoogleRepository) CompleteRegistration(ctx context.Context, hash []byte) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	codigo, err := bloquearCodigo(ctx, tx, hash, PropositoRegistro)
	if err != nil {
		return "", err
	}
	if codigo.UsuarioID == "" {
		return "", ErrCodigoInvalido
	}
	tag, err := tx.Exec(ctx, `
		UPDATE user_auth_providers SET registro_completo = TRUE, last_used_at = CURRENT_TIMESTAMP
		WHERE usuario_id = $1 AND provider = $2
	`, codigo.UsuarioID, ProveedorGoogle)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 1 {
		return "", ErrCodigoInvalido
	}
	if _, err := tx.Exec(ctx, `
		UPDATE oauth_login_codes
		SET used_at = CURRENT_TIMESTAMP, nombres_sugeridos = NULL, apellidos_sugeridos = NULL
		WHERE id = $1
	`, codigo.ID); err != nil {
		return "", err
	}
	return codigo.UsuarioID, tx.Commit(ctx)
}

// Resultados de public.cancelar_registro_google_pendiente (migración 023).
const resultadoCuentaEliminada = "ELIMINADA"

// CancelRegistration: compensación cuando Usuarios Service rechaza el perfil
// (p. ej. DNI ya registrado allí). El código REGISTRO (bloqueado en esta
// transacción) identifica la cuenta creada por ESTE registro; la eliminación
// la hace la función SECURITY DEFINER cancelar_registro_google_pendiente,
// porque auth_app no tiene DELETE sobre usuarios. La función vuelve a
// comprobar que la cuenta es un registro Google incompleto, sin refresh
// tokens y con el único rol SOLICITANTE; si no, no borra nada. La eliminación
// arrastra su rol, su identidad y sus códigos (ON DELETE CASCADE).
func (r *GoogleRepository) CancelRegistration(ctx context.Context, hash []byte) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	codigo, err := bloquearCodigo(ctx, tx, hash, PropositoRegistro)
	if err != nil {
		return "", err
	}
	if codigo.UsuarioID == "" {
		return "", ErrRegistroNoCancelable
	}
	var resultado string
	if err := tx.QueryRow(ctx, `SELECT public.cancelar_registro_google_pendiente($1)`, codigo.UsuarioID).Scan(&resultado); err != nil {
		return "", err
	}
	if resultado != resultadoCuentaEliminada {
		return "", ErrRegistroNoCancelable
	}
	return codigo.UsuarioID, tx.Commit(ctx)
}

// LinkIdentity: vinculación explícita. Consume el código VINCULACION del
// usuario autenticado y le agrega la identidad Google, solo si su cuenta es
// SOLICITANTE y está activa. No cambia correo, clave, rol, DNI ni estado.
func (r *GoogleRepository) LinkIdentity(ctx context.Context, hash []byte, subject, email string) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	codigo, err := bloquearCodigo(ctx, tx, hash, PropositoVinculacion)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE oauth_login_codes SET used_at = CURRENT_TIMESTAMP WHERE id = $1`, codigo.ID); err != nil {
		return "", err
	}
	var activo bool
	if err := tx.QueryRow(ctx, `SELECT estado FROM usuarios WHERE id = $1`, codigo.UsuarioID).Scan(&activo); err != nil {
		return "", classifyNotFound(err, ErrCodigoInvalido)
	}
	if !activo {
		return "", ErrCuentaNoDisponible
	}
	rol, err := rolDe(ctx, tx, codigo.UsuarioID)
	if err != nil {
		return "", err
	}
	if rol != RolSolicitante {
		return "", ErrRolNoPermitido
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_auth_providers (usuario_id, provider, provider_user_id, provider_email, registro_completo, last_used_at)
		VALUES ($1, $2, $3, $4, TRUE, CURRENT_TIMESTAMP)
	`, codigo.UsuarioID, ProveedorGoogle, subject, email); err != nil {
		return "", clasificarIdentidad(err)
	}
	return codigo.UsuarioID, tx.Commit(ctx)
}

// LogEvent registra un evento de autenticación (sin tokens, códigos,
// secretos ni datos personales). Es de mejor esfuerzo: un fallo aquí no
// cambia el resultado de la operación.
func (r *GoogleRepository) LogEvent(ctx context.Context, usuarioID, evento, resultado, motivo string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth_eventos (usuario_id, evento, proveedor, resultado, motivo)
		VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5)
	`, usuarioID, recortar(evento, 40), ProveedorGoogle, resultado, recortar(motivo, 60))
	return err
}

func clasificarIdentidad(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_user_auth_providers_identidad":
			return ErrIdentidadVinculada
		case "uq_user_auth_providers_usuario":
			return ErrCuentaYaVinculada
		}
	}
	return err
}

func recortar(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
