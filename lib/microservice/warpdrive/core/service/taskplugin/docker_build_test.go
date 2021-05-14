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

func TestDockerBuildTaskPlugin(t *testing.T) {
	assert := assert.New(t)
	log := xlog.NewDummy()

	namespace := "fake-test-ns"
	jobname := "fake-docker-build-job"

	defer func() {
		FakeKubeCli.DeleteNamespace(namespace)
		FakeKubeCli.DeleteConfigMap(namespace, jobname)
		FakeKubeCli.DeleteJob(namespace, jobname)
	}()

	dockerBuildPlugin := InitializeDockerBuildTaskPlugin(config.TaskDockerBuild)
	assert.NotNil(dockerBuildPlugin)
	assert.Equal(dockerBuildPlugin.Type(), config.TaskDockerBuild)
	dockerBuildPlugin.Init(jobname, jobname, log)

	dockerBuildTask := &task.DockerBuild{
		TaskType:   config.TaskDockerBuild,
		Enabled:    true,
		WorkDir:    "~/test",
		DockerFile: "~/test/Dockerfile",
	}

	dockerBuildSubTask, err := dockerBuildTask.ToSubTask()
	assert.Nil(err)
	pipelineTask := &task.Task{
		TaskID:       1,
		PipelineName: "test-pipeline-name",
		SubTasks: []map[string]interface{}{
			dockerBuildSubTask,
		},
		ConfigPayload: &task.ConfigPayload{
			Build: task.BuildConfig{
				KubeNamespace: namespace,
			},
			//docker build is using this image
			Release: task.ReleaseConfig{
				PredatorImage: "testtesttest",
			},
			//NFS: config.NfsConfig{
			//	Path: "testtesttest",
			//},
			Registry: task.RegistryConfig{
				Addr: "127.0.0.1",
			},
			Aslan: task.AslanConfig{},
		},
		TaskArgs: &task.TaskArgs{
			Deploy: task.DeployArgs{
				Image: "test:beta",
			},
		},
	}

	pipelineCtx := &task.PipelineCtx{
		DockerHost: "",
	}
	ctx := context.Background()

	dockerBuildPlugin.SetTask(dockerBuildSubTask)

	assert.Equal(dockerBuildTask.Enabled, dockerBuildPlugin.IsTaskEnabled())
	assert.Equal(dockerBuildTask.TaskType, dockerBuildPlugin.Type())
	assert.Equal(dockerBuildTask.TaskStatus, dockerBuildPlugin.Status())

	//Run method will create cm/job
	dockerBuildPlugin.Run(ctx, pipelineTask, pipelineCtx, "test123")

	//After Run method, configmap and job should be created successfully
	configmap, err := FakeKubeCli.GetConfigMap(namespace, jobname)
	assert.Nil(err)
	assert.Equal(jobname, configmap.Name)
	job, err := FakeKubeCli.GetJob(namespace, jobname)
	assert.Nil(err)
	assert.Equal(jobname, job.Name)
	//TODO: add more assertions here
}
