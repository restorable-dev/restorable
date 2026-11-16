package cmd

import "github.com/spf13/cobra"

// retiredFlagNotice is shown when someone still passes a flag that belonged
// to the hosted dashboard.
const retiredFlagNotice = "the hosted dashboard was retired in v0.2.0, so this flag does nothing; use notify: in agent.yaml for alerts"

// addRetiredCloudFlags keeps the hosted dashboard's flags accepted, so cron
// lines written for v0.1 keep working after the upgrade. They do nothing,
// are hidden from help, and print a deprecation notice when used.
func addRetiredCloudFlags(cmd *cobra.Command) {
	cmd.Flags().String("credentials", "", "")
	cmd.Flags().Bool("no-report", false, "")
	for _, name := range []string{"credentials", "no-report"} {
		_ = cmd.Flags().MarkDeprecated(name, retiredFlagNotice)
	}
}
