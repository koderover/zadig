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
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

var errAgentMismatch = errors.New("agent platform needs registration")

func InitAgent() error {
	// init agent config
	if !agentconfig.InitConfig() {
		return fmt.Errorf("failed to init agent config")
	}

	log.Infof("start to verify agent, this action is executed every time the agent starts.")
	err := verifyAgent()
	if err != nil {
		if !errors.Is(err, errAgentMismatch) {
			return err
		}
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
		if err := daemon.SetConnectionAllowed(false); err != nil {
			return err
		}
		return daemon.ErrDisconnected
	}
	if !resp.Verified {
		return errAgentMismatch
	}
	log.Infof("agent verified successfully!!!")
	return nil
}
