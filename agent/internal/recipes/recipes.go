// Package recipes parses declarative verification recipes and runs their
// checks against a restored snapshot. Phase 1 ships the files check type;
// postgres, mysql, sqlite, and docker-app arrive in Phase 2.
package recipes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/dabelle/restorable/agent/internal/report"
)

// Dumper streams one file's content out of a snapshot (implemented by the
// restic runner). Used to verify restored files against repository content.
type Dumper interface {
	Dump(ctx context.Context, snapshotID, path string, w io.Writer) error
}

// Root pairs a restored directory with the absolute path it had inside the
// snapshot. restic recreates the full original path under the sandbox, so a
// snapshot of /srv/app restored into /tmp/sb yields
// Root{Dir: "/tmp/sb/srv/app", SnapPath: "/srv/app"}.
type Root struct {
	Dir      string
	SnapPath string
}

// Target is what checks run against: the restored tree plus enough context
// to read original content back out of the repository and to spin up
// throwaway containers.
type Target struct {
	Roots      []Root
	SnapshotID string
	Dumper     Dumper
	// Docker lazily provides the container runner. It returns a
	// user-facing error when Docker is unavailable, so checks that need
	// containers degrade gracefully while everything else keeps working.
	Docker func(ctx context.Context) (ContainerRunner, error)
}

// Check is one verification step within a recipe.
type Check interface {
	// TypeName is the YAML `type` value (e.g. "files").
	TypeName() string
	// Run executes the check and reports status and message. The engine
	// fills in recipe name, type, and duration.
	Run(ctx context.Context, t *Target) (report.Status, string)
}

// Recipe is a named list of checks.
type Recipe struct {
	Name   string
	Checks []Check
}

// Load reads and validates a recipe YAML file.
func Load(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read recipe: %w", err)
	}
	r, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("recipe %s: %w", filepath.Base(path), err)
	}
	return r, nil
}

// Parse parses and validates recipe YAML.
func Parse(data []byte) (*Recipe, error) {
	var raw struct {
		Name   string      `yaml:"name"`
		Checks []yaml.Node `yaml:"checks"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("recipe is empty")
		}
		return nil, fmt.Errorf("parse recipe: %w", err)
	}
	if raw.Name == "" {
		return nil, errors.New("recipe needs a name")
	}
	if len(raw.Checks) == 0 {
		return nil, fmt.Errorf("recipe %q has no checks", raw.Name)
	}

	rec := &Recipe{Name: raw.Name}
	for i, node := range raw.Checks {
		check, err := parseCheck(node)
		if err != nil {
			return nil, fmt.Errorf("recipe %q check %d: %w", raw.Name, i+1, err)
		}
		rec.Checks = append(rec.Checks, check)
	}
	return rec, nil
}

func parseCheck(node yaml.Node) (Check, error) {
	var head struct {
		Type string `yaml:"type"`
	}
	if err := node.Decode(&head); err != nil {
		return nil, fmt.Errorf("parse check: %w", err)
	}
	var check interface {
		Check
		validate() error
	}
	switch head.Type {
	case "files":
		check = &FilesCheck{}
	case "postgres":
		check = &PostgresCheck{}
	case "mysql":
		check = &MySQLCheck{}
	case "sqlite":
		check = &SQLiteCheck{}
	case "docker-app":
		check = &DockerAppCheck{}
	case "":
		return nil, errors.New("check is missing a type")
	default:
		return nil, fmt.Errorf("unknown check type %q (supported: files, postgres, mysql, sqlite, docker-app)", head.Type)
	}
	if err := strictDecode(node, check); err != nil {
		return nil, fmt.Errorf("%s check: %w", head.Type, err)
	}
	if err := check.validate(); err != nil {
		return nil, fmt.Errorf("%s check: %w", head.Type, err)
	}
	return check, nil
}

// strictDecode re-decodes a YAML node with unknown fields rejected, so typos
// in recipe files fail loudly instead of being silently ignored.
func strictDecode(node yaml.Node, out any) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(&node); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	dec := yaml.NewDecoder(&buf)
	dec.KnownFields(true)
	return dec.Decode(out)
}

// Run executes every check in the recipe, always in order, and never stops
// early: a failing check should not hide the results of the ones after it.
func (r *Recipe) Run(ctx context.Context, t *Target) []report.CheckResult {
	results := make([]report.CheckResult, 0, len(r.Checks))
	for _, c := range r.Checks {
		start := time.Now()
		status, msg := c.Run(ctx, t)
		results = append(results, report.CheckResult{
			Recipe:     r.Name,
			Type:       c.TypeName(),
			Status:     status,
			Message:    msg,
			DurationMS: time.Since(start).Milliseconds(),
		})
	}
	return results
}

// joinIssues folds multiple failure messages into one, capped for sanity.
func joinIssues(issues []string) string {
	const maxShown = 5
	if len(issues) > maxShown {
		issues = append(issues[:maxShown], fmt.Sprintf("… and %d more", len(issues)-maxShown))
	}
	return strings.Join(issues, "; ")
}
