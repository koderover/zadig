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
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func TestBuildTaskPlugin_TaskTimeout_Default(t *testing.T) {
	assert := assert.New(t)
	plugin := InitializeBuildTaskPlugin(task.TaskBuild, FakeKubeCli).(*BuildTaskPlugin)

	plugin.Task = buildTaskForTest()
	assert.Equal(BuildTaskV2Timeout, plugin.TaskTimeout())
}

func TestBuildTaskPlugin_TaskTimeout_Restart(t *testing.T) {
	assert := assert.New(t)
	plugin := InitializeBuildTaskPlugin(task.TaskBuild, FakeKubeCli).(*BuildTaskPlugin)

	initTimeout := 20
	plugin.Task = buildTaskForTest()
	plugin.Task.Timeout = initTimeout
	plugin.Task.IsRestart = true

	assert.Equal(initTimeout, plugin.TaskTimeout())
}

func TestBuildTaskPlugin_TaskTimeout_NotRestart(t *testing.T) {
	assert := assert.New(t)
	plugin := InitializeBuildTaskPlugin(task.TaskBuild, FakeKubeCli).(*BuildTaskPlugin)

	initTimeout := 20
	plugin.Task = buildTaskForTest()
	plugin.Task.Timeout = initTimeout
	plugin.Task.IsRestart = false

	assert.Equal(initTimeout*60, plugin.TaskTimeout())
	// Each call will extend current timeout to be 60x. Not sure if this is a feature or a bug, but just
	// add the unit test to test current code logic as it is
	assert.Equal(initTimeout*60*60, plugin.TaskTimeout())
}

func TestBuildTaskPlugin_SetBuildStatusCompleted(t *testing.T) {
	plugin := InitializeBuildTaskPlugin(task.TaskBuild, FakeKubeCli).(*BuildTaskPlugin)
	plugin.Task = buildTaskForTest()
	plugin.SetBuildStatusCompleted(task.StatusPassed)

	assert.Equal(t, task.StatusPassed, plugin.Task.BuildStatus.Status)
}

func TestBuildTaskPlugin_Run(t *testing.T) {
	assert := assert.New(t)
	log := xlog.NewDummy()

	namespace := "faketestns"
	jobname := "fakebuildjob"

	defer func() {
		FakeKubeCli.DeleteNamespace(namespace)
		FakeKubeCli.DeleteConfigMap(namespace, jobname)
		FakeKubeCli.DeleteJob(namespace, jobname)
	}()

	buildTaskPlugin := InitializeBuildTaskPlugin(task.TaskBuild).(*BuildTaskPlugin)
	assert.NotNil(buildTaskPlugin)
	assert.Equal(buildTaskPlugin.Type(), config.TaskBuild)
	buildTaskPlugin.Init(jobname, jobname, log)
	buildTaskPlugin.SetAckFunc(func() {
		fmt.Println("Acknowledge")
	})

	buildTask := buildTaskForTest()

	buildSubTask, err := buildTask.ToSubTask()
	assert.Nil(err)
	pipelineTask := &task.Task{
		TaskID:       1,
		PipelineName: "testpipelinename",
		SubTasks: []map[string]interface{}{
			buildSubTask,
		},
		ConfigPayload: &task.ConfigPayload{
			Build: task.BuildConfig{
				KubeNamespace: namespace,
			},
		},
	}

	pipelineCtx := &task.PipelineCtx{
		DockerHost: "",
	}
	ctx := context.Background()

	buildTaskPlugin.SetTask(buildSubTask)

	assert.Equal(buildTask.Enabled, buildTaskPlugin.IsTaskEnabled())
	assert.Equal(buildTask.TaskType, buildTaskPlugin.Type())
	assert.Equal(buildTask.TaskStatus, buildTaskPlugin.Status())

	//Run method will create cm/job
	buildTaskPlugin.Run(ctx, pipelineTask, pipelineCtx, buildTask.ServiceName)

	//After Run method, configmap and job should be created successfully
	configmap, err := FakeKubeCli.GetConfigMap(namespace, jobname)
	assert.Nil(err)
	assert.Equal(jobname, configmap.Name)
	job, err := FakeKubeCli.GetJob(namespace, jobname)
	assert.Nil(err)
	assert.Equal(jobname, job.Name)
	//TODO: add more assertions here
}

func buildTaskForTest() *task.Build {
	buildTask := &task.Build{
		TaskType:    task.TaskBuild,
		Enabled:     true,
		ServiceName: "test123",
		BuildOS:     "trusty",
		TaskStatus:  task.StatusRunning,
		BuildStatus: &task.BuildStatus{},
		InstallCtx: []*task.Install{
			{
				Name:    "go",
				Enabled: true,
				BinPath: "$HOME/go/bin",
				Scripts: "",
				Version: "1.10.1",
			},
		},
		JobCtx: task.JobCtx{
			Builds: []*task.Repository{
				{
					RepoOwner: "koderover",
					RepoName:  "zadig",
					Branch:    "master",
				},
			},
			BuildSteps: []*task.BuildStep{
				{
					BuildType: "shell",
					Scripts:   "set ex",
				},
			},
		},
	}
	return buildTask
}
