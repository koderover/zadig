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
)

type Agent struct {
	Ctx             context.Context
	Cancel          context.CancelFunc
	SignalsStopChan chan struct{}
}

func newAgent() *Agent {
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		Ctx:             ctx,
		Cancel:          cancel,
		SignalsStopChan: make(chan struct{}, 1),
	}
}

func (a *Agent) start(stop chan struct{}) {
	log.Infof("================================ Zadig Agent  ================================")

	// Initialize the agent
	InitAgent()

	// Start the agent core service
	agentCtl := agent.NewAgentController()
	go agentCtl.Start(a.Ctx)

	// Start the heartbeat service
	go monitor.NewHeartbeatService(agentCtl, 3, 10, stop).Start(a.Ctx)
}

func (a *Agent) handleSignals() {
	defer close(a.SignalsStopChan)

	// Listen for system signals
	sigCh := make(chan os.Signal, 1)
	defer close(sigCh)

	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signals in another goroutine
	sig := <-sigCh
	log.Infof("Received signal: %v", sig)

	a.SignalsStopChan <- struct{}{}
}

func StartAgent() {
	zadigAgent := newAgent()
	stopAgentChan := make(chan struct{}, 1)

	// Start the agent
	go zadigAgent.start(stopAgentChan)

	// Handle system signals
	go zadigAgent.handleSignals()

	// Wait for the agent to exit
	for {
		select {
		case <-stopAgentChan:
			msg := fmt.Sprintf("agent stopped by stop agent channel, time: %s", time.Now().Format("2006-01-02 15:04:05"))
			zadigAgent.Stop(msg)
			time.Sleep(5 * time.Second)
			return
		case <-zadigAgent.SignalsStopChan:
			msg := fmt.Sprintf("agent stopped by signal, time: %s", time.Now().Format("2006-01-02 15:04:05"))
			zadigAgent.Stop(msg)
			return
		default:
			if checkStopSignalFile() {
				msg := fmt.Sprintf("agent stopped by stop signal file, time: %s", time.Now().Format("2006-01-02 15:04:05"))
				zadigAgent.Stop(msg)
				return
			}
			time.Sleep(1 * time.Second)
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
