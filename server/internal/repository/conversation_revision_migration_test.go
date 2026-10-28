package repository

import (
	"database/sql"
	"fmt"
	"meowgram/server/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func revisionMigrationSQL(t *testing.T, suffix string) string {
	t.Helper()
	root := os.Getenv("MIGRATIONS_PATH")
	if root == "" {
		root = "../../migrations"
	}
	b, e := os.ReadFile(filepath.Join(root, "000008_membership_revisions."+suffix+".sql"))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func revisionFixture(t *testing.T) *sql.DB {
	t.Helper()
	db, m, _ := testutil.DatabaseThrough(t, 7)
	if e := m.Up(); e != nil {
		t.Fatal(e)
	}
	seedConversationLegacy(t, db)
	schemaExec(t, db, `INSERT INTO conversations(id,name,created_by_kind,created_by_id) VALUES ('30000000-0000-0000-0000-000000000001','retained','human','human-a'); INSERT INTO conversation_members(conversation_id,member_kind,human_sub,bot_id) VALUES ('30000000-0000-0000-0000-000000000001','human','human-a',NULL),('30000000-0000-0000-0000-000000000001','bot',NULL,'20000000-0000-0000-0000-000000000001'); INSERT INTO conversation_user_state(user_sub,channel_id,muted) VALUES ('human-a','30000000-0000-0000-0000-000000000001',true)`)
	return db
}
func revisionProjection(t *testing.T, db *sql.DB) schemaProjection {
	return captureColumns(t, db, "users", "bot_principals", "bot_directory_identities", "messages", "message_idempotency", "push_devices", "conversations", "conversation_members", "conversation_user_state")
}
func TestConversationRevisionMigrationPreservesRowsAndReapplies(t *testing.T) {
	db := revisionFixture(t)
	p := revisionProjection(t, db)
	before := snapshotColumns(t, db, p)
	up := revisionMigrationSQL(t, "up")
	down := revisionMigrationSQL(t, "down")
	for _, statement := range []string{up, up, down, up} {
		schemaExec(t, db, statement)
		assertSnapshot(t, db, p, before)
	}
	human := managementRevision(t, db, "human", "human-b")
	schemaExec(t, db, "INSERT INTO conversation_members(conversation_id,member_kind,human_sub) VALUES ($1,'human','human-b')", schemaGroup)
	if managementRevision(t, db, "human", "human-b") != human+1 {
		t.Fatal("insert revision not exactly once after reinstall")
	}
	schemaExec(t, db, "DELETE FROM conversation_members WHERE conversation_id=$1 AND human_sub='human-b'", schemaGroup)
	if managementRevision(t, db, "human", "human-b") != human+2 {
		t.Fatal("delete revision not exactly once")
	}
	bot := managementRevision(t, db, "bot", schemaBot)
	schemaExec(t, db, "DELETE FROM users WHERE authelia_sub='human-a'")
	if managementRevision(t, db, "bot", schemaBot) != bot+1 {
		t.Fatal("cascade last-human revision missing")
	}
	schemaExec(t, db, "INSERT INTO conversation_members(conversation_id,member_kind,human_sub) VALUES ($1,'human','human-b')", schemaGroup)
	if managementRevision(t, db, "bot", schemaBot) != bot+2 {
		t.Fatal("first-human restoration revision missing")
	}
	bot = managementRevision(t, db, "bot", schemaBot)
	schemaExec(t, db, "DELETE FROM conversation_members WHERE conversation_id=$1 AND member_kind='bot'", schemaGroup)
	if managementRevision(t, db, "bot", schemaBot) != bot+1 {
		t.Fatal("bot deletion revision missing")
	}
}
func TestConversationRevisionMigrationPartialPrefixes(t *testing.T) {
	up := revisionMigrationSQL(t, "up")
	steps := strings.Split(strings.ReplaceAll(up, "\r\n", "\n"), "-- migration:step\n")[1:]
	if len(steps) == 0 {
		t.Fatal("no migration step boundaries")
	}
	for n := 1; n <= len(steps); n++ {
		t.Run(fmt.Sprintf("prefix_%02d", n), func(t *testing.T) {
			db := revisionFixture(t)
			p := revisionProjection(t, db)
			before := snapshotColumns(t, db, p)
			for _, step := range steps[:n] {
				schemaExec(t, db, step)
			}
			schemaExec(t, db, up)
			assertSnapshot(t, db, p, before)
			initial := managementRevision(t, db, "human", "human-b")
			schemaExec(t, db, "INSERT INTO conversation_members(conversation_id,member_kind,human_sub) VALUES ($1,'human','human-b')", schemaGroup)
			if managementRevision(t, db, "human", "human-b") != initial+1 {
				t.Fatal("recovered trigger not exactly once")
			}
		})
	}
}
