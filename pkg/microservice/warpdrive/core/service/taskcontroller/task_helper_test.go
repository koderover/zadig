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

package taskcontroller

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/tool/log"
)

func TestGetStageStatus(t *testing.T) {
	assert := assert.New(t)
	log := log.SugaredLogger()
	var tasks []*Task
	task1 := &Task{
		Err:    nil,
		Status: config.StatusPassed,
	}

	tasks = append(tasks, task1)
	stageStatus := getStageStatus(tasks, log)
	log.Info(stageStatus)
	assert.Equal(config.StatusPassed, stageStatus)

	task2 := &Task{
		Err:    fmt.Errorf("test failed"),
		Status: config.StatusFailed,
	}
	tasks = append(tasks, task2)
	stageStatus = getStageStatus(tasks, log)
	log.Info(stageStatus)
	assert.Equal(config.StatusFailed, stageStatus)

	task3 := &Task{
		Err:    fmt.Errorf("test timeout"),
		Status: config.StatusTimeout,
	}
	tasks = append(tasks, task3)
	stageStatus = getStageStatus(tasks, log)
	log.Info(stageStatus)
	assert.Equal(config.StatusTimeout, stageStatus)

	task4 := &Task{
		Err:    fmt.Errorf("test timeout"),
		Status: config.StatusCancelled,
	}
	tasks = append(tasks, task4)
	stageStatus = getStageStatus(tasks, log)
	log.Info(stageStatus)
	assert.Equal(config.StatusCancelled, stageStatus)
}
