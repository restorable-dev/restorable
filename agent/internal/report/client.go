package report

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrNoCredentials means the agent is not registered with a control plane —
// standalone mode, which is fully supported, so callers treat it as "skip
// reporting", never as a failure.
var ErrNoCredentials = errors.New("no cloud credentials")

// Credentials connect an agent to the control plane. The API key is the only
// secret; it authorizes exactly this agent's own submissions.
type Credentials struct {
	URL     string `json:"url"`
	AgentID string `json:"agent_id"`
	APIKey  string `json:"api_key"`
}

// DefaultCredentialsPath is where `restorable register` stores credentials
// unless told otherwise: <user config dir>/restorable/credentials.json.
func DefaultCredentialsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(dir, "restorable", "credentials.json"), nil
}

// LoadCredentials reads credentials, returning ErrNoCredentials when the
// file does not exist.
func LoadCredentials(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse credentials %s: %w", path, err)
	}
	if c.URL == "" || c.APIKey == "" {
		return nil, fmt.Errorf("credentials %s are incomplete", path)
	}
	return &c, nil
}

// SaveCredentials writes credentials with owner-only permissions.
func SaveCredentials(path string, c *Credentials) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

// Fingerprint identifies a repository to the control plane without revealing
// its location: sha256 of the credential-scrubbed repo string. Used as a
// fallback when the repository's canonical ID isn't available.
func Fingerprint(repo string) string {
	sum := sha256.Sum256([]byte(Scrub(repo)))
	return hex.EncodeToString(sum[:])
}

// FingerprintID derives a repo fingerprint from restic's canonical repository
// ID. This is stable across how the repo location is spelled (relative vs
// absolute path, trailing slash), so the same repo is never counted twice.
func FingerprintID(repoID string) string {
	sum := sha256.Sum256([]byte("restic-repo-id:" + repoID))
	return hex.EncodeToString(sum[:])
}

// Client talks to the control plane's /api/v1.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient builds a client from credentials.
func NewClient(creds *Credentials) *Client {
	return &Client{
		baseURL: strings.TrimRight(creds.URL, "/"),
		apiKey:  creds.APIKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

type runPayload struct {
	RepoFingerprint string    `json:"repo_fingerprint"`
	RepoLabel       string    `json:"repo_label"`
	SnapshotID      string    `json:"snapshot_id,omitempty"`
	Status          Status    `json:"status"`
	Error           string    `json:"error,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	// RestoreDurationMS is metadata (a timing), so it may leave the machine.
	RestoreDurationMS int64         `json:"restore_duration_ms,omitempty"`
	AgentVersion      string        `json:"agent_version"`
	Checks            []CheckResult `json:"checks"`
}

// maxRepoLabelBytes mirrors the control plane's bound on repo_label. Keeping
// the agent inside it matters more than it looks: the server rejected the
// whole submission when a label ran long, so the repo row was never created,
// the run never appeared on the dashboard, and stale detection could never
// fire for that repo — while the agent still printed PASS and exited 0. Long
// B2 and S3 URLs reach 200 characters easily.
//
// Bounding bytes is enough: the server counts UTF-16 units, which is never
// more than the UTF-8 byte count.
const maxRepoLabelBytes = 200

// truncateRepoLabel keeps both ends of an over-long repo string. The head
// carries the scheme and host, the tail carries the path that tells two repos
// on the same host apart, so cutting either end alone loses the half that
// makes the label worth showing. The label is display only; repos are
// identified by fingerprint, so shortening it costs nothing.
func truncateRepoLabel(s string) string {
	if len(s) <= maxRepoLabelBytes {
		return s
	}
	const ellipsis = "…"
	budget := maxRepoLabelBytes - len(ellipsis)
	head := budget / 2
	tailStart := len(s) - (budget - head)
	for head > 0 && !utf8.RuneStart(s[head]) {
		head--
	}
	for tailStart < len(s) && !utf8.RuneStart(s[tailStart]) {
		tailStart++
	}
	return s[:head] + ellipsis + s[tailStart:]
}

// SubmitRun reports one run result. Only pass/fail metadata leaves the
// machine: the repo is reduced to a fingerprint plus its scrubbed label, and
// every string in the result was scrubbed when the result was built.
func (c *Client) SubmitRun(ctx context.Context, r *RunResult) error {
	// Redact every check message before it leaves the machine: check messages
	// are the only field that can embed raw output from the user's restored
	// databases. Local stdout already showed the full text; the cloud gets a
	// scrubbed, DB-detail-stripped, length-bounded version.
	checks := make([]CheckResult, len(r.Checks))
	for i, ck := range r.Checks {
		ck.Message = RedactForTransport(ck.Message)
		checks[i] = ck
	}
	fingerprint := r.RepoFingerprint
	if fingerprint == "" {
		fingerprint = Fingerprint(r.Repo) // fallback for old restic without a repo ID
	}
	payload := runPayload{
		RepoFingerprint: fingerprint,
		RepoLabel:       truncateRepoLabel(r.Repo), // scrubbed at result construction
		SnapshotID:      r.SnapshotID,
		Status:          r.Status,
		// Same treatment as check messages. A failed restore enumerates paths
		// out of the user's snapshot, so this field needs the DB-detail strip
		// and the length bound too, not just the credential scrub it already
		// carries from result construction.
		Error:             RedactForTransport(r.Error),
		StartedAt:         r.StartedAt,
		FinishedAt:        r.FinishedAt,
		RestoreDurationMS: r.RestoreDurationMS,
		AgentVersion:      r.AgentVersion,
		Checks:            checks,
	}
	var resp struct {
		RunID string `json:"run_id"`
	}
	return c.post(ctx, "/api/v1/runs", payload, &resp, http.StatusCreated)
}

// Heartbeat pings the control plane so stale detection knows we are alive.
func (c *Client) Heartbeat(ctx context.Context) error {
	return c.post(ctx, "/api/v1/heartbeat", struct{}{}, nil, http.StatusNoContent)
}

func (c *Client) post(ctx context.Context, path string, body, out any, wantStatus int) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("control plane request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response fully read
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("control plane %s: HTTP %d: %s", path, resp.StatusCode, apiError(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("control plane %s: parse response: %w", path, err)
		}
	}
	return nil
}

// Register exchanges a one-time registration token for agent credentials.
func Register(ctx context.Context, baseURL, token, name, version string) (*Credentials, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	body, err := json.Marshal(map[string]string{
		"token":         token,
		"name":          name,
		"agent_version": version,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/api/v1/agents/register", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registration request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response fully read
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration failed: HTTP %d: %s", resp.StatusCode, apiError(raw))
	}
	var out struct {
		AgentID string `json:"agent_id"`
		APIKey  string `json:"api_key"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse registration response: %w", err)
	}
	if out.APIKey == "" {
		return nil, errors.New("registration response is missing the API key")
	}
	return &Credentials{URL: baseURL, AgentID: out.AgentID, APIKey: out.APIKey}, nil
}

// apiError extracts the error field from an API response body for messages.
func apiError(raw []byte) string {
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Error != "" {
		return e.Error
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
