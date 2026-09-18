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

// ReplacePermissionsByRoleID (Paso 21C): reemplaza ATÓMICAMENTE el conjunto
// completo de permisos de un rol — nunca "agrega", siempre dice cuál es el
// conjunto FINAL. Reutiliza exactamente el mismo patrón transaccional ya
// establecido por UserRepository.CreateUserWithRole (Begin -> defer
// Rollback -> validar dentro de la MISMA tx -> Commit): valida que el rol y
// CADA permission_id existan de verdad (vía findRoleByID/findPermissionByID,
// pasándoles la propia tx como executor) antes de tocar una sola fila —
// si cualquier validación falla, el rollback deja el estado anterior
// intacto, sin ninguna modificación parcial.
//
// permissionIDs debe llegar SIN duplicados (el Logic los normaliza antes de
// llamar acá) — la propia PK compuesta (role_id, permission_id) de
// role_permissions haría fallar un INSERT duplicado de todos modos, así
// que esta función no vuelve a deduplicar por su cuenta.
func (r *RolePermissionRepository) ReplacePermissionsByRoleID(ctx context.Context, roleID string, permissionIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := findRoleByID(ctx, tx, roleID); err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		if _, err := findPermissionByID(ctx, tx, permissionID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	for _, permissionID := range permissionIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			VALUES ($1, $2)
		`, roleID, permissionID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
