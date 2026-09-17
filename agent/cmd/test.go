package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/restorable-dev/restorable/agent/internal/config"
	"github.com/restorable-dev/restorable/agent/internal/verify"
)

// exitError carries a specific process exit code without printing anything:
// the result has already been written by the time it is returned.
type exitError struct{ code int }

func (e *exitError) Error() string { return "" }

// ExitCode implements the interface main uses to map errors to exit codes.
func (e *exitError) ExitCode() int { return e.code }

func newTestCmd() *cobra.Command {
	var (
		cfgPath   string
		jsonOut   bool
		credsPath string
		noReport  bool
	)
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Restore the latest snapshot into a sandbox and verify it",
		Long: "Runs one full restore-verification cycle: disk-space pre-flight, restore of\n" +
			"the latest snapshot into a disposable sandbox, verification recipes, sandbox\n" +
			"cleanup. Exit codes: 0 = pass, 1 = verification failed, 2 = could not test.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			logf := func(format string, args ...any) {
				cmd.PrintErrf(format+"\n", args...)
			}
			res := verify.Run(cmd.Context(), verify.Options{
				Config:       cfg,
				AgentVersion: version,
				Logf:         logf,
			})
			maybeReport(cmd.Context(), credsPath, noReport, res, logf)
			if jsonOut {
				if err := res.WriteJSON(cmd.OutOrStdout()); err != nil {
					return err
				}
			} else {
				// The result goes to stdout, progress to stderr, so `restorable test
				// >> log` captures the verdict. cmd.Println writes to stderr.
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), res.Human()); err != nil {
					return err
				}
			}
			if code := res.ExitCode(); code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "agent.yaml", "path to agent config file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the result as JSON on stdout")
	cmd.Flags().StringVar(&credsPath, "credentials", "", "cloud credentials file (default: user config dir)")
	cmd.Flags().BoolVar(&noReport, "no-report", false, "skip reporting this run to the control plane")
	return cmd
}
