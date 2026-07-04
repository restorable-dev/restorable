package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Set at build time via -ldflags. Defaults describe a from-source build.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the agent version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "restorable %s (commit %s, built %s, %s/%s)\n",
				version, commit, date, runtime.GOOS, runtime.GOARCH)
			return err
		},
	}
}
