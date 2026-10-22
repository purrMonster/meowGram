// Package testutil supplies isolated synthetic databases to integration tests.
package testutil

import (
	"database/sql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func Database(t *testing.T) (*sql.DB, *migrate.Migrate, string) {
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
		t.Skip("TEST_DATABASE_URL must select /verification")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	name := "send_probe_" + uuid.NewString()[:8]
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
	m, err := migrate.New((&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return db, m, root
}
