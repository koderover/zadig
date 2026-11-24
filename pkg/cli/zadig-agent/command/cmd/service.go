package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	"github.com/spf13/cobra"
)

func serviceInstallOptions(cmd *cobra.Command, account string, auto bool) (daemon.Options, error) {
	options := daemon.Options{User: account, AutoStart: auto}
	readPassword, _ := cmd.Flags().GetBool("password-stdin")
	if readPassword {
		data, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 4097))
		if err != nil {
			return options, err
		}
		if len(data) > 4096 {
			return options, fmt.Errorf("service account password exceeds limit")
		}
		options.Password = strings.TrimRight(string(data), "\r\n")
	}
	return options, nil
}

func init() {
	service := &cobra.Command{Use: "service", Short: "Manage agent boot startup and system service"}
	rootCmd.AddCommand(service)
	install := &cobra.Command{Use: "install", Args: cobra.NoArgs, Short: "Install boot startup using the current agent configuration", RunE: func(cmd *cobra.Command, args []string) error {
		account, _ := cmd.Flags().GetString("user")
		auto, _ := cmd.Flags().GetBool("auto-start")
		options, err := serviceInstallOptions(cmd, account, auto)
		if err != nil {
			return err
		}
		s, err := daemon.Install(options)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Installed %s boot startup for %s (auto-start=%t). Use service start to connect now.\n", s.Backend, s.User, s.AutoStart)
		return nil
	}}
	install.Flags().String("user", "", "Service account; defaults to the current account")
	install.Flags().Bool("auto-start", true, "Enable startup on boot")
	install.Flags().Bool("password-stdin", false, "Read the Windows service account password from stdin; it is never saved")
	service.AddCommand(install)
	service.AddCommand(&cobra.Command{Use: "run", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := daemon.PrepareRuntime(); err != nil {
			return err
		}
		return daemon.Run(cmd.Context(), runBackgroundAgent)
	}})
	service.AddCommand(&cobra.Command{Use: "start", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemon.SetConnectionAllowed(true); err != nil {
			return err
		}
		s, err := daemon.Load()
		if err != nil {
			return err
		}
		if s.Backend == daemon.Cron {
			if _, err := daemon.PrepareRuntime(); err != nil {
				return err
			}
			return runBackgroundAgent(cmd.Context(), func() {})
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
		defer cancel()
		return daemon.Start(ctx)
	}})
	service.AddCommand(&cobra.Command{Use: "stop", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := stopRun(); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
		defer cancel()
		return daemon.Stop(ctx)
	}})
	service.AddCommand(&cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		status, err := daemon.Status(cmd.Context())
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
	}})
	service.AddCommand(&cobra.Command{Use: "uninstall", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := stopRun(); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
		defer cancel()
		return daemon.Uninstall(ctx)
	}})
	for _, entry := range []struct {
		name    string
		enabled bool
	}{{"enable", true}, {"disable", false}} {
		entry := entry
		service.AddCommand(&cobra.Command{Use: entry.name, Args: cobra.NoArgs, Short: "Change boot startup without changing the current connection", RunE: func(cmd *cobra.Command, args []string) error { return daemon.SetAutoStart(entry.enabled) }})
	}
}

func runBackgroundAgent(ctx context.Context, ready func()) error {
	allowed, err := daemon.ConnectionAllowed()
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}
	if err := os.Setenv(daemon.BackgroundEnv, "1"); err != nil {
		return err
	}
	lease, err := updater.AcquireInstance()
	if err != nil {
		return fmt.Errorf("another agent instance is running: %w", err)
	}
	defer lease.Close()
	if err := updater.CheckStartup(); err != nil {
		return err
	}
	if err := startPreRun("", "", ""); err != nil {
		return err
	}
	return startRun(ctx, ready)
}
