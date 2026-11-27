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
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/monitor"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
)

type Agent struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

func (a *Agent) start(stop chan struct{}) error {
	log.Infof("================================ Zadig Agent  ================================")

	log.Infof("OS PATH: %v", os.Getenv("PATH"))

	// Initialize the agent
	if err := InitAgent(); err != nil {
		return err
	}

	// Start the agent core service
	agentCtl := agent.NewAgentController()
	manager, err := updater.NewManager(agentCtl, stop)
	if err != nil {
		return fmt.Errorf("initialize agent updater: %w", err)
	}
	agentCtl.Start(a.Ctx)

	// Start the heartbeat service
	go monitor.NewHeartbeatService(agentCtl, manager, 3, 10, stop).Start(a.Ctx)
	return nil
}

func StartAgent(ctx context.Context, ready func()) error {
	ctx, stopSignals := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	agentCtx, cancel := context.WithCancel(ctx)
	zadigAgent := &Agent{Ctx: agentCtx, Cancel: cancel}
	defer zadigAgent.Cancel()
	stopAgentChan := make(chan struct{}, 1)
	if err := zadigAgent.start(stopAgentChan); err != nil {
		if errors.Is(err, config.ErrDisconnected) {
			return nil
		}
		return err
	}
	ready()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stopAgentChan:
			zadigAgent.Stop("agent stopped by platform or upgrade")
			return nil
		case <-ctx.Done():
			zadigAgent.Stop("agent stopped by signal or service manager")
			return nil
		case <-ticker.C:
			if checkStopSignalFile() {
				if err := config.SetConnectionAllowed(false); err != nil {
					return err
				}
				zadigAgent.Stop("agent stopped by user")
				return nil
			}
		}
	}
}

func (a *Agent) Stop(msg string) {
	// Stop the agent
	config.SetAgentStatus(common.AGENT_STATUS_STOP)
	config.SetAgentErrMsg(msg)

	// Cancel the context
	a.Cancel()
	// Wait for the agent core components to exit
	time.Sleep(5 * time.Second)
}

func checkStopSignalFile() bool {
	stopFilePath, err := config.GetStopFilePath()
	if err != nil {
		log.Errorf("failed to get stop file path: %v", err)
		return false
	}
	if _, err := os.Stat(stopFilePath); err == nil {
		err = os.Remove(stopFilePath)
		if err != nil {
			log.Errorf("failed to remove stop file: %v", err)
			return false
		}
		return true
	}
	return false
}
