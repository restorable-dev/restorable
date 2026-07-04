package sandbox

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAndDestroy(t *testing.T) {
	base := t.TempDir()
	sb, err := New(base, 1024)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(sb.Dir()), "restorable-") {
		t.Errorf("sandbox dir %q lacks restorable- prefix", sb.Dir())
	}
	if filepath.Dir(sb.Dir()) != base {
		t.Errorf("sandbox %q not under base %q", sb.Dir(), base)
	}

	// Put content in, destroy, verify gone.
	if err := os.WriteFile(filepath.Join(sb.Dir(), "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sb.Destroy(); err != nil {
		t.Fatalf("Destroy() error: %v", err)
	}
	if _, err := os.Stat(sb.Dir()); !os.IsNotExist(err) {
		t.Errorf("sandbox dir still exists after Destroy")
	}

	// Idempotent.
	if err := sb.Destroy(); err != nil {
		t.Errorf("second Destroy() error: %v", err)
	}
	// Nil-safe.
	var nilSb *Sandbox
	if err := nilSb.Destroy(); err != nil {
		t.Errorf("nil Destroy() error: %v", err)
	}
}

func TestNewInsufficientSpace(t *testing.T) {
	_, err := New(t.TempDir(), math.MaxUint64)
	if err == nil {
		t.Fatal("New() with MaxUint64 required bytes should fail")
	}
	if !strings.Contains(err.Error(), "insufficient disk space") {
		t.Errorf("error %q lacks clear message", err)
	}
	if !strings.Contains(err.Error(), "aborting before restore") {
		t.Errorf("error %q should state it aborted before restore", err)
	}
}

func TestNewDefaultBaseDir(t *testing.T) {
	sb, err := New("", 0)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer sb.Destroy() //nolint:errcheck // test cleanup
	if !strings.HasPrefix(sb.Dir(), os.TempDir()) {
		t.Errorf("sandbox %q not under os.TempDir %q", sb.Dir(), os.TempDir())
	}
}

func TestFreeBytes(t *testing.T) {
	free, err := freeBytes(t.TempDir())
	if err != nil {
		t.Fatalf("freeBytes() error: %v", err)
	}
	if free == 0 {
		t.Error("freeBytes() = 0 on a writable temp dir")
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{2 << 10, "2.0KiB"},
		{5 << 30, "5.0GiB"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.in); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
