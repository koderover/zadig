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

	"github.com/spf13/cobra"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Zadig-agent",
	Long:  `Print the version number of Zadig-agent.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return versionPreRun()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := versionRun(); err != nil {
			log.Fatal(err)
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := versionPostRun(); err != nil {
			fmt.Println(err)
		}
	},
}

func versionPreRun() error {
	return nil
}

func versionRun() error {

	msg := fmt.Sprintf("zadig-agent version %s", config.BuildAgentVersion)

	platform := osutil.GetPlatform()
	if platform != "" {
		msg = fmt.Sprintf("%s %s", msg, platform)
	}

	fmt.Println(msg)
	fmt.Println()
	fmt.Printf("Build Time: %s\n", config.BuildTime)
	fmt.Printf("Build Commit: %s\n", config.BuildCommit)
	fmt.Printf("Build Agent Version: %s\n", config.BuildAgentVersion)
	fmt.Printf("Build Go Version: %s\n", config.BuildGoVersion)

	return nil
}

func versionPostRun() error {
	return nil
}
