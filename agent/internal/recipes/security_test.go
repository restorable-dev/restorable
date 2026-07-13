package recipes

import (
	"strings"
	"testing"
)

// Path traversal must be rejected by EVERY check type that resolves a
// user-supplied path — not just the files check (the gap the review found).
func TestPathTraversalRejectedEverywhere(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"files require", "name: x\nchecks:\n  - type: files\n    require:\n      - path: ../../etc/passwd\n"},
		{"postgres dump", "name: x\nchecks:\n  - type: postgres\n    dump: ../../../../etc/passwd\n"},
		{"mysql dump", "name: x\nchecks:\n  - type: mysql\n    dump: ../../secrets.sql\n"},
		{"sqlite path", "name: x\nchecks:\n  - type: sqlite\n    path: ../../../etc/passwd\n"},
		{"sqlite absolute", "name: x\nchecks:\n  - type: sqlite\n    path: /etc/passwd\n"},
		{"docker-app mount", "name: x\nchecks:\n  - type: docker-app\n    image: alpine\n    mount: {restored: ../../.., at: /data}\n    ready: {http: \"http://localhost/\"}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatalf("traversal path was accepted — sandbox escape possible")
			}
			if !strings.Contains(err.Error(), "escape") && !strings.Contains(err.Error(), "relative") {
				t.Errorf("error %q should explain the path rejection", err)
			}
		})
	}
}

// resolve must never return a path outside a root, even if a malformed value
// slips past validation (defense in depth).
func TestResolveContainment(t *testing.T) {
	roots := []Root{{Dir: t.TempDir(), SnapPath: "/srv"}}
	if _, ok := resolve(roots, "../../etc/passwd"); ok {
		t.Error("resolve returned a path escaping the root")
	}
}
