package recipes

import (
	"strings"
	"testing"
)

// psql and mysql print the reason first and then echo the offending statement
// with a caret under it. tail() kept the caret and dropped the reason, so
// users saw an arrow pointing at nothing while the sentence explaining the
// failure was discarded.
func TestDiagnosticKeepsTheReason(t *testing.T) {
	tests := []struct {
		name     string
		out      string
		n        int
		mustHave string
	}{
		{
			name: "postgres missing relation",
			out: "ERROR:  relation \"assets\" does not exist\n" +
				"LINE 1: SELECT count(*) FROM \"assets\"\n" +
				"                             ^",
			n:        2,
			mustHave: `relation "assets" does not exist`,
		},
		{
			name: "reason preceded by noise",
			out: "Pager usage is off.\n" +
				"psql:/tmp/dump:3: ERROR:  syntax error at or near \")\"\n" +
				"LINE 1: ...\n" +
				"        ^",
			n:        2,
			mustHave: "syntax error at or near",
		},
		{
			name:     "mysql fatal",
			out:      "mysql: [Warning] Using a password on the command line\nERROR 1146 (42S02) at line 3: Table 'restorable.users' doesn't exist",
			n:        2,
			mustHave: "doesn't exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnostic(tt.out, tt.n)
			if !strings.Contains(got, tt.mustHave) {
				t.Errorf("dropped the reason\n  want substring: %s\n  got: %s", tt.mustHave, got)
			}
		})
	}
}

// With nothing error-shaped to anchor on, fall back to the old behaviour
// rather than returning nothing.
func TestDiagnosticFallsBackToTail(t *testing.T) {
	out := "line one\nline two\nline three"
	got := diagnostic(out, 2)
	if !strings.Contains(got, "line three") {
		t.Errorf("expected tail fallback, got %q", got)
	}
}
