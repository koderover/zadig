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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func TestReleaseImagePlugin_TaskTimeout_Default(t *testing.T) {
	plugin := InitializeReleaseImagePlugin(task.TaskReleaseImage, FakeKubeCli).(*ReleaseImagePlugin)
	plugin.Task = releaseTaskForTest()
	assert.Equal(t, RelealseImageTaskTimeout, plugin.TaskTimeout())
}

func TestReleaseImagePlugin_TaskTimeout_GivenTimeout(t *testing.T) {
	plugin := InitializeReleaseImagePlugin(task.TaskReleaseImage, FakeKubeCli).(*ReleaseImagePlugin)
	task := releaseTaskForTest()
	task.Timeout = 20
	plugin.Task = task
	assert.Equal(t, 20, plugin.TaskTimeout())
}

func TestReleaseImagePlugin_Run(t *testing.T) {
	assert := assert.New(t)
	log := xlog.NewDummy()

	const (
		namespace = "fake-test-ns"
		jobName   = "fake-build-job"
		repoId    = "repoId"
	)

	defer func() {
		FakeKubeCli.DeleteNamespace(namespace)
		FakeKubeCli.DeleteConfigMap(namespace, jobName)
		FakeKubeCli.DeleteJob(namespace, jobName)
	}()

	plugin := InitializeReleaseImagePlugin(task.TaskReleaseImage, FakeKubeCli).(*ReleaseImagePlugin)
	assert.NotNil(plugin)
	assert.Equal(plugin.Type(), task.TaskReleaseImage)
	plugin.Init(jobName, jobName, log)

	releaseImageTask := releaseTaskForTest()
	releaseImageSubTask, err := releaseImageTask.ToSubTask()
	assert.Nil(err)

	releaseImageTask.Releases = []task.RepoImage{
		{
			RepoId:    repoId,
			Name:      "releaseName",
			Host:      "os.koderover.com",
			Namespace: namespace,
		},
	}
	plugin.Task = releaseImageTask

	pipelineTask := &task.Task{
		TaskID:       1,
		PipelineName: "test-pipeline-name",
		SubTasks: []map[string]interface{}{
			releaseImageSubTask,
		},
		ConfigPayload: &task.ConfigPayload{
			Build: config.BuildConfig{
				KubeNamespace: namespace,
			},
			//docker build is using this image
			Release: task.ReleaseConfig{
				PredatorImage: "testtesttest",
			},
			ImageRelease: config.ImageReleaseConfig{
				AccessKey: "123",
				SecretKey: "456",
			},
			Registry: task.RegistryConfig{
				Addr: "127.0.0.1",
			},
			RepoConfigs: map[string]*task.RegistryNamespace{
				repoId: {},
			},
		},
	}
	pipelineCtx := &task.PipelineCtx{
		DockerHost: "",
	}

	assert.Equal(releaseImageTask.Enabled, plugin.IsTaskEnabled())
	assert.Equal(releaseImageTask.TaskType, plugin.Type())
	assert.Equal(releaseImageTask.TaskStatus, plugin.Status())

	//Run method will create cm/job
	plugin.Run(context.Background(), pipelineTask, pipelineCtx, "test123")

	//After Run method, configmap and job should be created successfully
	configmap, err := FakeKubeCli.GetConfigMap(namespace, jobName)
	assert.Nil(err)
	assert.Equal(jobName, configmap.Name)
	job, err := FakeKubeCli.GetJob(namespace, jobName)
	assert.Nil(err)
	assert.Equal(jobName, job.Name)
	//TODO: add more assertions here
}

func releaseTaskForTest() *task.ReleaseImage {
	return &task.ReleaseImage{
		TaskType:     task.TaskReleaseImage,
		Enabled:      true,
		ImageTest:    "xxx.com/repo/image:test",
		ImageRelease: "xxx.com/repo/image:release",
		ImageRepo:    "imagerepo",
	}
}
