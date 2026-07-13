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
func TestRedactForTransportStripsDBDetail(t *testing.T) {
	msg := "dump failed to load into postgres: ERROR: duplicate key value violates unique constraint\n" +
		"DETAIL: Key (email)=(alice@example.com) already exists."
	got := RedactForTransport(msg)
	if strings.Contains(got, "alice@example.com") || strings.Contains(got, "DETAIL") {
		t.Errorf("RedactForTransport leaked DB detail: %q", got)
	}
	if !strings.Contains(got, "failed to load") {
		t.Errorf("RedactForTransport over-redacted, lost the error class: %q", got)
	}
}

func TestRedactForTransportBoundsLength(t *testing.T) {
	got := RedactForTransport(strings.Repeat("x", 5000))
	if len(got) > 520 {
		t.Errorf("RedactForTransport did not bound length: %d chars", len(got))
	}
}
