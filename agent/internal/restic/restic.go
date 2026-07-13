// Package restic wraps the restic binary behind a whitelist of read-only
// subcommands. The subcommand type is unexported and its only values are the
// constants below, so calling anything outside the whitelist (forget, prune,
// backup, ...) is impossible at compile time.
package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// subcommand is the closed set of restic subcommands the agent may run.
// All of them are read-only with respect to repository data.
type subcommand string

const (
	subVersion   subcommand = "version"
	subSnapshots subcommand = "snapshots"
	subLs        subcommand = "ls"
	subRestore   subcommand = "restore"
	subCheck     subcommand = "check"
	subDump      subcommand = "dump"
	subCat       subcommand = "cat" // read-only: `cat config` for the repo ID
)

// ErrNoSnapshots is returned when the repository contains no snapshots.
var ErrNoSnapshots = errors.New("repository contains no snapshots")

// Snapshot is the subset of restic's snapshot JSON the agent uses.
type Snapshot struct {
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Time     time.Time `json:"time"`
	Paths    []string  `json:"paths"`
	Hostname string    `json:"hostname"`
	Summary  *Summary  `json:"summary"`
}

// Summary carries snapshot statistics (present since restic 0.17).
type Summary struct {
	TotalFilesProcessed uint64 `json:"total_files_processed"`
	TotalBytesProcessed uint64 `json:"total_bytes_processed"`
}

// Runner executes whitelisted restic subcommands against one repository.
type Runner struct {
	bin         string
	repo        string
	passwordEnv string
}

// NewRunner locates the restic binary and validates that the password
// environment variable is set. The repository password itself is never
// stored; it is forwarded from the environment at exec time.
func NewRunner(repo, passwordEnv string) (*Runner, error) {
	bin, err := exec.LookPath("restic")
	if err != nil {
		return nil, fmt.Errorf("restic binary not found in PATH: %w", err)
	}
	if os.Getenv(passwordEnv) == "" {
		return nil, fmt.Errorf("password environment variable %s is not set (or empty)", passwordEnv)
	}
	return &Runner{bin: bin, repo: repo, passwordEnv: passwordEnv}, nil
}

// env builds the child process environment, mapping the configured password
// variable onto RESTIC_PASSWORD. Later duplicates win per os/exec semantics.
func (r *Runner) env() []string {
	return append(os.Environ(), "RESTIC_PASSWORD="+os.Getenv(r.passwordEnv))
}

// run executes a whitelisted subcommand, streaming stdout to out.
func (r *Runner) run(ctx context.Context, sub subcommand, out io.Writer, args ...string) error {
	full := append([]string{string(sub), "--repo", r.repo}, args...)
	cmd := exec.CommandContext(ctx, r.bin, full...)
	var stderr bytes.Buffer
	cmd.Stdout = out
	cmd.Stderr = &stderr
	cmd.Env = r.env()
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return fmt.Errorf("restic %s: %w", sub, err)
		}
		return fmt.Errorf("restic %s: %w: %s", sub, err, msg)
	}
	return nil
}

// Version returns the restic version string.
func (r *Runner) Version(ctx context.Context) (string, error) {
	var out bytes.Buffer
	if err := r.run(ctx, subVersion, &out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// RepoID returns restic's canonical repository ID (from `cat config`). This
// uniquely identifies the repository regardless of how its location is
// spelled (relative vs absolute path, trailing slash, http vs https), so it
// is a far better fingerprint than the repo string. Empty (no error) if the
// restic version doesn't expose an id.
func (r *Runner) RepoID(ctx context.Context) (string, error) {
	var out bytes.Buffer
	if err := r.run(ctx, subCat, &out, "config"); err != nil {
		return "", err
	}
	var cfg struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &cfg); err != nil {
		return "", fmt.Errorf("parse restic config: %w", err)
	}
	return cfg.ID, nil
}

// LatestSnapshot returns the most recent snapshot in the repository.
func (r *Runner) LatestSnapshot(ctx context.Context) (*Snapshot, error) {
	var out bytes.Buffer
	if err := r.run(ctx, subSnapshots, &out, "latest", "--json"); err != nil {
		return nil, err
	}
	var snaps []Snapshot
	if err := json.Unmarshal(out.Bytes(), &snaps); err != nil {
		return nil, fmt.Errorf("parse restic snapshots output: %w", err)
	}
	if len(snaps) == 0 {
		return nil, ErrNoSnapshots
	}
	return &snaps[len(snaps)-1], nil
}

// Restore restores the given snapshot into target.
func (r *Runner) Restore(ctx context.Context, snapshotID, target string) error {
	return r.run(ctx, subRestore, io.Discard, snapshotID, "--target", target)
}

// Check verifies repository integrity (restic check).
func (r *Runner) Check(ctx context.Context) error {
	return r.run(ctx, subCheck, io.Discard)
}

// Dump streams the contents of one file from the snapshot to w. path is the
// absolute path of the file as recorded inside the snapshot.
func (r *Runner) Dump(ctx context.Context, snapshotID, path string, w io.Writer) error {
	return r.run(ctx, subDump, w, snapshotID, path)
}

// lsNode is one NDJSON line of `restic ls --json` (message_type "node").
type lsNode struct {
	MessageType string `json:"message_type"`
	Type        string `json:"type"`
	Path        string `json:"path"`
}

// Ls lists the paths of all files (not dirs) in the snapshot.
func (r *Runner) Ls(ctx context.Context, snapshotID string) ([]string, error) {
	var out bytes.Buffer
	if err := r.run(ctx, subLs, &out, snapshotID, "--json"); err != nil {
		return nil, err
	}
	var paths []string
	dec := json.NewDecoder(&out)
	for {
		var node lsNode
		if err := dec.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("parse restic ls output: %w", err)
		}
		if node.MessageType == "node" && node.Type == "file" {
			paths = append(paths, node.Path)
		}
	}
	return paths, nil
}
