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
	"time"

	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/network"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

type HeartbeatService struct {
	minInterval   int
	Interval      int
	MaxInterval   int
	FailedTime    int
	StopAgentChan chan struct{}
	AgentCtl      *agent.AgentController
	Updater       *updater.Manager
}

func NewHeartbeatService(agentCtl *agent.AgentController, manager *updater.Manager, interval, maxInterval int, stop chan struct{}) *HeartbeatService {
	return &HeartbeatService{AgentCtl: agentCtl, Updater: manager, Interval: interval, MaxInterval: maxInterval, StopAgentChan: stop}
}

func (h *HeartbeatService) Start(ctx context.Context) {
	h.minInterval = h.Interval
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := h.heartbeat(ctx); err != nil {
				log.Errorf("failed to ping zadig server: %v", err)
				h.FailedTime++
				if h.FailedTime > 3 {
					h.Interval *= 2
					if h.Interval > h.MaxInterval {
						h.Interval = h.MaxInterval
					}
					agentconfig.SetAgentStatus(common.AGENT_STATUS_ABNORMAL)
				}
			} else {
				h.FailedTime = 0
				h.Interval = h.minInterval
				if agentconfig.GetAgentStatus() != common.AGENT_STATUS_RUNNING {
					agentconfig.SetAgentStatus(common.AGENT_STATUS_RUNNING)
				}
			}
			timer.Reset(time.Duration(h.Interval) * time.Second)
		}
	}
}

func (h *HeartbeatService) heartbeat(ctx context.Context) error {
	parameters, err := osutil.GetPlatformParameters()
	if err != nil {
		return fmt.Errorf("failed to get platform parameters: %v", err)
	}

	params := new(network.HeartbeatParameters)
	err = osutil.IToi(parameters, params)
	if err != nil {
		return fmt.Errorf("failed to convert platform parameters to heartbeat parameters: %v", err)
	}
	params.AgentVersion = agentconfig.GetAgentVersion()
	params.UpgradeSupported = updater.Supported()
	params.UpgradeReport, err = h.Updater.Report()
	if err != nil {
		return err
	}

	config := &network.AgentConfig{
		Token: agentconfig.GetAgentToken(),
		URL:   agentconfig.GetServerURL(),
	}

	resp, err := network.Heartbeat(config, params)
	if err != nil {
		return err
	}
	if resp.NeedOffline {
		if err := daemon.SetConnectionAllowed(false); err != nil {
			return err
		}
		log.Infof("agent is offline")
		select {
		case h.StopAgentChan <- struct{}{}:
		default:
		}
		return nil
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

	return h.Updater.Handle(ctx, resp)
}
