// Package report defines run result types, local human/JSON output, and the
// scrubbing helpers that keep credentials out of anything the agent emits.
// The control-plane client arrives in Phase 3.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Status is the outcome of a run or an individual check.
type Status string

const (
	// StatusPass means the restore worked and every check passed.
	StatusPass Status = "pass"
	// StatusFail means a verification assertion did not hold.
	StatusFail Status = "fail"
	// StatusError means the test could not be carried out (infrastructure,
	// configuration, or repository problems).
	StatusError Status = "error"
)

// CheckResult is the outcome of one check within a recipe.
type CheckResult struct {
	Recipe     string `json:"recipe"`
	Type       string `json:"type"`
	Status     Status `json:"status"`
	Message    string `json:"message,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

// RunResult is the outcome of one full restore-verification run.
type RunResult struct {
	AgentVersion string        `json:"agent_version"`
	Repo         string        `json:"repo"`
	SnapshotID   string        `json:"snapshot_id,omitempty"`
	Status       Status        `json:"status"`
	Error        string        `json:"error,omitempty"`
	StartedAt    time.Time     `json:"started_at"`
	FinishedAt   time.Time     `json:"finished_at"`
	Checks       []CheckResult `json:"checks"`
}

// credRe matches the userinfo section of URLs (user:password@ or user@).
var credRe = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^@/\s]+@`)

// Scrub masks credentials embedded in URLs anywhere in s. Repo strings and
// error strings must pass through Scrub before entering a RunResult.
func Scrub(s string) string {
	return credRe.ReplaceAllString(s, "${1}***@")
}

// ExitCode maps the run status to a cron-friendly process exit code:
// 0 = pass, 1 = verification failed, 2 = could not test.
func (r *RunResult) ExitCode() int {
	switch r.Status {
	case StatusPass:
		return 0
	case StatusFail:
		return 1
	default:
		return 2
	}
}

// Duration returns the wall-clock duration of the run.
func (r *RunResult) Duration() time.Duration {
	return r.FinishedAt.Sub(r.StartedAt)
}

// WriteJSON writes the result as indented JSON.
func (r *RunResult) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// Human renders the result for terminal output.
func (r *RunResult) Human() string {
	lines := []string{
		fmt.Sprintf("restorable: %s", strings.ToUpper(string(r.Status))),
		fmt.Sprintf("  repo:      %s", r.Repo),
	}
	if r.SnapshotID != "" {
		id := r.SnapshotID
		if len(id) > 8 {
			id = id[:8]
		}
		lines = append(lines, fmt.Sprintf("  snapshot:  %s", id))
	}
	lines = append(lines, fmt.Sprintf("  duration:  %s", r.Duration().Round(time.Millisecond)))
	if r.Error != "" {
		lines = append(lines, fmt.Sprintf("  error:     %s", r.Error))
	}
	if len(r.Checks) > 0 {
		lines = append(lines, "  checks:")
		for _, c := range r.Checks {
			mark := "✓"
			if c.Status != StatusPass {
				mark = "✗"
			}
			line := fmt.Sprintf("    %s %s/%s (%dms)", mark, c.Recipe, c.Type, c.DurationMS)
			if c.Message != "" {
				line += ": " + c.Message
			}
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
