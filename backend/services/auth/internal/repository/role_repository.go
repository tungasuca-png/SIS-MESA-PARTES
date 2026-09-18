package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Role struct {
	ID          string
	Name        string
	Descripcion string
	Estado      bool
}

type queryExecutor interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type RoleRepository struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) FindByName(ctx context.Context, name string) (*Role, error) {
	return findRoleByName(ctx, r.db, name)
}

func findRoleByName(ctx context.Context, executor queryExecutor, name string) (*Role, error) {
	role := &Role{}
	err := executor.QueryRow(ctx, `
		SELECT id, nombre
		FROM roles
		WHERE nombre = $1
		  AND estado = TRUE
	`, name).Scan(&role.ID, &role.Name)
	if err != nil {
		return nil, classifyNotFound(err, ErrRoleNotFound)
	}

	return role, nil
}

// GetByID (Paso 21B): valida que un role_id real exista antes de consultar
// sus permisos (GetRolePermissions) — a diferencia de FindByName, NO filtra
// por estado=TRUE: un rol inactivo sigue siendo un rol real (mismo criterio
// ya usado por PermissionRepository.List, que tampoco filtra por estado).
func (r *RoleRepository) GetByID(ctx context.Context, roleID string) (*Role, error) {
	return findRoleByID(ctx, r.db, roleID)
}

// findRoleByID (Paso 21C): misma función libre reutilizable dentro de una
// transacción (mismo patrón ya establecido por findRoleByName, usada por
// CreateUserWithRole) — RolePermissionRepository.ReplacePermissionsByRoleID
// la reutiliza pasándole un pgx.Tx en vez de r.db, para validar el rol
// DENTRO de la misma transacción que hace el reemplazo.
func findRoleByID(ctx context.Context, executor queryExecutor, roleID string) (*Role, error) {
	role := &Role{}
	err := executor.QueryRow(ctx, `
		SELECT id, nombre, COALESCE(descripcion, ''), estado
		FROM roles
		WHERE id = $1
	`, roleID).Scan(&role.ID, &role.Name, &role.Descripcion, &role.Estado)
	if err != nil {
		return nil, classifyNotFound(err, ErrRoleNotFound)
	}

	return role, nil
}

// List (Paso 21B): catálogo completo de roles reales, para la futura
// configuración administrativa — mismo criterio que PermissionRepository.
// List (sin filtrar por estado, orden determinista).
func (r *RoleRepository) List(ctx context.Context) ([]*Role, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, nombre, COALESCE(descripcion, ''), estado
		FROM roles
		ORDER BY nombre
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]*Role, 0)
	for rows.Next() {
		role := &Role{}
		if err := rows.Scan(&role.ID, &role.Name, &role.Descripcion, &role.Estado); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}
