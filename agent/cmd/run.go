package cmd

import (
	"errors"
	"log"
	"os"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/restorable-dev/restorable/agent/internal/config"
	"github.com/restorable-dev/restorable/agent/internal/report"
	"github.com/restorable-dev/restorable/agent/internal/verify"
)

func newRunCmd() *cobra.Command {
	var (
		cfgPath   string
		jsonOut   bool
		credsPath string
		noReport  bool
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run as a daemon, testing restores on the configured schedule",
		Long: "Starts an internal cron scheduler and runs a full restore-verification cycle\n" +
			"on the schedule from the config file. Results go to stdout (human lines, or\n" +
			"one JSON document per run with --json); progress logs go to stderr.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if cfg.Schedule == "" {
				return errors.New("config has no schedule; `restorable run` needs one (or use `restorable test` with external cron)")
			}

			logger := log.New(os.Stderr, "restorable ", log.LstdFlags)
			ctx := cmd.Context()

			doRun := func() {
				logger.Printf("starting restore test")
				res := verify.Run(ctx, verify.Options{
					Config:       cfg,
					AgentVersion: version,
					Logf:         logger.Printf,
				})
				maybeReport(ctx, credsPath, noReport, res, logger.Printf)
				if jsonOut {
					if err := res.WriteJSON(cmd.OutOrStdout()); err != nil {
						logger.Printf("write result: %v", err)
					}
				} else {
					cmd.Println(res.Human())
				}
				logger.Printf("restore test finished: %s", res.Status)
			}

			c := cron.New(cron.WithChain(
				cron.Recover(cron.PrintfLogger(logger)),
				cron.SkipIfStillRunning(cron.PrintfLogger(logger)),
			))
			id, err := c.AddFunc(cfg.Schedule, doRun)
			if err != nil {
				return err
			}
			c.Start()
			logger.Printf("scheduler started (schedule %q), next run %s", cfg.Schedule, c.Entry(id).Next)

			if creds := loadCredentialsQuiet(credsPath); creds != nil && !noReport {
				logger.Printf("registered with %s, heartbeating every %s", creds.URL, heartbeatInterval)
				go heartbeatLoop(ctx, report.NewClient(creds), logger.Printf)
			}

			<-ctx.Done()
			logger.Printf("shutting down, waiting for any running test to finish")
			<-c.Stop().Done()
			return nil
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "agent.yaml", "path to agent config file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit each result as JSON on stdout")
	cmd.Flags().StringVar(&credsPath, "credentials", "", "cloud credentials file (default: user config dir)")
	cmd.Flags().BoolVar(&noReport, "no-report", false, "never report runs to the control plane")
	return cmd
}
