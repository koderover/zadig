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
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/koderover/zadig/pkg/microservice/warpdrive/config"
	"github.com/koderover/zadig/pkg/microservice/warpdrive/core/service/types/task"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/label"
	"github.com/koderover/zadig/pkg/tool/log"
)

// Test create configmap named jobname
func TestCreateJobConfigMap(t *testing.T) {

	assert := assert.New(t)
	log := log.SugaredLogger()

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

	jobLabel := &label.JobLabel{
		PipelineName: pipelineName,
		ServiceName:  serviceName,
		TaskID:       taskID,
		TaskType:     fmt.Sprintf("%s", config.TaskBuild),
	}

	createJobConfigMap(namespace, jobname, jobLabel, jobCtx, FakeKubeCli)
	jobConfigmap, err := FakeKubeCli.GetConfigMap(namespace, jobname)
	assert.Nil(err)
	assert.Equal(jobname, jobConfigmap.Name)

}

func TestEnsureDeleteConfigMap(t *testing.T) {

	assert := assert.New(t)
	log := log.SugaredLogger()

	namespace := "fake-test-ns"
	jobname := "fake-test-jobname"
	jobCtx := ""

	jobLabel := &label.JobLabel{
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
	log.Error(err)
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

	_, err := buildJob(config.TaskBuild, jobImage, "test-build-job", "test123", setting.LowRequest, setting.LowRequestSpec, pipelineCtx, pipelineTask, nil)
	log.Error(err)

	const (
		jobName     = "test-build-job"
		serviceName = "Test123"
	)
	job, err := buildJob(config.TaskBuild, jobImage, jobName, serviceName,
		setting.LowRequest, setting.LowRequestSpec, pipelineCtx, pipelineTask, make([]*task.RegistryNamespace, 0))
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
	log := log.SugaredLogger()

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

	job, err := buildJob(config.TaskBuild, jobImage, jobname, servicename, setting.LowRequest, setting.LowRequestSpec, pipelineCtx, pipelineTask, nil)
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

	jobLabel := &label.JobLabel{
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

func Test_getResourceRequirements(t *testing.T) {
	type args struct {
		resReq     setting.Request
		resReqSpec setting.RequestSpec
	}
	tests := []struct {
		name string
		args args
		want corev1.ResourceRequirements
	}{
		{
			name: "high",
			args: args{
				resReq:     setting.HighRequest,
				resReqSpec: setting.HighRequestSpec,
			},
			want: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("16000m"),
					corev1.ResourceMemory: resource.MustParse("32768Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4000m"),
					corev1.ResourceMemory: resource.MustParse("4096Mi"),
				},
			},
		},
		{
			name: "define",
			args: args{
				resReq: setting.DefineRequest,
				resReqSpec: setting.RequestSpec{
					CpuLimit:    4000,
					MemoryLimit: 2048,
				},
			},
			want: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4000m"),
					corev1.ResourceMemory: resource.MustParse("2048Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getResourceRequirements(tt.args.resReq, tt.args.resReqSpec)
			fmt.Println("Limit:", got.Limits.Cpu().String(), got.Limits.Memory().String())
			fmt.Println("Memory:", got.Requests.Cpu().String(), got.Requests.Memory().String())
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getResourceRequirements() = %v, want %v", got, tt.want)
			}
		})
	}
}
