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
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

func init() {
	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Print Zadig-agent information",
	Long:  `Print Zadig-agent information.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return infoPreRun()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := infoRun(); err != nil {
			fmt.Printf("Get agent information failed: %v.\n", err)
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := infoPostRun(); err != nil {
			log.Error(err)
		}
	},
}

func infoPreRun() error {
	return nil
}

func infoRun() error {
	configFilePath, err := agentconfig.GetAgentConfigFilePath()
	if err != nil {
		return fmt.Errorf("failed to get agent config file path: %v", err)
	}

	agentConfig, err := agentconfig.GetAgentConfig()
	if err != nil {
		return fmt.Errorf("failed to get agent config from local disk path:%s: error:%v.\nThis indicates that you need to follow the prompts in the integration module of the Zadig server", configFilePath, err)
	}

	osParams, err := osutil.GetPlatformParameters()
	if err != nil {
		return fmt.Errorf("failed to get platform parameters: %v", err)
	}

	agentLogFilePath, err := agentconfig.GetAgentLogFilePath()
	if err != nil {
		return fmt.Errorf("failed to get agent log file path: %v", err)
	}

	cacheDirectory, err := agentconfig.GetCacheDir(agentConfig.WorkDirectory)
	if err != nil {
		return fmt.Errorf("failed to get agent cache directory: %v", err)
	}

	// print vm name
	fmt.Printf("vm info              	%-60s\n", fmt.Sprintf("name=%s description=%s", agentConfig.VmName, agentConfig.Description))
	fmt.Printf("agent info           	%-60s\n", fmt.Sprintf("version=%s status=%s install_time=%s install_user=%s", agentConfig.AgentVersion, agentConfig.Status, time.Unix(agentConfig.InstallTime, 0).Format("2006-01-02 15:04:05"), agentConfig.InstallUser))
	fmt.Printf("zadig info           	%-60s\n", fmt.Sprintf("url=%s", agentConfig.ServerURL))
	fmt.Printf("platform info        	%-60s\n", fmt.Sprintf("platform=%s architecture=%s ip=%s", osParams.OS, osParams.Arch, osParams.IP))
	fmt.Printf("agent config file    	%-60s\n", fmt.Sprintf("path=%s", configFilePath))
	fmt.Printf("agent log file       	%-60s\n", fmt.Sprintf("path=%s", filepath.Join(agentLogFilePath, "zadig-agent.log")))
	fmt.Printf("agent cache directory	%-60s\n", fmt.Sprintf("path=%s", cacheDirectory))
	fmt.Printf("agent work directory 	%-60s\n", fmt.Sprintf("path=%s", agentConfig.WorkDirectory))

	return nil
}

func infoPostRun() error {
	return nil
}
