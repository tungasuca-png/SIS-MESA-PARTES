package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RolePermission struct {
	RoleID       string
	PermissionID string
	CreatedAt    time.Time
}

type RolePermissionRepository struct {
	db *pgxpool.Pool
}

func NewRolePermissionRepository(db *pgxpool.Pool) *RolePermissionRepository {
	return &RolePermissionRepository{db: db}
}

func (r *RolePermissionRepository) GetPermissionsByRoleID(ctx context.Context, roleID string) ([]*Permission, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.codigo, p.nombre, COALESCE(p.descripcion, ''), p.modulo, p.estado, p.created_at, p.updated_at
		FROM role_permissions rp
		INNER JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		  AND p.estado = TRUE
		ORDER BY p.modulo, p.codigo
	`, roleID)
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

func (r *RolePermissionRepository) AssignPermissionToRole(ctx context.Context, roleID, permissionID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		VALUES ($1, $2)
	`, roleID, permissionID)
	if err != nil {
		return err
	}

	return nil
}

func (r *RolePermissionRepository) RemovePermissionFromRole(ctx context.Context, roleID, permissionID string) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM role_permissions
		WHERE role_id = $1
		  AND permission_id = $2
	`, roleID, permissionID)
	return err
}
