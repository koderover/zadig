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

package updater

import (
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent"
)

func UpdateAgent(agentCtl *agent.AgentController, version string) error {
	// stop agent polling job from zadig
	agentCtl.StopPollingJob()

	for {
		// check agent current job number
		if agentCtl.CurrentJobNum == 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// stop agent run job
	agentCtl.StopRunJob()

	// update agent
	// 分四个步骤：1. 配置文件更新 2. 二进制文件更新 3. 启动新的agent 4. 删除旧的agent
	// 1. 配置文件更新
	err := UpdateAgentConfig(version)
	if err != nil {
		log.Errorf("failed to update agent config: %v", err)
		return err
	}

	// 2. 二进制文件更新

	return nil
}

func UpdateAgentConfig(newVersion string) error {
	// get old agent config
	oldConfig, err := config.GetAgentConfig()
	if err != nil {
		return fmt.Errorf("failed to get agent config: %v", err)
	}
	// update agent config
	oldVersion := oldConfig.AgentVersion
	log.Infof("update agent config from version %s to %s", oldVersion, newVersion)
	oldConfig.AgentVersion = newVersion

	// save new agent config
	path, err := config.GetAgentConfigFilePathWithCheck()
	if err != nil {
		return fmt.Errorf("failed to get agent config file path: %v", err)
	}

	err = config.UpdateAgentConfigFile(oldConfig, path)
	if err != nil {
		return fmt.Errorf("failed to update agent config file: %v", err)
	}
	return nil
}
