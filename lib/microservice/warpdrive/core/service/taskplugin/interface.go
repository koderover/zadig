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

package taskplugin

import (
	"context"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/tool/xlog"
)

// TaskPluginInitiator function for init plugins
type TaskPluginInitiator func(taskType config.TaskType) TaskPlugin

// TaskPlugin ...
/*
A Specific Task Plugin is to run one specified task.
*/
type TaskPlugin interface {
	Init(jobname, filename string, xl *xlog.Logger)
	// Type ...
	Type() config.TaskType

	// Status ...
	Status() config.Status

	// Run ...
	Run(ctx context.Context, pipelineTask *task.Task, pipelineCtx *task.PipelineCtx, serviceName string)

	// Wait ...
	Wait(ctx context.Context)

	// Complete ...
	Complete(ctx context.Context, pipelineTask *task.Task, serviceName string)

	SetTask(map[string]interface{}) error

	GetTask() interface{}

	// TaskTimeout ...
	TaskTimeout() int

	// IsTaskDone ...
	IsTaskDone() bool

	// IsTaskFailed ...
	IsTaskFailed() bool

	// IsTaskEnabled ...
	IsTaskEnabled() bool

	// SetStatus ...
	SetStatus(status config.Status)

	// SetStartTime ...
	SetStartTime()

	// SetEndTime ...
	SetEndTime()

	// ResetError ...
	ResetError()

	// SetAckFunc ...
	SetAckFunc(func())
}
