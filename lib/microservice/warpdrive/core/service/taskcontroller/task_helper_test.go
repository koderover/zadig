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

	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/pkg/types/pipeline"
	"github.com/koderover/zadig/pkg/types/task"
)

func generateSubtasks() []*task.SubTask {
	buildtask := &task.Build{
		TaskType:    task.TaskBuild,
		Enabled:     true,
		ServiceName: "test123",
	}
	subtask1, _ := buildtask.ToSubTask()
	return []*task.SubTask{subtask1}
}

func TestTransformToStages(t *testing.T) {
	assert := assert.New(t)
	log := xlog.NewDummy()
	subtasks := generateSubtasks()
	pipelineTask := &pipeline.Task{
		TaskID:       1,
		PipelineName: "test-transform-subtask",
		SubTasks:     subtasks,
	}

	transformToStages(pipelineTask, log)
	assert.Len(pipelineTask.Stages, len(subtasks))

}

func TestInitPipelineTask(t *testing.T) {
	assert := assert.New(t)
	log := xlog.NewDummy()
	subtasks := generateSubtasks()
	pipelineTask := &pipeline.Task{
		TaskID:       1,
		PipelineName: "test-transform-subtask",
		SubTasks:     subtasks,
	}
	initPipelineTask(pipelineTask, log)
	assert.Equal(pipelineTask.Status, task.StatusRunning)
}

func TestUpdatePipelineSubTask(t *testing.T) {
	assert := assert.New(t)
	log := xlog.NewDummy()
	//These subtasks have no status fields
	subtasks := generateSubtasks()
	//pipeline 1.0
	pipelineTask := &pipeline.Task{
		TaskID:       1,
		PipelineName: "test-transform-subtask",
		SubTasks:     subtasks,
	}
	buildtask, err := pipelineTask.SubTasks[0].ToBuildTask()
	assert.Nil(err)
	assert.Equal(task.Status(""), buildtask.TaskStatus)

	updateBuild := &task.Build{
		TaskType:    task.TaskBuild,
		TaskStatus:  task.StatusPassed,
		Enabled:     true,
		ServiceName: "test123",
	}

	updatePipelineSubTask(updateBuild, pipelineTask, 0, "test123", log)
	updatedBuildtask, err := pipelineTask.SubTasks[0].ToBuildTask()
	assert.Nil(err)
	assert.Equal(task.StatusPassed, updatedBuildtask.TaskStatus)
	updatedBuildStageTask, err := pipelineTask.Stages[0].SubTasks["test123"].ToBuildTask()
	assert.Nil(err)
	assert.Equal(task.StatusPassed, updatedBuildStageTask.TaskStatus)
}

func TestGetStageStatus(t *testing.T) {
	assert := assert.New(t)
	log := xlog.NewDummy()
	var tasks []*Task
	task1 := &Task{
		Err:    nil,
		Status: task.StatusPassed,
	}

	tasks = append(tasks, task1)
	stageStatus := getStageStatus(tasks, log)
	log.Info(stageStatus)
	assert.Equal(task.StatusPassed, stageStatus)

	task2 := &Task{
		Err:    fmt.Errorf("test failed"),
		Status: task.StatusFailed,
	}
	tasks = append(tasks, task2)
	stageStatus = getStageStatus(tasks, log)
	log.Info(stageStatus)
	assert.Equal(task.StatusFailed, stageStatus)

	task3 := &Task{
		Err:    fmt.Errorf("test timeout"),
		Status: task.StatusTimeout,
	}
	tasks = append(tasks, task3)
	stageStatus = getStageStatus(tasks, log)
	log.Info(stageStatus)
	assert.Equal(task.StatusTimeout, stageStatus)

	task4 := &Task{
		Err:    fmt.Errorf("test timeout"),
		Status: task.StatusCancelled,
	}
	tasks = append(tasks, task4)
	stageStatus = getStageStatus(tasks, log)
	log.Info(stageStatus)
	assert.Equal(task.StatusCancelled, stageStatus)
}
