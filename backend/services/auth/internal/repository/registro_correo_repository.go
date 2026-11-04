package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Registro manual de SOLICITANTES con verificación del correo (migración
// 024). Cada operación bloquea la fila del registro (SELECT ... FOR UPDATE)
// en una transacción: dos verificaciones simultáneas no crean dos cuentas y
// un reenvío no se cruza con una verificación. El correo se envía ANTES del
// COMMIT (callback enviar): si el proveedor falla, no queda nada guardado y
// el código anterior sigue siendo el válido.

const ProveedorCorreo = "correo"

var (
	ErrRegistroNoDisponible       = errors.New("registro inexistente, anulado o ya completado")
	ErrRegistroExpirado           = errors.New("código de verificación vencido")
	ErrRegistroIntentos           = errors.New("intentos de verificación agotados")
	ErrRegistroCodigo             = errors.New("código de verificación incorrecto")
	ErrRegistroEspera             = errors.New("todavía no se puede reenviar el código")
	ErrRegistroEnvios             = errors.New("reenvíos agotados")
	ErrRegistroDemasiados         = errors.New("demasiados registros para este correo")
	ErrRegistroCorreoNoCancelable = errors.New("el registro manual no se puede cancelar")
	ErrRegistroNoVerificado       = errors.New("el registro todavía no fue verificado")
)

// NuevoRegistroCorreo: datos ya validados; la clave ya es bcrypt.
type NuevoRegistroCorreo struct {
	ID              string
	Email           string
	DNI             string
	PasswordHash    string
	Nombres         string
	ApellidoPaterno string
	ApellidoMaterno string
	// HashCodigo: HMAC del código enviado. HashSinCodigo: HMAC de un código
	// aleatorio que nadie conoce, para cuando el correo ya tiene cuenta.
	HashCodigo      []byte
	HashSinCodigo   []byte
	TerminosVersion string
	ExpiraEn        time.Time
}

// RegistroVerificado: datos para crear el perfil en Usuarios Service.
type RegistroVerificado struct {
	UsuarioID       string
	Username        string
	Email           string
	DNI             string
	Nombres         string
	ApellidoPaterno string
	ApellidoMaterno string
}

// IntentosRestantes acompaña a ErrRegistroCodigo.
type ErrCodigoIncorrecto struct{ Restantes int }

func (e *ErrCodigoIncorrecto) Error() string { return ErrRegistroCodigo.Error() }
func (e *ErrCodigoIncorrecto) Unwrap() error { return ErrRegistroCodigo }

// ErrEsperaReenvio acompaña a ErrRegistroEspera.
type ErrEsperaReenvio struct{ Restante time.Duration }

func (e *ErrEsperaReenvio) Error() string { return ErrRegistroEspera.Error() }
func (e *ErrEsperaReenvio) Unwrap() error { return ErrRegistroEspera }

type RegistroCorreoRepository struct {
	db *pgxpool.Pool
}

func NewRegistroCorreoRepository(db *pgxpool.Pool) *RegistroCorreoRepository {
	return &RegistroCorreoRepository{db: db}
}

// Crear: anula los registros pendientes anteriores del mismo correo, guarda
// el nuevo y envía el correo (código o aviso de cuenta existente). Un
// candado transaccional por correo serializa los registros simultáneos del
// mismo correo (límite por hora y anulación consistentes).
func (r *RegistroCorreoRepository) Crear(ctx context.Context, n NuevoRegistroCorreo, maxPorHora int,
	enviar func(cuentaExistente bool) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('registros_correo:' || lower($1), 0))`, n.Email); err != nil {
		return err
	}
	var recientes int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM registros_correo
		WHERE email = $1 AND created_at > CURRENT_TIMESTAMP - interval '1 hour'
	`, n.Email).Scan(&recientes); err != nil {
		return err
	}
	if recientes >= maxPorHora {
		return ErrRegistroDemasiados
	}
	var existente bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM usuarios WHERE email = $1)`, n.Email).Scan(&existente); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE registros_correo
		SET anulado_at = CURRENT_TIMESTAMP, password_hash = NULL,
		    nombres = NULL, apellido_paterno = NULL, apellido_materno = NULL
		WHERE email = $1 AND usuario_id IS NULL AND anulado_at IS NULL AND completado_at IS NULL
	`, n.Email); err != nil {
		return err
	}
	hash := n.HashCodigo
	if existente {
		hash = n.HashSinCodigo
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO registros_correo (id, email, dni, password_hash, nombres, apellido_paterno, apellido_materno,
		                              code_hash, cuenta_existente, expires_at, terminos_version, terminos_aceptados_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11, CURRENT_TIMESTAMP)
	`, n.ID, n.Email, n.DNI, n.PasswordHash, n.Nombres, n.ApellidoPaterno, n.ApellidoMaterno,
		hash, existente, n.ExpiraEn, n.TerminosVersion); err != nil {
		return err
	}
	if err := enviar(existente); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type registroBloqueado struct {
	email       string
	codeHash    []byte
	existente   bool
	expira      time.Time
	intentos    int
	envios      int
	ultimoEnvio time.Time
	usuarioID   *string
	completado  bool
	anulado     bool
}

func bloquearRegistro(ctx context.Context, tx pgx.Tx, id string) (*registroBloqueado, error) {
	b := &registroBloqueado{}
	err := tx.QueryRow(ctx, `
		SELECT email::text, code_hash, cuenta_existente, expires_at, intentos, envios, ultimo_envio_at,
		       usuario_id::text, completado_at IS NOT NULL, anulado_at IS NOT NULL
		FROM registros_correo WHERE id = $1 FOR UPDATE
	`, id).Scan(&b.email, &b.codeHash, &b.existente, &b.expira, &b.intentos, &b.envios, &b.ultimoEnvio,
		&b.usuarioID, &b.completado, &b.anulado)
	if err != nil {
		return nil, classifyNotFound(err, ErrRegistroNoDisponible)
	}
	return b, nil
}

// Reenviar: nuevo código (el anterior deja de valer), intentos en cero y
// vigencia renovada; respeta la espera entre envíos y el máximo de envíos.
func (r *RegistroCorreoRepository) Reenviar(ctx context.Context, id string, nuevoHash []byte, duracion, espera time.Duration,
	maxEnvios int, enviar func(email string, cuentaExistente bool) error) (time.Time, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)
	b, err := bloquearRegistro(ctx, tx, id)
	if err != nil {
		return time.Time{}, err
	}
	if b.anulado || b.completado || b.usuarioID != nil {
		return time.Time{}, ErrRegistroNoDisponible
	}
	if falta := time.Until(b.ultimoEnvio.Add(espera)); falta > 0 {
		return time.Time{}, &ErrEsperaReenvio{Restante: falta}
	}
	if b.envios >= maxEnvios {
		return time.Time{}, ErrRegistroEnvios
	}
	var expira time.Time
	if err := tx.QueryRow(ctx, `
		UPDATE registros_correo
		SET code_hash = $2, intentos = 0, envios = envios + 1, ultimo_envio_at = CURRENT_TIMESTAMP,
		    expires_at = CURRENT_TIMESTAMP + make_interval(secs => $3)
		WHERE id = $1
		RETURNING expires_at
	`, id, nuevoHash, duracion.Seconds()).Scan(&expira); err != nil {
		return time.Time{}, err
	}
	if err := enviar(b.email, b.existente); err != nil {
		return time.Time{}, err
	}
	return expira, tx.Commit(ctx)
}

// Verificar: con el código correcto crea, en la misma transacción, la cuenta
// (correo verificado, rol SOLICITANTE fijo). Si el registro ya estaba
// verificado (falta el perfil: reintento), vuelve a comprobar el código y
// devuelve la misma cuenta sin crear nada. Un código incorrecto suma un
// intento (se guarda aunque la respuesta sea un error). DNI o correo ya
// ocupados anulan el registro.
func (r *RegistroCorreoRepository) Verificar(ctx context.Context, id string, coincide func(hash []byte) bool,
	maxIntentos int, username string) (*RegistroVerificado, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	b, err := bloquearRegistro(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if b.anulado || b.completado {
		return nil, ErrRegistroNoDisponible
	}
	// El límite de intentos vale también en el reintento de un registro ya
	// verificado (el código correcto entrega un token del usuario).
	if b.intentos >= maxIntentos {
		return nil, ErrRegistroIntentos
	}
	if b.usuarioID == nil && time.Now().After(b.expira) {
		return nil, ErrRegistroExpirado
	}
	if b.existente || !coincide(b.codeHash) {
		return nil, sumarIntento(ctx, tx, id, maxIntentos)
	}

	v := &RegistroVerificado{}
	var passwordHash string
	if err := tx.QueryRow(ctx, `
		SELECT email::text, dni, coalesce(password_hash, ''), coalesce(nombres, ''), coalesce(apellido_paterno, ''), coalesce(apellido_materno, '')
		FROM registros_correo WHERE id = $1
	`, id).Scan(&v.Email, &v.DNI, &passwordHash, &v.Nombres, &v.ApellidoPaterno, &v.ApellidoMaterno); err != nil {
		return nil, err
	}
	if b.usuarioID != nil {
		// Reintento: la cuenta ya existe y espera su perfil.
		v.UsuarioID = *b.usuarioID
		if err := tx.QueryRow(ctx, `SELECT username FROM usuarios WHERE id = $1`, v.UsuarioID).Scan(&v.Username); err != nil {
			return nil, classifyNotFound(err, ErrRegistroNoDisponible)
		}
		return v, tx.Commit(ctx)
	}

	// La cuenta se crea dentro de un punto de guardado: si el DNI o el correo
	// ya existen, se anula el registro y se confirma esa anulación.
	sp, err := tx.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// usuarios.dni queda en NULL: el DNI solo se DECLARA (migración 025) y se
	// escribe cuando Secretaría verifica la identidad.
	err = sp.QueryRow(ctx, `
		INSERT INTO usuarios (username, email, password_hash, email_verified, email_verified_at)
		VALUES ($1, $2, $3, TRUE, CURRENT_TIMESTAMP)
		RETURNING id, username
	`, username, v.Email, passwordHash).Scan(&v.UsuarioID, &v.Username)
	if err != nil {
		_ = sp.Rollback(ctx)
		ocupado := classifyUniqueViolation(err)
		if errors.Is(ocupado, ErrDNIExists) || errors.Is(ocupado, ErrEmailExists) {
			if _, err := tx.Exec(ctx, `
				UPDATE registros_correo
				SET anulado_at = CURRENT_TIMESTAMP, password_hash = NULL,
				    nombres = NULL, apellido_paterno = NULL, apellido_materno = NULL
				WHERE id = $1
			`, id); err != nil {
				return nil, err
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
		}
		return nil, ocupado
	}
	rol, err := findRoleByName(ctx, sp, RolSolicitante)
	if err != nil {
		return nil, err
	}
	if _, err := sp.Exec(ctx, `INSERT INTO usuario_roles (usuario_id, rol_id) VALUES ($1, $2)`, v.UsuarioID, rol.ID); err != nil {
		return nil, errors.Join(ErrRoleAssignment, err)
	}
	if err := declararIdentidad(ctx, sp, v.UsuarioID, v.DNI, OrigenIdentidadCorreo); err != nil {
		return nil, err
	}
	if err := sp.Commit(ctx); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE registros_correo SET usuario_id = $2, verificado_at = CURRENT_TIMESTAMP WHERE id = $1
	`, id, v.UsuarioID); err != nil {
		return nil, err
	}
	return v, tx.Commit(ctx)
}

// sumarIntento registra un código incorrecto y CONFIRMA la transacción (el
// intento cuenta aunque la operación falle).
func sumarIntento(ctx context.Context, tx pgx.Tx, id string, maxIntentos int) error {
	var intentos int
	if err := tx.QueryRow(ctx, `UPDATE registros_correo SET intentos = intentos + 1 WHERE id = $1 RETURNING intentos`, id).Scan(&intentos); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if intentos >= maxIntentos {
		return ErrRegistroIntentos
	}
	return &ErrCodigoIncorrecto{Restantes: maxIntentos - intentos}
}

// bloquearVerificado: registro verificado (con cuenta) y no completado ni
// anulado, con el código correcto (un código incorrecto cuenta un intento).
func bloquearVerificado(ctx context.Context, tx pgx.Tx, id string, coincide func(hash []byte) bool, maxIntentos int) (*registroBloqueado, error) {
	b, err := bloquearRegistro(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if b.anulado || b.completado || b.existente {
		return nil, ErrRegistroNoDisponible
	}
	if b.intentos >= maxIntentos {
		return nil, ErrRegistroIntentos
	}
	if !coincide(b.codeHash) {
		return nil, sumarIntento(ctx, tx, id, maxIntentos)
	}
	if b.usuarioID == nil {
		return nil, ErrRegistroNoVerificado
	}
	return b, nil
}

// Completar: el perfil ya existe en Usuarios Service. Desde aquí la cuenta
// puede iniciar sesión y el código deja de servir; se borran del registro
// los datos personales y el hash de la clave (ya están en sus tablas).
func (r *RegistroCorreoRepository) Completar(ctx context.Context, id string, coincide func(hash []byte) bool, maxIntentos int) (string, string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	b, err := bloquearVerificado(ctx, tx, id, coincide, maxIntentos)
	if err != nil {
		return "", "", err
	}
	var dni string
	if err := tx.QueryRow(ctx, `
		UPDATE registros_correo
		SET completado_at = CURRENT_TIMESTAMP, password_hash = NULL,
		    nombres = NULL, apellido_paterno = NULL, apellido_materno = NULL
		WHERE id = $1
		RETURNING dni
	`, id).Scan(&dni); err != nil {
		return "", "", err
	}
	return *b.usuarioID, dni, tx.Commit(ctx)
}

// Resultados de public.cancelar_registro_correo_pendiente (migración 024).
const resultadoRegistroEliminado = "ELIMINADA"

// Cancelar: compensación cuando Usuarios Service rechaza el perfil. La
// eliminación la hace la función SECURITY DEFINER (auth_app no tiene
// DELETE sobre usuarios), que vuelve a comprobar las condiciones.
func (r *RegistroCorreoRepository) Cancelar(ctx context.Context, id string, coincide func(hash []byte) bool, maxIntentos int) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	b, err := bloquearVerificado(ctx, tx, id, coincide, maxIntentos)
	if err != nil {
		return "", err
	}
	var resultado string
	if err := tx.QueryRow(ctx, `SELECT public.cancelar_registro_correo_pendiente($1)`, id).Scan(&resultado); err != nil {
		return "", err
	}
	if resultado != resultadoRegistroEliminado {
		return "", ErrRegistroCorreoNoCancelable
	}
	return *b.usuarioID, tx.Commit(ctx)
}

// RegistroIncompleto: la cuenta viene de un registro verificado cuyo perfil
// todavía no se creó (no puede iniciar sesión).
func (r *RegistroCorreoRepository) RegistroIncompleto(ctx context.Context, usuarioID string) (bool, error) {
	var incompleto bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM registros_correo
			WHERE usuario_id = $1 AND completado_at IS NULL AND anulado_at IS NULL
		)
	`, usuarioID).Scan(&incompleto)
	// Sin la migración 024 no hay registros manuales: nada está incompleto.
	if EsquemaPendiente(err) {
		return false, nil
	}
	return incompleto, err
}

// ClavePendientePorCorreo: hash de la clave del registro pendiente (sin
// verificar) más reciente de ese correo, para avisar en el login por correo
// que falta verificarlo. ErrRegistroNoDisponible si no hay ninguno.
func (r *RegistroCorreoRepository) ClavePendientePorCorreo(ctx context.Context, email string) (string, error) {
	var hash string
	err := r.db.QueryRow(ctx, `
		SELECT password_hash FROM registros_correo
		WHERE email = $1 AND usuario_id IS NULL AND anulado_at IS NULL AND completado_at IS NULL
		  AND NOT cuenta_existente AND password_hash IS NOT NULL
		ORDER BY created_at DESC
		LIMIT 1
	`, email).Scan(&hash)
	if EsquemaPendiente(err) {
		return "", ErrRegistroNoDisponible
	}
	return hash, classifyNotFound(err, ErrRegistroNoDisponible)
}

// LogEvent: auditoría del registro manual (sin códigos, claves ni datos
// personales).
func (r *RegistroCorreoRepository) LogEvent(ctx context.Context, usuarioID, evento, resultado, motivo string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO auth_eventos (usuario_id, evento, proveedor, resultado, motivo)
		VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5)
	`, usuarioID, recortar(evento, 40), ProveedorCorreo, resultado, recortar(motivo, 60))
	return err
}
