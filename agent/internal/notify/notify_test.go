package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/restorable-dev/restorable/agent/internal/config"
	"github.com/restorable-dev/restorable/agent/internal/report"
)

// request is what the fake endpoint saw.
type request struct {
	Path   string
	Query  string
	Header http.Header
	Body   string
}

// recorder is a fake endpoint that records every request and answers with
// status.
type recorder struct {
	mu     sync.Mutex
	reqs   []request
	status int
	srv    *httptest.Server
}

func newRecorder(t *testing.T, status int) *recorder {
	t.Helper()
	rec := &recorder{status: status}
	rec.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.reqs = append(rec.reqs, request{
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Header: r.Header.Clone(),
			Body:   string(body),
		})
		rec.mu.Unlock()
		w.WriteHeader(rec.status)
	}))
	t.Cleanup(rec.srv.Close)
	return rec
}

func (r *recorder) url(path string) string { return r.srv.URL + path }

func (r *recorder) paths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.reqs))
	for i, q := range r.reqs {
		out[i] = q.Path
	}
	return out
}

func (r *recorder) find(t *testing.T, path string) request {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, q := range r.reqs {
		if q.Path == path {
			return q
		}
	}
	t.Fatalf("no request to %s; got %v", path, r.paths())
	return request{}
}

// logSink collects log lines.
type logSink struct {
	mu    sync.Mutex
	lines []string
}

func (l *logSink) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *logSink) all() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

const leakyDBMessage = `pg_restore failed: ERROR: duplicate key value violates unique constraint "users_email_key" / DETAIL: Key (email)=(alice@example.com) already exists.`

func passResult() *report.RunResult {
	start := time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC)
	return &report.RunResult{
		AgentVersion:      "v0.2.0",
		Repo:              "/srv/backups/restic",
		SnapshotID:        "cd10c302aabbccdd",
		Status:            report.StatusPass,
		StartedAt:         start,
		FinishedAt:        start.Add(5500 * time.Millisecond),
		RestoreDurationMS: 4100,
		Checks: []report.CheckResult{
			{Recipe: "nextcloud", Type: "files", Status: report.StatusPass, Message: "2 required path(s) present", DurationMS: 3922},
		},
	}
}

func failResult() *report.RunResult {
	res := passResult()
	res.Status = report.StatusFail
	res.Repo = "rest:https://backup:hunter2@nas.lan:8000/restic"
	res.Checks = append(res.Checks, report.CheckResult{
		Recipe: "app", Type: "postgres", Status: report.StatusFail, Message: leakyDBMessage, DurationMS: 812,
	})
	return res
}

func allEndpoints(rec *recorder, when string) config.NotifyConfig {
	return config.NotifyConfig{
		When:        when,
		Ntfy:        rec.url("/ntfy-topic"),
		Webhook:     rec.url("/hook"),
		Healthcheck: rec.url("/ping/abc-uuid"),
	}
}

func TestPassOnFailurePolicyOnlyPingsHealthcheck(t *testing.T) {
	rec := newRecorder(t, http.StatusOK)
	logs := &logSink{}
	New(allEndpoints(rec, config.NotifyOnFailure), logs.logf).Finish(context.Background(), passResult())

	if got := rec.paths(); len(got) != 1 || got[0] != "/ping/abc-uuid" {
		t.Fatalf("requests = %v, want only the success ping", got)
	}
	if body := rec.find(t, "/ping/abc-uuid").Body; !strings.Contains(body, "Restorable: PASS") {
		t.Errorf("ping body = %q, want the run summary", body)
	}
	if l := logs.all(); l != "" {
		t.Errorf("unexpected log output: %s", l)
	}
}

func TestPassAlwaysPolicyNotifiesEverywhere(t *testing.T) {
	rec := newRecorder(t, http.StatusOK)
	New(allEndpoints(rec, config.NotifyAlways), (&logSink{}).logf).Finish(context.Background(), passResult())

	for _, p := range []string{"/ping/abc-uuid", "/ntfy-topic", "/hook"} {
		rec.find(t, p)
	}
	if h := rec.find(t, "/ntfy-topic").Header; h.Get("Priority") != "" {
		t.Errorf("a pass should use ntfy's default priority, got %q", h.Get("Priority"))
	}
}

func TestFailureNotifiesEverywhereWithRedaction(t *testing.T) {
	rec := newRecorder(t, http.StatusOK)
	New(allEndpoints(rec, config.NotifyOnFailure), (&logSink{}).logf).Finish(context.Background(), failResult())

	ping := rec.find(t, "/ping/abc-uuid/fail")
	ntfy := rec.find(t, "/ntfy-topic")
	hook := rec.find(t, "/hook")

	if ntfy.Header.Get("Priority") != "high" || !strings.Contains(ntfy.Header.Get("Title"), "FAILED") {
		t.Errorf("ntfy headers = %v, want high priority and a FAILED title", ntfy.Header)
	}
	if hook.Header.Get("Content-Type") != "application/json" {
		t.Errorf("webhook Content-Type = %q", hook.Header.Get("Content-Type"))
	}

	for name, body := range map[string]string{"ping": ping.Body, "ntfy": ntfy.Body, "webhook": hook.Body} {
		if strings.Contains(body, "alice@example.com") {
			t.Errorf("%s body leaks a row value from the restored database: %s", name, body)
		}
		if strings.Contains(body, "hunter2") {
			t.Errorf("%s body leaks the repository password: %s", name, body)
		}
		if !strings.Contains(body, "users_email_key") {
			t.Errorf("%s body lost the failure reason: %s", name, body)
		}
	}

	var p payload
	if err := json.Unmarshal([]byte(hook.Body), &p); err != nil {
		t.Fatalf("webhook body is not JSON: %v", err)
	}
	if p.Event != "restore_test.finished" || p.Status != report.StatusFail || len(p.Checks) != 2 {
		t.Errorf("webhook payload = %+v", p)
	}
	if p.SnapshotID != "cd10c302aabbccdd" || p.RestoreDurationMS != 4100 || p.AgentVersion != "v0.2.0" {
		t.Errorf("webhook payload lost run metadata: %+v", p)
	}
}

func TestErrorStatusCountsAsFailure(t *testing.T) {
	rec := newRecorder(t, http.StatusOK)
	res := passResult()
	res.Status = report.StatusError
	res.Error = "find latest snapshot: repository does not exist"
	res.Checks = nil
	New(allEndpoints(rec, config.NotifyOnFailure), (&logSink{}).logf).Finish(context.Background(), res)

	rec.find(t, "/ping/abc-uuid/fail")
	if title := rec.find(t, "/ntfy-topic").Header.Get("Title"); !strings.Contains(title, "could not run") {
		t.Errorf("ntfy title = %q", title)
	}
}

func TestStartPing(t *testing.T) {
	rec := newRecorder(t, http.StatusOK)
	New(allEndpoints(rec, config.NotifyOnFailure), (&logSink{}).logf).Start(context.Background())
	if got := rec.paths(); len(got) != 1 || got[0] != "/ping/abc-uuid/start" {
		t.Fatalf("requests = %v, want only the start ping", got)
	}
}

func TestPingKeepsQueryString(t *testing.T) {
	rec := newRecorder(t, http.StatusOK)
	cfg := config.NotifyConfig{When: config.NotifyOnFailure, Healthcheck: rec.url("/ping/key/backup-check/?create=1")}
	New(cfg, (&logSink{}).logf).Finish(context.Background(), failResult())

	q := rec.find(t, "/ping/key/backup-check/fail")
	if q.Query != "create=1" {
		t.Errorf("query = %q, want create=1", q.Query)
	}
}

func TestNothingConfiguredSendsNothing(t *testing.T) {
	logs := &logSink{}
	n := New(config.NotifyConfig{When: config.NotifyAlways}, logs.logf)
	n.Start(context.Background())
	n.Finish(context.Background(), failResult())
	if l := logs.all(); l != "" {
		t.Errorf("unexpected log output: %s", l)
	}
	if d := n.Describe(); d != "none configured" {
		t.Errorf("Describe() = %q", d)
	}
}

func TestDeliveryFailuresAreLoggedWithoutURL(t *testing.T) {
	rec := newRecorder(t, http.StatusInternalServerError)
	logs := &logSink{}
	New(allEndpoints(rec, config.NotifyOnFailure), logs.logf).Finish(context.Background(), failResult())

	out := logs.all()
	if !strings.Contains(out, "HTTP 500") {
		t.Errorf("logs = %q, want the HTTP status", out)
	}

	// An unreachable endpoint: the transport error names the URL unless we
	// strip it.
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	logs = &logSink{}
	cfg := config.NotifyConfig{
		When:        config.NotifyOnFailure,
		Ntfy:        deadURL + "/secret-topic",
		Healthcheck: deadURL + "/ping/secret-uuid",
	}
	New(cfg, logs.logf).Finish(context.Background(), failResult())
	out = logs.all()
	if !strings.Contains(out, "failed") {
		t.Fatalf("logs = %q, want delivery failures", out)
	}
	for _, secret := range []string{"secret-topic", "secret-uuid", deadURL} {
		if strings.Contains(out, secret) {
			t.Errorf("log leaks %q: %s", secret, out)
		}
	}
}

func TestDescribe(t *testing.T) {
	rec := newRecorder(t, http.StatusOK)
	got := New(allEndpoints(rec, config.NotifyOnFailure), nil).Describe()
	want := "ntfy and webhook on failure; healthcheck ping after every run"
	if got != want {
		t.Errorf("Describe() = %q, want %q", got, want)
	}
	if strings.Contains(got, rec.srv.URL) {
		t.Errorf("Describe() leaks a URL: %q", got)
	}
}

func TestSummaryCapsCheckList(t *testing.T) {
	res := passResult()
	res.Checks = nil
	for i := range maxChecksListed + 3 {
		res.Checks = append(res.Checks, report.CheckResult{Recipe: "r", Type: fmt.Sprintf("files%d", i), Status: report.StatusPass})
	}
	s := Summary(res)
	if !strings.Contains(s, "... and 3 more check(s)") {
		t.Errorf("summary = %q, want a capped check list", s)
	}
}

func TestTruncateForNtfy(t *testing.T) {
	long := strings.Repeat("é", maxNtfyBody) // 2 bytes per rune
	got := truncateForNtfy(long)
	if len(got) > maxNtfyBody {
		t.Errorf("len = %d, over %d", len(got), maxNtfyBody)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("missing ellipsis")
	}
	trimmed := strings.TrimSuffix(got, "...")
	if strings.ContainsRune(trimmed, '�') || !strings.HasSuffix(trimmed, "é") {
		t.Errorf("cut split a UTF-8 sequence")
	}
	if short := "ok"; truncateForNtfy(short) != short {
		t.Errorf("short strings must pass through unchanged")
	}
}
