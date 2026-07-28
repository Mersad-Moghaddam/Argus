package mysql

import (
	"context"
	"database/sql"
	"time"

	"argus/internal/models"
)

func (r *Store) CreateUser(ctx context.Context, user models.User) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO users (email, name, password_hash) VALUES (?, ?, ?)`, user.Email, user.Name, user.PasswordHash)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.db.QueryRowContext(ctx, `SELECT id, email, name, password_hash, created_at, updated_at FROM users WHERE email=? LIMIT 1`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

func (r *Store) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	err := r.db.QueryRowContext(ctx, `SELECT id, email, name, password_hash, created_at, updated_at FROM users WHERE id=? LIMIT 1`, id).
		Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

func (r *Store) UpdateUserPassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash=? WHERE id=?`, passwordHash, id)
	return err
}

func (r *Store) CreateToken(ctx context.Context, token models.AuthToken) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO auth_tokens (user_id, token_hash, name, expires_at) VALUES (?, ?, ?, ?)`, token.UserID, token.TokenHash, token.Name, token.ExpiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Store) GetTokenByHash(ctx context.Context, tokenHash string) (*models.AuthToken, error) {
	var t models.AuthToken
	var lastUsed sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT id, user_id, token_hash, name, created_at, last_used_at, expires_at FROM auth_tokens WHERE token_hash=? LIMIT 1`, tokenHash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Name, &t.CreatedAt, &lastUsed, &t.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastUsed.Valid {
		t.LastUsedAt = &lastUsed.Time
	}
	return &t, nil
}

func (r *Store) ListTokensByUser(ctx context.Context, userID int64) ([]models.AuthToken, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, token_hash, name, created_at, last_used_at, expires_at FROM auth_tokens WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.AuthToken{}
	for rows.Next() {
		var token models.AuthToken
		var lastUsed sql.NullTime
		if err := rows.Scan(&token.ID, &token.UserID, &token.TokenHash, &token.Name, &token.CreatedAt, &lastUsed, &token.ExpiresAt); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			token.LastUsedAt = &lastUsed.Time
		}
		out = append(out, token)
	}
	return out, rows.Err()
}

func (r *Store) TouchToken(ctx context.Context, id int64, usedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE auth_tokens SET last_used_at=? WHERE id=?`, usedAt, id)
	return err
}

func (r *Store) DeleteToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM auth_tokens WHERE token_hash=?`, tokenHash)
	return err
}

func (r *Store) DeleteTokensByUserExcept(ctx context.Context, userID int64, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM auth_tokens WHERE user_id=? AND token_hash<>?`, userID, tokenHash)
	return err
}
