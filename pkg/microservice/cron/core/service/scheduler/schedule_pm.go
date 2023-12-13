/*
Copyright 2022 The KodeRover Authors.

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

package scheduler

import (
	"errors"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.uber.org/zap"
)

func (c *CronClient) UpdatePmHostStatusScheduler(log *zap.SugaredLogger) {
	hosts, err := c.AslanCli.ListPmHosts(log)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("start init pm host status scheduler..")
	for _, hostElem := range hosts {
		if hostElem.Type == setting.NewVMType && hostElem.Agent != nil {
			if hostElem.Status != setting.VMNormal {
				continue
			}

			var newStatus setting.PMHostStatus
			if time.Unix(hostElem.Agent.LastHeartbeatTime, 0).Add(time.Duration(setting.AgentDefaultHeartbeatTimeout) * time.Second).Before(time.Now()) {
				newStatus = setting.VMAbnormal

				if hostElem.Status != newStatus {
					hostElem.Error = "agent heartbeat timeout"
					hostElem.Status = newStatus
					err = c.AslanCli.UpdatePmHost(hostElem, log)
					if err != nil {
						log.Error(err)
					}
				}
			}
			continue
		}
		go func(hostPm *service.PrivateKeyHosts, log *zap.SugaredLogger) {
			if hostPm.Port == 0 {
				hostPm.Port = setting.PMHostDefaultPort
			}
			newStatus := setting.PMHostStatusAbnormal
			if hostPm.Probe == nil {
				hostPm.Probe = &types.Probe{ProbeScheme: setting.ProtocolTCP}
			}
			var err error
			msg := ""

			switch hostPm.Probe.ProbeScheme {
			case setting.ProtocolHTTP, setting.ProtocolHTTPS:
				if hostPm.Probe.HttpProbe == nil {
					break
				}
				msg, err = doHTTPProbe(string(hostPm.Probe.ProbeScheme), hostPm.IP, hostPm.Probe.HttpProbe.Path, hostPm.Probe.HttpProbe.Port, hostPm.Probe.HttpProbe.HTTPHeaders, time.Duration(hostPm.Probe.HttpProbe.TimeOutSecond)*time.Second, hostPm.Probe.HttpProbe.ResponseSuccessFlag, log)
				if err != nil {
					log.Warnf("doHttpProbe err:%s", err)
				}
			case setting.ProtocolTCP:
				msg, err = doTCPProbe(hostPm.IP, int(hostPm.Port), 3*time.Second, log)
				if err != nil {
					log.Warnf("doTCPProbe TCP %s:%d err: %s)", hostPm.IP, hostPm.Port, err)
				}
			}

			if msg == Success {
				newStatus = setting.PMHostStatusNormal
			}

			if hostPm.Status == newStatus {
				return
			}
			hostPm.Status = newStatus
			hostPm.Error = errors.New("probe failed to connect to the host").Error()

			err = c.AslanCli.UpdatePmHost(hostPm, log)
			if err != nil {
				log.Error(err)
			}
		}(hostElem, log)
	}
}
