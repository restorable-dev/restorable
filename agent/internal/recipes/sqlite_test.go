package recipes

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dabelle/restorable/agent/internal/report"
)

// makeSQLiteDB creates a real database file with a little data.
func makeSQLiteDB(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close() //nolint:errcheck // test helper
	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	// Enough rows to guarantee multiple pages, so structural corruption in
	// page 2+ is possible.
	for i := 0; i < 200; i++ {
		if _, err := db.Exec("INSERT INTO users (name) VALUES (?)", strings.Repeat("x", 100)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSQLiteCheck(t *testing.T) {
	t.Run("healthy database passes", func(t *testing.T) {
		root := t.TempDir()
		makeSQLiteDB(t, filepath.Join(root, "data", "app.db"))
		target := &Target{Roots: []Root{{Dir: root, SnapPath: "/srv"}}}
		check := SQLiteCheck{Path: "data/app.db"}
		status, msg := check.Run(context.Background(), target)
		if status != report.StatusPass {
			t.Fatalf("status = %s (%s)", status, msg)
		}
		if !strings.Contains(msg, "integrity_check") {
			t.Errorf("message %q should mention integrity_check", msg)
		}
	})

	t.Run("garbage file fails", func(t *testing.T) {
		root := t.TempDir()
		bad := filepath.Join(root, "data", "app.db")
		if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(bad, []byte("this is not a database at all"), 0o600); err != nil {
			t.Fatal(err)
		}
		target := &Target{Roots: []Root{{Dir: root, SnapPath: "/srv"}}}
		check := SQLiteCheck{Path: "data/app.db"}
		status, msg := check.Run(context.Background(), target)
		if status != report.StatusFail {
			t.Fatalf("status = %s (%s), want fail", status, msg)
		}
	})

	t.Run("corrupted page fails integrity check", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "app.db")
		makeSQLiteDB(t, path)
		// SQLite has no page checksums, so payload corruption is invisible
		// to integrity_check — corrupt a page HEADER (b-tree structure)
		// instead, which it does verify.
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		const pageSize = 4096
		if len(data) < 3*pageSize {
			t.Fatalf("db unexpectedly small (%d bytes), fixture needs more rows", len(data))
		}
		for i := 2 * pageSize; i < 2*pageSize+64; i++ {
			data[i] ^= 0xFF
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		target := &Target{Roots: []Root{{Dir: root, SnapPath: "/srv"}}}
		check := SQLiteCheck{Path: "app.db"}
		status, msg := check.Run(context.Background(), target)
		if status == report.StatusPass {
			t.Fatalf("corrupted database passed integrity check (msg %q)", msg)
		}
	})

	t.Run("missing file fails", func(t *testing.T) {
		target := &Target{Roots: []Root{{Dir: t.TempDir(), SnapPath: "/srv"}}}
		check := SQLiteCheck{Path: "nope.db"}
		status, msg := check.Run(context.Background(), target)
		if status != report.StatusFail || !strings.Contains(msg, "not found") {
			t.Fatalf("status = %s (%s), want fail/not-found", status, msg)
		}
	})

	t.Run("check leaves no journal or wal files behind", func(t *testing.T) {
		root := t.TempDir()
		makeSQLiteDB(t, filepath.Join(root, "app.db"))
		target := &Target{Roots: []Root{{Dir: root, SnapPath: "/srv"}}}
		check := SQLiteCheck{Path: "app.db"}
		if status, msg := check.Run(context.Background(), target); status != report.StatusPass {
			t.Fatalf("status = %s (%s)", status, msg)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "app.db" {
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Errorf("read-only check left extra files: %v", names)
		}
	})
}
