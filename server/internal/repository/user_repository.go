package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"meowgram/server/internal/model"
)

// UserRepository handles user persistence and auto-provisioning.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository instantiates a new UserRepository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetOrCreateBySub fetches an existing user by Authelia subject or auto-provisions a new record.
func (r *UserRepository) GetOrCreateBySub(ctx context.Context, sub string, preferredUsername string) (*model.User, error) {
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return nil, fmt.Errorf("authelia sub cannot be empty")
	}

	username := strings.TrimSpace(preferredUsername)
	if username == "" {
		username = sub
	}
	// users.username is VARCHAR(100); truncate (rune-safe) instead of failing login.
	if r := []rune(username); len(r) > 100 {
		username = string(r[:100])
	}

	query := `
		INSERT INTO users (authelia_sub, username)
		VALUES ($1, $2)
		ON CONFLICT (authelia_sub) DO UPDATE
		SET username = CASE
			WHEN EXCLUDED.username <> '' AND EXCLUDED.username <> users.username THEN EXCLUDED.username
			ELSE users.username
		END
		RETURNING id, authelia_sub, username, created_at;
	`

	var user model.User
	err := r.db.QueryRowContext(ctx, query, sub, username).Scan(
		&user.ID,
		&user.AutheliaSub,
		&user.Username,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get or auto-provision user for sub %s: %w", sub, err)
	}

	return &user, nil
}
