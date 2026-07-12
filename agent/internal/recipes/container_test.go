package recipes

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/restorable-dev/restorable/agent/internal/report"
	"github.com/restorable-dev/restorable/agent/internal/sandbox"
)

func init() {
	// Readiness probes poll fast in tests.
	pollInterval = 5 * time.Millisecond
}

// fakeRunner scripts container behavior for check tests.
type fakeRunner struct {
	specs   []sandbox.ContainerSpec
	execs   [][]string // every Exec cmd, in order
	copied  bytes.Buffer
	port    string
	stopped bool // State reports not-running
	exec    func(cmd []string) (int, string, error)
}

func (f *fakeRunner) StartContainer(_ context.Context, spec sandbox.ContainerSpec) (string, error) {
	f.specs = append(f.specs, spec)
	return fmt.Sprintf("container-%d", len(f.specs)), nil
}

func (f *fakeRunner) Exec(_ context.Context, _ string, cmd []string) (int, string, error) {
	f.execs = append(f.execs, cmd)
	return f.exec(cmd)
}

func (f *fakeRunner) CopyTo(_ context.Context, _, _, _ string, content io.Reader, _ int64) error {
	_, err := io.Copy(&f.copied, content)
	return err
}

func (f *fakeRunner) MappedPort(_ context.Context, _, _ string) (string, error) {
	return f.port, nil
}

func (f *fakeRunner) State(_ context.Context, _ string) (bool, int, error) {
	return !f.stopped, 1, nil
}

// dbTarget builds a restored tree containing files and wires the fake runner.
func dbTarget(t *testing.T, files map[string][]byte, runner *fakeRunner) *Target {
	t.Helper()
	rootDir := filepath.Join(t.TempDir(), "srv", "app")
	for rel, content := range files {
		abs := filepath.Join(rootDir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &Target{
		Roots:      []Root{{Dir: rootDir, SnapPath: "/srv/app"}},
		SnapshotID: "snap1",
		Docker: func(context.Context) (ContainerRunner, error) {
			return runner, nil
		},
	}
}

// pgExec scripts a healthy postgres: ready probes pass, loads succeed, and
// row counts answer with the given value.
func pgExec(count string, loadCode int, loadOut string) func(cmd []string) (int, string, error) {
	return func(cmd []string) (int, string, error) {
		switch {
		case cmd[0] == "pg_isready":
			return 0, "accepting connections", nil
		case cmd[0] == "psql" && contains(cmd, "-f"):
			return loadCode, loadOut, nil
		case cmd[0] == "pg_restore":
			return loadCode, loadOut, nil
		case cmd[0] == "psql": // count query
			return 0, count + "\n", nil
		}
		return 127, "unexpected command", nil
	}
}

func contains(cmd []string, s string) bool {
	for _, c := range cmd {
		if c == s {
			return true
		}
	}
	return false
}

func TestPostgresCheck(t *testing.T) {
	dump := []byte("CREATE TABLE users (id int);\nCOPY users FROM stdin;\n1\n\\.\n")

	tests := []struct {
		name       string
		files      map[string][]byte
		check      PostgresCheck
		exec       func(cmd []string) (int, string, error)
		wantStatus report.Status
		wantMsg    string
		wantExec   string // substring that must appear in some exec'd command
	}{
		{
			name:       "dump loads and row assertion holds",
			files:      map[string][]byte{"db/dump.sql": dump},
			check:      PostgresCheck{Dump: "db/dump.sql", Tables: []TableAssertion{{Name: "users", MinRows: 1}}},
			exec:       pgExec("3", 0, ""),
			wantStatus: report.StatusPass,
			wantMsg:    "1 table assertion(s) hold",
		},
		{
			name:       "truncated dump fails with clear message",
			files:      map[string][]byte{"db/dump.sql": dump[:20]},
			check:      PostgresCheck{Dump: "db/dump.sql"},
			exec:       pgExec("0", 3, "ERROR: unexpected end of file\ninvalid command \\."),
			wantStatus: report.StatusFail,
			wantMsg:    "failed to load into postgres",
		},
		{
			name:       "row assertion below minimum fails",
			files:      map[string][]byte{"db/dump.sql": dump},
			check:      PostgresCheck{Dump: "db/dump.sql", Tables: []TableAssertion{{Name: "users", MinRows: 100}}},
			exec:       pgExec("3", 0, ""),
			wantStatus: report.StatusFail,
			wantMsg:    "has 3 rows, need at least 100",
		},
		{
			name:       "missing dump file fails",
			files:      map[string][]byte{},
			check:      PostgresCheck{Dump: "db/dump.sql"},
			exec:       pgExec("0", 0, ""),
			wantStatus: report.StatusFail,
			wantMsg:    "not found in restored snapshot",
		},
		{
			name:       "custom format dump uses pg_restore",
			files:      map[string][]byte{"db/dump.pgc": append([]byte("PGDMP"), dump...)},
			check:      PostgresCheck{Dump: "db/dump.pgc"},
			exec:       pgExec("0", 0, ""),
			wantStatus: report.StatusPass,
			wantExec:   "pg_restore",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{exec: tt.exec}
			target := dbTarget(t, tt.files, runner)
			status, msg := tt.check.Run(context.Background(), target)
			if status != tt.wantStatus {
				t.Fatalf("status = %s (msg %q), want %s", status, msg, tt.wantStatus)
			}
			if tt.wantMsg != "" && !strings.Contains(msg, tt.wantMsg) {
				t.Errorf("message %q does not contain %q", msg, tt.wantMsg)
			}
			if tt.wantExec != "" {
				found := false
				for _, cmd := range runner.execs {
					if cmd[0] == tt.wantExec {
						found = true
					}
				}
				if !found {
					t.Errorf("no exec'd command started with %q: %v", tt.wantExec, runner.execs)
				}
			}
		})
	}
}

func TestPostgresGzipDumpDecompressed(t *testing.T) {
	plain := []byte("CREATE TABLE t (id int);")
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{exec: pgExec("0", 0, "")}
	target := dbTarget(t, map[string][]byte{"db/dump.sql.gz": gz.Bytes()}, runner)
	check := PostgresCheck{Dump: "db/dump.sql.gz"}
	status, msg := check.Run(context.Background(), target)
	if status != report.StatusPass {
		t.Fatalf("status = %s (%s)", status, msg)
	}
	if runner.copied.String() != string(plain) {
		t.Errorf("copied content = %q, want decompressed %q", runner.copied.String(), plain)
	}
}

func TestPostgresDockerUnavailable(t *testing.T) {
	target := dbTarget(t, map[string][]byte{"d.sql": []byte("x")}, nil)
	target.Docker = func(context.Context) (ContainerRunner, error) {
		return nil, errors.New("docker is not available (daemon not running)")
	}
	check := PostgresCheck{Dump: "d.sql"}
	status, msg := check.Run(context.Background(), target)
	if status != report.StatusError {
		t.Fatalf("status = %s, want error", status)
	}
	if !strings.Contains(msg, "docker is not available") {
		t.Errorf("message %q should explain docker absence", msg)
	}
}

func TestPostgresDefaultImage(t *testing.T) {
	runner := &fakeRunner{exec: pgExec("0", 0, "")}
	target := dbTarget(t, map[string][]byte{"d.sql": []byte("SELECT 1;")}, runner)
	check := PostgresCheck{Dump: "d.sql"}
	if status, msg := check.Run(context.Background(), target); status != report.StatusPass {
		t.Fatalf("status = %s (%s)", status, msg)
	}
	if runner.specs[0].Image != defaultPostgresImage {
		t.Errorf("image = %q, want default %q", runner.specs[0].Image, defaultPostgresImage)
	}
}

func TestMySQLCheck(t *testing.T) {
	mysqlExec := func(count string, loadCode int, loadOut string) func(cmd []string) (int, string, error) {
		return func(cmd []string) (int, string, error) {
			switch cmd[0] {
			case "mysqladmin":
				return 0, "mysqld is alive", nil
			case "sh": // load via shell redirect
				return loadCode, loadOut, nil
			case "mysql":
				return 0, "mysql: [Warning] Using a password on the command line interface can be insecure.\n" + count + "\n", nil
			}
			return 127, "unexpected", nil
		}
	}

	t.Run("dump loads and count parses through warning noise", func(t *testing.T) {
		runner := &fakeRunner{exec: mysqlExec("42", 0, "")}
		target := dbTarget(t, map[string][]byte{"db.sql": []byte("CREATE TABLE t (id int);")}, runner)
		check := MySQLCheck{Dump: "db.sql", Tables: []TableAssertion{{Name: "t", MinRows: 10}}}
		status, msg := check.Run(context.Background(), target)
		if status != report.StatusPass {
			t.Fatalf("status = %s (%s)", status, msg)
		}
		if runner.specs[0].Image != defaultMySQLImage {
			t.Errorf("image = %q, want %q", runner.specs[0].Image, defaultMySQLImage)
		}
	})

	t.Run("broken dump fails clearly", func(t *testing.T) {
		runner := &fakeRunner{exec: mysqlExec("0", 1, "ERROR 1064 (42000) at line 3: You have an error in your SQL syntax")}
		target := dbTarget(t, map[string][]byte{"db.sql": []byte("garbage")}, runner)
		check := MySQLCheck{Dump: "db.sql"}
		status, msg := check.Run(context.Background(), target)
		if status != report.StatusFail {
			t.Fatalf("status = %s (%s)", status, msg)
		}
		if !strings.Contains(msg, "failed to load into mysql") || !strings.Contains(msg, "1064") {
			t.Errorf("message %q lacks clear load error", msg)
		}
	})
}

func TestWaitForContainerExit(t *testing.T) {
	runner := &fakeRunner{stopped: true}
	err := waitFor(context.Background(), runner, "id", time.Second, func() (bool, string) {
		return false, "probing"
	})
	if err == nil || !strings.Contains(err.Error(), "exited early") {
		t.Fatalf("err = %v, want exited-early", err)
	}
}

func TestWaitForTimeout(t *testing.T) {
	runner := &fakeRunner{}
	start := time.Now()
	err := waitFor(context.Background(), runner, "id", 30*time.Millisecond, func() (bool, string) {
		return false, "still starting"
	})
	if err == nil || !strings.Contains(err.Error(), "still starting") {
		t.Fatalf("err = %v, want timeout with last probe message", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("waitFor did not respect timeout")
	}
}
