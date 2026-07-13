// Package verify orchestrates one restore-verification run: pre-flight
// checks, restore into a sandbox, recipe execution, and guaranteed sandbox
// cleanup. It never returns a Go error — every outcome, including
// infrastructure failures, lands in the report.RunResult so callers have one
// uniform thing to render and one exit-code mapping.
package verify

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/restorable-dev/restorable/agent/internal/config"
	"github.com/restorable-dev/restorable/agent/internal/recipes"
	"github.com/restorable-dev/restorable/agent/internal/report"
	"github.com/restorable-dev/restorable/agent/internal/restic"
	"github.com/restorable-dev/restorable/agent/internal/sandbox"
)

// sizeHeadroom is how much larger than the snapshot's recorded size the
// sandbox filesystem's free space must be, to absorb metadata overhead and
// concurrent disk use during restore.
const sizeHeadroom = 1.2

// defaultRequiredBytes is the pre-flight requirement when the snapshot has
// no size summary (repos written by restic < 0.17) and no min_free_space is
// configured.
const defaultRequiredBytes = 1 << 30 // 1 GiB

// Options configures a run.
type Options struct {
	Config       *config.Config
	AgentVersion string
	// Logf receives progress lines (safe to leave nil).
	Logf func(format string, args ...any)
}

// Run executes one full restore-verification cycle.
func Run(ctx context.Context, opts Options) *report.RunResult {
	cfg := opts.Config
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	res := &report.RunResult{
		AgentVersion: opts.AgentVersion,
		Repo:         report.Scrub(cfg.Repo),
		StartedAt:    time.Now(),
		Checks:       []report.CheckResult{},
	}
	fail := func(format string, args ...any) *report.RunResult {
		res.Status = report.StatusError
		res.Error = report.Scrub(fmt.Sprintf(format, args...))
		res.FinishedAt = time.Now()
		return res
	}

	// Parse recipes first: a typo in a recipe file should fail fast, before
	// any repository work.
	loaded := make([]*recipes.Recipe, 0, len(cfg.Recipes))
	for _, path := range cfg.Recipes {
		r, err := recipes.Load(path)
		if err != nil {
			return fail("load recipe: %v", err)
		}
		loaded = append(loaded, r)
	}

	runner, err := restic.NewRunner(cfg.Repo, cfg.PasswordEnv)
	if err != nil {
		return fail("%v", err)
	}

	// Fingerprint the repo by its canonical restic ID so the same repo is
	// never counted twice for being addressed two ways (relative vs absolute
	// path, trailing slash). Best-effort: falls back to the repo string.
	if id, idErr := runner.RepoID(ctx); idErr == nil && id != "" {
		res.RepoFingerprint = report.FingerprintID(id)
	}

	logf("looking up latest snapshot in %s", report.Scrub(cfg.Repo))
	snap, err := runner.LatestSnapshot(ctx)
	if err != nil {
		return fail("find latest snapshot: %v", err)
	}
	res.SnapshotID = snap.ID
	logf("latest snapshot %s from %s", snap.ShortID, snap.Time.Format(time.RFC3339))

	required := uint64(defaultRequiredBytes)
	if snap.Summary != nil && snap.Summary.TotalBytesProcessed > 0 {
		required = uint64(float64(snap.Summary.TotalBytesProcessed) * sizeHeadroom)
	}
	if min := cfg.MinFreeBytes(); min > required {
		required = min
	}

	sb, err := sandbox.New(cfg.Sandbox.Dir, required)
	if err != nil {
		return fail("%v", err)
	}
	// The one non-negotiable: the sandbox is destroyed on every path out of
	// this function, including panics in recipe code.
	defer func() {
		if derr := sb.Destroy(); derr != nil {
			res.Status = report.StatusError
			res.Error = joinNonEmpty(res.Error, report.Scrub(fmt.Sprintf("sandbox cleanup failed: %v", derr)))
			res.FinishedAt = time.Now()
		} else {
			logf("sandbox destroyed")
		}
	}()

	logf("restoring snapshot %s into sandbox", snap.ShortID)
	if err := runner.Restore(ctx, snap.ID, sb.Dir()); err != nil {
		return fail("restore failed: %v", err)
	}

	roots := make([]recipes.Root, 0, len(snap.Paths))
	for _, p := range snap.Paths {
		roots = append(roots, recipes.Root{
			Dir:      filepath.Join(sb.Dir(), p),
			SnapPath: p,
		})
	}

	// Docker is created lazily — only when a check asks for it — and every
	// container it makes is force-removed (with volumes) on all paths out.
	var docker *sandbox.Docker
	defer func() {
		if docker == nil {
			return
		}
		if cerr := docker.Cleanup(); cerr != nil {
			res.Status = report.StatusError
			res.Error = joinNonEmpty(res.Error, report.Scrub(fmt.Sprintf("container cleanup failed: %v", cerr)))
			res.FinishedAt = time.Now()
		} else {
			logf("containers cleaned up")
		}
	}()
	target := &recipes.Target{
		Roots:      roots,
		SnapshotID: snap.ID,
		Dumper:     runner,
		TempDir:    sb.Dir(), // scratch (e.g. decompressed dumps) on the space-checked volume
		Docker: func(ctx context.Context) (recipes.ContainerRunner, error) {
			if docker != nil {
				return docker, nil
			}
			logf("connecting to docker for container checks")
			d, err := sandbox.NewDocker(ctx)
			if err != nil {
				return nil, err
			}
			docker = d
			return d, nil
		},
	}

	for _, rec := range loaded {
		logf("running recipe %q", rec.Name)
		res.Checks = append(res.Checks, rec.Run(ctx, target)...)
	}

	res.Status = report.StatusPass
	for _, c := range res.Checks {
		switch c.Status {
		case report.StatusError:
			res.Status = report.StatusError
		case report.StatusFail:
			if res.Status == report.StatusPass {
				res.Status = report.StatusFail
			}
		}
	}
	res.FinishedAt = time.Now()
	return res
}

func joinNonEmpty(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}
