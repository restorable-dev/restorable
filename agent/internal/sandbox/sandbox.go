// Package sandbox manages disposable restore targets. Phase 1 provides temp
// directories; Docker sandboxes arrive in Phase 2. Creation runs a disk-space
// pre-flight, and Destroy is idempotent so callers can defer it
// unconditionally — the cleanup path runs even on failure or panic.
package sandbox

import (
	"fmt"
	"os"
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
	dir, err := os.MkdirTemp(baseDir, "restorable-")
	if err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	return &Sandbox{dir: dir}, nil
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
