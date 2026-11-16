package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/restorable-dev/restorable/agent/internal/config"
	"github.com/restorable-dev/restorable/agent/internal/notify"
	"github.com/restorable-dev/restorable/agent/internal/verify"
)

func newRunCmd() *cobra.Command {
	var (
		cfgPath string
		jsonOut bool
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
			notifier := notify.New(cfg.Notify, logger.Printf)

			doRun := func() {
				logger.Printf("starting restore test")
				notifier.Start(ctx)
				res := verify.Run(ctx, verify.Options{
					Config:       cfg,
					AgentVersion: version,
					Logf:         logger.Printf,
				})
				if jsonOut {
					if err := res.WriteJSON(cmd.OutOrStdout()); err != nil {
						logger.Printf("write result: %v", err)
					}
				} else {
					// The result goes to stdout, progress to stderr, so `restorable test
					// >> log` captures the verdict. cmd.Println writes to stderr.
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), res.Human()); err != nil {
						logger.Printf("write result: %v", err)
					}
				}
				notifier.Finish(ctx, res)
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
			logger.Printf("notifications: %s", notifier.Describe())

			<-ctx.Done()
			logger.Printf("shutting down, waiting for any running test to finish")
			<-c.Stop().Done()
			return nil
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "agent.yaml", "path to agent config file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit each result as JSON on stdout")
	addRetiredCloudFlags(cmd)
	return cmd
}
