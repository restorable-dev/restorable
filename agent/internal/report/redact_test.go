package report

import (
	"strings"
	"testing"
)

// The credential scrubber must fully mask passwords containing '/' or '@',
// which restic permits in rest:/s3: URLs (the review found the old regex
// leaked these).
func TestScrubTrickyPasswords(t *testing.T) {
	tests := []struct {
		in       string
		mustHide string
	}{
		{"rest:https://user:pa/ss@host/repo", "pa/ss"},
		{"rest:https://user:p@ss@host/repo", "ss"},
		{"s3:https://AKIAEXAMPLE:se/cr/et@s3.amazonaws.com/b", "se/cr/et"},
		{"rest:https://u:p+a=s/s@host", "p+a=s/s"},
	}
	for _, tt := range tests {
		got := Scrub(tt.in)
		if strings.Contains(got, tt.mustHide) {
			t.Errorf("Scrub(%q) = %q still leaks %q", tt.in, got, tt.mustHide)
		}
		if !strings.Contains(got, "***@") {
			t.Errorf("Scrub(%q) = %q did not mask", tt.in, got)
		}
	}
}

// Check messages that leave the machine must not carry DB row data (Postgres
// DETAIL lines echo actual values from the user's backup).
//
// Every case here is the shape the recipe engine actually produces. An earlier
// version of this test used only the newline-joined form, which the agent
// never generates: the engine joins command output with " / ", so the
// line-anchored pattern this test was guarding never fired in production and
// the test passed against a message that could not occur.
func TestRedactForTransportStripsDBDetail(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		mustHide []string
		mustKeep string
	}{
		{
			name: "slash-joined, the shape tail() produces",
			msg: `dump "db/dump.sql" failed to load into postgres: psql:/tmp/dump:5: ERROR:  duplicate key value violates unique constraint "users_pkey"` +
				` / DETAIL:  Key (email)=(alice@example.com) already exists.` +
				` / CONTEXT:  COPY users, line 2`,
			mustHide: []string{"alice@example.com", "DETAIL", "CONTEXT", "COPY users"},
			mustKeep: "failed to load",
		},
		{
			name:     "newline-joined",
			msg:      "dump failed to load into postgres: ERROR: duplicate key\nDETAIL: Key (email)=(bob@example.com) already exists.",
			mustHide: []string{"bob@example.com", "DETAIL"},
			mustKeep: "failed to load",
		},
		{
			name:     "psql LINE echo carries the statement text",
			msg:      `dump failed: ERROR:  syntax error at or near ")" / LINE 1: ...email) VALUES ('a@example.com'), ('b@example.com'), ('c@exam`,
			mustHide: []string{"a@example.com", "b@example.com", "LINE 1"},
			mustKeep: "dump failed",
		},
		{
			name:     "mysql HINT",
			msg:      `load failed / HINT:  row 4 value "carol@example.com" is invalid`,
			mustHide: []string{"carol@example.com", "HINT"},
			mustKeep: "load failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactForTransport(tt.msg)
			for _, secret := range tt.mustHide {
				if strings.Contains(got, secret) {
					t.Errorf("leaked %q\n  got: %s", secret, got)
				}
			}
			if !strings.Contains(got, tt.mustKeep) {
				t.Errorf("over-redacted, lost %q\n  got: %s", tt.mustKeep, got)
			}
		})
	}
}

func TestRedactForTransportBoundsLength(t *testing.T) {
	got := RedactForTransport(strings.Repeat("x", 5000))
	if len(got) > 520 {
		t.Errorf("RedactForTransport did not bound length: %d chars", len(got))
	}
}
