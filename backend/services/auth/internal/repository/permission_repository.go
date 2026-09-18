package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Permission struct {
	ID          string
	Codigo      string
	Nombre      string
	Descripcion string
	Modulo      string
	Estado      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PermissionRepository struct {
	db *pgxpool.Pool
}

func NewPermissionRepository(db *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{db: db}
}

func (r *PermissionRepository) Create(ctx context.Context, permission *Permission) (*Permission, error) {
	if permission == nil {
		return nil, errors.New("permission is required")
	}

	created := &Permission{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO permissions (codigo, nombre, descripcion, modulo, estado)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, codigo, nombre, COALESCE(descripcion, ''), modulo, estado, created_at, updated_at
	`, permission.Codigo, permission.Nombre, permission.Descripcion, permission.Modulo, permission.Estado).Scan(
		&created.ID,
		&created.Codigo,
		&created.Nombre,
		&created.Descripcion,
		&created.Modulo,
		&created.Estado,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return nil, classifyUniqueViolation(err)
	}

	return created, nil
}

func (r *PermissionRepository) GetByID(ctx context.Context, permissionID string) (*Permission, error) {
	return findPermissionByID(ctx, r.db, permissionID)
}

// findPermissionByID (Paso 21C): función libre reutilizable dentro de una
// transacción — mismo patrón ya establecido por findRoleByName/findRoleByID
// — usada por RolePermissionRepository.ReplacePermissionsByRoleID para
// validar cada permission_id DENTRO de la misma transacción del reemplazo.
func findPermissionByID(ctx context.Context, executor queryExecutor, permissionID string) (*Permission, error) {
	permission := &Permission{}
	err := executor.QueryRow(ctx, `
		SELECT id, codigo, nombre, COALESCE(descripcion, ''), modulo, estado, created_at, updated_at
		FROM permissions
		WHERE id = $1
	`, permissionID).Scan(
		&permission.ID,
		&permission.Codigo,
		&permission.Nombre,
		&permission.Descripcion,
		&permission.Modulo,
		&permission.Estado,
		&permission.CreatedAt,
		&permission.UpdatedAt,
	)
	if err != nil {
		return nil, classifyNotFound(err, ErrPermissionNotFound)
	}

	return permission, nil
}

func (r *PermissionRepository) GetByCode(ctx context.Context, code string) (*Permission, error) {
	permission := &Permission{}
	err := r.db.QueryRow(ctx, `
		SELECT id, codigo, nombre, COALESCE(descripcion, ''), modulo, estado, created_at, updated_at
		FROM permissions
		WHERE codigo = $1
	`, code).Scan(
		&permission.ID,
		&permission.Codigo,
		&permission.Nombre,
		&permission.Descripcion,
		&permission.Modulo,
		&permission.Estado,
		&permission.CreatedAt,
		&permission.UpdatedAt,
	)
	if err != nil {
		return nil, classifyNotFound(err, ErrPermissionNotFound)
	}

	return permission, nil
}

func (r *PermissionRepository) List(ctx context.Context) ([]*Permission, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, codigo, nombre, COALESCE(descripcion, ''), modulo, estado, created_at, updated_at
		FROM permissions
		ORDER BY modulo, codigo
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]*Permission, 0)
	for rows.Next() {
		permission := &Permission{}
		if err := rows.Scan(
			&permission.ID,
			&permission.Codigo,
			&permission.Nombre,
			&permission.Descripcion,
			&permission.Modulo,
			&permission.Estado,
			&permission.CreatedAt,
			&permission.UpdatedAt,
		); err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (r *PermissionRepository) Update(ctx context.Context, permission *Permission) (*Permission, error) {
	if permission == nil {
		return nil, errors.New("permission is required")
	}

	updated := &Permission{}
	err := r.db.QueryRow(ctx, `
		UPDATE permissions
		SET codigo = $1,
		    nombre = $2,
		    descripcion = $3,
		    modulo = $4,
		    estado = $5,
		    updated_at = NOW()
		WHERE id = $6
		RETURNING id, codigo, nombre, COALESCE(descripcion, ''), modulo, estado, created_at, updated_at
	`, permission.Codigo, permission.Nombre, permission.Descripcion, permission.Modulo, permission.Estado, permission.ID).Scan(
		&updated.ID,
		&updated.Codigo,
		&updated.Nombre,
		&updated.Descripcion,
		&updated.Modulo,
		&updated.Estado,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, ErrPermissionNotFound) {
			return nil, ErrPermissionNotFound
		}
		return nil, classifyUniqueViolation(err)
	}

	return updated, nil
}
