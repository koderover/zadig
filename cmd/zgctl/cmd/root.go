/*
Copyright 2022 The KodeRover Authors.

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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

var zadigHost string
var zadigToken string
var homeDir string

func init() {
	rootCmd.PersistentFlags().StringVar(&zadigHost, "zadig-host", "", "host of zadig system")
	rootCmd.PersistentFlags().StringVar(&zadigToken, "zadig-token", "", "token to access zadig system")
	rootCmd.PersistentFlags().StringVar(&homeDir, "home-dir", "$HOME/.zadig", "home dir of plugin")

	log.Init(&log.Config{
		Level:       config.LogLevel(),
		SendToFile:  config.SendLogToFile(),
		Filename:    filepath.Join(os.ExpandEnv(homeDir), "log"),
		Development: config.Mode() != setting.ReleaseMode,
		MaxSize:     5,
	})
}

var rootCmd = &cobra.Command{
	Use:   "zgctl",
	Short: "zgctl is a cli used for IDE plugin.",
	RunE: func(cmd *cobra.Command, args []string) error {

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}
