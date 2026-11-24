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

package register

import (
	"fmt"
	"os"
	"path/filepath"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/network"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

func RegisterAgent() error {
	agentConfig := agentconfig.NewAgentConfig()
	args := &network.RegisterAgentParameters{}

	parameters, err := osutil.GetPlatformParameters()
	if err != nil {
		return fmt.Errorf("failed to get platform parameters: %w", err)
	}
	err = osutil.IToi(parameters, args)
	if err != nil {
		return fmt.Errorf("failed to convert platform parameters: %w", err)
	}

	args.AgentVersion = agentconfig.BuildAgentVersion
	if agentConfig.Token == "" {
		return fmt.Errorf("agent token is empty")
	}
	if agentConfig.ServerURL == "" {
		return fmt.Errorf("zadig server URL is empty")
	}
	if agentConfig.WorkDirectory != "" {
		args.WorkDir = agentConfig.WorkDirectory
	}

	config := &network.AgentConfig{
		URL:   agentConfig.ServerURL,
		Token: agentConfig.Token,
	}

	if config.URL == "" || config.Token == "" {
		return fmt.Errorf("server URL and token are required")
	}
	// register agent to zadig-server
	resp, err := network.RegisterAgent(config, args)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}
	if resp.NeedOffline {
		if err := agentconfig.SetConnectionAllowed(false); err != nil {
			return err
		}
		return agentconfig.ErrDisconnected
	}
	log.Infof("agent registered successfully!!!")

	// write agent token and status to config file
	return afterRegister(resp, agentConfig)
}

func afterRegister(resp *network.RegisterAgentResponse, config *agentconfig.AgentConfig) error {
	log.Infof("agent to write agent config file to local disk.")
	// load all Environment variables
	config.Status = common.AGENT_STATUS_REGISTERED

	// 更新配置值
	config.Token = resp.Token
	config.Description = resp.Description
	config.VmName = resp.VmName
	config.Concurrency = resp.Concurrency
	config.ScheduleWorkflow = resp.ScheduleWorkflow
	config.ZadigVersion = resp.ZadigVersion

	if config.Concurrency == 0 {
		config.Concurrency = common.DefaultAgentConcurrency
	}
	if resp.WorkDir != "" {
		config.WorkDirectory = resp.WorkDir
	}

	config.ZadigVersion = resp.ZadigVersion
	config.AgentPlatform = osutil.GetOSName()
	config.AgentArchitecture = osutil.GetOSArch()

	path, err := agentconfig.GetAgentConfigFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := agentconfig.UpdateAgentConfigFile(config, path); err != nil {
		return err
	}
	agentconfig.SetAgentConfig(config)
	return nil
}
