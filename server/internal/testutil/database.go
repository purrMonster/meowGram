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
	"strconv"
	"strings"
	"testing"
)

func Database(t *testing.T) (*sql.DB, *migrate.Migrate, string) {
	t.Helper()
	return DatabaseThrough(t, 0)
}

// DatabaseThrough bounds historical migration tests; zero uses every migration.
func DatabaseThrough(t *testing.T, last int) (*sql.DB, *migrate.Migrate, string) {
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
	if last > 0 {
		bounded := t.TempDir()
		files, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			prefix, _, ok := strings.Cut(file.Name(), "_")
			version, err := strconv.Atoi(prefix)
			if file.IsDir() || !ok || err != nil || version > last {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, file.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(bounded, file.Name()), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		root = bounded
	}
	m, err := migrate.New((&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return db, m, root
}
