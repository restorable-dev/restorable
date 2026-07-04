package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes an agent.yaml (and an empty recipe file if referenced)
// into a temp dir and returns the config path.
func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		recipe  bool // create recipe.yaml next to the config
		wantErr string
		check   func(t *testing.T, c *Config)
	}{
		{
			name: "minimal valid config",
			yaml: "repo: /srv/backups/restic\n",
			check: func(t *testing.T, c *Config) {
				if c.PasswordEnv != "RESTIC_PASSWORD" {
					t.Errorf("PasswordEnv = %q, want default RESTIC_PASSWORD", c.PasswordEnv)
				}
			},
		},
		{
			name:   "full config with recipe resolution",
			yaml:   "repo: s3:s3.amazonaws.com/bucket\npassword_env: MY_PASS\nschedule: \"0 3 * * *\"\nrecipes: [recipe.yaml]\nsandbox:\n  dir: /var/tmp\n  min_free_space: 2GiB\n",
			recipe: true,
			check: func(t *testing.T, c *Config) {
				if !filepath.IsAbs(c.Recipes[0]) {
					t.Errorf("recipe path %q not resolved to absolute", c.Recipes[0])
				}
				if got := c.MinFreeBytes(); got != 2<<30 {
					t.Errorf("MinFreeBytes = %d, want %d", got, 2<<30)
				}
			},
		},
		{name: "missing repo", yaml: "schedule: \"0 3 * * *\"\n", wantErr: "repo is required"},
		{name: "empty file", yaml: "", wantErr: "empty"},
		{name: "unknown field", yaml: "repo: /r\nrepositry: oops\n", wantErr: "repositry"},
		{name: "bad schedule", yaml: "repo: /r\nschedule: \"99 99 * * *\"\n", wantErr: "invalid schedule"},
		{name: "bad size", yaml: "repo: /r\nsandbox:\n  min_free_space: lots\n", wantErr: "min_free_space"},
		{name: "bad password env name", yaml: "repo: /r\npassword_env: \"1BAD-NAME\"\n", wantErr: "password_env"},
		{name: "missing recipe file", yaml: "repo: /r\nrecipes: [nope.yaml]\n", wantErr: "recipe file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.yaml)
			if tt.recipe {
				recipePath := filepath.Join(filepath.Dir(path), "recipe.yaml")
				if err := os.WriteFile(recipePath, []byte("name: x\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := Load(path)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() succeeded, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error %q does not contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		in      string
		want    uint64
		wantErr bool
	}{
		{in: "1024", want: 1024},
		{in: "500MB", want: 500e6},
		{in: "2GiB", want: 2 << 30},
		{in: "1.5 GB", want: 15e8},
		{in: "10kib", want: 10 << 10},
		{in: "0", want: 0},
		{in: "lots", wantErr: true},
		{in: "-5MB", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseSize(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseSize(%q) = %d, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSize(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseSize(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
