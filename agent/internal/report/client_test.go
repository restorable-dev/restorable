package report

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCredentialsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "credentials.json")
	creds := &Credentials{URL: "https://cloud.example", AgentID: "a1", APIKey: "rsk_x"}

	if err := SaveCredentials(path, creds); err != nil {
		t.Fatalf("SaveCredentials() error: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("credentials mode = %o, want 600", info.Mode().Perm())
		}
	}

	got, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("LoadCredentials() error: %v", err)
	}
	if *got != *creds {
		t.Errorf("round trip = %+v, want %+v", got, creds)
	}
}

func TestLoadCredentialsMissing(t *testing.T) {
	_, err := LoadCredentials(filepath.Join(t.TempDir(), "nope.json"))
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("error = %v, want ErrNoCredentials", err)
	}
}

func TestFingerprint(t *testing.T) {
	// Deterministic, and identical for credential variants of the same repo:
	// the fingerprint is computed over the SCRUBBED string.
	withCreds := Fingerprint("rest:https://user:pass@host/repo")
	scrubbed := Fingerprint("rest:https://***@host/repo")
	if withCreds != scrubbed {
		t.Error("fingerprint must be computed over the scrubbed repo string")
	}
	if len(withCreds) != 64 {
		t.Errorf("fingerprint length = %d, want 64 hex chars", len(withCreds))
	}
	if Fingerprint("/srv/a") == Fingerprint("/srv/b") {
		t.Error("different repos must have different fingerprints")
	}
}

func TestRegister(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotBody map[string]string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/agents/register" {
				t.Errorf("path = %s", r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Error(err)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"agent_id": "a1", "api_key": "rsk_abc"})
		}))
		defer srv.Close()

		creds, err := Register(context.Background(), srv.URL+"/", "rrt_tok", "box", "v1.0.0")
		if err != nil {
			t.Fatalf("Register() error: %v", err)
		}
		if creds.APIKey != "rsk_abc" || creds.AgentID != "a1" || creds.URL != srv.URL {
			t.Errorf("creds = %+v", creds)
		}
		if gotBody["token"] != "rrt_tok" || gotBody["name"] != "box" || gotBody["agent_version"] != "v1.0.0" {
			t.Errorf("request body = %v", gotBody)
		}
	})

	t.Run("used token is a clear error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "registration token is invalid, expired, or already used",
			})
		}))
		defer srv.Close()

		_, err := Register(context.Background(), srv.URL, "rrt_tok", "box", "v1")
		if err == nil || !strings.Contains(err.Error(), "already used") {
			t.Fatalf("error = %v, want server message surfaced", err)
		}
	})
}

func TestSubmitRun(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": "r1"})
	}))
	defer srv.Close()

	start := time.Date(2026, 7, 4, 3, 0, 0, 0, time.UTC)
	res := &RunResult{
		AgentVersion:      "v0.1.0",
		Repo:              "/srv/backups",
		SnapshotID:        "abc123",
		Status:            StatusPass,
		StartedAt:         start,
		FinishedAt:        start.Add(time.Minute),
		RestoreDurationMS: 42500,
		Checks: []CheckResult{
			{Recipe: "r", Type: "files", Status: StatusPass, Message: "ok", DurationMS: 5},
		},
	}
	client := NewClient(&Credentials{URL: srv.URL, APIKey: "rsk_k"})
	if err := client.SubmitRun(context.Background(), res); err != nil {
		t.Fatalf("SubmitRun() error: %v", err)
	}

	if auth != "Bearer rsk_k" {
		t.Errorf("auth header = %q", auth)
	}
	if got["repo_fingerprint"] != Fingerprint("/srv/backups") {
		t.Errorf("fingerprint = %v", got["repo_fingerprint"])
	}
	if got["repo_label"] != "/srv/backups" || got["status"] != "pass" {
		t.Errorf("payload = %v", got)
	}
	checks, ok := got["checks"].([]any)
	if !ok || len(checks) != 1 {
		t.Errorf("checks = %v", got["checks"])
	}
	if got["restore_duration_ms"] != float64(42500) {
		t.Errorf("restore_duration_ms = %v, want 42500", got["restore_duration_ms"])
	}
}

func TestSubmitRunNilChecksBecomesEmptyArray(t *testing.T) {
	var raw map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"run_id":"r1"}`))
	}))
	defer srv.Close()

	res := &RunResult{Repo: "/r", Status: StatusError, StartedAt: time.Now(), FinishedAt: time.Now()}
	client := NewClient(&Credentials{URL: srv.URL, APIKey: "rsk_k"})
	if err := client.SubmitRun(context.Background(), res); err != nil {
		t.Fatalf("SubmitRun() error: %v", err)
	}
	if string(raw["checks"]) != "[]" {
		t.Errorf("checks marshaled as %s, want []", raw["checks"])
	}
}

func TestSubmitRunServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid or missing agent API key"}`))
	}))
	defer srv.Close()

	client := NewClient(&Credentials{URL: srv.URL, APIKey: "rsk_bad"})
	err := client.SubmitRun(context.Background(), &RunResult{
		Repo: "/r", Status: StatusPass, StartedAt: time.Now(), FinishedAt: time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("error = %v, want HTTP 401 surfaced", err)
	}
}
