package repository

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
)

// Each test owns a database; never migrate or drop the supplied database itself.
func botMigrationDatabase(t *testing.T) (*sql.DB, *migrate.Migrate, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("scratch PostgreSQL required")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/verification" {
		t.Skip("requires disposable PostgreSQL: TEST_DATABASE_URL database must be /verification")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	name := "bot_migration_" + uuid.New().String()[:8]
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP DATABASE ` + name); err != nil {
			t.Error(err)
		}
	})
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	root := os.Getenv("MIGRATIONS_PATH")
	if root == "" {
		root = "../../migrations"
	}
	root, err = filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	// Keep this historical Up/down/Up regression pinned to migration 000005
	// even after later migrations are added to the application.
	bounded := t.TempDir()
	files, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		prefix, _, ok := strings.Cut(file.Name(), "_")
		version, err := strconv.Atoi(prefix)
		if !ok || err != nil || version > 5 || file.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bounded, file.Name()), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	root = bounded
	source := (&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String()
	m, err := migrate.New(source, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sourceErr, databaseErr := m.Close(); sourceErr != nil || databaseErr != nil {
			t.Errorf("close migrate: source=%v database=%v", sourceErr, databaseErr)
		}
	})
	content, err := os.ReadFile(filepath.Join(root, "000005_bot_principals.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return db, m, string(content)
}

func assertBotMigrationSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	for label, query := range map[string]string{
		"sender constraint": `SELECT count(*) FROM pg_constraint WHERE conrelid='messages'::regclass AND conname='messages_exactly_one_sender' AND convalidated`,
		"bot index":         `SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE i.indrelid='messages'::regclass AND c.relname='idx_messages_bot_principal_id' AND i.indisvalid`,
	} {
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s count=%d, want 1: %v", label, count, err)
		}
	}
}

func TestBotPrincipalsMigrationRollbackReapply(t *testing.T) {
	db, m, _ := botMigrationDatabase(t)
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO users(authelia_sub,username) VALUES('legacy','human');
		INSERT INTO bot_principals(id,username) VALUES('20000000-0000-0000-0000-000000000001','bot');
		INSERT INTO messages(id,sender_id,text_content,created_at)
		VALUES('10000000-0000-0000-0000-000000000001','legacy','legacy message','2020-01-01T00:00:00Z');
		INSERT INTO messages(id,bot_principal_id,text_content,created_at)
		VALUES('10000000-0000-0000-0000-000000000002','20000000-0000-0000-0000-000000000001','bot message','2020-01-02T00:00:00Z');
	`); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		t.Helper()
		var rows string
		if err := db.QueryRow(`SELECT jsonb_agg(to_jsonb(m) ORDER BY id)::text FROM messages m`).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	before := snapshot()
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	if version, dirty, err := m.Version(); err != nil || version != 4 || dirty {
		t.Fatalf("rollback version=%d dirty=%v: %v", version, dirty, err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	if version, dirty, err := m.Version(); err != nil || version != 5 || dirty {
		t.Fatalf("reapply version=%d dirty=%v: %v", version, dirty, err)
	}
	if after := snapshot(); after != before {
		t.Fatalf("messages changed: before=%s after=%s", before, after)
	}
	assertBotMigrationSchema(t, db)
}

func TestBotPrincipalsMigrationRepeatedUp(t *testing.T) {
	db, m, up := botMigrationDatabase(t)
	if err := m.Migrate(4); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(up); err != nil {
			t.Fatalf("direct up execution %d: %v", i+1, err)
		}
		assertBotMigrationSchema(t, db)
	}
}

func TestBotPrincipalsMigrationPartialReapply(t *testing.T) {
	// Commit each DDL prefix separately to model interruption between statements.
	// Do not split the DO block on its internal semicolons.
	for prefixLength := 1; prefixLength <= 5; prefixLength++ {
		db, m, up := botMigrationDatabase(t)
		if err := m.Migrate(4); err != nil {
			t.Fatal(err)
		}
		beforeDO, rest, found := strings.Cut(up, "DO $$")
		if !found {
			t.Fatal("missing guarded constraint block")
		}
		block, _, found := strings.Cut(rest, "END $$;")
		if !found {
			t.Fatal("missing constraint block terminator")
		}
		statements := strings.Split(strings.TrimSpace(beforeDO), ";")
		statements = append(statements[:len(statements)-1], "DO $$"+block+"END $$;")
		for _, statement := range statements[:prefixLength] {
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("prefix %d: %v", prefixLength, err)
			}
		}
		if err := m.Up(); err != nil {
			t.Fatalf("reapply after %d statements: %v", prefixLength, err)
		}
		if version, dirty, err := m.Version(); err != nil || version != 5 || dirty {
			t.Fatalf("prefix %d version=%d dirty=%v: %v", prefixLength, version, dirty, err)
		}
		assertBotMigrationSchema(t, db)
	}
}
