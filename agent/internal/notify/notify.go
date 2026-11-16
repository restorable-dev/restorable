// Package notify tells you how a restore test went: an ntfy push, a JSON
// webhook, and a Healthchecks-style ping that turns a dead agent into an
// alert of its own.
//
// Delivery is best effort. A failed notification is logged and never changes
// the run's outcome or the process exit code.
//
// Everything sent here leaves the machine, so every message goes through
// report.RedactForTransport first: a check message can quote rows from the
// restored database, and a repo string can carry credentials.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/restorable-dev/restorable/agent/internal/config"
	"github.com/restorable-dev/restorable/agent/internal/report"
)

const (
	// requestTimeout bounds each delivery, so a slow endpoint can delay the
	// end of a run by seconds, never hang it.
	requestTimeout = 15 * time.Second
	// maxNtfyBody keeps a message under ntfy's 4096-byte limit with room to
	// spare for multi-byte characters.
	maxNtfyBody = 3500
	// maxChecksListed caps the per-check lines in a text summary.
	maxChecksListed = 10
)

// Notifier delivers run outcomes to the endpoints in the agent config.
type Notifier struct {
	cfg    config.NotifyConfig
	client *http.Client
	logf   func(format string, args ...any)
}

// New returns a Notifier for cfg. Delivery failures are reported through
// logf, without the endpoint URL.
func New(cfg config.NotifyConfig, logf func(format string, args ...any)) *Notifier {
	return &Notifier{
		cfg:    cfg,
		client: &http.Client{Timeout: requestTimeout},
		logf:   logf,
	}
}

// Describe summarizes what is configured, for a startup log line. It names
// endpoint kinds only, never URLs.
func (n *Notifier) Describe() string {
	var parts []string
	var messages []string
	if n.cfg.Ntfy != "" {
		messages = append(messages, "ntfy")
	}
	if n.cfg.Webhook != "" {
		messages = append(messages, "webhook")
	}
	if len(messages) > 0 {
		when := "on failure"
		if n.cfg.When == config.NotifyAlways {
			when = "after every run"
		}
		parts = append(parts, strings.Join(messages, " and ")+" "+when)
	}
	if n.cfg.Healthcheck != "" {
		parts = append(parts, "healthcheck ping after every run")
	}
	if len(parts) == 0 {
		return "none configured"
	}
	return strings.Join(parts, "; ")
}

// Start tells the healthcheck a run has begun, so the monitor can time it and
// flag one that never finishes. It does nothing without a healthcheck URL.
func (n *Notifier) Start(ctx context.Context) {
	if n.cfg.Healthcheck == "" {
		return
	}
	n.post(ctx, "healthcheck start ping", withPath(n.cfg.Healthcheck, "start"), "", nil, nil)
}

// Finish reports a completed run: always to the healthcheck, and to ntfy and
// the webhook when the run failed or notify.when is "always".
func (n *Notifier) Finish(ctx context.Context, res *report.RunResult) {
	text := Summary(res)
	passed := res.Status == report.StatusPass

	if n.cfg.Healthcheck != "" {
		target := n.cfg.Healthcheck
		if !passed {
			target = withPath(target, "fail")
		}
		n.post(ctx, "healthcheck ping", target, "text/plain; charset=utf-8", []byte(text), nil)
	}

	if passed && n.cfg.When != config.NotifyAlways {
		return
	}
	if n.cfg.Ntfy != "" {
		n.post(ctx, "ntfy notification", n.cfg.Ntfy, "text/plain; charset=utf-8",
			[]byte(truncateForNtfy(text)), ntfyHeader(res.Status))
	}
	if n.cfg.Webhook != "" {
		body, err := json.Marshal(newPayload(res, text))
		if err != nil {
			n.logf("webhook notification: encode payload: %v", err)
			return
		}
		n.post(ctx, "webhook notification", n.cfg.Webhook, "application/json", body, nil)
	}
}

// post sends one request and logs, rather than returns, any failure.
func (n *Notifier) post(ctx context.Context, what, target, contentType string, body []byte, header http.Header) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		n.logf("%s failed: %v", what, withoutURL(err))
		return
	}
	maps.Copy(req.Header, header)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", "restorable-agent")

	resp, err := n.client.Do(req)
	if err != nil {
		n.logf("%s failed: %v", what, withoutURL(err))
		return
	}
	defer resp.Body.Close() //nolint:errcheck // body drained below, nothing to report
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		n.logf("%s failed: HTTP %d", what, resp.StatusCode)
	}
}

// withoutURL drops the request URL from an HTTP client error. Ping URLs, ntfy
// topics and webhook paths are bearer secrets, so they stay out of logs.
func withoutURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// withPath appends one path segment to a ping URL and keeps its query string
// (Healthchecks uses ?create=1 for checks it provisions on first ping).
func withPath(raw, segment string) string {
	u, err := url.Parse(raw)
	if err != nil {
		// Unreachable: config validation rejects unparseable URLs at load.
		return raw
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + segment
	u.RawPath = ""
	return u.String()
}

func ntfyHeader(status report.Status) http.Header {
	h := http.Header{}
	switch status {
	case report.StatusPass:
		h.Set("Title", "Restorable: restore test passed")
		h.Set("Tags", "white_check_mark")
	case report.StatusFail:
		h.Set("Title", "Restorable: restore test FAILED")
		h.Set("Tags", "rotating_light")
		h.Set("Priority", "high")
	default:
		h.Set("Title", "Restorable: restore test could not run")
		h.Set("Tags", "warning")
		h.Set("Priority", "high")
	}
	return h
}

// payload is the webhook body. Field names are a public contract; add to it,
// don't rename.
type payload struct {
	Event             string               `json:"event"`
	Status            report.Status        `json:"status"`
	Summary           string               `json:"summary"`
	Repo              string               `json:"repo"`
	SnapshotID        string               `json:"snapshot_id,omitempty"`
	Error             string               `json:"error,omitempty"`
	StartedAt         time.Time            `json:"started_at"`
	FinishedAt        time.Time            `json:"finished_at"`
	RestoreDurationMS int64                `json:"restore_duration_ms,omitempty"`
	AgentVersion      string               `json:"agent_version"`
	Checks            []report.CheckResult `json:"checks"`
}

func newPayload(res *report.RunResult, summary string) payload {
	checks := make([]report.CheckResult, len(res.Checks))
	for i, c := range res.Checks {
		c.Message = report.RedactForTransport(c.Message)
		checks[i] = c
	}
	return payload{
		Event:             "restore_test.finished",
		Status:            res.Status,
		Summary:           summary,
		Repo:              report.Scrub(res.Repo),
		SnapshotID:        res.SnapshotID,
		Error:             report.RedactForTransport(res.Error),
		StartedAt:         res.StartedAt,
		FinishedAt:        res.FinishedAt,
		RestoreDurationMS: res.RestoreDurationMS,
		AgentVersion:      res.AgentVersion,
		Checks:            checks,
	}
}

// Summary renders a run as plain text for a notification. It carries the
// same facts as the terminal output, with every message redacted for
// transport.
func Summary(res *report.RunResult) string {
	lines := []string{
		"Restorable: " + strings.ToUpper(string(res.Status)),
		"repo: " + report.Scrub(res.Repo),
	}
	if res.SnapshotID != "" {
		id := res.SnapshotID
		if len(id) > 8 {
			id = id[:8]
		}
		lines = append(lines, "snapshot: "+id)
	}
	if res.RestoreDurationMS > 0 {
		restore := time.Duration(res.RestoreDurationMS) * time.Millisecond
		lines = append(lines, "restore time: "+restore.Round(time.Millisecond).String())
	}
	lines = append(lines, "total time: "+res.Duration().Round(time.Millisecond).String())
	if res.Error != "" {
		lines = append(lines, "error: "+report.RedactForTransport(res.Error))
	}
	for i, c := range res.Checks {
		if i == maxChecksListed {
			lines = append(lines, fmt.Sprintf("... and %d more check(s)", len(res.Checks)-maxChecksListed))
			break
		}
		mark := "PASS"
		if c.Status != report.StatusPass {
			mark = "FAIL"
		}
		line := fmt.Sprintf("%s %s/%s", mark, c.Recipe, c.Type)
		if c.Status != report.StatusPass && c.Message != "" {
			line += ": " + report.RedactForTransport(c.Message)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// truncateForNtfy cuts s to at most maxNtfyBody bytes without splitting a
// UTF-8 sequence.
func truncateForNtfy(s string) string {
	if len(s) <= maxNtfyBody {
		return s
	}
	const ellipsis = "..."
	cut := maxNtfyBody - len(ellipsis)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + ellipsis
}
