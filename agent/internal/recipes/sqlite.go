package recipes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	// Pure-Go sqlite driver (no CGO), registered as "sqlite".
	_ "modernc.org/sqlite"

	"github.com/restorable-dev/restorable/agent/internal/report"
)

// SQLiteCheck opens a restored SQLite database read-only and runs
// PRAGMA integrity_check. Needs no Docker.
type SQLiteCheck struct {
	Type string `yaml:"type"`
	// Path to the database file, relative to the snapshot root.
	Path string `yaml:"path"`
}

// TypeName implements Check.
func (c *SQLiteCheck) TypeName() string { return "sqlite" }

func (c *SQLiteCheck) validate() error {
	if c.Path == "" {
		return errors.New("needs a path")
	}
	return validateRelPath(c.Path)
}

// Run implements Check. Any way the database is unusable — missing, not a
// database, integrity errors — is a verification failure: the backup does
// not contain a working database.
func (c *SQLiteCheck) Run(ctx context.Context, t *Target) (report.Status, string) {
	abs, ok := resolve(t.Roots, strings.TrimSuffix(c.Path, "/"))
	if !ok {
		return report.StatusFail, fmt.Sprintf("database %q not found in restored snapshot", c.Path)
	}

	// immutable=1 guarantees the check never writes (no WAL, no journal
	// recovery) — the sandbox is disposable, but read-only is the promise.
	dsn := (&url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro&immutable=1"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return report.StatusFail, fmt.Sprintf("open %q: %v", c.Path, err)
	}
	defer db.Close() //nolint:errcheck // read-only handle

	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return report.StatusFail, fmt.Sprintf("%q is not a usable SQLite database: %v", c.Path, err)
	}
	defer rows.Close() //nolint:errcheck // fully drained below

	var problems []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return report.StatusError, fmt.Sprintf("read integrity_check result: %v", err)
		}
		if line != "ok" {
			problems = append(problems, line)
		}
	}
	if err := rows.Err(); err != nil {
		return report.StatusFail, fmt.Sprintf("integrity_check on %q: %v", c.Path, err)
	}
	if len(problems) > 0 {
		return report.StatusFail, fmt.Sprintf("integrity_check on %q found problems: %s", c.Path, joinIssues(problems))
	}
	return report.StatusPass, fmt.Sprintf("%q passes integrity_check", c.Path)
}
