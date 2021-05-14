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
	"github.com/jasonlvhit/gocron"

	"github.com/koderover/zadig/lib/microservice/cron/core/service"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// RunScheduledTask ...
func (c *CronClient) RunScheduledPipelineTask(pipeline *service.Pipeline, params *service.TaskArgs, log *xlog.Logger) {

	log.Infof("start pipeline cron job: %s ...", pipeline.Name)

	args := &service.TaskArgs{
		PipelineName: pipeline.Name,
		TaskCreator:  setting.CronTaskCreator,
		ReqID:        log.ReqID(),
	}
	if params != nil {
		args.Builds = params.Builds
		args.Deploy = params.Deploy
		args.Test = params.Test
	}
	if err := c.AslanCli.RunPipelineTask(args, log); err != nil {
		log.Errorf("[%s]RunScheduledTask err: %v", pipeline.Name, err)
	}
}

// BuildScheduledPipelineJob ...
func BuildScheduledPipelineJob(scheduler *gocron.Scheduler, schedule *service.Schedule) *gocron.Job {

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
