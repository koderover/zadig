/*
Copyright 2022 The KodeRover Authors.

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

package job

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/log"
	"helm.sh/helm/v3/pkg/time"
	"k8s.io/apimachinery/pkg/labels"
)

type BlueGreenDeployJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.BlueGreenDeployJobSpec
}

func (j *BlueGreenDeployJob) Instantiate() error {
	j.spec = &commonmodels.BlueGreenDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BlueGreenDeployJob) SetPreset() error {
	j.spec = &commonmodels.BlueGreenDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *BlueGreenDeployJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.BlueGreenDeployJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
		argsSpec := &commonmodels.BlueGreenDeployJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		j.spec.Targets = argsSpec.Targets
		j.job.Spec = j.spec
	}
	return nil
}

func (j *BlueGreenDeployJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.BlueGreenDeployJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), j.spec.ClusterID)
	if err != nil {
		logger.Errorf("Failed to get kube client, err: %v", err)
		return resp, err
	}

	for _, target := range j.spec.Targets {
		service, exist, err := getter.GetService(j.spec.Namespace, target.K8sServiceName, kubeClient)
		if err != nil || !exist {
			msg := fmt.Sprintf("Failed to get service, err: %v", err)
			logger.Error(msg)
			return resp, errors.New(msg)
		}
		delete(service.Spec.Selector, config.BlueGreenVerionLabelName)
		selector := labels.Set(service.Spec.Selector).AsSelector()
		deployments, err := getter.ListDeployments(j.spec.Namespace, selector, kubeClient)
		if err != nil {
			msg := fmt.Sprintf("list deployments error: %v", err)
			logger.Error(msg)
			return resp, errors.New(msg)
		}
		if len(deployments) == 0 {
			msg := "no deployment found"
			logger.Error(msg)
			return resp, errors.New(msg)
		}
		if len(deployments) > 1 {
			msg := "more than one deployment found"
			logger.Error(msg)
			return resp, errors.New(msg)
		}
		deployment := deployments[0]

		version := fmt.Sprintf("v%d", time.Now().Unix())
		target.Version = version
		target.WorkloadName = deployment.Name
		target.WorkloadType = setting.Deployment
		target.BlueK8sServiceName = target.K8sServiceName + config.BlueServiceNameSuffix
		target.BlueWorkloadName = getBlueWorkloadName(deployment.Name, version)
		task := &commonmodels.JobTask{
			Name:    jobNameFormat(j.job.Name + "-" + target.K8sServiceName),
			JobType: string(config.JobK8sBlueGreenDeploy),
			Spec: &commonmodels.JobTaskBlueGreenDeploySpec{
				Namespace:          j.spec.Namespace,
				ClusterID:          j.spec.ClusterID,
				DockerRegistryID:   j.spec.DockerRegistryID,
				DeployTimeout:      target.DeployTimeout,
				K8sServiceName:     target.K8sServiceName,
				BlueK8sServiceName: target.BlueK8sServiceName,
				WorkloadType:       setting.Deployment,
				WorkloadName:       deployment.Name,
				BlueWorkloadName:   target.BlueWorkloadName,
				ContainerName:      target.ContainerName,
				Image:              target.Image,
				Version:            version,
			},
		}
		resp = append(resp, task)
	}

	j.job.Spec = j.spec
	return resp, nil
}

func (j *BlueGreenDeployJob) LintJob() error {
	j.spec = &commonmodels.BlueGreenDeployJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	quoteJobs := []*commonmodels.Job{}
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobK8sBlueGreenRelease {
				continue
			}
			releaseJobSpec := &commonmodels.BlueGreenReleaseJobSpec{}
			if err := commonmodels.IToiYaml(job.Spec, releaseJobSpec); err != nil {
				return err
			}
			if releaseJobSpec.FromJob == j.job.Name {
				quoteJobs = append(quoteJobs, job)
			}
		}
	}
	if len(quoteJobs) == 0 {
		return fmt.Errorf("no blue-green release job quote blue-green deploy job %s", j.job.Name)
	}
	if len(quoteJobs) > 1 {
		return fmt.Errorf("more than one blue-green release job quote blue-green deploy job %s", j.job.Name)
	}
	jobRankmap := getJobRankMap(j.workflow.Stages)
	if jobRankmap[j.job.Name] >= jobRankmap[quoteJobs[0].Name] {
		return fmt.Errorf("blue-green release job %s should run before blue-green deploy job %s", quoteJobs[0].Name, j.job.Name)
	}
	return nil
}

func getBlueWorkloadName(name, version string) string {
	reg, _ := regexp.Compile("v[0-9]{10}$")
	blueWorkfloadName := reg.ReplaceAllString(name, version)
	if blueWorkfloadName == name {
		blueWorkfloadName = name + "-" + version
	}
	return blueWorkfloadName
}
