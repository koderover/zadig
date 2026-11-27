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

package agent

import (
	"errors"
	"fmt"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	errhelper "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/error"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/network"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/register"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

func InitAgent() error {
	// init agent config
	if !agentconfig.InitConfig() {
		return fmt.Errorf("failed to init agent config")
	}

	log.Infof("start to verify agent, this action is executed every time the agent starts.")
	err := verifyAgent()
	if err != nil {
		if errors.Is(err, errhelper.ErrVerrifyAgentFailedWithNoConfig) {
			log.Errorf("failed to verify agent: %s, please use zadig-agent --server-url=xxx --token=xxx to start agent, "+
				"if you not first time to start agent, the reason for this error may be that you started the agent in sudo instead of the previous non-sudo mode. "+
				"This behavior will affect the default configuration file path, which is stored under the $HOME/.zadig-agent/ directory. "+
				"Please use the zadig-agent --help command to view the specific usage.", err.Error())
			return err
		}
		if errors.Is(err, agentconfig.ErrDisconnected) {
			return err
		}
		log.Errorf("failed to verify agent: %s, attempt to register agent again.", err.Error())
		if err := register.RegisterAgent(); err != nil {
			return err
		}
	}

	// deal with old agent status
	if agentconfig.GetAgentStatus() != common.AGENT_STATUS_REGISTERED {
		agentconfig.SetAgentStatus(common.AGENT_STATUS_REGISTERED)
		agentconfig.SetAgentErrMsg("")
	}

	log.Infof("initializing agent config successfully!!!")

	log.Infof("agent version: %v, build commit: %v, build go version: %v, build time: %v", agentconfig.GetAgentVersion(), agentconfig.GetBuildCommit(), agentconfig.GetBuildGoVersion(), agentconfig.GetBuildTime())
	return nil
}

func verifyAgent() error {
	args := &network.VerifyAgentParameters{}

	parameters, err := osutil.GetPlatformParameters()
	if err != nil {
		return fmt.Errorf("failed to get platform parameters: %w", err)
	}
	err = osutil.IToi(parameters, args)
	if err != nil {
		return fmt.Errorf("failed to convert platform parameters: %w", err)
	}

	log.Infof("start to verify agent to zadig-server. url:%s", agentconfig.GetServerURL())
	config := &network.AgentConfig{
		Token: agentconfig.GetAgentToken(),
		URL:   agentconfig.GetServerURL(),
	}

	if config.Token == "" || config.URL == "" {
		return errhelper.ErrVerrifyAgentFailedWithNoConfig
	}

	resp, err := network.VerifyAgent(config, args)
	if err != nil {
		return err
	}
	if resp.NeedOffline {
		if err := agentconfig.SetConnectionAllowed(false); err != nil {
			return err
		}
		return agentconfig.ErrDisconnected
	}
	if !resp.Verified {
		return fmt.Errorf("failed to verify agent")
	}
	log.Infof("agent verified successfully!!!")
	return nil
}
