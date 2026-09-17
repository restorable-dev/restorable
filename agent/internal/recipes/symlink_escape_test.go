package recipes

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// restic restores symlinks as symlinks, so a backup of /srv/app where `data`
// is a link to /var/lib/app restores a link pointing at the live host copy.
// os.Stat follows it, so before this was fixed the check read the running
// system and reported the backup healthy — a silent false PASS on exactly the
// question this tool exists to answer.
//
// Note t.TempDir() sits under /var/folders on macOS, and /var is itself a
// symlink to /private/var, so these cases also cover the sandbox living under
// a symlinked prefix. A containment check that resolved only one side would
// reject every legitimate path here.
func TestResolveSymlinkContainment(t *testing.T) {
	sandbox := t.TempDir()
	outside := t.TempDir()

	if err := os.WriteFile(filepath.Join(outside, "live.db"), []byte("LIVE HOST DATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sandbox, "real.db"), []byte("restored"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(sandbox, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sandbox, "sub", "inner.db"), []byte("restored"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The dangerous one: points out of the restored tree entirely.
	if err := os.Symlink(filepath.Join(outside, "live.db"), filepath.Join(sandbox, "escape.db")); err != nil {
		t.Fatal(err)
	}
	// The legitimate one: a relative link the backup itself contained.
	if err := os.Symlink("sub/inner.db", filepath.Join(sandbox, "inside.db")); err != nil {
		t.Fatal(err)
	}

	roots := []Root{{Dir: sandbox, SnapPath: "/srv/app"}}

	tests := []struct {
		name    string
		rel     string
		wantErr error
	}{
		{"plain restored file resolves", "real.db", nil},
		{"symlink within the snapshot resolves", "inside.db", nil},
		{"symlink out of the snapshot is refused", "escape.db", errPathEscapes},
		{"absent path is still just missing", "nope.db", errPathMissing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolve(roots, tt.rel)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("resolve(%q) err = %v, want %v", tt.rel, err, tt.wantErr)
			}
			if tt.wantErr == nil && got == "" {
				t.Fatalf("resolve(%q) returned no path", tt.rel)
			}
		})
	}
}

// The failure mode that matters is not the error value, it is that a check
// must never read the host copy. This asserts the data itself never surfaces.
func TestResolveNeverReachesHostData(t *testing.T) {
	sandbox := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "live.db")
	if err := os.WriteFile(secret, []byte("LIVE HOST DATA"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(sandbox, "data.db")); err != nil {
		t.Fatal(err)
	}

	got, err := resolve([]Root{{Dir: sandbox, SnapPath: "/srv/app"}}, "data.db")
	if err == nil {
		content, _ := os.ReadFile(got)
		t.Fatalf("resolved a path outside the sandbox (%s) reading %q", got, content)
	}
}
