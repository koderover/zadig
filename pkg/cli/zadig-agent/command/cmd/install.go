package cmd

import (
	"context"
	"fmt"
	"os"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	"github.com/spf13/cobra"
)

func init() {
	install := &cobra.Command{Use: "install", Args: cobra.NoArgs, Short: "Register the agent, configure boot startup and start it", RunE: func(cmd *cobra.Command, args []string) error {
		lease, err := updater.AcquireInstance()
		if err != nil {
			return fmt.Errorf("stop the existing agent before installing: %w", err)
		}
		defer lease.Close()
		if err := updater.CheckStartup(); err != nil {
			return err
		}
		server, _ := cmd.Flags().GetString("server-url")
		token, _ := cmd.Flags().GetString("token")
		workdir, _ := cmd.Flags().GetString("work-dir")
		if err := startPreRun(server, token, workdir); err != nil {
			return err
		}
		if err := agentconfig.SetConnectionAllowed(true); err != nil {
			return err
		}
		auto, _ := cmd.Flags().GetBool("auto-start")
		account, _ := cmd.Flags().GetString("service-user")
		options, err := serviceInstallOptions(cmd, account, auto)
		if err != nil {
			return err
		}
		var settings *daemon.Settings
		if auto {
			settings, err = installAgentService(cmd, options)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Automatic boot startup could not be configured: %v. Starting ordinary agent execution; run start again after reboot.\n", err)
			}
		} else if saved, loadErr := daemon.Load(); loadErr == nil {
			if err := serviceOperation(cmd, "disable", func(context.Context) error { return daemon.SetAutoStart(false) }); err != nil {
				return err
			}
			settings = saved
		} else if !os.IsNotExist(loadErr) {
			return loadErr
		}
		lease.Close()
		if settings == nil || settings.Backend == daemon.Cron {
			if err := daemon.StartOrdinary(); err != nil {
				return err
			}
		} else {
			if err := serviceOperation(cmd, "start", daemon.Start); err != nil {
				return err
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Agent launch requested. Check the host status in Zadig and use service status to inspect boot startup.")
		return nil
	}}
	install.Flags().String("server-url", "", "Zadig server URL")
	install.Flags().String("token", "", "Agent registration token")
	install.Flags().String("work-dir", "", "Agent working directory")
	install.Flags().Bool("auto-start", true, "Enable boot startup; unavailable managers fall back to ordinary execution")
	install.Flags().String("upgrade-public-key", "", "Base64 Ed25519 publisher public key for the restricted Unix upgrade helper")
	install.Flags().String("service-user", "", "Service account (defaults to current account)")
	install.Flags().Bool("password-stdin", false, "Read the Windows service account password from stdin")
	rootCmd.AddCommand(install)
}
