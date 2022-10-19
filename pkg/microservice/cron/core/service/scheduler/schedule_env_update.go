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

	"github.com/koderover/zadig/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/pkg/microservice/cron/core/service/client"
	"github.com/koderover/zadig/pkg/setting"
)

// EnvUpdateInterval TODO interval will be set on product in future
const EnvUpdateInterval = 30

func needCreateSchedule(rendersetObj *service.ProductRenderset) bool {
	if rendersetObj.YamlData != nil {
		if rendersetObj.YamlData.Source == setting.SourceFromGitRepo || rendersetObj.YamlData.Source == setting.SourceFromVariableSet {
			return true
		}
	}
	for _, chartData := range rendersetObj.ChartInfos {
		if chartData.OverrideYaml != nil && chartData.OverrideYaml.Source == setting.SourceFromGitRepo {
			return true
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

	log.Info("start init env values sync scheduler..")
	for _, env := range envs {
		envObj, err := c.AslanCli.GetEnvService(env.ProductName, env.EnvName, log)
		if err != nil {
			log.Errorf("failed to get env data, productName:%s envName:%s err:%v", env.ProductName, env.EnvName, err)
			continue
		}
		if envObj.Render == nil {
			log.Errorf("render info of product: %s:%s is nil", env.ProductName, env.EnvName)
			continue
		}

		rendersetObj, err := c.AslanCli.GetRenderset(envObj.Render.Name, envObj.Render.Revision, log)
		if err != nil {
			log.Errorf("failed to get renderset data: %s, err:%s", envObj.Render.Name, err)
			continue
		}

		envKey := buildEnvNameKey(env)
		if lastEnvData, ok := c.lastEnvSchedulerData[envKey]; ok {
			// render not changed, no need to update scheduler
			if lastEnvData.Render.Revision == rendersetObj.Revision {
				continue
			}
		}
		c.lastEnvSchedulerData[envKey] = envObj
		if _, ok := c.SchedulerController[envKey]; ok {
			c.SchedulerController[envKey] <- true
		}
		if _, ok := c.Schedulers[envKey]; ok {
			c.Schedulers[envKey].Clear()
			delete(c.Schedulers, envKey)
		}

		if !needCreateSchedule(rendersetObj) {
			continue
		}

		newScheduler := gocron.NewScheduler()
		newScheduler.Every(EnvUpdateInterval).Seconds().Do(c.RunScheduledEnvUpdate, env.ProductName, env.EnvName, log)
		c.Schedulers[envKey] = newScheduler
		log.Infof("[%s] add schedulers..", envKey)
		c.SchedulerController[envKey] = c.Schedulers[envKey].Start()
	}
}

func (c *CronClient) RunScheduledEnvUpdate(productName, envName string, log *zap.SugaredLogger) {
	log.Infof("start to Run ScheduledEnvUpdate, productName: %s, envName: %s", productName, envName)
	err := c.AslanCli.SyncEnvVariables(productName, envName, log)
	if err != nil {
		log.Errorf("failed to sync variables for env: %s:%s", productName, envName)
	}
}
