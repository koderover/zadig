package scheduler

import (
	"time"

	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/pkg/setting"
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
		go func(hostPm *service.PrivateKeyHosts, log *zap.SugaredLogger) {
			if hostPm.Port == 0 {
				hostPm.Port = setting.PMHostDefaultPort
			}
			newStatus := setting.PMHostStatusAbnormal
			msg, err := doTCPProbe(hostPm.IP, int(hostPm.Port), 3*time.Second, log)
			if err != nil {
				log.Errorf("doTCPProbe TCP %s:%d err: %s)", hostPm.IP, hostPm.Port, err)
			}
			if msg == Success {
				newStatus = setting.PMHostStatusNormal
			}

			if hostPm.Status == newStatus {
				return
			}
			hostPm.Status = newStatus

			err = c.AslanCli.UpdatePmHost(hostPm, log)
			if err != nil {
				log.Error(err)
			}
		}(hostElem, log)
	}
}
