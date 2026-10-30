package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID                  string
	Username            string
	DNI                 string
	Email               string
	PasswordHash        string
	Estado              bool
	FailedLoginAttempts int
	LockedUntil         pgtype.Timestamptz
}

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

// FindByDNI es la búsqueda de credencial del login (DNI + clave). El DNI
// llega ya normalizado (8 dígitos, texto).
func (r *UserRepository) FindByDNI(ctx context.Context, dni string) (*User, error) {
	return r.findBy(ctx, `
		SELECT id, username, email, password_hash, estado, failed_login_attempts, locked_until
		FROM usuarios
		WHERE dni = $1
	`, dni)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	return r.findBy(ctx, `
		SELECT id, username, email, password_hash, estado, failed_login_attempts, locked_until
		FROM usuarios
		WHERE email = $1
	`, email)
}

// FindByID delega en findUserByID (misma función libre que ReplaceUserRole
// reutiliza dentro de una transacción — mismo patrón ya establecido por
// RoleRepository.GetByID/findRoleByID).
func (r *UserRepository) FindByID(ctx context.Context, userID string) (*User, error) {
	return findUserByID(ctx, r.db, userID)
}

func (r *UserRepository) findBy(ctx context.Context, query string, value string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx, query, value).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Estado,
		&user.FailedLoginAttempts,
		&user.LockedUntil,
	)
	if err != nil {
		return nil, classifyNotFound(err, ErrUserNotFound)
	}

	return user, nil
}

func findUserByID(ctx context.Context, executor queryExecutor, userID string) (*User, error) {
	user := &User{}
	err := executor.QueryRow(ctx, `
		SELECT id, username, email, password_hash, estado, failed_login_attempts, locked_until
		FROM usuarios
		WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.Estado,
		&user.FailedLoginAttempts,
		&user.LockedUntil,
	)
	if err != nil {
		return nil, classifyNotFound(err, ErrUserNotFound)
	}

	return user, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, user *User) (*User, error) {
	created := &User{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO usuarios (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email
	`, user.Username, user.Email, user.PasswordHash).Scan(
		&created.ID,
		&created.Username,
		&created.Email,
	)
	if err != nil {
		return nil, classifyUniqueViolation(err)
	}

	return created, nil
}

func (r *UserRepository) AssignRole(ctx context.Context, userID string, roleID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO usuario_roles (usuario_id, rol_id)
		VALUES ($1, $2)
	`, userID, roleID)
	if err != nil {
		return errors.Join(ErrRoleAssignment, err)
	}

	return nil
}

// RoleAssignedAfter informa si el rol actual del usuario se asignó DESPUÉS del
// instante indicado (comparación a resolución de segundos, la misma del claim
// iat de un JWT). Como ReplaceUserRole/CreateUserWithRole(ID) insertan una
// fila nueva en usuario_roles al asignar/cambiar el rol, created_at es el
// momento del último cambio de rol. Un access token emitido antes de ese
// momento lleva un claim "role" que ya no es el rol real.
func (r *UserRepository) RoleAssignedAfter(ctx context.Context, userID string, issuedAt time.Time) (bool, error) {
	var stale bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM usuario_roles
			WHERE usuario_id = $1
			  AND date_trunc('second', created_at) > $2
		)
	`, userID, issuedAt.UTC().Truncate(time.Second)).Scan(&stale)
	return stale, err
}

func (r *UserRepository) GetRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.nombre
		FROM roles r
		INNER JOIN usuario_roles ur ON ur.rol_id = r.id
		WHERE ur.usuario_id = $1
		  AND r.estado = TRUE
		ORDER BY r.nombre
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]string, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

// RecordFailedLogin suma un intento fallido y, al llegar a 5, bloquea la
// cuenta 15 minutos. Devuelve el bloqueo vigente (inválido si no quedó
// bloqueada) para que el login pueda informarlo.
func (r *UserRepository) RecordFailedLogin(ctx context.Context, userID string) (pgtype.Timestamptz, error) {
	var lockedUntil pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		UPDATE usuarios
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE
			    WHEN failed_login_attempts + 1 >= 5 THEN NOW() + INTERVAL '15 minutes'
			    ELSE locked_until
		    END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING locked_until
	`, userID).Scan(&lockedUntil)
	return lockedUntil, err
}

func (r *UserRepository) ClearExpiredLoginLock(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE usuarios
		SET failed_login_attempts = 0,
		    locked_until = NULL,
		    updated_at = NOW()
		WHERE id = $1
	  AND locked_until IS NOT NULL
	  AND locked_until <= NOW()
	`, userID)
	return err
}

func (r *UserRepository) ResetLoginState(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE usuarios
		SET failed_login_attempts = 0,
		    locked_until = NULL,
		    last_login_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
}

// CreateUserWithRole guarantees that user creation and role assignment share one transaction.
func (r *UserRepository) CreateUserWithRole(ctx context.Context, user *User, roleName string) (*User, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	role, err := findRoleByName(ctx, tx, roleName)
	if err != nil {
		return nil, err
	}

	created := &User{}
	err = tx.QueryRow(ctx, `
		INSERT INTO usuarios (username, email, password_hash, dni)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, username, email
	`, user.Username, user.Email, user.PasswordHash, user.DNI).Scan(
		&created.ID,
		&created.Username,
		&created.Email,
	)
	if err != nil {
		return nil, classifyUniqueViolation(err)
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO usuario_roles (usuario_id, rol_id)
		VALUES ($1, $2)
	`, created.ID, role.ID); err != nil {
		return nil, errors.Join(ErrRoleAssignment, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	created.PasswordHash = ""
	return created, nil
}

// CreateUserWithRoleID (administrativo, distinto de CreateUserWithRole):
// resuelve el rol por ID en vez de por nombre — a diferencia de Register
// (público, self-service, solo SOLICITANTE), esta operación la ejecuta un
// actor autorizado que elige cualquier rol real por su UUID, y debe
// distinguir explícitamente "rol inexistente" de "rol inactivo" (Register
// no lo necesita: FindByName ya filtra por estado=TRUE). Mismo patrón
// transaccional que CreateUserWithRole/ReplaceUserRole: BEGIN -> validar
// TODO dentro de la misma tx -> INSERT usuario -> INSERT usuario_roles ->
// COMMIT. Sigue siendo "1 usuario = 1 rol": una sola fila en usuario_roles.
func (r *UserRepository) CreateUserWithRoleID(ctx context.Context, user *User, roleID string) (*User, *Role, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	role, err := findRoleByID(ctx, tx, roleID)
	if err != nil {
		return nil, nil, err
	}
	if !role.Estado {
		return nil, nil, ErrRoleInactive
	}

	created := &User{}
	err = tx.QueryRow(ctx, `
		INSERT INTO usuarios (username, email, password_hash, dni)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		RETURNING id, username, email
	`, user.Username, user.Email, user.PasswordHash, user.DNI).Scan(
		&created.ID,
		&created.Username,
		&created.Email,
	)
	if err != nil {
		return nil, nil, classifyUniqueViolation(err)
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO usuario_roles (usuario_id, rol_id)
		VALUES ($1, $2)
	`, created.ID, role.ID); err != nil {
		return nil, nil, errors.Join(ErrRoleAssignment, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	created.PasswordHash = ""
	return created, role, nil
}

// ReplaceUserRole (Usuario -> Rol): reemplaza atómicamente el ÚNICO rol de
// un usuario ya existente — mismo patrón transaccional ya establecido por
// CreateUserWithRole y por RolePermissionRepository.ReplacePermissionsByRoleID
// (BEGIN -> validar TODO dentro de la misma tx -> DELETE -> INSERT ->
// COMMIT). El sistema trabaja con "1 usuario = 1 rol": esto nunca agrega
// un rol adicional, siempre reemplaza el que el usuario tenía (si tenía
// alguno — un usuario sin ningún rol también queda con exactamente el
// nuevo, sin caso especial). Al ejecutar siempre DELETE+INSERT, pedir el
// mismo rol que el usuario ya tiene es naturalmente idempotente (no hace
// falta detectarlo aparte). No usa AssignRole (ese solo hace INSERT, sin
// transacción ni reemplazo — dejaría dos roles).
func (r *UserRepository) ReplaceUserRole(ctx context.Context, userID, roleID string) (*User, *Role, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	user, err := findUserByID(ctx, tx, userID)
	if err != nil {
		return nil, nil, err
	}
	if !user.Estado {
		return nil, nil, ErrUserInactive
	}

	role, err := findRoleByID(ctx, tx, roleID)
	if err != nil {
		return nil, nil, err
	}
	if !role.Estado {
		return nil, nil, ErrRoleInactive
	}

	if _, err := tx.Exec(ctx, `DELETE FROM usuario_roles WHERE usuario_id = $1`, userID); err != nil {
		return nil, nil, errors.Join(ErrRoleAssignment, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO usuario_roles (usuario_id, rol_id)
		VALUES ($1, $2)
	`, userID, roleID); err != nil {
		return nil, nil, errors.Join(ErrRoleAssignment, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	return user, role, nil
}
