package recipes

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/restorable-dev/restorable/agent/internal/report"
)

// buildTree creates a fake restored tree and returns the matching Target.
// Layout mimics restic: root dir = sandbox + original snapshot path.
func buildTree(t *testing.T, files map[string]string) *Target {
	t.Helper()
	sandboxDir := t.TempDir()
	snapPath := "/srv/app"
	rootDir := filepath.Join(sandboxDir, snapPath)
	for rel, content := range files {
		abs := filepath.Join(rootDir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return &Target{
		Roots:      []Root{{Dir: rootDir, SnapPath: snapPath}},
		SnapshotID: "snap1",
		Dumper:     &treeDumper{files: files, snapPath: snapPath},
	}
}

// treeDumper serves file content by snapshot path, like restic dump would.
// corrupt lists snapshot-relative paths whose repo content should differ.
type treeDumper struct {
	files    map[string]string
	snapPath string
	corrupt  map[string]bool
}

func (d *treeDumper) Dump(_ context.Context, _, path string, w io.Writer) error {
	rel := strings.TrimPrefix(path, d.snapPath+"/")
	content, ok := d.files[rel]
	if !ok {
		return fmt.Errorf("path %s not in snapshot", path)
	}
	if d.corrupt[rel] {
		content += "-CORRUPTED"
	}
	_, err := io.WriteString(w, content)
	return err
}

func TestFilesCheckRun(t *testing.T) {
	baseFiles := map[string]string{
		"config/config.php": "<?php",
		"data/a.txt":        "aaa",
		"data/b.txt":        "bbb",
		"data/sub/c.txt":    "ccc",
	}

	tests := []struct {
		name       string
		check      FilesCheck
		corrupt    map[string]bool
		wantStatus report.Status
		wantMsg    string
	}{
		{
			name: "all requirements met",
			check: FilesCheck{Require: []FileRequirement{
				{Path: "config/config.php"},
				{Path: "data/", MinFiles: 3},
			}},
			wantStatus: report.StatusPass,
		},
		{
			name:       "missing required path",
			check:      FilesCheck{Require: []FileRequirement{{Path: "config/missing.php"}}},
			wantStatus: report.StatusFail,
			wantMsg:    "not found in restored snapshot",
		},
		{
			name:       "min_files not met",
			check:      FilesCheck{Require: []FileRequirement{{Path: "data/", MinFiles: 99}}},
			wantStatus: report.StatusFail,
			wantMsg:    "need at least 99",
		},
		{
			name:       "min_files on single file counts as one",
			check:      FilesCheck{Require: []FileRequirement{{Path: "config/config.php", MinFiles: 1}}},
			wantStatus: report.StatusPass,
		},
		{
			name:       "checksums match",
			check:      FilesCheck{ChecksumSample: 100},
			wantStatus: report.StatusPass,
			wantMsg:    "checksum(s) match",
		},
		{
			name:       "checksum mismatch detected",
			check:      FilesCheck{ChecksumSample: 100},
			corrupt:    map[string]bool{"data/a.txt": true},
			wantStatus: report.StatusFail,
			wantMsg:    "checksum mismatch",
		},
		{
			name: "multiple failures reported together",
			check: FilesCheck{Require: []FileRequirement{
				{Path: "nope1"},
				{Path: "nope2"},
			}},
			wantStatus: report.StatusFail,
			wantMsg:    "nope2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := buildTree(t, baseFiles)
			target.Dumper.(*treeDumper).corrupt = tt.corrupt
			status, msg := tt.check.Run(context.Background(), target)
			if status != tt.wantStatus {
				t.Fatalf("status = %s (msg %q), want %s", status, msg, tt.wantStatus)
			}
			if tt.wantMsg != "" && !strings.Contains(msg, tt.wantMsg) {
				t.Errorf("message %q does not contain %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestFilesCheckNoDumper(t *testing.T) {
	target := buildTree(t, map[string]string{"a.txt": "x"})
	target.Dumper = nil
	check := FilesCheck{ChecksumSample: 1}
	status, msg := check.Run(context.Background(), target)
	if status != report.StatusError {
		t.Fatalf("status = %s, want error", status)
	}
	if !strings.Contains(msg, "dumper") {
		t.Errorf("message %q should explain the missing dumper", msg)
	}
}

func TestRecipeRunAggregates(t *testing.T) {
	target := buildTree(t, map[string]string{"a.txt": "x"})
	rec := &Recipe{
		Name: "r",
		Checks: []Check{
			&FilesCheck{Require: []FileRequirement{{Path: "a.txt"}}},
			&FilesCheck{Require: []FileRequirement{{Path: "missing"}}},
		},
	}
	results := rec.Run(context.Background(), target)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (failing check must not stop the run)", len(results))
	}
	if results[0].Status != report.StatusPass || results[1].Status != report.StatusFail {
		t.Errorf("statuses = %s, %s", results[0].Status, results[1].Status)
	}
	for _, r := range results {
		if r.Recipe != "r" || r.Type != "files" {
			t.Errorf("result metadata = %+v", r)
		}
	}
}
