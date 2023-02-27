/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package workflowcontroller

import (
	"fmt"
	"sync"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

// WorkflowTaskWithLock is used for editing workflow task data when it is running
// At present, it is only designed for changing breakpoint in a running task for debug
var globalWorkflowTaskWithLock workflowTaskMapWithLock

func init() {
	globalWorkflowTaskWithLock = workflowTaskMapWithLock{
		m: make(map[string]*WorkflowTaskWithLock),
	}
}

type workflowTaskMapWithLock struct {
	m map[string]*WorkflowTaskWithLock
	sync.RWMutex
}

type WorkflowTaskWithLock struct {
	WorkflowTask *models.WorkflowTask
	Ack          func()
	sync.RWMutex
}

func GetWorkflowTaskInMap(workflowName string, taskID int64) *WorkflowTaskWithLock {
	globalWorkflowTaskWithLock.RLock()
	defer globalWorkflowTaskWithLock.RUnlock()
	return globalWorkflowTaskWithLock.m[fmt.Sprintf("%s-%d", workflowName, taskID)]
}

func addWorkflowTaskInMap(workflowName string, taskID int64, task *models.WorkflowTask, ack func()) {
	globalWorkflowTaskWithLock.Lock()
	defer globalWorkflowTaskWithLock.Unlock()
	globalWorkflowTaskWithLock.m[fmt.Sprintf("%s-%d", workflowName, taskID)] = &WorkflowTaskWithLock{
		WorkflowTask: task,
		Ack:          ack,
	}
}

func removeWorkflowTaskInMap(workflowName string, taskID int64) {
	globalWorkflowTaskWithLock.Lock()
	defer globalWorkflowTaskWithLock.Unlock()
	delete(globalWorkflowTaskWithLock.m, fmt.Sprintf("%s-%d", workflowName, taskID))
}
