/*
Copyright 2021 The KodeRover Authors.

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
	"reflect"
	"strings"

	"github.com/jasonlvhit/gocron"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/cron/core/service"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func (c *CronClient) UpsertWorkflowScheduler(log *zap.SugaredLogger) {
	workflows, err := c.AslanCli.ListWorkflows(log)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("start init workflow scheduler..")
	taskMap := make(map[string]bool)
	for _, workflow := range workflows {
		key := "workflow-" + workflow.Name
		taskMap[key] = true
		if workflow.Schedules == nil {
			workflow.Schedules = &service.ScheduleCtrl{}
		}

		c.enabledMapRWMutex.Lock()
		c.lastSchedulersRWMutex.Lock()
		if _, ok := c.lastSchedulers[key]; ok && reflect.DeepEqual(workflow.Schedules.Items, c.lastSchedulers[key]) {
			// 增加判断：enabled的值未被更新时才能跳过
			if enabled, ok := c.enabledMap[key]; ok && enabled == workflow.Schedules.Enabled {
				c.lastSchedulersRWMutex.Unlock()
				c.enabledMapRWMutex.Unlock()
				continue
			}
		}
		c.enabledMap[key] = workflow.Schedules.Enabled
		c.lastSchedulers[key] = workflow.Schedules.Items
		c.lastSchedulersRWMutex.Unlock()
		c.enabledMapRWMutex.Unlock()

		newScheduler := gocron.NewScheduler()
		for _, schedule := range workflow.Schedules.Items {
			if schedule != nil {
				if err := schedule.Validate(); err != nil {
					log.Errorf("[%s] invalid schedule: %v", key, err)
					continue
				}
				BuildScheduledJob(newScheduler, schedule).Do(c.RunScheduledTask, workflow, schedule.WorkflowArgs, log)
			}
		}
		// 所有scheduler总开关
		if !workflow.Schedules.Enabled {
			newScheduler.Clear()
		}

		c.SchedulersRWMutex.Lock()
		c.Schedulers[key] = newScheduler
		c.SchedulersRWMutex.Unlock()

		log.Infof("[%s] building schedulers..", key)
		// 停掉旧的scheduler
		c.SchedulerControllerRWMutex.Lock()
		sc, ok := c.SchedulerController[key]
		c.SchedulerControllerRWMutex.Unlock()
		if ok {
			sc <- true
		}

		log.Infof("[%s]lens of scheduler: %d", key, c.Schedulers[key].Len())
		c.SchedulerControllerRWMutex.Lock()
		c.SchedulerController[key] = c.Schedulers[key].Start()
		c.SchedulerControllerRWMutex.Unlock()
	}

	ScheduleNames := sets.NewString(
		CleanJobScheduler, UpsertWorkflowScheduler, UpsertTestScheduler,
		InitStatScheduler, InitOperationStatScheduler,
		CleanProductScheduler, InitHealthCheckScheduler, InitHealthCheckPmHostScheduler,
		UpsertColliePipelineScheduler, InitHelmEnvSyncValuesScheduler, EnvResourceSyncScheduler)

	// 停掉已被删除的pipeline对应的scheduler
	for name := range c.Schedulers {
		if _, ok := taskMap[name]; !ok && !ScheduleNames.Has(name) {
			// exclude service heath check timers
			if strings.HasPrefix(name, "service-") && strings.Contains(name, "pm") {
				continue
			}
			// exclude test timers
			if strings.HasPrefix(name, "test-timer-") {
				continue
			}
			// exclude collie pipeline timers
			if strings.HasPrefix(name, "collie-pipeline-timer-") {
				continue
			}
			// exclude helm env values sync timers
			if strings.HasPrefix(name, "helm-values-sync-") {
				continue
			}
			// exclude env resource sync timers
			if strings.HasPrefix(name, "env-resource") {
				continue
			}
			log.Warnf("[%s]deleted workflow detached", name)

			c.SchedulerControllerRWMutex.RLock()
			sc, ok := c.SchedulerController[name]
			c.SchedulerControllerRWMutex.RUnlock()
			if ok {
				sc <- true
			}

			c.SchedulersRWMutex.Lock()
			delete(c.Schedulers, name)
			c.SchedulersRWMutex.Unlock()

			c.lastSchedulersRWMutex.Lock()
			delete(c.lastSchedulers, name)
			c.lastSchedulersRWMutex.Unlock()
		}
	}
}

// RunScheduledTask ...
func (c *CronClient) RunScheduledTask(workflow *service.Workflow, params *service.WorkflowTaskArgs, log *zap.SugaredLogger) {

	log.Infof("start workflow cron job: %s ...", workflow.Name)

	args := &service.WorkflowTaskArgs{
		WorkflowName:       workflow.Name,
		WorklowTaskCreator: setting.CronTaskCreator,
	}

	if params != nil {
		args.Description = params.Description
		args.ProductTmplName = params.ProductTmplName
		args.Target = params.Target
		args.Namespace = params.Namespace
		args.Tests = params.Tests
		args.DistributeEnabled = params.DistributeEnabled
	}

	if err := c.AslanCli.RunWorkflowTask(args, log); err != nil {
		log.Errorf("[%s]RunScheduledTask err: %v", workflow.Name, err)
	}
}

// BuildScheduledJob ...
func BuildScheduledJob(scheduler *gocron.Scheduler, schedule *service.Schedule) *gocron.Job {

	switch schedule.Frequency {

	case setting.FrequencyMinutes:
		return scheduler.Every(schedule.Number).Minutes()

	case setting.FrequencyHour:
		return scheduler.Every(schedule.Number).Hour()

	case setting.FrequencyHours:
		return scheduler.Every(schedule.Number).Hours()

	case setting.FrequencyDay:
		return scheduler.Every(schedule.Number).Day().At(schedule.Time)

	case setting.FrequencyDays:
		return scheduler.Every(schedule.Number).Days().At(schedule.Time)

	case setting.FrequencyMondy:
		return scheduler.Every(schedule.Number).Monday().At(schedule.Time)

	case setting.FrequencyTuesday:
		return scheduler.Every(schedule.Number).Tuesday().At(schedule.Time)

	case setting.FrequencyWednesday:
		return scheduler.Every(schedule.Number).Wednesday().At(schedule.Time)

	case setting.FrequencyThursday:
		return scheduler.Every(schedule.Number).Thursday().At(schedule.Time)

	case setting.FrequencyFriday:
		return scheduler.Every(schedule.Number).Friday().At(schedule.Time)

	case setting.FrequencySaturday:
		return scheduler.Every(schedule.Number).Saturday().At(schedule.Time)

	case setting.FrequencySunday:
		return scheduler.Every(schedule.Number).Sunday().At(schedule.Time)
	}

	return nil
}
