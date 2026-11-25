package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	"github.com/spf13/cobra"
)

func serviceInstallOptions(cmd *cobra.Command, account string, auto bool) (daemon.Options, error) {
	key, _ := cmd.Flags().GetString("upgrade-public-key")
	options := daemon.Options{User: account, AutoStart: auto, UpgradePublicKey: key}
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
		s, err := installAgentService(cmd, options)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Installed %s boot startup for %s (auto-start=%t). Use service start to connect now.\n", s.Backend, s.User, s.AutoStart)
		return nil
	}}
	install.Flags().String("upgrade-public-key", "", "Base64 Ed25519 publisher public key; installs the restricted Unix upgrade helper")
	install.Flags().String("user", "", "Service account; defaults to the current account")
	install.Flags().Bool("auto-start", true, "Enable startup on boot")
	install.Flags().Bool("password-stdin", false, "Read the Windows service account password from stdin; it is never saved")
	service.AddCommand(install)
	run := &cobra.Command{Use: "run", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		path, err := cmd.Flags().GetString("settings")
		if err != nil {
			return err
		}
		if _, err := daemon.PrepareRuntime(path); err != nil {
			return err
		}
		return daemon.Run(cmd.Context(), runBackgroundAgent)
	}}
	run.Flags().String("settings", "", "Registered service settings file")
	service.AddCommand(run)
	service.AddCommand(&cobra.Command{Use: "start", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := agentconfig.SetConnectionAllowed(true); err != nil {
			return err
		}
		s, err := daemon.Load()
		if err != nil {
			return err
		}
		if s.Backend == daemon.Cron {
			if _, err := daemon.PrepareRuntime(""); err != nil {
				return err
			}
			return runBackgroundAgent(cmd.Context(), func() {})
		}
		return serviceOperation(cmd, "start", daemon.Start)
	}})
	service.AddCommand(&cobra.Command{Use: "stop", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if err := stopRun(); err != nil {
			return err
		}
		return serviceOperation(cmd, "stop", daemon.Stop)
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
		return serviceOperation(cmd, "uninstall", daemon.Uninstall)
	}})
	for _, entry := range []struct {
		name    string
		enabled bool
	}{{"enable", true}, {"disable", false}} {
		entry := entry
		service.AddCommand(&cobra.Command{Use: entry.name, Args: cobra.NoArgs, Short: "Change boot startup without changing the current connection", RunE: func(cmd *cobra.Command, args []string) error {
			return serviceOperation(cmd, entry.name, func(context.Context) error { return daemon.SetAutoStart(entry.enabled) })
		}})
	}
}

func runBackgroundAgent(ctx context.Context, ready func()) error {
	allowed, err := agentconfig.ConnectionAllowed()
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}
	if err := os.Setenv(agentconfig.BackgroundEnv, "1"); err != nil {
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
		if errors.Is(err, agentconfig.ErrDisconnected) {
			return nil
		}
		return err
	}
	return startRun(ctx, ready)
}

// Elevate service management only; registration and ordinary execution retain
// the original account and HOME. The public CLI has no home override.
func installAgentService(cmd *cobra.Command, options daemon.Options) (*daemon.Settings, error) {
	settings, err := daemon.Install(options)
	if !errors.Is(err, daemon.ErrPermission) || runtime.GOOS == "windows" {
		return settings, err
	}
	account := options.User
	if account == "" {
		current, err := user.Current()
		if err != nil {
			return nil, err
		}
		account = current.Username
	}
	arguments := []string{"install", "--user", account, fmt.Sprintf("--auto-start=%t", options.AutoStart)}
	if options.UpgradePublicKey != "" {
		arguments = append(arguments, "--upgrade-public-key", options.UpgradePublicKey)
	}
	if err := runElevatedService(cmd, arguments...); err != nil {
		return nil, err
	}
	return daemon.Load()
}

func serviceOperation(cmd *cobra.Command, action string, operation func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
	defer cancel()
	err := operation(ctx)
	if errors.Is(err, daemon.ErrPermission) && runtime.GOOS != "windows" {
		return runElevatedService(cmd, action)
	}
	return err
}

func runElevatedService(cmd *cobra.Command, args ...string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
	defer cancel()
	arguments := append([]string{"-E", "env", "HOME=" + home, executable, "service"}, args...)
	process := exec.CommandContext(ctx, "sudo", arguments...)
	process.Stdin, process.Stdout, process.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	if err := process.Run(); err != nil {
		return fmt.Errorf("manage agent service with elevated privileges: %w", err)
	}
	return nil
}
