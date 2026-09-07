package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID                  string
	Username            string
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

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	return r.findBy(ctx, `
		SELECT id, username, email, password_hash, estado, failed_login_attempts, locked_until
		FROM usuarios
		WHERE username = $1
	`, username)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	return r.findBy(ctx, `
		SELECT id, username, email, password_hash, estado, failed_login_attempts, locked_until
		FROM usuarios
		WHERE email = $1
	`, email)
}

func (r *UserRepository) FindByID(ctx context.Context, userID string) (*User, error) {
	return r.findBy(ctx, `
		SELECT id, username, email, password_hash, estado, failed_login_attempts, locked_until
		FROM usuarios
		WHERE id = $1
	`, userID)
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

func (r *UserRepository) RecordFailedLogin(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE usuarios
		SET failed_login_attempts = failed_login_attempts + 1,
		    locked_until = CASE
			    WHEN failed_login_attempts + 1 >= 5 THEN NOW() + INTERVAL '15 minutes'
			    ELSE locked_until
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`, userID)
	return err
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
