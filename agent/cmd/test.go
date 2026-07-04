package cmd

import (
	"github.com/spf13/cobra"

	"github.com/dabelle/restorable/agent/internal/config"
	"github.com/dabelle/restorable/agent/internal/verify"
)

// exitError carries a specific process exit code without printing anything:
// the result has already been written by the time it is returned.
type exitError struct{ code int }

func (e *exitError) Error() string { return "" }

// ExitCode implements the interface main uses to map errors to exit codes.
func (e *exitError) ExitCode() int { return e.code }

func newTestCmd() *cobra.Command {
	var (
		cfgPath string
		jsonOut bool
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
			if jsonOut {
				if err := res.WriteJSON(cmd.OutOrStdout()); err != nil {
					return err
				}
			} else {
				cmd.Println(res.Human())
			}
			if code := res.ExitCode(); code != 0 {
				return &exitError{code: code}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "agent.yaml", "path to agent config file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the result as JSON on stdout")
	return cmd
}
