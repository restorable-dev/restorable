package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestScrub(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "rest server credentials",
			in:   "rest:https://user:secret@backup.example.com:8000/repo",
			want: "rest:https://***@backup.example.com:8000/repo",
		},
		{
			name: "sftp user",
			in:   "sftp://backup@nas.local/srv/restic",
			want: "sftp://***@nas.local/srv/restic",
		},
		{
			name: "credentials inside an error string",
			in:   "restic restore: exit 1: cannot open https://u:p@host/repo: timeout",
			want: "restic restore: exit 1: cannot open https://***@host/repo: timeout",
		},
		{name: "local path untouched", in: "/srv/backups/restic", want: "/srv/backups/restic"},
		{name: "s3 without creds untouched", in: "s3:s3.amazonaws.com/bucket/repo", want: "s3:s3.amazonaws.com/bucket/repo"},
		{name: "empty", in: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Scrub(tt.in); got != tt.want {
				t.Errorf("Scrub(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		status Status
		want   int
	}{
		{StatusPass, 0},
		{StatusFail, 1},
		{StatusError, 2},
		{Status("bogus"), 2},
	}
	for _, tt := range tests {
		r := &RunResult{Status: tt.status}
		if got := r.ExitCode(); got != tt.want {
			t.Errorf("ExitCode(%s) = %d, want %d", tt.status, got, tt.want)
		}
	}
}

func sampleResult() *RunResult {
	start := time.Date(2026, 7, 3, 3, 0, 0, 0, time.UTC)
	return &RunResult{
		AgentVersion: "v0.1.0",
		Repo:         "/srv/backups",
		SnapshotID:   "dda3522e5a853671a9e2ea2d175ad18cfcf4e988",
		Status:       StatusFail,
		StartedAt:    start,
		FinishedAt:   start.Add(90 * time.Second),
		Checks: []CheckResult{
			{Recipe: "nextcloud", Type: "files", Status: StatusPass, Message: "3 required path(s) present", DurationMS: 42},
			{Recipe: "nextcloud", Type: "files", Status: StatusFail, Message: "data/ holds 2 files, need at least 100", DurationMS: 7},
		},
	}
}

func TestWriteJSONRoundTrip(t *testing.T) {
	r := sampleResult()
	var buf bytes.Buffer
	if err := r.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON() error: %v", err)
	}
	var back RunResult
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if back.Status != StatusFail || len(back.Checks) != 2 || back.Checks[1].Message == "" {
		t.Errorf("round-trip lost data: %+v", back)
	}
	if !strings.Contains(buf.String(), `"duration_ms"`) {
		t.Error("JSON lacks duration_ms field")
	}
}

func TestHuman(t *testing.T) {
	out := sampleResult().Human()
	for _, want := range []string{"FAIL", "dda3522e", "✓", "✗", "need at least 100", "1m30s"} {
		if !strings.Contains(out, want) {
			t.Errorf("Human() output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "dda3522e5a85") {
		t.Error("Human() should truncate the snapshot id to 8 chars")
	}
}
