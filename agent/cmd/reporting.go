package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/dabelle/restorable/agent/internal/report"
)

// heartbeatInterval keeps registered daemon agents visibly alive between
// scheduled tests, so agent-silent detection (>24h) never false-positives on
// a healthy daemon with a weekly schedule.
const heartbeatInterval = 6 * time.Hour

// loadCredentialsQuiet returns credentials when the agent is registered, nil
// otherwise (standalone mode or unreadable file).
func loadCredentialsQuiet(credsPath string) *report.Credentials {
	path := credsPath
	if path == "" {
		var err error
		path, err = report.DefaultCredentialsPath()
		if err != nil {
			return nil
		}
	}
	creds, err := report.LoadCredentials(path)
	if err != nil {
		return nil
	}
	return creds
}

// heartbeatLoop pings the control plane immediately and then every
// heartbeatInterval until ctx is cancelled.
func heartbeatLoop(ctx context.Context, client *report.Client, logf func(format string, args ...any)) {
	beat := func() {
		hbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := client.Heartbeat(hbCtx); err != nil {
			logf("heartbeat failed: %v", err)
		}
	}
	beat()
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			beat()
		}
	}
}

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
