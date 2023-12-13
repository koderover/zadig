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

package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/network"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
	"github.com/koderover/zadig/v2/pkg/util"
)

var HeartbeatMonitor *HeartbeatService
var once sync.Once

type HeartbeatService struct {
	minInterval   int
	Interval      int
	MaxInterval   int
	FailedTime    int
	StopAgentChan chan struct{}
	AgentCtl      *agent.AgentController
}

func NewHeartbeatService(agentCtl *agent.AgentController, interval, maxInterval int, stop chan struct{}) *HeartbeatService {
	once.Do(func() {
		HeartbeatMonitor = &HeartbeatService{
			AgentCtl:      agentCtl,
			Interval:      interval,
			MaxInterval:   maxInterval,
			StopAgentChan: stop,
		}
	})
	return HeartbeatMonitor
}

func (h *HeartbeatService) Start(ctx context.Context) {
	h.minInterval = h.Interval
	ticker := time.NewTicker(time.Duration(h.Interval) * time.Second) // 3秒检查一次心跳
	defer ticker.Stop()

	errChan := make(chan error, 1)
	successChan := make(chan struct{}, 1)

	log.Infof("start to heartbeat")
	for {
		select {
		case <-ticker.C:
			if h.FailedTime > 3 {
				// TODO: how to deal with this situation
				h.Interval *= 2
				if h.Interval > h.MaxInterval {
					h.Interval = h.MaxInterval
				}
				agentconfig.SetAgentStatus(common.AGENT_STATUS_ABNORMAL)
			}
			// execute heartbeat detection logic
			util.Go(func() {
				Heartbeat(h.AgentCtl, errChan, successChan, h.StopAgentChan)
			})
		case err := <-errChan:
			log.Errorf("failed to ping zadig server, err: %v", err)
			h.FailedTime++
			ticker.Reset(time.Duration(h.Interval) * time.Second)
		case _ = <-successChan:
			h.FailedTime = 0
			h.Interval = h.minInterval
			ticker.Reset(time.Duration(h.Interval) * time.Second)

			// update agent status
			if agentconfig.GetAgentStatus() != common.AGENT_STATUS_RUNNING {
				agentconfig.SetAgentStatus(common.AGENT_STATUS_RUNNING)
			}
		case <-h.StopAgentChan:
			log.Infof("stop heartbeat testing with the zadig service")
			// stop heatbeat with zadig
			return
		case <-ctx.Done():
			log.Infof("stop heartbeat testing with the zadig service, received context cancel signal.")
			return
		}
	}

}

func Heartbeat(agentCtl *agent.AgentController, errChan chan error, successChan chan struct{}, stopChan chan struct{}) {
	parameters, err := osutil.GetPlatformParameters()
	if err != nil {
		panic(fmt.Errorf("failed to get platform parameters: %v", err))
	}

	params := new(network.HeartbeatParameters)
	err = osutil.IToi(parameters, params)
	if err != nil {
		panic(fmt.Errorf("failed to convert platform parameters to register agent parameters: %v", err))
	}

	config := &network.AgentConfig{
		Token: agentconfig.GetAgentToken(),
		URL:   agentconfig.GetServerURL(),
	}

	resp, err := network.Heartbeat(config, params)
	if err != nil {
		errChan <- err
		return
	}
	if resp.NeedUpdateAgentVersion {
		err = updater.UpdateAgent(agentCtl, resp.AgentVersion)
		if err != nil {
			log.Errorf("failed to update agent: %v", err)
			errChan <- err
			return
		}
	}

	if resp.NeedOffline {
		log.Infof("agent is offline")
		stopChan <- struct{}{}
		close(stopChan)
	}

	agentConfig := new(agentconfig.AgentConfig)
	if resp.VmName != "" {
		agentConfig.VmName = resp.VmName
	}
	if resp.Description != "" {
		agentConfig.Description = resp.Description
	}
	if resp.ZadigVersion != "" {
		agentConfig.ZadigVersion = resp.ZadigVersion
	}
	if resp.ServerURL != "" {
		agentConfig.ServerURL = resp.ServerURL
	}
	if resp.WorkDir != "" {
		agentConfig.WorkDirectory = resp.WorkDir
	}
	agentConfig.ScheduleWorkflow = resp.ScheduleWorkflow

	if resp.Concurrency > 0 {
		agentConfig.Concurrency = resp.Concurrency
	}

	if resp.CacheType != "" {
		agentConfig.CacheType = resp.CacheType
	}

	err = agentconfig.BatchUpdateAgentConfig(agentConfig)
	if err != nil {
		log.Errorf("failed to update agent config: %v", err)
	}

	successChan <- struct{}{}
}
