package cmd

import (
	"context"
	"errors"

	"github.com/dabelle/restorable/agent/internal/report"
)

// maybeReport submits the result to the control plane when credentials exist.
// Reporting is additive: any failure is logged and never changes the local
// outcome or exit code — the core loop must work without the cloud.
func maybeReport(ctx context.Context, credsPath string, noReport bool,
	res *report.RunResult, logf func(format string, args ...any)) {
	if noReport {
		return
	}
	path := credsPath
	if path == "" {
		var err error
		path, err = report.DefaultCredentialsPath()
		if err != nil {
			return
		}
	}
	creds, err := report.LoadCredentials(path)
	if errors.Is(err, report.ErrNoCredentials) {
		return // standalone mode
	}
	if err != nil {
		logf("cloud reporting skipped: %v", err)
		return
	}
	if err := report.NewClient(creds).SubmitRun(ctx, res); err != nil {
		logf("cloud reporting failed (local result unaffected): %v", err)
		return
	}
	logf("result reported to %s", creds.URL)
}
