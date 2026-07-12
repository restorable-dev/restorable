package recipes

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/restorable-dev/restorable/agent/internal/report"
	"github.com/restorable-dev/restorable/agent/internal/sandbox"
)

const (
	defaultPostgresImage = "postgres:16-alpine"
	defaultReadyTimeout  = 120 * time.Second
	// containerPassword is throwaway: the container lives minutes, holds
	// only data the user already owns, and is never network-published.
	containerPassword = "restorable-sandbox"
)

// tableNameRe deliberately accepts only plain (optionally schema-qualified)
// identifiers so table names can be safely quoted into SQL.
var tableNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// TableAssertion asserts a minimum row count on one table.
type TableAssertion struct {
	Name    string `yaml:"name"`
	MinRows int64  `yaml:"min_rows"`
}

func validateTables(tables []TableAssertion) error {
	for _, tbl := range tables {
		if !tableNameRe.MatchString(tbl.Name) {
			return fmt.Errorf("table name %q must be a plain identifier like users or public.users", tbl.Name)
		}
		if tbl.MinRows < 0 {
			return fmt.Errorf("table %q: min_rows cannot be negative", tbl.Name)
		}
	}
	return nil
}

// quoteIdent renders a validated (schema-qualified) identifier with double
// quotes for postgres, or backticks for mysql.
func quoteIdent(name string, mysql bool) string {
	parts := strings.Split(name, ".")
	for i, p := range parts {
		if mysql {
			parts[i] = "`" + p + "`"
		} else {
			parts[i] = `"` + p + `"`
		}
	}
	return strings.Join(parts, ".")
}

// PostgresCheck loads a pg_dump file from the restored snapshot into a
// throwaway postgres container and asserts row counts.
type PostgresCheck struct {
	Type string `yaml:"type"`
	// Dump is the path to the pg_dump file, relative to the snapshot root.
	// Plain SQL, custom format (PGDMP), and gzipped dumps are supported.
	Dump    string           `yaml:"dump"`
	Image   string           `yaml:"image"`
	Timeout Duration         `yaml:"timeout"`
	Tables  []TableAssertion `yaml:"tables"`
}

// TypeName implements Check.
func (c *PostgresCheck) TypeName() string { return "postgres" }

func (c *PostgresCheck) validate() error {
	if c.Dump == "" {
		return errors.New("needs a dump path")
	}
	return validateTables(c.Tables)
}

// Run implements Check.
func (c *PostgresCheck) Run(ctx context.Context, t *Target) (report.Status, string) {
	dumpPath, ok := resolve(t.Roots, strings.TrimSuffix(c.Dump, "/"))
	if !ok {
		return report.StatusFail, fmt.Sprintf("dump file %q not found in restored snapshot", c.Dump)
	}
	runner, err := t.Docker(ctx)
	if err != nil {
		return report.StatusError, err.Error()
	}

	image := c.Image
	if image == "" {
		image = defaultPostgresImage
	}
	id, err := runner.StartContainer(ctx, sandbox.ContainerSpec{
		Image: image,
		Env:   []string{"POSTGRES_PASSWORD=" + containerPassword},
	})
	if err != nil {
		return report.StatusError, err.Error()
	}

	// The official image restarts postgres once during init, so require two
	// consecutive ready probes before trusting it.
	ready := 0
	err = waitFor(ctx, runner, id, c.Timeout.orDefault(defaultReadyTimeout), func() (bool, string) {
		code, out, perr := runner.Exec(ctx, id, []string{"pg_isready", "-U", "postgres"})
		if perr != nil {
			return false, perr.Error()
		}
		if code != 0 {
			ready = 0
			return false, tail(out, 1)
		}
		ready++
		return ready >= 2, "server ready once, confirming"
	})
	if err != nil {
		return report.StatusError, fmt.Sprintf("postgres container (%s): %v", image, err)
	}

	dump, size, custom, err := openDump(dumpPath)
	if err != nil {
		return report.StatusFail, fmt.Sprintf("open dump %q: %v", c.Dump, err)
	}
	defer dump.Close() //nolint:errcheck // read-only
	if err := runner.CopyTo(ctx, id, "/tmp", "dump", dump, size); err != nil {
		return report.StatusError, err.Error()
	}

	loadCmd := []string{"psql", "-U", "postgres", "-v", "ON_ERROR_STOP=1", "-q", "-f", "/tmp/dump"}
	if custom {
		loadCmd = []string{"pg_restore", "-U", "postgres", "-d", "postgres", "--exit-on-error", "/tmp/dump"}
	}
	code, out, err := runner.Exec(ctx, id, loadCmd)
	if err != nil {
		return report.StatusError, err.Error()
	}
	if code != 0 {
		return report.StatusFail, fmt.Sprintf("dump %q failed to load into postgres: %s", c.Dump, tail(out, 3))
	}

	for _, tbl := range c.Tables {
		query := "SELECT count(*) FROM " + quoteIdent(tbl.Name, false)
		code, out, err := runner.Exec(ctx, id, []string{"psql", "-U", "postgres", "-tA", "-c", query})
		if err != nil {
			return report.StatusError, err.Error()
		}
		if code != 0 {
			return report.StatusFail, fmt.Sprintf("count rows in %q: %s", tbl.Name, tail(out, 2))
		}
		n, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		if err != nil {
			return report.StatusError, fmt.Sprintf("unexpected count output for %q: %q", tbl.Name, tail(out, 1))
		}
		if n < tbl.MinRows {
			return report.StatusFail, fmt.Sprintf("table %q has %d rows, need at least %d", tbl.Name, n, tbl.MinRows)
		}
	}

	msg := "dump loaded into " + image
	if len(c.Tables) > 0 {
		msg += fmt.Sprintf(", %d table assertion(s) hold", len(c.Tables))
	}
	return report.StatusPass, msg
}
