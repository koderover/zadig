/*
Copyright 2023 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "zadig-agent",
	Short: "An workflow job executor for Zadig",
	Long:  `zadig-agent is a workflow job executor, it supports different architectures of multiple operating systems and mainly performs construction and common tasks in work tasks. Support for other tasks will continue to be added in the future.`,
}

func init() {
	rootCmd.PersistentFlags().String("home", "", "Agent home directory containing .zadig-agent")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		home, err := cmd.Flags().GetString("home")
		if err != nil {
			return err
		}
		if home != "" {
			if !filepath.IsAbs(home) {
				return fmt.Errorf("home must be an absolute path")
			}
			return os.Setenv(config.HomeEnv, filepath.Clean(home))
		}
		return nil
	}
}

// Execute executes the root command.
func Execute() error {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	return rootCmd.Execute()
}
