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
			name:    "unknown type",
			yaml:    "name: x\nchecks:\n  - type: zfs\n",
			wantErr: `unknown check type "zfs"`,
		},
		{
			name: "valid postgres check",
			yaml: "name: x\nchecks:\n  - type: postgres\n    dump: db/dump.sql\n    tables:\n      - name: users\n        min_rows: 1\n",
		},
		{
			name:    "postgres without dump",
			yaml:    "name: x\nchecks:\n  - type: postgres\n",
			wantErr: "needs a dump path",
		},
		{
			name:    "postgres bad table name",
			yaml:    "name: x\nchecks:\n  - type: postgres\n    dump: d.sql\n    tables:\n      - name: \"users; drop table x\"\n",
			wantErr: "plain identifier",
		},
		{
			name: "valid mysql check",
			yaml: "name: x\nchecks:\n  - type: mysql\n    dump: db/dump.sql\n",
		},
		{
			name: "valid sqlite check",
			yaml: "name: x\nchecks:\n  - type: sqlite\n    path: data/app.db\n",
		},
		{
			name:    "sqlite without path",
			yaml:    "name: x\nchecks:\n  - type: sqlite\n",
			wantErr: "needs a path",
		},
		{
			name: "valid docker-app check",
			yaml: "name: x\nchecks:\n  - type: docker-app\n    image: nextcloud:apache\n    mount: {restored: \".\", at: /var/www/html}\n    ready: {http: \"http://localhost:8080/status.php\", contains: installed, timeout: 90s}\n",
		},
		{
			name:    "docker-app without ready probe",
			yaml:    "name: x\nchecks:\n  - type: docker-app\n    image: i\n    mount: {restored: \".\", at: /data}\n",
			wantErr: "ready.http",
		},
		{
			name:    "docker-app bad timeout",
			yaml:    "name: x\nchecks:\n  - type: docker-app\n    image: i\n    mount: {at: /data}\n    ready: {http: \"http://localhost/\", timeout: soon}\n",
			wantErr: "invalid duration",
		},
		{
			name:    "docker-app relative mount point",
			yaml:    "name: x\nchecks:\n  - type: docker-app\n    image: i\n    mount: {at: html}\n    ready: {http: \"http://localhost/\"}\n",
			wantErr: "absolute container path",
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
