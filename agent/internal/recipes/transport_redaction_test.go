package recipes

import (
	"strings"
	"testing"

	"github.com/restorable-dev/restorable/agent/internal/report"
)

// The redaction bug this guards against was not in either function on its own.
// tail() joins output with " / " and report's filter was anchored to line
// starts, so each half was individually reasonable and the seam between them
// leaked every row value a failed database load echoes. Unit tests on either
// side passed throughout.
//
// This asserts the two agree, using output captured verbatim from psql and
// mysql, so changing either joiner or pattern in isolation fails here.
func TestTailOutputSurvivesTransportRedaction(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		mustHide []string
	}{
		{
			name: "postgres unique violation echoes the key",
			raw: "psql:/tmp/dump:5: ERROR:  duplicate key value violates unique constraint \"users_pkey\"\n" +
				"DETAIL:  Key (email)=(alice@example.com) already exists.\n" +
				"CONTEXT:  COPY users, line 2",
			mustHide: []string{"alice@example.com", "COPY users"},
		},
		{
			name: "postgres syntax error echoes the statement",
			raw: "psql:/tmp/dump:1: ERROR:  syntax error at or near \")\"\n" +
				"LINE 1: ...email) VALUES ('a@example.com'), ('b@example.com')\n" +
				"                                                          ^",
			mustHide: []string{"a@example.com", "b@example.com"},
		},
		{
			// MySQL puts the row value on the ERROR line itself rather than in
			// a DETAIL segment. An earlier version of this case carried
			// dave@example.com in the input and never asserted it was hidden,
			// so the suite stayed green while the address went out on the wire.
			name: "mysql inlines the row value on the ERROR line",
			raw: "ERROR 1062 (23000) at line 3: Duplicate entry 'dave@example.com' for key 'users.email'\n" +
				"DETAIL:  row 3 rejected",
			mustHide: []string{"dave@example.com", "row 3 rejected"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Exactly what the database checks do before building a message.
			joined := tail(tt.raw, 2)
			got := report.RedactForTransport(joined)
			for _, secret := range tt.mustHide {
				if strings.Contains(got, secret) {
					t.Errorf("row data survived redaction: %q\n  joined: %s\n  sent:   %s", secret, joined, got)
				}
			}
		})
	}
}
