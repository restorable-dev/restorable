package recipes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/restorable-dev/restorable/agent/internal/report"
)

// FilesCheck asserts that the restore produced the files it should have:
// required paths exist, directories hold at least min_files files, and a
// random sample of restored files hashes identically to repository content.
type FilesCheck struct {
	Type    string            `yaml:"type"`
	Require []FileRequirement `yaml:"require"`
	// ChecksumSample is how many restored files to verify byte-for-byte
	// against the repository via restic dump. 0 disables sampling.
	ChecksumSample int `yaml:"checksum_sample"`
}

// FileRequirement is one asserted path, relative to the snapshot root.
type FileRequirement struct {
	Path string `yaml:"path"`
	// MinFiles, when > 0, requires Path to be a directory containing at
	// least this many regular files (recursively).
	MinFiles int `yaml:"min_files"`
}

// TypeName implements Check.
func (f *FilesCheck) TypeName() string { return "files" }

func (f *FilesCheck) validate() error {
	if len(f.Require) == 0 && f.ChecksumSample == 0 {
		return errors.New("needs at least one require entry or checksum_sample > 0")
	}
	if f.ChecksumSample < 0 {
		return errors.New("checksum_sample cannot be negative")
	}
	for _, req := range f.Require {
		if err := validateRelPath(req.Path); err != nil {
			return err
		}
		if req.MinFiles < 0 {
			return fmt.Errorf("path %q: min_files cannot be negative", req.Path)
		}
	}
	return nil
}

// Run implements Check.
func (f *FilesCheck) Run(ctx context.Context, t *Target) (report.Status, string) {
	var issues []string
	errored := false

	for _, req := range f.Require {
		rel := strings.TrimSuffix(req.Path, "/")
		abs, ok := resolve(t.Roots, rel)
		if !ok {
			issues = append(issues, fmt.Sprintf("required path %q not found in restored snapshot", req.Path))
			continue
		}
		if req.MinFiles > 0 {
			n, err := countFiles(abs)
			if err != nil {
				issues = append(issues, fmt.Sprintf("count files under %q: %v", req.Path, err))
				errored = true
				continue
			}
			if n < req.MinFiles {
				issues = append(issues, fmt.Sprintf("%q holds %d files, need at least %d", req.Path, n, req.MinFiles))
			}
		}
	}

	sampled := 0
	if f.ChecksumSample > 0 {
		var err error
		sampled, err = f.verifySample(ctx, t, &issues)
		if err != nil {
			issues = append(issues, err.Error())
			errored = true
		}
	}

	if len(issues) > 0 {
		if errored {
			return report.StatusError, joinIssues(issues)
		}
		return report.StatusFail, joinIssues(issues)
	}
	msg := fmt.Sprintf("%d required path(s) present", len(f.Require))
	if sampled > 0 {
		msg += fmt.Sprintf(", %d sampled checksum(s) match repository", sampled)
	}
	return report.StatusPass, msg
}

// validateRelPath rejects any recipe path that isn't a plain path relative to
// the snapshot root. Recipes are shared/community artifacts, so this is a
// trust boundary: without it a recipe could read or mount arbitrary host
// files by escaping the sandbox with "../". Every check type that resolves a
// user-supplied path must call this in its validate().
func validateRelPath(p string) error {
	trimmed := strings.TrimSuffix(p, "/")
	switch {
	case trimmed == "":
		return errors.New("path is empty")
	case filepath.IsAbs(trimmed):
		return fmt.Errorf("path %q must be relative to the snapshot root", p)
	case trimmed == ".." || strings.HasPrefix(trimmed, "../") || strings.Contains(trimmed, "/../"):
		return fmt.Errorf("path %q must not escape the snapshot root", p)
	}
	return nil
}

// resolve finds rel under one of the restored roots, enforcing containment:
// even if a malformed path slips past validation, the resolved location must
// stay inside a root directory (defense in depth against traversal).
func resolve(roots []Root, rel string) (string, bool) {
	for _, root := range roots {
		abs := filepath.Join(root.Dir, rel)
		if !withinRoot(root.Dir, abs) {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			return abs, true
		}
	}
	return "", false
}

// withinRoot reports whether abs is inside dir (after cleaning), blocking
// "../" escapes that survived earlier validation.
func withinRoot(dir, abs string) bool {
	rel, err := filepath.Rel(dir, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// countFiles counts regular files under dir recursively. A file path counts
// as one file, so `path: file.txt, min_files: 1` also works.
func countFiles(dir string) (int, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 1, nil
	}
	n := 0
	err = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			n++
		}
		return nil
	})
	return n, err
}

// sampleFile is a restored regular file eligible for checksum verification.
type sampleFile struct {
	abs      string // path on disk in the sandbox
	snapPath string // absolute path inside the snapshot, for restic dump
}

// verifySample hashes a random sample of restored files and compares each
// against the same file streamed straight out of the repository. A mismatch
// means the backup does not contain what is on disk after restore — the
// exact corruption this product exists to catch.
func (f *FilesCheck) verifySample(ctx context.Context, t *Target, issues *[]string) (int, error) {
	if t.Dumper == nil {
		return 0, errors.New("checksum_sample requires repository access (no dumper configured)")
	}
	var files []sampleFile
	for _, root := range t.Roots {
		err := filepath.WalkDir(root.Dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root.Dir, p)
			if err != nil {
				return err
			}
			files = append(files, sampleFile{abs: p, snapPath: path.Join(root.SnapPath, rel)})
			return nil
		})
		if err != nil {
			return 0, fmt.Errorf("walk restored tree: %w", err)
		}
	}
	if len(files) == 0 {
		*issues = append(*issues, "checksum_sample: restored snapshot contains no regular files")
		return 0, nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // sampling, not crypto
	rng.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })
	n := f.ChecksumSample
	if n > len(files) {
		n = len(files)
	}

	for _, sf := range files[:n] {
		restoredSum, err := sha256File(sf.abs)
		if err != nil {
			return 0, fmt.Errorf("hash restored file: %w", err)
		}
		repoHash := sha256.New()
		if err := t.Dumper.Dump(ctx, t.SnapshotID, sf.snapPath, repoHash); err != nil {
			return 0, fmt.Errorf("read %q back from repository: %w", sf.snapPath, err)
		}
		repoSum := hex.EncodeToString(repoHash.Sum(nil))
		if restoredSum != repoSum {
			*issues = append(*issues, fmt.Sprintf("checksum mismatch: restored %q differs from repository content", sf.snapPath))
		}
	}
	return n, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only file
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
