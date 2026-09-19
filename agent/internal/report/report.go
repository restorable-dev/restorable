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
	AgentVersion string `json:"agent_version"`
	Repo         string `json:"repo"`
	// RepoFingerprint, when set, is the stable per-repository fingerprint
	// (from restic's repo ID). Empty falls back to hashing the repo string.
	RepoFingerprint string    `json:"-"`
	SnapshotID      string    `json:"snapshot_id,omitempty"`
	Status          Status    `json:"status"`
	Error           string    `json:"error,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	// RestoreDurationMS is how long the restic restore itself took, separate
	// from verification. Zero when the run failed before the restore finished.
	RestoreDurationMS int64         `json:"restore_duration_ms,omitempty"`
	Checks            []CheckResult `json:"checks"`
}

// credRe matches the userinfo section of an authority: everything up to the
// LAST '@' before the authority ends (start of path, whitespace, or string
// end). Greedy up to '@' so passwords containing '/' or '@' (which restic
// permits in rest:/s3: URLs) are fully masked, not partially.
var credRe = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^\s]+@`)

// Scrub masks credentials embedded in URLs anywhere in s. Repo strings and
// error strings must pass through Scrub before entering a RunResult. The
// regex is whitespace-bounded, so it can only ever over-mask within a token
// (safe), never leak.
func Scrub(s string) string {
	return credRe.ReplaceAllString(s, "${1}***@")
}

// dbDetailRe matches the start of a message segment that echoes row values out
// of the user's restored database: Postgres and MySQL put real column data in
// DETAIL, HINT and CONTEXT, and psql's "LINE n:" echoes the offending
// statement text.
//
// This is deliberately not anchored with (?m)^...$. The recipe engine joins
// command output with " / " before the message is ever built, so a
// line-anchored pattern matches nothing on a real message and the filter
// silently does nothing. Segment the message first, then match per segment.
var dbDetailRe = regexp.MustCompile(`(?i)^\s*(DETAIL|HINT|CONTEXT|LINE\s+\d+|Key\s*\()`)

// segmentRe splits a check message back into the pieces command output was
// joined from, whichever joiner produced it.
var segmentRe = regexp.MustCompile(`\n| / `)

// mysqlRowValueRe matches the places MySQL inlines a value from the user's
// data into an error.
//
// Postgres isolates row data in DETAIL and CONTEXT, so dropping whole segments
// removes it. MySQL does not: it puts the offending value on the same ERROR
// line as the reason ("Duplicate entry 'alice@example.com' for key
// 'users.email'"), so segment filtering alone let it through. Dropping the
// whole line instead would take the reason with it, which is the thing the
// message exists to convey.
//
// Only the quoted value is replaced. The identifier after "for key" or "for
// column" is schema rather than data, and naming the constraint that failed is
// most of the diagnostic worth.
var mysqlRowValueRe = regexp.MustCompile(`(?i)\b(Duplicate entry|value:)\s+'[^']*'`)

// RedactForTransport prepares a check message to leave the machine. Check
// messages are the one field that can embed raw command output from the
// user's restored databases, so anything sent to the control plane is stripped
// of DB detail segments, scrubbed of URL credentials, and length-bounded.
// Local stdout keeps the full text.
func RedactForTransport(msg string) string {
	segs := segmentRe.Split(msg, -1)
	for i, seg := range segs {
		if dbDetailRe.MatchString(seg) {
			segs[i] = "[redacted]"
		}
	}
	msg = mysqlRowValueRe.ReplaceAllString(strings.Join(segs, " / "), "$1 '[redacted]'")
	msg = Scrub(msg)
	const max = 500
	if len(msg) > max {
		msg = msg[:max] + "…"
	}
	return msg
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
	if r.RestoreDurationMS > 0 {
		restore := time.Duration(r.RestoreDurationMS) * time.Millisecond
		lines = append(lines, fmt.Sprintf("  restore:   %s", restore.Round(time.Millisecond)))
	}
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
