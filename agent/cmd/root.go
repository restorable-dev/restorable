// Package cmd contains the cobra commands for the restorable agent CLI.
package cmd

import (
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "restorable",
		Short:         "Restorable proves your backups would actually restore",
		Long:          "Restorable restores your latest backup snapshot into a disposable sandbox,\nruns verification recipes against it, and reports whether the restore worked.",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(newVersionCmd())
	return root
}

// Execute runs the root command. It returns the error instead of exiting so
// main owns the process exit code.
func Execute() error {
	return newRootCmd().Execute()
}
