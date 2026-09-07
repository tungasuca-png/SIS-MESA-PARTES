package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenRevoked  = errors.New("refresh token revoked")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
)

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash []byte
	ExpiresAt time.Time
	RevokedAt pgtype.Timestamptz
	CreatedAt time.Time
}

type RefreshTokenRepository struct {
	db *pgxpool.Pool
}

func NewRefreshTokenRepository(db *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (usuario_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash []byte) (*RefreshToken, error) {
	return scanRefreshToken(r.db.QueryRow(ctx, `
		SELECT id, usuario_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, tokenHash))
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE id = $1
	`, tokenID)
	return err
}

func (r *RefreshTokenRepository) Rotate(ctx context.Context, tokenHash, newTokenHash []byte, expiresAt time.Time) (*RefreshToken, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	current, err := scanRefreshToken(tx.QueryRow(ctx, `
		SELECT id, usuario_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE
	`, tokenHash))
	if err != nil {
		return nil, err
	}
	if current.RevokedAt.Valid {
		return nil, ErrRefreshTokenRevoked
	}
	if !time.Now().Before(current.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}

	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE id = $1
		  AND revoked_at IS NULL
	`, current.ID); err != nil {
		return nil, err
	}

	rotated, err := scanRefreshToken(tx.QueryRow(ctx, `
		INSERT INTO refresh_tokens (usuario_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, usuario_id, token_hash, expires_at, revoked_at, created_at
	`, current.UserID, newTokenHash, expiresAt))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return rotated, nil
}

func scanRefreshToken(row pgx.Row) (*RefreshToken, error) {
	token := &RefreshToken{}
	err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefreshTokenNotFound
	}
	if err != nil {
		return nil, err
	}

	return token, nil
}
