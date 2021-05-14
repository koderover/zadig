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

	"github.com/jasonlvhit/gocron"

	"github.com/koderover/zadig/lib/microservice/cron/core/service"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// UpsertColliePipelineScheduler ...
func (c *CronClient) UpsertColliePipelineScheduler(log *xlog.Logger) {
	pipelines, err := c.CollieCli.ListColliePipelines(log)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("start init collie pipeline task scheduler..")
	for _, pipeline := range pipelines {
		log.Infof("deal collie pipeline data, name:%s, schedule:%v", pipeline.Metadata.Name, pipeline.Spec.Schedules)
		key := "collie-pipeline-timer-" + pipeline.Metadata.Name

		schedules := pipeline.Spec.Schedules
		// 如果该pipeline先配置了定时器，然后将定时器全部删除了，这时需要把原来的schedule暂停并删除
		if len(schedules) == 0 {
			// 暂停
			if _, ok := c.SchedulerController[key]; ok {
				c.SchedulerController[key] <- true
				delete(c.SchedulerController, key)
			}
			// 删除
			if _, ok := c.Schedulers[key]; ok {
				log.Warnf("delete expired test from Schedulers, key:%s", key)
				delete(c.Schedulers, key)
			}
			if _, ok := c.lastSchedulers[key]; ok {
				log.Warnf("delete expired test from lastSchedulers, key:%s", key)
				delete(c.lastSchedulers, key)
			}
			continue
		}

		// 判断该测试的定时器配置是否有更新，没有更新则跳过
		if _, ok := c.lastSchedulers[key]; ok && reflect.DeepEqual(schedules, c.lastSchedulers[key]) {
			continue
		} else {
			c.lastSchedulers[key] = schedules
		}

		// 定时器有更新，则重新设置该schedule
		newScheduler := gocron.NewScheduler()
		for _, schedule := range schedules {
			if schedule == nil || !schedule.Enabled {
				continue
			}
			log.Infof("deal collie pipeline schedule, name:%s, item:%v", pipeline.Metadata.Name, schedule)
			if err := schedule.Validate(); err != nil {
				log.Errorf("[%s] invalid schedule: %v", key, err)
				continue
			}
			BuildScheduledJob(newScheduler, schedule).Do(c.RunColliePipelineScheduledTask, pipeline, log)
		}

		c.Schedulers[key] = newScheduler
		log.Infof("[%s] building schedulers..", key)
		// 停掉旧的scheduler
		if _, ok := c.SchedulerController[key]; ok {
			c.SchedulerController[key] <- true
		}
		log.Infof("[%s] lens of scheduler: %d", key, c.Schedulers[key].Len())
		c.SchedulerController[key] = c.Schedulers[key].Start()
	}
}

// RunTestScheduledTask ...
func (c *CronClient) RunColliePipelineScheduledTask(pipeline *service.PipelineResource, log *xlog.Logger) {
	log.Infof("start collie pipeline cron job: %s ...", pipeline.Metadata.Name)

	args := &service.CreateBuildRequest{
		UserId:       setting.CronTaskCreator,
		UserName:     setting.CronTaskCreator,
		NoCache:      false,
		ResetVolume:  false,
		Variables:    pipeline.Spec.Variables,
		PipelineName: pipeline.Metadata.Name,
		ProductName:  pipeline.Metadata.Project,
	}

	if err := c.CollieCli.RunColliePipelineTask(args, log); err != nil {
		log.Errorf("[%s]RunTestScheduledTask err: %v", pipeline.Metadata.Name, err)
	}
}
