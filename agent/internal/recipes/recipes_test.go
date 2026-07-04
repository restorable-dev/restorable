package recipes

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "valid files recipe",
			yaml: `name: nextcloud
checks:
  - type: files
    require:
      - path: config/config.php
      - path: data/
        min_files: 100
    checksum_sample: 5
`,
		},
		{
			name:    "missing name",
			yaml:    "checks:\n  - type: files\n    require:\n      - path: a\n",
			wantErr: "needs a name",
		},
		{
			name:    "no checks",
			yaml:    "name: x\n",
			wantErr: "no checks",
		},
		{
			name:    "empty recipe",
			yaml:    "",
			wantErr: "empty",
		},
		{
			name:    "check missing type",
			yaml:    "name: x\nchecks:\n  - require:\n      - path: a\n",
			wantErr: "missing a type",
		},
		{
			name:    "phase 2 type has helpful error",
			yaml:    "name: x\nchecks:\n  - type: postgres\n",
			wantErr: "not available yet",
		},
		{
			name:    "unknown type",
			yaml:    "name: x\nchecks:\n  - type: zfs\n",
			wantErr: `unknown check type "zfs"`,
		},
		{
			name:    "unknown field in files check",
			yaml:    "name: x\nchecks:\n  - type: files\n    requires:\n      - path: a\n",
			wantErr: "requires",
		},
		{
			name:    "files check with nothing to do",
			yaml:    "name: x\nchecks:\n  - type: files\n",
			wantErr: "at least one require",
		},
		{
			name:    "absolute path rejected",
			yaml:    "name: x\nchecks:\n  - type: files\n    require:\n      - path: /etc/passwd\n",
			wantErr: "must be relative",
		},
		{
			name:    "path escape rejected",
			yaml:    "name: x\nchecks:\n  - type: files\n    require:\n      - path: ../outside\n",
			wantErr: "escape",
		},
		{
			name:    "negative min_files rejected",
			yaml:    "name: x\nchecks:\n  - type: files\n    require:\n      - path: a\n        min_files: -1\n",
			wantErr: "negative",
		},
		{
			name:    "negative checksum_sample rejected",
			yaml:    "name: x\nchecks:\n  - type: files\n    checksum_sample: -2\n",
			wantErr: "negative",
		},
		{
			name:    "unknown top-level field",
			yaml:    "name: x\nsteps: []\nchecks:\n  - type: files\n    require: [{path: a}]\n",
			wantErr: "steps",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := Parse([]byte(tt.yaml))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Parse() succeeded, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Parse() error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if len(rec.Checks) == 0 {
				t.Fatal("Parse() returned recipe with no checks")
			}
		})
	}
}
