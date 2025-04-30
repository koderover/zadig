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
	"sync"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func DeleteTestModule(name, productName, requestID string, log *zap.SugaredLogger) error {
	opt := new(mongodb.ListQueueOption)
	taskQueue, err := mongodb.NewQueueColl().List(opt)
	if err != nil {
		log.Errorf("List queued task error: %v", err)
		return fmt.Errorf("List queued task error: %v", err)
	}
	pipelineName := fmt.Sprintf("%s-%s", name, "job")
	// 当task还在运行时，先取消任务
	for _, task := range taskQueue {
		if task.PipelineName == pipelineName && task.Type == config.TestType {
			if err = CancelTaskV2("system", task.PipelineName, task.TaskID, config.TestType, requestID, log); err != nil {
				log.Errorf("test task still running,cancel pipeline %s task %d", task.PipelineName, task.TaskID)
			}
		}
	}

	return Delete(name, productName, log)
}

func Delete(name, productName string, log *zap.SugaredLogger) error {
	if len(name) == 0 {
		return e.ErrDeleteTestModule.AddDesc("empty Name")
	}

	// find test data to delete webhook
	testModule, err := commonrepo.NewTestingColl().Find(name, productName)
	if err != nil {
		return e.ErrDeleteTestModule.AddDesc(err.Error())
	}

	// delete webhooks of test
	err = ProcessWebhook(nil, testModule.HookCtl.Items, webhook.TestingPrefix+name, log)
	if err != nil {
		return e.ErrDeleteTestModule.AddErr(err)
	}

	err = mongodb.NewTestingColl().Delete(name, productName)
	if err != nil {
		log.Errorf("[Testing.Delete] %s error: %v", name, err)
		return e.ErrDeleteTestModule.AddErr(err)
	}

	if err := mongodb.NewTaskColl().DeleteByPipelineNameAndType(fmt.Sprintf("%s-%s", name, "job"), config.TestType); err != nil {
		log.Errorf("[Testing.Delete] PipelineTaskV2.DeleteByPipelineNameAndType test %s error: %v", name, err)
	}

	if err := mongodb.NewTestTaskStatColl().Delete(name); err != nil {
		log.Errorf("[TestTaskStat.Delete] %s error: %v", name, err)
	}

	pipelineName := fmt.Sprintf("%s-%s", name, "job")
	counterName := fmt.Sprintf(setting.TestTaskFmt, pipelineName)
	if err := mongodb.NewCounterColl().Delete(counterName); err != nil {
		log.Errorf("[Counter.Delete] counterName:%s, error: %v", counterName, err)
	}

	return nil
}

type TestService struct {
	TestMap         sync.Map
	TestTemplateMap sync.Map
}

func NewTestingService() *TestService {
	return &TestService{
		TestMap:         sync.Map{},
		TestTemplateMap: sync.Map{},
	}
}

func (c *TestService) GetByName(projectName, name string) (*commonmodels.Testing, error) {
	var err error
	testInfo := new(commonmodels.Testing)
	key := fmt.Sprintf("%s++%s", projectName, name)
	buildMapValue, ok := c.TestMap.Load(key)
	if !ok {
		testInfo, err = commonrepo.NewTestingColl().Find(name, projectName)
		if err != nil {
			c.TestMap.Store(key, nil)
			return nil, fmt.Errorf("find scan: %s error: %v", key, err)
		}
		c.TestMap.Store(key, testInfo)
	} else {
		if buildMapValue == nil {
			return nil, fmt.Errorf("failed to find scanning: %s", key)
		}
		testInfo = buildMapValue.(*commonmodels.Testing)
	}

	return testInfo, nil
}
