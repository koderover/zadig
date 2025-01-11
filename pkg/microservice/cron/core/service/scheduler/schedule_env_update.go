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
	"github.com/jasonlvhit/gocron"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/v2/pkg/microservice/cron/core/service/client"
	"github.com/koderover/zadig/v2/pkg/setting"
)

// EnvUpdateInterval TODO interval will be set on product in future
const EnvUpdateInterval = 30

func needCreateProdSchedule(env *service.ProductResp) bool {
	if env.YamlData != nil {
		if env.YamlData.Source == setting.SourceFromGitRepo || env.YamlData.Source == setting.SourceFromVariableSet {
			return true
		}
	}
	for _, svcGroup := range env.Services {
		for _, svc := range svcGroup {
			if svc.GetServiceRender().OverrideYaml != nil && svc.GetServiceRender().OverrideYaml.Source == setting.SourceFromGitRepo {
				return true
			}
		}
	}
	return false
}

func (c *CronClient) UpsertEnvValueSyncScheduler(log *zap.SugaredLogger) {
	envs, err := c.AslanCli.ListEnvs(log, &client.EvnListOption{DeployType: []string{setting.HelmDeployType}})
	if err != nil {
		log.Errorf("failed to list envs for env values sync: %s", err)
		return
	}

	//compare to last revision, delete related schedulers when env is deleted
	c.compareHelmProductEnvRevision(envs, log)

	log.Infof("start init env values sync scheduler... env count: %v", len(envs))
	for _, env := range envs {
		// log.Debugf("schedule_env_update handle single helm env: %s/%s", env.ProductName, env.EnvName)
		envObj, err := c.AslanCli.GetEnvService(env.ProductName, env.EnvName, log)
		if err != nil {
			log.Errorf("failed to get env data, productName:%s envName:%s err:%v", env.ProductName, env.EnvName, err)
			continue
		}

		envKey := buildEnvNameKey(env)
		c.lastEnvSchedulerDataRWMutex.Lock()
		if lastEnvData, ok := c.lastEnvSchedulerData[envKey]; ok {
			// render not changed, no need to update scheduler
			if lastEnvData.UpdateTime == envObj.UpdateTime {
				c.lastEnvSchedulerDataRWMutex.Unlock()
				continue
			}
		}
		c.lastEnvSchedulerData[envKey] = envObj
		c.lastEnvSchedulerDataRWMutex.Unlock()

		c.SchedulerControllerRWMutex.Lock()
		sc, ok := c.SchedulerController[envKey]
		c.SchedulerControllerRWMutex.Unlock()
		if ok {
			sc <- true
		}

		c.SchedulersRWMutex.Lock()
		if _, ok := c.Schedulers[envKey]; ok {
			c.Schedulers[envKey].Clear()
			delete(c.Schedulers, envKey)
		}
		c.SchedulersRWMutex.Unlock()

		if !needCreateProdSchedule(envObj) {
			continue
		}

		newScheduler := gocron.NewScheduler()
		newScheduler.Every(EnvUpdateInterval).Seconds().Do(c.RunScheduledEnvUpdate, env.ProductName, env.EnvName, log)
		c.SchedulersRWMutex.Lock()
		c.Schedulers[envKey] = newScheduler
		c.SchedulersRWMutex.Unlock()

		log.Infof("[%s] add schedulers..", envKey)
		c.SchedulerControllerRWMutex.Lock()
		c.SchedulerController[envKey] = c.Schedulers[envKey].Start()
		c.SchedulerControllerRWMutex.Unlock()
	}
}

func (c *CronClient) RunScheduledEnvUpdate(productName, envName string, log *zap.SugaredLogger) {
	log.Infof("start to Run ScheduledEnvUpdate, productName: %s, envName: %s", productName, envName)
	err := c.AslanCli.SyncEnvVariables(productName, envName, log)
	if err != nil {
		log.Errorf("failed to sync variables for env: %s:%s", productName, envName)
	}
}
