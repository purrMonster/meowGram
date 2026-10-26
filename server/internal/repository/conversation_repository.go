package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrConversationForbidden = errors.New("request not permitted")
	ErrConversationInput     = errors.New("invalid conversation input")
	ErrConversationBusy      = errors.New("conversation changed concurrently")
	errConversationRetry     = errors.New("retry conversation snapshot")
)

type PrincipalRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type ConversationMember struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	DisplayLabel string `json:"display_label"`
}
type MemberSummary struct {
	Count   int                  `json:"count"`
	Members []ConversationMember `json:"members"`
}
type Conversation struct {
	ChannelID      *string       `json:"channel_id"`
	Kind           string        `json:"kind"`
	Name           *string       `json:"name"`
	MemberSummary  MemberSummary `json:"member_summary"`
	LastActivityAt *time.Time    `json:"last_activity_at"`
	UnreadCount    int           `json:"unread_count"`
	Muted          bool          `json:"muted"`
}
type ConversationPage struct {
	Conversations []Conversation `json:"conversations"`
	NextCursor    *string        `json:"next_cursor"`
}
type MemberPage struct {
	Members    []ConversationMember `json:"members"`
	NextCursor *string              `json:"next_cursor"`
}
type PrincipalPage struct {
	Principals []ConversationMember `json:"principals"`
	NextCursor *string              `json:"next_cursor"`
}

type ConversationRepository struct{ db *sql.DB }

func NewConversationRepository(db *sql.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func ParseChannel(s string) (string, error) {
	if len(s) != 36 {
		return "", ErrConversationInput
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return "", ErrConversationInput
	}
	return id.String(), nil
}
func NormalizePrincipal(p PrincipalRef) (PrincipalRef, error) {
	if !utf8.ValidString(p.ID) || p.ID == "" || strings.ContainsRune(p.ID, 0) {
		return p, ErrConversationInput
	}
	switch p.Kind {
	case "human":
		if utf8.RuneCountInString(p.ID) > 255 {
			return p, ErrConversationInput
		}
	case "bot":
		id, err := ParseChannel(p.ID)
		if err != nil {
			return p, err
		}
		p.ID = id
	default:
		return p, ErrConversationInput
	}
	return p, nil
}
func normalizedName(name *string) (*string, error) {
	if name == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*name)
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) || utf8.RuneCountInString(value) > 100 {
		return nil, ErrConversationInput
	}
	if value == "" {
		return nil, nil
	}
	return &value, nil
}
func principalKey(p PrincipalRef) string { return p.Kind + ":" + p.ID }
func sortRefs(refs []PrincipalRef) {
	sort.Slice(refs, func(i, j int) bool { return principalKey(refs[i]) < principalKey(refs[j]) })
}
func sameRefs(a, b []PrincipalRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func includes(refs []PrincipalRef, p PrincipalRef) bool {
	for _, x := range refs {
		if x == p {
			return true
		}
	}
	return false
}
func setLock(refs []PrincipalRef) int64 {
	h := sha256.New()
	var size [8]byte
	for _, p := range refs {
		for _, s := range []string{p.Kind, p.ID} {
			binary.BigEndian.PutUint64(size[:], uint64(len(s)))
			h.Write(size[:])
			h.Write([]byte(s))
		}
	}
	return int64(binary.BigEndian.Uint64(h.Sum(nil)[:8]))
}
func lockSets(ctx context.Context, tx *sql.Tx, sets ...[]PrincipalRef) error {
	keys := make([]int64, 0, len(sets))
	for _, s := range sets {
		keys = append(keys, setLock(s))
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for i, k := range keys {
		if i > 0 && keys[i-1] == k {
			continue
		}
		if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", k); err != nil {
			return err
		}
	}
	return nil
}
func (r *ConversationRepository) write(ctx context.Context, fn func(*sql.Tx) error) error {
	for attempt := 0; attempt < 8; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		err = fn(tx)
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		var pg *pgconn.PgError
		if errors.Is(err, errConversationRetry) || (errors.As(err, &pg) && (pg.Code == "40P01" || pg.Code == "40001")) {
			continue
		}
		// Invalid/missing selected principals can disappear concurrently with creation.
		if errors.As(err, &pg) && (pg.Code == "23503" || pg.Code == "23514" || pg.Code == "22001") {
			return ErrConversationInput
		}
		return err
	}
	return ErrConversationBusy
}
func memberRefs(ctx context.Context, tx *sql.Tx, id string) ([]PrincipalRef, error) {
	rows, err := tx.QueryContext(ctx, `SELECT member_kind,coalesce(human_sub,bot_id::text) FROM conversation_members WHERE conversation_id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refs := []PrincipalRef{}
	for rows.Next() {
		var p PrincipalRef
		if err = rows.Scan(&p.Kind, &p.ID); err != nil {
			return nil, err
		}
		refs = append(refs, p)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sortRefs(refs)
	return refs, nil
}
func requireHuman(refs []PrincipalRef, actor string) error {
	if !includes(refs, PrincipalRef{"human", actor}) {
		return ErrConversationForbidden
	}
	return nil
}
func lockConversation(ctx context.Context, tx *sql.Tx, id string) error {
	var found string
	err := tx.QueryRowContext(ctx, "SELECT id FROM conversations WHERE id=$1 FOR UPDATE", id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConversationForbidden
	}
	return err
}
func selectedExist(ctx context.Context, tx *sql.Tx, refs []PrincipalRef) error {
	// Stable principal order also bounds lock inversion with revision updates.
	for _, p := range refs {
		query := "SELECT authelia_sub FROM users WHERE authelia_sub=$1 FOR KEY SHARE"
		if p.Kind == "bot" {
			query = "SELECT id FROM bot_principals WHERE id=$1 FOR KEY SHARE"
		}
		var id string
		if err := tx.QueryRowContext(ctx, query, p.ID).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrConversationInput
			}
			return err
		}
	}
	return nil
}
func insertMember(ctx context.Context, tx *sql.Tx, id string, p PrincipalRef) error {
	var human, bot any
	if p.Kind == "human" {
		human = p.ID
	} else {
		bot = p.ID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO conversation_members(conversation_id,member_kind,human_sub,bot_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, id, p.Kind, human, bot)
	return err
}

// Cursors are opaque, versioned and bound to actor, route, filter and group.
// Authorization is re-evaluated regardless of cursor contents.
type conversationCursor struct {
	Version int       `json:"v"`
	Mode    string    `json:"m"`
	Actor   string    `json:"a"`
	Query   string    `json:"q"`
	Channel string    `json:"c"`
	At      time.Time `json:"t"`
	Kind    string    `json:"k"`
	ID      string    `json:"i"`
	Start   bool      `json:"s"`
}

func decodeConversationCursor(raw, mode, actor, query, channel string) (conversationCursor, error) {
	c := conversationCursor{Version: 1, Mode: mode, Actor: actor, Query: query, Channel: channel}
	if raw == "" {
		return c, nil
	}
	if len(raw) > 4096 {
		return c, ErrConversationInput
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return c, ErrConversationInput
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(&struct{}{}) != io.EOF || c.Version != 1 || c.Mode != mode || c.Actor != actor || c.Query != query || c.Channel != channel {
		return c, ErrConversationInput
	}
	if mode == "list" {
		if !c.Start {
			if _, err := ParseChannel(c.ID); err != nil || c.At.IsZero() {
				return c, ErrConversationInput
			}
		}
	} else {
		if _, err := NormalizePrincipal(PrincipalRef{c.Kind, c.ID}); err != nil {
			return c, err
		}
	}
	return c, nil
}
func encodeConversationCursor(c conversationCursor) *string {
	data, _ := json.Marshal(c)
	s := base64.RawURLEncoding.EncodeToString(data)
	return &s
}
func pageLimit(limit, def int) (int, error) {
	if limit == 0 {
		return def, nil
	}
	if limit < 1 || limit > 100 {
		return 0, ErrConversationInput
	}
	return limit, nil
}
