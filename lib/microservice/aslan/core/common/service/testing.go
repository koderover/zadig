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

package service

import (
	"fmt"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func DeleteTestModule(name, productName string, log *xlog.Logger) error {
	opt := new(repo.ListQueueOption)
	taskQueue, err := repo.NewQueueColl().List(opt)
	if err != nil {
		log.Errorf("List queued task error: %v", err)
		return fmt.Errorf("List queued task error: %v", err)
	}
	pipelineName := fmt.Sprintf("%s-%s", name, "job")
	// 当task还在运行时，先取消任务
	for _, task := range taskQueue {
		if task.PipelineName == pipelineName && task.Type == config.TestType {
			if err = CancelTaskV2("system", task.PipelineName, task.TaskID, config.TestType, log); err != nil {
				log.Errorf("test task still running,cancel pipeline %s task %d", task.PipelineName, task.TaskID)
			}
		}
	}

	return Delete(name, productName, log)
}

func Delete(name, productName string, log *xlog.Logger) error {
	if len(name) == 0 {
		return e.ErrDeleteTestModule.AddDesc("empty Name")
	}

	err := repo.NewTestingColl().Delete(name, productName)
	if err != nil {
		log.Errorf("[Testing.Delete] %s error: %v", name, err)
		return e.ErrDeleteTestModule.AddErr(err)
	}

	if err := repo.NewTaskColl().DeleteByPipelineNameAndType(fmt.Sprintf("%s-%s", name, "job"), config.TestType); err != nil {
		log.Errorf("[Testing.Delete] PipelineTaskV2.DeleteByPipelineNameAndType test %s error: %v", name, err)
	}

	if err := repo.NewTestTaskStatColl().Delete(name); err != nil {
		log.Errorf("[TestTaskStat.Delete] %s error: %v", name, err)
	}

	pipelineName := fmt.Sprintf("%s-%s", name, "job")
	counterName := fmt.Sprintf(setting.TestTaskFmt, pipelineName)
	if err := repo.NewCounterColl().Delete(counterName); err != nil {
		log.Errorf("[Counter.Delete] counterName:%s, error: %v", counterName, err)
	}

	return nil
}
