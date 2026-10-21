package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"meowgram/server/internal/model"
)

var ErrIdempotencyConflict = errors.New("idempotency key already used for different text")

type SendPrincipal struct{ Kind, ID, Username string }
type SendResult struct {
	Message *model.Message
	Body    []byte
	Replay  bool
}

// CreateIdempotent commits the message and exact response together. authorizeWrite
// runs only for a new operation under the transaction lock, never for a replay.
func (r *MessageRepository) CreateIdempotent(ctx context.Context, p SendPrincipal, key, rawText, text string, retention time.Duration, authorizeWrite func() error) (*SendResult, error) {
	if (p.Kind != "human" && p.Kind != "bot") || p.ID == "" || len(key) < 1 || len(key) > 255 || retention < 168*time.Hour {
		return nil, errors.New("invalid send parameters")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	lock := fmt.Sprintf("send:%s:%d:%s:%s", p.Kind, len(p.ID), p.ID, key)
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lock); err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(rawText))
	var oldHash, body []byte
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT fingerprint,response_body,expires_at > clock_timestamp() FROM message_idempotency WHERE sender_kind=$1 AND sender_id=$2 AND operation_key=$3`, p.Kind, p.ID, key).Scan(&oldHash, &body, &active)
	if err == nil && active {
		if !bytes.Equal(hash[:], oldHash) {
			return nil, ErrIdempotencyConflict
		}
		var message model.Message
		if err := json.Unmarshal(body, &message); err != nil {
			return nil, err
		}
		if err := finishIdempotent(ctx, tx); err != nil {
			return nil, err
		}
		return &SendResult{Message: &message, Body: body, Replay: true}, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err := authorizeWrite(); err != nil {
		return nil, err
	}
	message := &model.Message{SenderID: p.ID, SenderKind: p.Kind, Username: p.Username, TextContent: text}
	if p.Kind == "bot" {
		err = tx.QueryRowContext(ctx, `INSERT INTO messages(bot_principal_id,text_content) VALUES($1,$2) RETURNING id,created_at`, p.ID, text).Scan(&message.ID, &message.CreatedAt)
	} else {
		err = tx.QueryRowContext(ctx, `INSERT INTO messages(sender_id,text_content) VALUES($1,$2) RETURNING id,created_at`, p.ID, text).Scan(&message.ID, &message.CreatedAt)
	}
	if err != nil {
		return nil, err
	}
	message.CreatedAt = message.CreatedAt.UTC()
	body, err = json.Marshal(message)
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	_, err = tx.ExecContext(ctx, `INSERT INTO message_idempotency(sender_kind,sender_id,operation_key,fingerprint,message_id,response_body,expires_at)
 VALUES($1,$2,$3,$4,$5,$6,clock_timestamp()+($7 * interval '1 second'))
 ON CONFLICT(sender_kind,sender_id,operation_key) DO UPDATE SET fingerprint=EXCLUDED.fingerprint,message_id=EXCLUDED.message_id,response_body=EXCLUDED.response_body,expires_at=EXCLUDED.expires_at`, p.Kind, p.ID, key, hash[:], message.ID, body, retention.Seconds())
	if err != nil {
		return nil, err
	}
	if err = finishIdempotent(ctx, tx); err != nil {
		return nil, err
	}
	return &SendResult{Message: message, Body: body}, nil
}

// Cleanup shares the send transaction and never waits for another send's rows.
// The expiry index bounds candidate discovery; the batch bounds write work.
func finishIdempotent(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `WITH expired AS (
 SELECT sender_kind,sender_id,operation_key FROM message_idempotency
 WHERE expires_at <= clock_timestamp() ORDER BY expires_at
 LIMIT 100 FOR UPDATE SKIP LOCKED
 ) DELETE FROM message_idempotency m USING expired e
 WHERE m.sender_kind=e.sender_kind AND m.sender_id=e.sender_id AND m.operation_key=e.operation_key`)
	if err != nil {
		return err
	}
	return tx.Commit()
}
