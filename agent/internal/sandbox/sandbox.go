// Package sandbox manages disposable restore targets. Phase 1 provides temp
// directories; Docker sandboxes arrive in Phase 2. Creation runs a disk-space
// pre-flight, and Destroy is idempotent so callers can defer it
// unconditionally — the cleanup path runs even on failure or panic.
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Sandbox is a disposable directory a snapshot is restored into.
type Sandbox struct {
	dir string
}

// New creates a sandbox directory under baseDir (the OS temp dir when empty)
// after verifying at least requiredBytes of free disk space. It aborts with a
// clear error before any restore work if space is insufficient.
func New(baseDir string, requiredBytes uint64) (*Sandbox, error) {
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("create sandbox base dir: %w", err)
	}
	free, err := freeBytes(baseDir)
	if err != nil {
		return nil, fmt.Errorf("check free disk space in %s: %w", baseDir, err)
	}
	if free < requiredBytes {
		return nil, fmt.Errorf(
			"insufficient disk space in %s: %s free, %s required — aborting before restore",
			baseDir, humanBytes(free), humanBytes(requiredBytes))
	}
	reapOrphans(baseDir)
	dir, err := os.MkdirTemp(baseDir, "restorable-")
	if err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	return &Sandbox{dir: dir}, nil
}

// orphanAge is how old a sandbox must be before it is assumed abandoned. A
// restore is bounded by disk and network, never by days, so this is far above
// any legitimate run while still well inside "the user would rather not keep
// their restored data lying around".
const orphanAge = 24 * time.Hour

// reapOrphans removes sandboxes left behind by runs that died outright.
//
// Destroy is deferred and survives panics, failures and signals, but nothing
// survives SIGKILL, an OOM kill, or the power going out. Those leave a full
// copy of the user's restored data on disk with no owner, which is both a disk
// leak and a privacy problem, and no later run ever reclaimed it.
//
// Deliberately conservative, because this deletes directories: only entries
// directly under the configured base dir, only ones matching the prefix this
// package creates, and only after orphanAge, so a concurrent agent's live
// sandbox is never a candidate. Failures are ignored; reaping is housekeeping
// and must never be the reason a verification run cannot start.
func reapOrphans(baseDir string) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-orphanAge)
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "restorable-") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(baseDir, e.Name()))
	}
}

// Dir returns the sandbox directory path.
func (s *Sandbox) Dir() string { return s.dir }

// Destroy removes the sandbox and everything in it. It is idempotent and
// safe to call on a nil sandbox, so it can be deferred immediately after New.
func (s *Sandbox) Destroy() error {
	if s == nil || s.dir == "" {
		return nil
	}
	if err := os.RemoveAll(s.dir); err != nil {
		return fmt.Errorf("destroy sandbox %s: %w", s.dir, err)
	}
	return nil
}

// humanBytes renders a byte count for error messages (binary units).
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
