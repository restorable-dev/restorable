package report

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// A repo string longer than the control plane's bound used to fail the whole
// submission, so the repo row was never created, nothing appeared on the
// dashboard, and stale detection could never fire for it — while the run
// printed PASS and exited 0. Silence was the entire bug, so what matters here
// is that nothing the agent sends can exceed the bound.
func TestTruncateRepoLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"short path is untouched", "/srv/backups/restic"},
		{"exactly at the bound", strings.Repeat("a", maxRepoLabelBytes)},
		{"one past the bound", strings.Repeat("a", maxRepoLabelBytes+1)},
		{"long b2 url", "b2:my-bucket-with-a-long-name:" + strings.Repeat("nested/", 60) + "repo"},
		{"long s3 url", "s3:https://s3.eu-central-1.amazonaws.com/" + strings.Repeat("deep/", 70) + "restic"},
		{"multibyte runes at the cut points", strings.Repeat("ünïcödé-påth/", 40)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateRepoLabel(tt.in)
			if len(got) > maxRepoLabelBytes {
				t.Errorf("result is %d bytes, over the %d bound", len(got), maxRepoLabelBytes)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncation split a rune: %q", got)
			}
			if len(tt.in) <= maxRepoLabelBytes && got != tt.in {
				t.Errorf("modified a label that already fit:\n  in:  %q\n  got: %q", tt.in, got)
			}
		})
	}
}

// Truncating from either end alone would drop the half that makes the label
// readable, so both survive.
func TestTruncateRepoLabelKeepsBothEnds(t *testing.T) {
	in := "s3:https://s3.amazonaws.com/" + strings.Repeat("x/", 200) + "prod-repo"
	got := truncateRepoLabel(in)
	if !strings.HasPrefix(got, "s3:https://s3.amazonaws.com/") {
		t.Errorf("lost the scheme and host: %q", got)
	}
	if !strings.HasSuffix(got, "prod-repo") {
		t.Errorf("lost the distinguishing tail: %q", got)
	}
}
