package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type BotPrincipal struct {
	ID        string
	Username  string
	CreatedAt time.Time
}

// BotRepository maps verified directory identities, never credentials or grants.
// Callers must authenticate and check current group membership on every request.
type BotRepository struct{ db *sql.DB }

func NewBotRepository(db *sql.DB) *BotRepository { return &BotRepository{db: db} }

func (r *BotRepository) Resolve(ctx context.Context, namespace, externalID, username string) (*BotPrincipal, error) {
	namespace = strings.TrimSpace(namespace)
	id, err := uuid.Parse(externalID)
	if err != nil || id == uuid.Nil || namespace == "" || len([]rune(namespace)) > 255 {
		return nil, errors.New("invalid directory identity")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("bot display name is required")
	}
	if runes := []rune(username); len(runes) > 100 {
		username = string(runes[:100])
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Serialize first provisioning of the same identity. A hash collision only
	// serializes unrelated identities; the composite PK remains authoritative.
	key := fmt.Sprintf("lldap:%d:%s:%s", len(namespace), namespace, id.String())
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
		return nil, err
	}
	principal := &BotPrincipal{}
	err = tx.QueryRowContext(ctx, `SELECT p.id, p.username, p.created_at
		FROM bot_directory_identities i JOIN bot_principals p ON p.id=i.principal_id
		WHERE i.provider='lldap' AND i.directory_namespace=$1 AND i.external_id=$2`, namespace, id.String()).Scan(&principal.ID, &principal.Username, &principal.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `INSERT INTO bot_principals(username) VALUES($1) RETURNING id, username, created_at`, username).Scan(&principal.ID, &principal.Username, &principal.CreatedAt)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO bot_directory_identities(provider,directory_namespace,external_id,principal_id) VALUES('lldap',$1,$2,$3)`, namespace, id.String(), principal.ID)
	} else if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE bot_principals SET username=$1 WHERE id=$2`, username, principal.ID)
		principal.Username = username
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return principal, nil
}
