package restic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRestic writes a shell script that records its argv and prints canned
// stdout/stderr, and returns a Runner wired to it.
func fakeRestic(t *testing.T, stdout, stderr string, exitCode int) (*Runner, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "restic")
	argsFile := filepath.Join(dir, "args")
	stdoutFile := filepath.Join(dir, "stdout")
	stderrFile := filepath.Join(dir, "stderr")
	if err := os.WriteFile(stdoutFile, []byte(stdout), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stderrFile, []byte(stderr), 0o600); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %q
cat %q
cat %q >&2
exit %d
`, argsFile, stdoutFile, stderrFile, exitCode)
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return &Runner{bin: bin, repo: "/repo", passwordEnv: "RESTIC_PASSWORD"}, argsFile
}

func recordedArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("fake restic was not invoked: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// snapshotsJSON is real output from restic 0.18.0 (trimmed).
const snapshotsJSON = `[{"time":"2026-07-03T22:32:16.296943-04:00","paths":["/srv/app"],"hostname":"box","summary":{"total_files_processed":2,"total_bytes_processed":8},"id":"dda3522e5a853671a9e2ea2d175ad18cfcf4e988fa56b84ee3e0af92819b64c0","short_id":"dda3522e"}]`

func TestRunnerArgConstruction(t *testing.T) {
	tests := []struct {
		name     string
		invoke   func(r *Runner) error
		wantArgs []string
	}{
		{
			name:     "restore",
			invoke:   func(r *Runner) error { return r.Restore(context.Background(), "abc123", "/target") },
			wantArgs: []string{"restore", "--repo", "/repo", "abc123", "--target", "/target"},
		},
		{
			name:     "check",
			invoke:   func(r *Runner) error { return r.Check(context.Background()) },
			wantArgs: []string{"check", "--repo", "/repo"},
		},
		{
			name:     "dump",
			invoke:   func(r *Runner) error { return r.Dump(context.Background(), "abc", "/srv/f.txt", new(bytes.Buffer)) },
			wantArgs: []string{"dump", "--repo", "/repo", "abc", "/srv/f.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, argsFile := fakeRestic(t, "", "", 0)
			if err := tt.invoke(r); err != nil {
				t.Fatalf("invoke error: %v", err)
			}
			got := recordedArgs(t, argsFile)
			if strings.Join(got, " ") != strings.Join(tt.wantArgs, " ") {
				t.Errorf("args = %v, want %v", got, tt.wantArgs)
			}
		})
	}
}

func TestRepoID(t *testing.T) {
	r, argsFile := fakeRestic(t, `{"version":2,"id":"f4fbe368ebda1cb0deadbeef","chunker_polynomial":"3d"}`, "", 0)
	id, err := r.RepoID(context.Background())
	if err != nil {
		t.Fatalf("RepoID() error: %v", err)
	}
	if id != "f4fbe368ebda1cb0deadbeef" {
		t.Errorf("RepoID() = %q", id)
	}
	args := recordedArgs(t, argsFile)
	want := []string{"cat", "--repo", "/repo", "config"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", args, want)
	}
}

func TestLatestSnapshot(t *testing.T) {
	t.Run("parses snapshot", func(t *testing.T) {
		r, argsFile := fakeRestic(t, snapshotsJSON, "", 0)
		snap, err := r.LatestSnapshot(context.Background())
		if err != nil {
			t.Fatalf("LatestSnapshot() error: %v", err)
		}
		if snap.ShortID != "dda3522e" {
			t.Errorf("ShortID = %q", snap.ShortID)
		}
		if snap.Summary == nil || snap.Summary.TotalBytesProcessed != 8 {
			t.Errorf("Summary = %+v, want total_bytes_processed 8", snap.Summary)
		}
		if len(snap.Paths) != 1 || snap.Paths[0] != "/srv/app" {
			t.Errorf("Paths = %v", snap.Paths)
		}
		args := recordedArgs(t, argsFile)
		want := []string{"snapshots", "--repo", "/repo", "latest", "--json"}
		if strings.Join(args, " ") != strings.Join(want, " ") {
			t.Errorf("args = %v, want %v", args, want)
		}
	})

	t.Run("empty repository", func(t *testing.T) {
		r, _ := fakeRestic(t, "[]", "", 0)
		_, err := r.LatestSnapshot(context.Background())
		if !errors.Is(err, ErrNoSnapshots) {
			t.Fatalf("error = %v, want ErrNoSnapshots", err)
		}
	})

	t.Run("garbage output", func(t *testing.T) {
		r, _ := fakeRestic(t, "not json", "", 0)
		if _, err := r.LatestSnapshot(context.Background()); err == nil {
			t.Fatal("expected parse error")
		}
	})
}

func TestRunErrorIncludesStderr(t *testing.T) {
	r, _ := fakeRestic(t, "", "Fatal: wrong password", 1)
	err := r.Check(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "wrong password") {
		t.Errorf("error %q does not include stderr", err)
	}
	if !strings.Contains(err.Error(), "restic check") {
		t.Errorf("error %q does not name the subcommand", err)
	}
}

func TestDumpStreamsToWriter(t *testing.T) {
	r, _ := fakeRestic(t, "file-contents", "", 0)
	var out bytes.Buffer
	if err := r.Dump(context.Background(), "id", "/f", &out); err != nil {
		t.Fatalf("Dump() error: %v", err)
	}
	if out.String() != "file-contents" {
		t.Errorf("Dump output = %q", out.String())
	}
}

func TestLs(t *testing.T) {
	ndjson := `{"message_type":"snapshot","id":"abc"}
{"message_type":"node","type":"dir","path":"/srv"}
{"message_type":"node","type":"file","path":"/srv/a.txt"}
{"message_type":"node","type":"file","path":"/srv/b.txt"}
`
	r, _ := fakeRestic(t, ndjson, "", 0)
	paths, err := r.Ls(context.Background(), "abc")
	if err != nil {
		t.Fatalf("Ls() error: %v", err)
	}
	want := []string{"/srv/a.txt", "/srv/b.txt"}
	if len(paths) != len(want) || paths[0] != want[0] || paths[1] != want[1] {
		t.Errorf("Ls() = %v, want %v", paths, want)
	}
}

func TestNewRunner(t *testing.T) {
	// Put a fake restic on PATH so LookPath succeeds.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "restic"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	t.Run("password env set", func(t *testing.T) {
		t.Setenv("MY_PASS", "secret")
		r, err := NewRunner("/repo", "MY_PASS")
		if err != nil {
			t.Fatalf("NewRunner() error: %v", err)
		}
		env := r.env()
		last := env[len(env)-1]
		if last != "RESTIC_PASSWORD=secret" {
			t.Errorf("env password mapping = %q", last)
		}
	})

	t.Run("password env missing", func(t *testing.T) {
		_, err := NewRunner("/repo", "UNSET_VAR_FOR_TEST")
		if err == nil || !strings.Contains(err.Error(), "UNSET_VAR_FOR_TEST") {
			t.Fatalf("error = %v, want mention of missing env var", err)
		}
	})

	t.Run("restic not in PATH", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := NewRunner("/repo", "MY_PASS")
		if err == nil || !strings.Contains(err.Error(), "not found in PATH") {
			t.Fatalf("error = %v, want restic-not-found", err)
		}
	})
}
