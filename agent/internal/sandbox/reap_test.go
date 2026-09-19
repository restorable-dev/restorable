package sandbox

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Destroy is deferred and survives panics and signals, but nothing survives
// SIGKILL or an OOM kill. Those leave the user's restored data on disk with no
// owner, and before this nothing ever reclaimed it.
func TestReapOrphans(t *testing.T) {
	base := t.TempDir()

	mk := func(name string, age time.Duration) string {
		p := filepath.Join(base, name)
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
		// Sandboxes hold restored data, so put something in it.
		if err := os.WriteFile(filepath.Join(p, "restored.db"), []byte("customer data"), 0o600); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-age)
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
		return p
	}

	abandoned := mk("restorable-abandoned", 48*time.Hour)
	live := mk("restorable-live", time.Minute)          // a concurrent run
	borderline := mk("restorable-recent", 23*time.Hour) // inside the window
	foreign := mk("someone-elses-tempdir", 48*time.Hour)

	reapOrphans(base)

	for _, tc := range []struct {
		path     string
		wantGone bool
		why      string
	}{
		{abandoned, true, "old sandbox holding restored data must be reclaimed"},
		{live, false, "a concurrent run's sandbox must never be deleted"},
		{borderline, false, "inside the age window, still assume an owner"},
		{foreign, false, "only directories this package creates may be removed"},
	} {
		_, err := os.Stat(tc.path)
		gone := os.IsNotExist(err)
		if gone != tc.wantGone {
			t.Errorf("%s: gone=%v, want %v (%s)", filepath.Base(tc.path), gone, tc.wantGone, tc.why)
		}
	}
}

// Reaping is housekeeping. It must never be the reason a run cannot start.
func TestReapOrphansToleratesUnreadableBase(t *testing.T) {
	reapOrphans(filepath.Join(t.TempDir(), "does-not-exist"))
}

// The reaper runs inside New, so a live sandbox created immediately after an
// abandoned one is reclaimed must still be usable.
func TestNewStillWorksAfterReaping(t *testing.T) {
	base := t.TempDir()
	old := filepath.Join(base, "restorable-old")
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, when, when); err != nil {
		t.Fatal(err)
	}

	sb, err := New(base, 0)
	if err != nil {
		t.Fatalf("New after reaping: %v", err)
	}
	defer sb.Destroy() //nolint:errcheck // test cleanup

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("abandoned sandbox survived New")
	}
	if _, err := os.Stat(sb.Dir()); err != nil {
		t.Errorf("new sandbox unusable: %v", err)
	}
}
