package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/dabelle/restorable/agent/internal/report"
)

func newRegisterCmd() *cobra.Command {
	var (
		url       string
		token     string
		name      string
		credsPath string
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Connect this agent to the control plane",
		Long: "Exchanges a one-time registration token (from the dashboard) for an agent\n" +
			"API key, stored locally with owner-only permissions. Registration is\n" +
			"optional: `restorable test` works fully standalone without it.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if url == "" || token == "" {
				return errors.New("--url and --token are required (mint a token on the dashboard's Agents page)")
			}
			if name == "" {
				host, err := os.Hostname()
				if err != nil || host == "" {
					name = "agent"
				} else {
					name = host
				}
			}
			if credsPath == "" {
				var err error
				credsPath, err = report.DefaultCredentialsPath()
				if err != nil {
					return err
				}
			}

			creds, err := report.Register(cmd.Context(), url, token, name, version)
			if err != nil {
				return err
			}
			if err := report.SaveCredentials(credsPath, creds); err != nil {
				return err
			}
			cmd.Printf("Registered agent %q with %s\nCredentials saved to %s\n", name, creds.URL, credsPath)
			cmd.Println("Runs will now report to your dashboard. Use --no-report to skip for a single run.")
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "control plane base URL")
	cmd.Flags().StringVar(&token, "token", "", "one-time registration token (rrt_…)")
	cmd.Flags().StringVar(&name, "name", "", "agent name shown on the dashboard (default: hostname)")
	cmd.Flags().StringVar(&credsPath, "credentials", "", "where to store credentials (default: user config dir)")
	return cmd
}
