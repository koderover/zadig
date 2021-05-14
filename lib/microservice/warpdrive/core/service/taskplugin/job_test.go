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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/koderover/zadig/lib/microservice/warpdrive/config"
	"github.com/koderover/zadig/lib/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
)

//Test create configmap named jobname
func TestCreateJobConfigMap(t *testing.T) {

	assert := assert.New(t)
	log := xlog.NewDummy()

	namespace := "fake-test-ns"
	jobname := "fake-test-jobname"
	jobCtx := ""
	pipelineName := "testpipeline"
	serviceName := "test123"
	taskID := int64(1)

	defer func() {
		FakeKubeCli.DeleteNamespace(namespace)
		FakeKubeCli.DeleteConfigMap(namespace, jobname)
	}()
	FakeKubeCli.CreateNamespace(namespace)
	ns, err := FakeKubeCli.GetNamespace(namespace)
	log.Infof("%v", ns)
	assert.Nil(err)
	assert.Equal(namespace, ns.Name)
	//Test createJobConfigMap
	//created a configmap named jobname

	jobLabel := &JobLabel{
		PipelineName: pipelineName,
		ServiceName:  serviceName,
		TaskID:       taskID,
		TaskType:     fmt.Sprintf("%s", config.TaskBuild),
	}

	createJobConfigMap(FakeKubeCli, namespace, jobname, jobLabel, jobCtx)
	jobConfigmap, err := FakeKubeCli.GetConfigMap(namespace, jobname)
	assert.Nil(err)
	assert.Equal(jobname, jobConfigmap.Name)

}

func TestEnsureDeleteConfigMap(t *testing.T) {

	assert := assert.New(t)
	log := xlog.NewDummy()

	namespace := "fake-test-ns"
	jobname := "fake-test-jobname"
	jobCtx := ""

	jobLabel := &JobLabel{
		PipelineName: "test-pipeline",
		ServiceName:  "test-service",
		TaskID:       1,
		TaskType:     fmt.Sprintf("%s", config.TaskBuild),
	}

	log.Infof("%v", getJobLabels(jobLabel))

	log.Infof("%v", getJobSelector(getJobLabels(jobLabel)))

	// configmap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobname,
			Namespace: namespace,
			Labels:    getJobLabels(jobLabel),
		},
		Data: map[string]string{
			"job-config.xml": jobCtx,
		},
	}

	selector := getJobSelector(getJobLabels(jobLabel))

	defer func() {
		FakeKubeCli.DeleteNamespace(namespace)
		FakeKubeCli.DeleteConfigMaps(namespace, selector)
	}()
	FakeKubeCli.CreateNamespace(namespace)
	ns, err := FakeKubeCli.GetNamespace(namespace)
	log.Infof("%v", ns)
	assert.Nil(err)
	assert.Equal(namespace, ns.Name)
	err = FakeKubeCli.CreateConfigMap(namespace, cm)
	assert.Nil(err)

	configmaps, err := FakeKubeCli.ListConfigMaps(namespace, selector)
	log.Infof("%v", configmaps)
	assert.NotEmpty(configmaps)

	err = FakeKubeCli.DeleteConfigMaps(namespace, selector)
	log.Infof("%v", err)
	assert.Nil(err)

	// TODO: FakeKubeCli.DeleteConfigMaps cannot delete configmap collection
	// Need to fix it and assert configmap does not exist indeed.
	/*
		configmap, err := FakeKubeCli.GetConfigMap(namespace, jobname)
		log.Infof("%v", configmap)
		assert.Nil(configmap)
	*/

}

func TestBuildJob(t *testing.T) {
	assert := assert.New(t)

	jobImage := "xxx.com/resources/predator-plugin:test"
	pipelineCtx := &task.PipelineCtx{
		Workspace:         "/tmp",
		DistDir:           "/dist",
		DockerMountDir:    "/dockermnt",
		ConfigMapMountDir: "/cfgmnt",
	}

	pipelineTask := &task.Task{
		ConfigPayload: &task.ConfigPayload{},
	}

	_, err := buildJob(config.TaskBuild, jobImage, "test-build-job", "test123", setting.LowRequest, pipelineCtx, pipelineTask, nil)

	const (
		jobName     = "test-build-job"
		serviceName = "Test123"
	)
	job, err := buildJob(config.TaskBuild, jobImage, jobName, serviceName,
		setting.LowRequest, pipelineCtx, pipelineTask, make([]*task.RegistryNamespace, 0))
	assert.Nil(err)

	assert.Equal(jobName, job.ObjectMeta.Name)
	assert.Equal(strings.ToLower(serviceName), job.ObjectMeta.Labels[jobLabelServiceKey])
}

func TestGetVolumes(t *testing.T) {
	vols := getVolumes("demo-job")
	assert.Len(t, vols, 1)
	assert.Equal(t, "job-config", vols[0].Name)
}

func TestEnsureDeleteJob(t *testing.T) {

	assert := assert.New(t)
	log := xlog.NewDummy()

	namespace := "fake-test-ns"
	jobname := "fake-test-jobname"
	jobImage := "xxx.com/resources/predator-plugin:test"
	servicename := "test-service"

	pipelineCtx := &task.PipelineCtx{
		Workspace:         "/tmp",
		DistDir:           "/dist",
		DockerMountDir:    "/dockermnt",
		ConfigMapMountDir: "/cfgmnt",
	}

	pipelineTask := &task.Task{
		PipelineName:  "test-pipeline",
		TaskID:        123,
		ConfigPayload: &task.ConfigPayload{},
	}

	job, err := buildJob(config.TaskBuild, jobImage, jobname, servicename, setting.LowRequest, pipelineCtx, pipelineTask, nil)
	assert.Nil(err)

	defer func() {
		FakeKubeCli.DeleteNamespace(namespace)
		FakeKubeCli.DeleteJob(namespace, jobname)
	}()
	FakeKubeCli.CreateNamespace(namespace)
	ns, err := FakeKubeCli.GetNamespace(namespace)
	log.Infof("%v", ns)
	assert.Nil(err)

	err = FakeKubeCli.CreateJob(namespace, job)
	log.Infof("%v", err)
	assert.Nil(err)

	jobLabel := &JobLabel{
		PipelineName: pipelineTask.PipelineName,
		ServiceName:  servicename,
		TaskID:       pipelineTask.TaskID,
		TaskType:     fmt.Sprintf("%s", config.TaskBuild),
	}

	err = FakeKubeCli.DeleteJobs(namespace, getJobSelector(getJobLabels(jobLabel)))
	log.Infof("%v", err)
	assert.Nil(err)

	// TODO: FakeKubeCli.DeleteJobs cannot delete job collection
	// Need to fix it and assert job does not exist indeed.
	/*
		getJob, err := FakeKubeCli.GetJob(namespace, jobname)
		log.Infof("%v", getJob)
		assert.Nil(err)
		assert.Nil(getJob)
	*/
}
