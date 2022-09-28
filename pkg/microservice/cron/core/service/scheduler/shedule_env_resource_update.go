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
	"fmt"

	"github.com/jasonlvhit/gocron"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/pkg/microservice/cron/core/service/client"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func buildEnvResourceCronKey(envResource *service.EnvResource) string {
	return fmt.Sprintf("env-resource:%s:%s-type:%s-name:%s", envResource.ProductName, envResource.EnvName, envResource.Type, envResource.Name)
}

func (c *CronClient) deleteEnvResourceScheduler(envResourceKey string) {
	log.Infof("deleting single env resource scheduler: %s", envResourceKey)
	if _, ok := c.SchedulerController[envResourceKey]; ok {
		c.SchedulerController[envResourceKey] <- true
		delete(c.SchedulerController, envResourceKey)
	}
	if _, ok := c.Schedulers[envResourceKey]; ok {
		c.Schedulers[envResourceKey].Clear()
		delete(c.Schedulers, envResourceKey)
	}
}

func (c *CronClient) UpsertEnvResourceSyncScheduler(log *zap.SugaredLogger) {
	envs, err := c.AslanCli.ListEnvs(log, &client.EvnListOption{DeployType: []string{setting.HelmDeployType, setting.K8SDeployType}})
	if err != nil {
		log.Errorf("failed to list envs for env resource sync: %s", err)
		return
	}

	log.Info("start init env resource sync scheduler.")

	lastScheduler := sets.NewString()
	for k := range c.lastEnvResourceSchedulerData {
		lastScheduler.Insert(k)
	}

	for _, env := range envs {
		envResources, err := c.AslanCli.ListEnvResources(env.ProductName, env.EnvName, log)
		if err != nil {
			log.Error(err)
			return
		}

		for _, envResource := range envResources {
			envResourceKey := buildEnvResourceCronKey(envResource)

			lastScheduler.Delete(envResourceKey)

			if lastEnvResConfig, ok := c.lastEnvResourceSchedulerData[envResourceKey]; ok {
				if envResource.CreateTime == lastEnvResConfig.CreateTime {
					continue
				}
			}
			c.lastEnvResourceSchedulerData[envResourceKey] = envResource

			c.deleteEnvResourceScheduler(envResourceKey)

			newScheduler := gocron.NewScheduler()
			newScheduler.Every(EnvUpdateInterval).Seconds().Do(c.RunScheduledEnvResourceUpdate, envResource.ProductName, envResource.EnvName, envResource.Type, envResource.Name, log)

			log.Infof("[%s] add env resource schedulers..", envResourceKey)
			c.Schedulers[envResourceKey] = newScheduler
			c.SchedulerController[envResourceKey] = c.Schedulers[envResourceKey].Start()
		}

	}

	for _, k := range lastScheduler.List() {
		c.deleteEnvResourceScheduler(k)
		delete(c.lastEnvResourceSchedulerData, k)
	}
}

func (c *CronClient) RunScheduledEnvResourceUpdate(productName, envName, resType, resName string, log *zap.SugaredLogger) {
	log.Infof("start to Run RunScheduledEnvResourceUpdate, productName: %s, envName: %s, resType: %s, resName: %s", productName, envName, resType, resName)
	err := c.AslanCli.SyncEnvResource(productName, envName, resType, resName, log)
	if err != nil {
		log.Warnf("failed to sync variables for env: %s:%s", productName, envName)
	}
}
