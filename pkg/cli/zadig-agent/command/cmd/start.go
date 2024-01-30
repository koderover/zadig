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
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/command/cmd/agent"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/command/cmd/start"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/register"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("server-url", "s", "", "zadig-server url")
	startCmd.Flags().StringP("token", "t", "", "agent token")
	startCmd.Flags().StringP("work-dir", "d", "", "agent work directory")
}

var ExistCh chan struct{} = make(chan struct{})

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Zadig-agent",
	Long:  `Start Zadig-agent, if you use sudo zadig.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := cmd.Flags().GetString("work-dir")
		if err != nil {
			return fmt.Errorf("failed to get work-dir: %v", err)
		}

		serverUrl, err := cmd.Flags().GetString("server-url")
		if err != nil {
			return fmt.Errorf("failed to get server-url: %v", err)
		}

		token, err := cmd.Flags().GetString("token")
		if err != nil {
			return fmt.Errorf("failed to get token: %v", err)
		}

		if err := startPreRun(serverUrl, token, workDir); err != nil {
			fmt.Printf("Start pre run failed: %v.", err)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		startRun()
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := startPostRun(); err != nil {
			fmt.Printf("Start post run failed: %v.", err)
		}
	},
}

func startPreRun(serverURL, token, workDir string) error {
	// remove some running garbage
	resetAgentRunningFile()

	// init Viper
	// automatically load environment variables
	viper.AutomaticEnv()

	if !agentconfig.InitConfig() {
		log.Infof("agent config file not found, start to register agent to zadig-server.")

		if err := start.AskInstaller(&serverURL, &token, &workDir); err != nil {
			return fmt.Errorf("failed to ask installer: %v", err)
		}

		viper.Set(common.AGENT_INSTALL_TIME, time.Now().Unix())
		viper.Set(common.AGENT_VERSION, agentconfig.BuildAgentVersion)
		user, err := osutil.GetOSCurrentUser()
		if err != nil {
			return err
		}
		viper.Set(common.AGENT_INSTALL_USER, user)

		register.RegisterAgent()
		return nil
	}

	if serverURL != "" {
		agentconfig.SetServerURL(serverURL)
	}

	if token != "" {
		agentconfig.SetAgentToken(token)
	}

	if workDir != "" {
		agentconfig.SetAgentWorkDirectory(workDir)
	}

	log.Infof("find the agent config file in local disk")
	return nil
}

func startRun() {
	agent.StartAgent()
}

func startPostRun() error {
	return nil
}

func resetAgentRunningFile() {
	stopFilePath, err := agentconfig.GetStopFilePath()
	if err != nil {
		log.Errorf("failed to get stop file path: %v", err)
	}
	if _, err := os.Stat(stopFilePath); err == nil {
		err = os.Remove(stopFilePath)
		if err != nil {
			log.Errorf("failed to remove stop file: %v", err)
		}
	}
}
