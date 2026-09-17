package recipes

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/restorable-dev/restorable/agent/internal/report"
	"github.com/restorable-dev/restorable/agent/internal/sandbox"
)

const (
	defaultMySQLImage = "mysql:8"
	mysqlDatabase     = "restorable"
	// The official mysql image needs noticeably longer to initialize than
	// postgres, so its default readiness timeout is higher.
	defaultMySQLReadyTimeout = 180 * time.Second
)

// MySQLCheck loads a mysqldump file from the restored snapshot into a
// throwaway mysql container and asserts row counts.
type MySQLCheck struct {
	Type string `yaml:"type"`
	// Dump is the path to the mysqldump file, relative to the snapshot
	// root. Plain SQL and gzipped dumps are supported.
	Dump    string           `yaml:"dump"`
	Image   string           `yaml:"image"`
	Timeout Duration         `yaml:"timeout"`
	Tables  []TableAssertion `yaml:"tables"`
}

// TypeName implements Check.
func (c *MySQLCheck) TypeName() string { return "mysql" }

func (c *MySQLCheck) validate() error {
	if c.Dump == "" {
		return errors.New("needs a dump path")
	}
	if err := validateRelPath(c.Dump); err != nil {
		return err
	}
	return validateTables(c.Tables)
}

// Run implements Check.
func (c *MySQLCheck) Run(ctx context.Context, t *Target) (report.Status, string) {
	dumpPath, err := resolve(t.Roots, strings.TrimSuffix(c.Dump, "/"))
	if err != nil {
		return report.StatusFail, fmt.Sprintf("dump file %q %v", c.Dump, err)
	}
	runner, err := t.Docker(ctx)
	if err != nil {
		return report.StatusError, err.Error()
	}

	image := c.Image
	if image == "" {
		image = defaultMySQLImage
	}
	id, err := runner.StartContainer(ctx, sandbox.ContainerSpec{
		Image: image,
		Env: []string{
			"MYSQL_ROOT_PASSWORD=" + containerPassword,
			"MYSQL_DATABASE=" + mysqlDatabase,
		},
	})
	if err != nil {
		return report.StatusError, err.Error()
	}

	// Probe over TCP (-h127.0.0.1): during first-boot initialization the
	// image runs a temporary socket-only server that must not count as ready.
	mysqlArgs := []string{"-h127.0.0.1", "-uroot", "-p" + containerPassword}
	err = waitFor(ctx, runner, id, c.Timeout.orDefault(defaultMySQLReadyTimeout), func() (bool, string) {
		code, out, perr := runner.Exec(ctx, id,
			append([]string{"mysqladmin", "ping", "--silent"}, mysqlArgs...))
		if perr != nil {
			return false, perr.Error()
		}
		return code == 0, tail(out, 1)
	})
	if err != nil {
		return report.StatusError, fmt.Sprintf("mysql container (%s): %v", image, err)
	}

	dump, size, _, err := openDump(dumpPath, t.TempDir)
	if err != nil {
		return report.StatusFail, fmt.Sprintf("open dump %q: %v", c.Dump, err)
	}
	defer dump.Close() //nolint:errcheck // read-only
	if err := runner.CopyTo(ctx, id, "/tmp", "dump", dump, size); err != nil {
		return report.StatusError, err.Error()
	}

	load := fmt.Sprintf("mysql -h127.0.0.1 -uroot -p%s %s < /tmp/dump", containerPassword, mysqlDatabase)
	code, out, err := runner.Exec(ctx, id, []string{"sh", "-c", load})
	if err != nil {
		return report.StatusError, err.Error()
	}
	if code != 0 {
		return report.StatusFail, fmt.Sprintf("dump %q failed to load into mysql: %s", c.Dump, diagnostic(out, 3))
	}

	for _, tbl := range c.Tables {
		query := "SELECT COUNT(*) FROM " + quoteIdent(tbl.Name, true)
		code, out, err := runner.Exec(ctx, id,
			append([]string{"mysql", "-N", "-B", "-e", query, mysqlDatabase}, mysqlArgs...))
		if err != nil {
			return report.StatusError, err.Error()
		}
		if code != 0 {
			return report.StatusFail, fmt.Sprintf("count rows in %q: %s", tbl.Name, diagnostic(out, 2))
		}
		n, err := strconv.ParseInt(strings.TrimSpace(stripMySQLWarning(out)), 10, 64)
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

// stripMySQLWarning drops the "Using a password on the command line ..."
// warning the client prints on stderr, which Exec merges into the output.
func stripMySQLWarning(out string) string {
	var kept []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Using a password on the command line") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
