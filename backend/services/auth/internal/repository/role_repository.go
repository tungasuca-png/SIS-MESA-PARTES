package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Role struct {
	ID   string
	Name string
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
