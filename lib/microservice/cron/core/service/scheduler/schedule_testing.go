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

// UpsertTestScheduler ...
func (c *CronClient) UpsertTestScheduler(log *xlog.Logger) {
	tests, err := c.AslanCli.ListTests(log)
	if err != nil {
		log.Error(err)
		return
	}

	log.Info("start init test scheduler..")
	for _, test := range tests {
		log.Infof("deal testing data, name:%s, schedule:%v", test.Name, test.Schedules)
		key := "test-timer-" + test.Name
		// 如果该测试先配置了定时器，然后将定时器全部删除了，这时需要把原来的schedule暂停并删除
		if test.Schedules == nil || len(test.Schedules.Items) == 0 {
			c.StopTestScheduler(key, log)
			continue
		}

		// 判断该测试的定时器配置是否有更新，没有更新则跳过
		if _, ok := c.lastSchedulers[key]; ok && reflect.DeepEqual(test.Schedules.Items, c.lastSchedulers[key]) {
			continue
		} else {
			c.lastSchedulers[key] = test.Schedules.Items
		}

		// 定时器有更新，则重新设置该schedule
		newScheduler := gocron.NewScheduler()
		for _, schedule := range test.Schedules.Items {
			if schedule == nil {
				continue
			}
			log.Infof("deal schedule, test name:%s, schedule item:%v, testArgs:%v", test.Name, schedule, schedule.TestArgs)
			if err := schedule.Validate(); err != nil {
				log.Errorf("[%s] invalid schedule: %v", key, err)
				continue
			}
			BuildScheduledJob(newScheduler, schedule).Do(c.RunTestScheduledTask, test, log)
		}
		// 所有scheduler总开关
		if !test.Schedules.Enabled {
			newScheduler.Clear()
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

func (c *CronClient) StopTestScheduler(key string, log *xlog.Logger) {
	// 暂停
	if _, ok := c.SchedulerController[key]; ok {
		c.SchedulerController[key] <- true
		delete(c.SchedulerController, key) // 必须删除，否则下次执行时，会在上一行插入元素时阻塞。
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
}

// RunTestScheduledTask ...
func (c *CronClient) RunTestScheduledTask(test *service.TestingOpt, log *xlog.Logger) {

	log.Infof("start test cron job: %s ...", test.Name)

	args := &service.TestTaskArgs{
		TestName:        test.Name,
		ProductName:     test.ProductName,
		TestTaskCreator: setting.CronTaskCreator,
		ReqID:           log.ReqID(),
	}

	if err := c.AslanCli.RunTestTask(args, log); err != nil {
		log.Errorf("[%s]RunTestScheduledTask err: %v", test.Name, err)
	}
}
