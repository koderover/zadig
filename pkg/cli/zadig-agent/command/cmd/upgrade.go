package cmd

import (
	"encoding/json"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{Use: "upgrade-probe", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(updater.CurrentProbe())
	}})
	rootCmd.AddCommand(&cobra.Command{Use: "upgrade-apply", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return updater.Apply()
	}})
}
