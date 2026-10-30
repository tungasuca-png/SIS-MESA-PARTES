package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ServiceClientActivo   = "ACTIVO"
	ServiceClientRevocado = "REVOCADO"
)

var (
	ErrServiceClientNotFound = errors.New("service client not found")
	// ErrPermisoInexistente: algún código pedido no existe o está inactivo
	// en el catálogo de permissions.
	ErrPermisoInexistente = errors.New("permission not found or inactive")
)

type ServiceClient struct {
	ID         string
	Nombre     string
	ClientID   string
	SecretHash []byte
	Estado     string
	Permisos   []string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

type ServiceClientRepository struct {
	db *pgxpool.Pool
}

func NewServiceClientRepository(db *pgxpool.Pool) *ServiceClientRepository {
	return &ServiceClientRepository{db: db}
}

const serviceClientColumns = `
	c.id, c.nombre, c.client_id, c.secret_hash, c.estado, c.created_at, c.last_used_at, c.revoked_at,
	COALESCE(ARRAY(
		SELECT p.codigo FROM service_client_permissions scp
		JOIN permissions p ON p.id = scp.permission_id
		WHERE scp.service_client_id = c.id AND p.estado = TRUE
		ORDER BY p.codigo
	), '{}')`

func scanServiceClient(row pgx.Row) (*ServiceClient, error) {
	var c ServiceClient
	err := row.Scan(&c.ID, &c.Nombre, &c.ClientID, &c.SecretHash, &c.Estado, &c.CreatedAt, &c.LastUsedAt, &c.RevokedAt, &c.Permisos)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrServiceClientNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Create registra el cliente y sus permisos en una sola transacción.
// createdBy es el usuario administrador autenticado.
func (r *ServiceClientRepository) Create(ctx context.Context, nombre, clientID string, secretHash []byte, createdBy string, permisos []string) (*ServiceClient, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO service_clients (nombre, client_id, secret_hash, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, nombre, clientID, secretHash, createdBy).Scan(&id); err != nil {
		return nil, err
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO service_client_permissions (service_client_id, permission_id)
		SELECT $1, p.id FROM permissions p
		WHERE p.codigo = ANY($2) AND p.estado = TRUE
	`, id, permisos)
	if err != nil {
		return nil, err
	}
	if int(tag.RowsAffected()) != len(permisos) {
		return nil, ErrPermisoInexistente
	}

	created, err := scanServiceClient(tx.QueryRow(ctx, `SELECT `+serviceClientColumns+` FROM service_clients c WHERE c.id = $1`, id))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return created, nil
}

func (r *ServiceClientRepository) FindByClientID(ctx context.Context, clientID string) (*ServiceClient, error) {
	return scanServiceClient(r.db.QueryRow(ctx, `SELECT `+serviceClientColumns+` FROM service_clients c WHERE c.client_id = $1`, clientID))
}

// TouchLastUsed registra la última emisión de token del cliente.
func (r *ServiceClientRepository) TouchLastUsed(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE service_clients SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}

// Revoke acepta el id (UUID) o el client_id. Es idempotente: revocar un
// cliente ya revocado conserva la fecha original.
func (r *ServiceClientRepository) Revoke(ctx context.Context, idOrClientID string) (*ServiceClient, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		UPDATE service_clients
		SET estado = 'REVOCADO', revoked_at = COALESCE(revoked_at, NOW())
		WHERE id::text = $1 OR client_id = $1
		RETURNING id
	`, idOrClientID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrServiceClientNotFound
	}
	if err != nil {
		return nil, err
	}
	return scanServiceClient(r.db.QueryRow(ctx, `SELECT `+serviceClientColumns+` FROM service_clients c WHERE c.id = $1`, id))
}

func (r *ServiceClientRepository) List(ctx context.Context) ([]*ServiceClient, error) {
	rows, err := r.db.Query(ctx, `SELECT `+serviceClientColumns+` FROM service_clients c ORDER BY c.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ServiceClient
	for rows.Next() {
		c, err := scanServiceClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
