package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgconn"
	"meowgram/server/internal/testutil"
)

const conversationMigration = "000007_conversations.up.sql"
const schemaBot = "20000000-0000-0000-0000-000000000001"
const schemaGroup = "30000000-0000-0000-0000-000000000001"
const schemaGroup2 = "30000000-0000-0000-0000-000000000002"

func schemaExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func schemaVersion(t *testing.T, m *migrate.Migrate, want uint) {
	t.Helper()
	version, dirty, err := m.Version()
	if err != nil || dirty || version != want {
		t.Fatalf("version=%d dirty=%v want=%d: %v", version, dirty, want, err)
	}
}

func seedConversationLegacy(t *testing.T, db *sql.DB) {
	t.Helper()
	schemaExec(t, db, `
        INSERT INTO users(authelia_sub,username,created_at) VALUES
            ('human-a','A','2020-01-01T00:00:00.123456Z'),
            ('human-b','B','2020-01-01T00:00:00.123456Z'),
            ('20000000-0000-0000-0000-000000000001','same textual ID','2020-01-01Z');
        INSERT INTO bot_principals(id,username,created_at) VALUES
            ('20000000-0000-0000-0000-000000000001','bot','2020-01-01Z');
        INSERT INTO bot_directory_identities(provider,directory_namespace,external_id,principal_id)
            VALUES ('lldap','fixture','20000000-0000-0000-0000-000000000099','20000000-0000-0000-0000-000000000001');
        INSERT INTO messages(id,sender_id,bot_principal_id,text_content,created_at) VALUES
            ('10000000-0000-0000-0000-000000000001','human-a',NULL,'legacy 🐾','2020-01-02T03:04:05.123456Z'),
            ('10000000-0000-0000-0000-000000000002','human-b',NULL,'other human','2020-01-02T03:04:05.123456Z'),
            ('10000000-0000-0000-0000-000000000003',NULL,'20000000-0000-0000-0000-000000000001','bot','2020-01-02T03:04:05.123456Z');
        INSERT INTO message_idempotency(sender_kind,sender_id,operation_key,fingerprint,message_id,response_body,expires_at)
            SELECT CASE WHEN sender_id IS NULL THEN 'bot' ELSE 'human' END,
                   coalesce(sender_id,bot_principal_id::text), id::text, decode('0001ff','hex'), id,
                   decode('207b226c6567616379223a747275657d0a00ff','hex'),'2099-01-01Z'
            FROM messages;
        INSERT INTO push_devices(token,user_sub,expires_at) VALUES ('synthetic-device','human-a','2099-01-01Z');
    `)
}

// Capture columns before upgrading, then compare PostgreSQL binary record values,
// including raw BYTEA replay bytes and microsecond timestamps, not just row counts.
type schemaProjection map[string]string

func captureColumns(t *testing.T, db *sql.DB, tables ...string) schemaProjection {
	t.Helper()
	projection := schemaProjection{}
	for _, table := range tables {
		var cols string
		err := db.QueryRow(`SELECT string_agg(quote_ident(column_name),',' ORDER BY ordinal_position)
            FROM information_schema.columns WHERE table_schema='public' AND table_name=$1`, table).Scan(&cols)
		if err != nil {
			t.Fatal(err)
		}
		projection[table] = cols
	}
	return projection
}
func snapshotColumns(t *testing.T, db *sql.DB, projection schemaProjection) map[string]string {
	t.Helper()
	result := map[string]string{}
	for table, cols := range projection {
		// Identifiers originate only from the catalog and fixed test table names.
		q := fmt.Sprintf(`SELECT coalesce(string_agg(value,',' ORDER BY value),'') FROM
            (SELECT encode(record_send(ROW(%s)),'hex') AS value FROM %s) records`, cols, table)
		var value string
		if err := db.QueryRow(q).Scan(&value); err != nil {
			t.Fatal(err)
		}
		result[table] = value
	}
	return result
}
func legacyProjection(t *testing.T, db *sql.DB) schemaProjection {
	return captureColumns(t, db, "users", "bot_principals", "bot_directory_identities", "messages", "message_idempotency", "push_devices")
}
func assertSnapshot(t *testing.T, db *sql.DB, p schemaProjection, before map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(before, snapshotColumns(t, db, p)) {
		t.Fatal("persisted column bytes changed")
	}
}

func assertConversationSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	constraints := map[string][]string{
		"conversations":           {"conversations_pkey", "conversations_creator_check", "conversations_name_check"},
		"conversation_members":    {"conversation_members_principal_check", "conversation_members_conversation_fkey", "conversation_members_human_fkey", "conversation_members_bot_fkey"},
		"conversation_user_state": {"conversation_user_state_key", "conversation_user_state_user_fkey", "conversation_user_state_conversation_fkey", "conversation_user_state_read_check"},
		"messages":                {"messages_channel_fkey"},
		"users":                   {"users_membership_version_check"},
		"bot_principals":          {"bot_principals_membership_version_check"},
	}
	for table, names := range constraints {
		for _, name := range names {
			var count int
			err := db.QueryRow(`SELECT count(*) FROM pg_constraint WHERE conrelid=$1::regclass AND conname=$2 AND convalidated`, table, name).Scan(&count)
			if err != nil || count != 1 {
				t.Fatalf("constraint %s count=%d: %v", name, count, err)
			}
		}
	}
	for _, name := range []string{"conversations_pkey", "conversation_user_state_key", "idx_conversation_members_human", "idx_conversation_members_bot", "idx_messages_channel_page", "idx_messages_lounge_page"} {
		var count int
		err := db.QueryRow(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid
            WHERE c.relname=$1 AND c.relnamespace='public'::regnamespace AND i.indisvalid AND i.indisready`, name).Scan(&count)
		if err != nil || count != 1 {
			t.Fatalf("index %s count=%d: %v", name, count, err)
		}
	}
	var nullsEqual bool
	if err := db.QueryRow(`SELECT indnullsnotdistinct FROM pg_index WHERE indexrelid='conversation_user_state_key'::regclass`).Scan(&nullsEqual); err != nil || !nullsEqual {
		t.Fatalf("lounge uniqueness does not treat nulls as equal: %v", err)
	}
}

func legacyReads(t *testing.T, db *sql.DB) []byte {
	t.Helper()
	repo := NewMessageRepository(db)
	recent, err := repo.GetRecent(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	after, err := repo.GetMessagesAfter(context.Background(), time.Unix(0, 0), "", 500)
	if err != nil {
		t.Fatal(err)
	}
	// Cover a cursor within equal-time rows as well as the full lounge.
	page, err := repo.GetMessagesAfter(context.Background(), recent[0].CreatedAt, recent[0].ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal([]any{recent, after, page})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func seedConversationRows(t *testing.T, db *sql.DB) {
	t.Helper()
	schemaExec(t, db, `
        INSERT INTO conversations(id,name,created_by_kind,created_by_id) VALUES
            ('30000000-0000-0000-0000-000000000001',NULL,'human','human-a'),
            ('30000000-0000-0000-0000-000000000002','Group 🐾','human','human-a');
        INSERT INTO conversation_members(conversation_id,member_kind,human_sub)
            SELECT id,'human',p FROM conversations CROSS JOIN unnest(ARRAY['human-a','human-b']) AS p;
        INSERT INTO conversation_members(conversation_id,member_kind,bot_id)
            SELECT id,'bot','20000000-0000-0000-0000-000000000001' FROM conversations;
        INSERT INTO conversation_user_state(user_sub,channel_id,muted)
            SELECT p,id,true FROM conversations CROSS JOIN unnest(ARRAY['human-a','human-b']) AS p;
        INSERT INTO conversation_user_state(user_sub) VALUES ('human-a'),('human-b');
        INSERT INTO messages(id,sender_id,text_content,channel_id)
            VALUES ('10000000-0000-0000-0000-000000000004','human-b','group retained','30000000-0000-0000-0000-000000000001');
    `)
}

func TestConversationMigrationFresh(t *testing.T) {
	db, m, _ := testutil.DatabaseThrough(t, 7)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	schemaVersion(t, m, 7)
	assertConversationSchema(t, db)
	seedConversationLegacy(t, db)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM messages WHERE channel_id IS NOT NULL").Scan(&count); err != nil || count != 0 {
		t.Fatal("legacy insert did not default to lounge", err)
	}
	if err := m.Up(); err != migrate.ErrNoChange {
		t.Fatalf("second Up=%v", err)
	}
}

func TestConversationMigrationLegacyAndDownUp(t *testing.T) {
	db, m, _ := testutil.DatabaseThrough(t, 7)
	if err := m.Migrate(6); err != nil {
		t.Fatal(err)
	}
	seedConversationLegacy(t, db)
	projection := legacyProjection(t, db)
	before := snapshotColumns(t, db, projection)
	reads := legacyReads(t, db)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	schemaVersion(t, m, 7)
	assertConversationSchema(t, db)
	assertSnapshot(t, db, projection, before)
	if string(reads) != string(legacyReads(t, db)) {
		t.Fatal("legacy GetRecent/GetMessagesAfter changed")
	}
	seedConversationRows(t, db)
	all := captureColumns(t, db, "users", "bot_principals", "messages", "message_idempotency", "conversation_members", "conversations", "conversation_user_state")
	retained := snapshotColumns(t, db, all)
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	schemaVersion(t, m, 6)
	assertSnapshot(t, db, all, retained)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	schemaVersion(t, m, 7)
	assertSnapshot(t, db, all, retained)
	assertConversationSchema(t, db)
}

func TestConversationMigrationRepeatedUp(t *testing.T) {
	db, m, root := testutil.DatabaseThrough(t, 7)
	if err := m.Migrate(6); err != nil {
		t.Fatal(err)
	}
	seedConversationLegacy(t, db)
	p := legacyProjection(t, db)
	before := snapshotColumns(t, db, p)
	up, err := os.ReadFile(filepath.Join(root, conversationMigration))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		schemaExec(t, db, string(up))
		assertConversationSchema(t, db)
		assertSnapshot(t, db, p, before)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	schemaVersion(t, m, 7)
}

func TestConversationMigrationPartialPrefixes(t *testing.T) {
	// Each marker is one complete DDL statement/DO block, never an internal
	// semicolon. Commit every prefix separately, then recover via golang-migrate.
	root := os.Getenv("MIGRATIONS_PATH")
	if root == "" {
		root = "../../migrations"
	}
	up, err := os.ReadFile(filepath.Join(root, conversationMigration))
	if err != nil {
		t.Fatal(err)
	}
	steps := strings.Split(strings.ReplaceAll(string(up), "\r\n", "\n"), "-- migration:step\n")[1:]
	if len(steps) == 0 {
		t.Fatal("missing statement boundaries")
	}
	for n := 1; n <= len(steps); n++ {
		t.Run(fmt.Sprintf("prefix_%02d", n), func(t *testing.T) {
			db, m, _ := testutil.DatabaseThrough(t, 7)
			if err := m.Migrate(6); err != nil {
				t.Fatal(err)
			}
			seedConversationLegacy(t, db)
			p := legacyProjection(t, db)
			before := snapshotColumns(t, db, p)
			for _, step := range steps[:n] {
				schemaExec(t, db, step)
			}
			if err := m.Up(); err != nil {
				t.Fatal(err)
			}
			schemaVersion(t, m, 7)
			assertConversationSchema(t, db)
			assertSnapshot(t, db, p, before)
		})
	}
}

func TestConversationMigrationConstraints(t *testing.T) {
	db, m, _ := testutil.DatabaseThrough(t, 7)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	seedConversationLegacy(t, db)
	seedConversationRows(t, db)
	// Equal ID strings across human and bot namespaces are different members.
	schemaExec(t, db, "INSERT INTO conversation_members(conversation_id,member_kind,human_sub) VALUES($1,'human',$2)", schemaGroup, schemaBot)
	schemaExec(t, db, "INSERT INTO conversation_user_state(user_sub,channel_id) VALUES($1,NULL)", schemaBot)
	// Null and 100 Unicode code points are valid names; 101 is not.
	schemaExec(t, db, "INSERT INTO conversations(name,created_by_kind,created_by_id) VALUES($1,'human','human-a')", strings.Repeat("🐾", 100))
	schemaExec(t, db, "UPDATE conversation_user_state SET read_at='2020-01-02Z',read_message_id='10000000-0000-0000-0000-000000000001' WHERE user_sub='human-a' AND channel_id IS NULL")
	tests := []struct{ name, sql, code string }{
		{"both_principals", "INSERT INTO conversation_members VALUES('" + schemaGroup + "','human','human-a','" + schemaBot + "',now())", "23514"},
		{"neither_principal", "INSERT INTO conversation_members VALUES('" + schemaGroup + "','human',NULL,NULL,now())", "23514"},
		{"human_with_bot", "INSERT INTO conversation_members VALUES('" + schemaGroup + "','human',NULL,'" + schemaBot + "',now())", "23514"},
		{"bot_with_human", "INSERT INTO conversation_members VALUES('" + schemaGroup + "','bot','human-a',NULL,now())", "23514"},
		{"unknown_kind", "INSERT INTO conversation_members VALUES('" + schemaGroup + "','system','human-a',NULL,now())", "23514"},
		{"duplicate_human", "INSERT INTO conversation_members(conversation_id,member_kind,human_sub) VALUES('" + schemaGroup + "','human','human-a')", "23505"},
		{"duplicate_bot", "INSERT INTO conversation_members(conversation_id,member_kind,bot_id) VALUES('" + schemaGroup + "','bot','" + schemaBot + "')", "23505"},
		{"missing_human", "INSERT INTO conversation_members(conversation_id,member_kind,human_sub) VALUES('" + schemaGroup + "','human','absent')", "23503"},
		{"missing_bot", "INSERT INTO conversation_members(conversation_id,member_kind,bot_id) VALUES('" + schemaGroup + "','bot','20000000-0000-0000-0000-000000000099')", "23503"},
		{"missing_conversation", "INSERT INTO conversation_members(conversation_id,member_kind,human_sub) VALUES('30000000-0000-0000-0000-000000000099','human','human-a')", "23503"},
		{"duplicate_lounge_state", "INSERT INTO conversation_user_state(user_sub) VALUES('human-a')", "23505"},
		{"duplicate_group_state", "INSERT INTO conversation_user_state(user_sub,channel_id) VALUES('human-a','" + schemaGroup + "')", "23505"},
		{"state_requires_human", "INSERT INTO conversation_user_state(user_sub) VALUES('absent-bot-only')", "23503"},
		{"state_missing_group", "INSERT INTO conversation_user_state(user_sub,channel_id) VALUES('human-a','30000000-0000-0000-0000-000000000099')", "23503"},
		{"partial_read_timestamp", "UPDATE conversation_user_state SET read_at=now() WHERE user_sub='human-b'", "23514"},
		{"partial_read_id", "UPDATE conversation_user_state SET read_message_id='10000000-0000-0000-0000-000000000001' WHERE user_sub='human-b'", "23514"},
		{"empty_name", "UPDATE conversations SET name='' WHERE id='" + schemaGroup + "'", "23514"},
		{"space_name", "UPDATE conversations SET name=' ' WHERE id='" + schemaGroup + "'", "23514"},
		{"untrimmed_name", "UPDATE conversations SET name=E'\tname\n' WHERE id='" + schemaGroup + "'", "23514"},
		{"unicode_space_name", "UPDATE conversations SET name=U&'\\00A0name\\3000' WHERE id='" + schemaGroup + "'", "23514"},
		{"long_name", "UPDATE conversations SET name=repeat('🐾',101) WHERE id='" + schemaGroup + "'", "23514"},
		{"invalid_creator_kind", "UPDATE conversations SET created_by_kind='owner' WHERE id='" + schemaGroup + "'", "23514"},
		{"empty_creator", "UPDATE conversations SET created_by_id='' WHERE id='" + schemaGroup + "'", "23514"},
		{"missing_message_channel", "INSERT INTO messages(sender_id,text_content,channel_id) VALUES('human-a','invalid','30000000-0000-0000-0000-000000000099')", "23503"},
		{"negative_human_revision", "UPDATE users SET membership_version=-1 WHERE authelia_sub='human-a'", "23514"},
		{"negative_bot_revision", "UPDATE bot_principals SET membership_version=-1 WHERE id='" + schemaBot + "'", "23514"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := db.Exec(tt.sql)
			var pg *pgconn.PgError
			if !errors.As(err, &pg) || pg.Code != tt.code {
				t.Fatalf("want SQLSTATE %s, got %v", tt.code, err)
			}
		})
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM conversation_members WHERE conversation_id=$1 AND (human_sub=$2 OR bot_id::text=$2)", schemaGroup, schemaBot).Scan(&count); err != nil || count != 2 {
		t.Fatal("typed identity collision", err)
	}
	for _, table := range []string{"users", "bot_principals"} {
		if err := db.QueryRow("SELECT count(*) FROM " + table + " WHERE membership_version<>0").Scan(&count); err != nil || count != 0 {
			t.Fatal("bad initial revision", err)
		}
	}
}

func TestConversationMigrationDeleteSemantics(t *testing.T) {
	db, m, _ := testutil.DatabaseThrough(t, 7)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	seedConversationLegacy(t, db)
	seedConversationRows(t, db)
	// A read cursor may refer to a subsequently deleted message; it must not
	// prevent deleting that message's human sender (the existing R2 behavior).
	schemaExec(t, db, "UPDATE conversation_user_state SET read_at='2020-01-02Z',read_message_id='10000000-0000-0000-0000-000000000001' WHERE user_sub='human-b' AND channel_id IS NULL")
	var otherMessages string
	if err := db.QueryRow("SELECT jsonb_agg(to_jsonb(m) ORDER BY id)::text FROM messages m WHERE sender_id IS DISTINCT FROM 'human-a'").Scan(&otherMessages); err != nil {
		t.Fatal(err)
	}
	stateBefore := snapshotColumns(t, db, captureColumns(t, db, "conversations"))
	schemaExec(t, db, "DELETE FROM users WHERE authelia_sub='human-a'")
	checks := map[string]int{
		"SELECT count(*) FROM conversation_members WHERE human_sub='human-a'":                        0,
		"SELECT count(*) FROM conversation_user_state WHERE user_sub='human-a'":                      0,
		"SELECT count(*) FROM conversation_members WHERE human_sub='human-b'":                        2,
		"SELECT count(*) FROM conversation_members WHERE member_kind='bot'":                          2,
		"SELECT count(*) FROM conversation_user_state WHERE user_sub='human-b'":                      3,
		"SELECT count(*) FROM messages WHERE sender_id='human-a'":                                    0,
		"SELECT count(*) FROM message_idempotency WHERE sender_kind='human' AND sender_id='human-a'": 0,
	}
	for q, want := range checks {
		var got int
		if err := db.QueryRow(q).Scan(&got); err != nil || got != want {
			t.Fatalf("delete check got=%d want=%d: %v", got, want, err)
		}
	}
	assertSnapshot(t, db, captureColumns(t, db, "conversations"), stateBefore)
	var after string
	if err := db.QueryRow("SELECT jsonb_agg(to_jsonb(m) ORDER BY id)::text FROM messages m WHERE sender_id IS DISTINCT FROM 'human-a'").Scan(&after); err != nil || after != otherMessages {
		t.Fatal("other messages changed", err)
	}
	// Isolate the message FK from state/membership restrictions.
	schemaExec(t, db, "DELETE FROM conversation_user_state WHERE channel_id IS NOT NULL; DELETE FROM conversation_members")
	_, err := db.Exec("DELETE FROM conversations WHERE id=$1", schemaGroup)
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != "23503" || pg.ConstraintName != "messages_channel_fkey" {
		t.Fatalf("conversation history not protected: %v", err)
	}
	// Isolate membership references from pre-existing bot message/directory FKs.
	const onlyBot = "20000000-0000-0000-0000-000000000002"
	schemaExec(t, db, "INSERT INTO bot_principals(id,username) VALUES($1,'membership-only')", onlyBot)
	schemaExec(t, db, "INSERT INTO conversation_members(conversation_id,member_kind,bot_id) VALUES($1,'bot',$2)", schemaGroup2, onlyBot)
	_, err = db.Exec("DELETE FROM bot_principals WHERE id=$1", onlyBot)
	if !errors.As(err, &pg) || pg.Code != "23503" || pg.ConstraintName != "conversation_members_bot_fkey" {
		t.Fatalf("bot membership not protected: %v", err)
	}
	_, err = db.Exec("DELETE FROM conversations WHERE id=$1", schemaGroup2)
	if !errors.As(err, &pg) || pg.Code != "23503" || pg.ConstraintName != "conversation_members_conversation_fkey" {
		t.Fatalf("conversation membership not protected: %v", err)
	}
}
