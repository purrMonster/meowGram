package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"meowgram/server/internal/model"
	"strings"
)

var ErrReceiptConflict = errors.New("idempotency key reused with different content")

// CreateOnce commits the message and replay receipt together. The caller must
// authenticate on every attempt, including receipt replay.
func (r *MessageRepository) CreateOnce(ctx context.Context, kind, sender, name, key, text string) (*model.Message, bool, error) {
	if kind != "human" && kind != "bot" {
		return nil, false, errors.New("invalid sender kind")
	}
	digest := sha256.Sum256([]byte(text))
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	lock := fmt.Sprintf("receipt:%s:%d:%s:%s", kind, len(sender), sender, key)
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lock); err != nil {
		return nil, false, err
	}
	var oldHash []byte
	var messageID string
	err = tx.QueryRowContext(ctx, `SELECT request_hash,message_id FROM message_receipts WHERE sender_kind=$1 AND sender_id=$2 AND request_key=$3`, kind, sender, key).Scan(&oldHash, &messageID)
	replay := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	if replay && !bytes.Equal(oldHash, digest[:]) {
		return nil, false, ErrReceiptConflict
	}
	msg := &model.Message{SenderID: sender, SenderKind: kind, Username: name}
	if replay {
		err = tx.QueryRowContext(ctx, `SELECT id,text_content,created_at FROM messages WHERE id=$1`, messageID).Scan(&msg.ID, &msg.TextContent, &msg.CreatedAt)
	} else {
		column := "sender_id"
		if kind == "bot" {
			column = "bot_principal_id"
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO messages (`+column+`,text_content) VALUES($1,$2) RETURNING id,text_content,created_at`, sender, strings.TrimSpace(text)).Scan(&msg.ID, &msg.TextContent, &msg.CreatedAt)
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO message_receipts(sender_kind,sender_id,request_key,request_hash,message_id) VALUES($1,$2,$3,$4,$5)`, kind, sender, key, digest[:], msg.ID)
		}
	}
	if err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return msg, replay, nil
}
