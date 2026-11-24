package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "wait-for-stop", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
		defer cancel()
		lease, err := updater.WaitForInstance(ctx)
		if err != nil {
			return fmt.Errorf("old agent has not stopped: %w", err)
		}
		return lease.Close()
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "upgrade-probe", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(updater.CurrentProbe())
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "upgrade-apply", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return updater.Apply()
	}})
}
