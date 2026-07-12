package recipes

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/restorable-dev/restorable/agent/internal/sandbox"
)

// ContainerRunner is the slice of the Docker sandbox the database and app
// checks need. Tests substitute a fake; production wires *sandbox.Docker.
type ContainerRunner interface {
	StartContainer(ctx context.Context, spec sandbox.ContainerSpec) (string, error)
	Exec(ctx context.Context, id string, cmd []string) (int, string, error)
	CopyTo(ctx context.Context, id, dstDir, name string, content io.Reader, size int64) error
	MappedPort(ctx context.Context, id, containerPort string) (string, error)
	State(ctx context.Context, id string) (running bool, exitCode int, err error)
}

// Duration is a time.Duration that unmarshals from YAML strings like "120s".
type Duration struct {
	time.Duration
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("duration must be a string like \"120s\": %w", err)
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q (examples: 90s, 2m): %w", s, err)
	}
	if parsed <= 0 {
		return fmt.Errorf("duration %q must be positive", s)
	}
	d.Duration = parsed
	return nil
}

// orDefault returns the configured duration, or def when unset.
func (d Duration) orDefault(def time.Duration) time.Duration {
	if d.Duration == 0 {
		return def
	}
	return d.Duration
}

// pollInterval is how often readiness probes rerun. A variable so tests can
// shrink it.
var pollInterval = 2 * time.Second

// waitFor polls probe every pollInterval until it returns true, the
// container dies, or the timeout elapses. probe's second return is a
// progress message kept for the timeout error.
func waitFor(ctx context.Context, runner ContainerRunner, id string, timeout time.Duration,
	probe func() (bool, string)) error {
	deadline := time.Now().Add(timeout)
	last := "no probe result yet"
	for {
		if running, exitCode, err := runner.State(ctx, id); err == nil && !running {
			return fmt.Errorf("container exited early with code %d (last probe: %s)", exitCode, last)
		}
		var ok bool
		ok, last = probe()
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("not ready after %s (last probe: %s)", timeout, last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// openDump opens a dump file, transparently decompressing gzip. It returns a
// reader, the uncompressed size (streaming gzip to measure when needed), and
// whether the content is a pg_dump custom-format archive ("PGDMP" magic).
//
// Size is required because docker's copy-in uses tar, which needs lengths up
// front. For gzip we decompress to a temp file first; dumps in scope for a
// homelab fit on the disk that already holds the whole restored snapshot.
func openDump(path string) (r io.ReadCloser, size int64, custom bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, false, err
	}

	head := make([]byte, 5)
	n, err := io.ReadFull(f, head)
	if err != nil && n == 0 {
		_ = f.Close()
		return nil, 0, false, fmt.Errorf("dump file %s is empty", path)
	}
	head = head[:n]
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, 0, false, err
	}

	// gzip magic 1f 8b → decompress to a temp file to learn the size.
	if len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b {
		defer f.Close() //nolint:errcheck // read-only file
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, 0, false, fmt.Errorf("open gzip dump: %w", err)
		}
		tmp, err := os.CreateTemp("", "restorable-dump-")
		if err != nil {
			return nil, 0, false, err
		}
		// Unlink immediately: the fd keeps it alive, nothing can leak.
		_ = os.Remove(tmp.Name())
		size, err = io.Copy(tmp, gz)
		if err != nil {
			_ = tmp.Close()
			return nil, 0, false, fmt.Errorf("decompress dump: %w", err)
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			_ = tmp.Close()
			return nil, 0, false, err
		}
		magic := make([]byte, 5)
		m, _ := io.ReadFull(tmp, magic)
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			_ = tmp.Close()
			return nil, 0, false, err
		}
		return tmp, size, string(magic[:m]) == "PGDMP", nil
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, false, err
	}
	return f, info.Size(), string(head) == "PGDMP", nil
}

// tail returns the last n lines of command output for error messages.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = append([]string{"…"}, lines[len(lines)-n:]...)
	}
	return strings.Join(lines, " / ")
}
