/*
Copyright 2025 The KodeRover Authors.

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
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/types"
)

// TODO: Targets => target_options in configuration

type CanaryDeployJobController struct {
	*BasicInfo

	jobSpec *commonmodels.CanaryDeployJobSpec
}

func CreateCanaryDeployJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.CanaryDeployJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return CanaryDeployJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j CanaryDeployJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j CanaryDeployJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j CanaryDeployJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	quoteJobs := make([]*commonmodels.Job, 0)
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobK8sCanaryRelease {
				continue
			}
			releaseJobSpec := &commonmodels.CanaryReleaseJobSpec{}
			if err := commonmodels.IToiYaml(job.Spec, releaseJobSpec); err != nil {
				return err
			}
			if releaseJobSpec.FromJob == j.name {
				quoteJobs = append(quoteJobs, job)
			}
		}
	}
	if len(quoteJobs) == 0 {
		return fmt.Errorf("no canary release job quote canary deploy job %s", j.name)
	}
	if len(quoteJobs) > 1 {
		return fmt.Errorf("more than one canary release job quote canary deploy job %s", j.name)
	}
	jobRankMap := GetJobRankMap(j.workflow.Stages)
	if jobRankMap[j.name] >= jobRankMap[quoteJobs[0].Name] {
		return fmt.Errorf("canary release job %s should run before canary deploy job %s", quoteJobs[0].Name, j.name)
	}

	configuredContainerMap := make(map[string]*commonmodels.CanaryTarget)
	for _, container := range j.jobSpec.TargetOptions {
		if _, ok := configuredContainerMap[container.ContainerName]; ok {
			return fmt.Errorf("duplicated container name [%s] in options", container.ContainerName)
		}
		configuredContainerMap[container.ContainerName] = container
	}

	if isExecution {
		latestJob, err := j.workflow.FindJob(j.name, j.jobType)
		if err != nil {
			return fmt.Errorf("failed to find job: %s in workflow %s's latest config, error: %s", j.name, j.workflow.Name, err)
		}

		currJobSpec := new(commonmodels.CanaryDeployJobSpec)
		if err := commonmodels.IToi(latestJob.Spec, currJobSpec); err != nil {
			return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
		}

		configuredContainerMap := make(map[string]*commonmodels.CanaryTarget)
		for _, container := range currJobSpec.TargetOptions {
			if _, ok := configuredContainerMap[container.ContainerName]; ok {
				return fmt.Errorf("duplicated container name [%s] in options", container.ContainerName)
			}
			configuredContainerMap[container.ContainerName] = container
		}

		for _, container := range j.jobSpec.Targets {
			if _, ok := configuredContainerMap[container.ContainerName]; !ok {
				return fmt.Errorf("selected container [%s] not configured in the options", container.ContainerName)
			}
		}
	}
	return nil
}

func (j CanaryDeployJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.CanaryDeployJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	if useUserInput {
		if j.jobSpec.DockerRegistryID != currJobSpec.DockerRegistryID {
			return fmt.Errorf("docker registry has been changed, the new registry id in the config is: %s", currJobSpec.DockerRegistryID)
		}
		if j.jobSpec.Namespace != currJobSpec.Namespace {
			return fmt.Errorf("namespace has been changed, the new namespace in the config is: %s", currJobSpec.Namespace)
		}
		if j.jobSpec.ClusterID != currJobSpec.ClusterID {
			return fmt.Errorf("cluster has been changed, the new cluster in the config is: %s", currJobSpec.ClusterID)
		}
	}

	// TODO: recalculate the resources in the namespace, for now we just use the configured options
	j.jobSpec.TargetOptions = currJobSpec.TargetOptions
	return nil
}

func (j CanaryDeployJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

// ClearOptions does nothing since the option field happens to be the user configured field, clear it would cause problems
func (j CanaryDeployJobController) ClearOptions() {
	return
}

func (j CanaryDeployJobController) ClearSelection() {
	j.jobSpec.Targets = make([]*commonmodels.CanaryTarget, 0)
	return
}

func (j CanaryDeployJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(j.jobSpec.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get kube client: %s, err: %v", j.jobSpec.ClusterID, err)
	}

	for jobSubTaskID, target := range j.jobSpec.Targets {
		service, exist, err := getter.GetService(j.jobSpec.Namespace, target.K8sServiceName, kubeClient)
		if err != nil || !exist {
			return nil, fmt.Errorf("failed to get service, err: %v", err)
		}
		if service.Spec.ClusterIP == "None" {
			return nil, fmt.Errorf("service :%s was a headless service, which canry deployment do not support", err)
		}
		selector := labels.Set(service.Spec.Selector).AsSelector()
		deployments, err := getter.ListDeployments(j.jobSpec.Namespace, selector, kubeClient)
		if err != nil {
			return nil, fmt.Errorf("list deployments error: %v", err)
		}
		if len(deployments) == 0 {
			return nil, fmt.Errorf("no deployment found")
		}
		if len(deployments) > 1 {
			return nil, fmt.Errorf("more than one deployment found")
		}
		deployment := deployments[0]
		target.WorkloadName = deployment.Name
		target.WorkloadType = setting.Deployment
		canaryReplica := math.Ceil(float64(*deployment.Spec.Replicas) * (float64(target.CanaryPercentage) / 100))
		task := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, jobSubTaskID),
			Key:         genJobKey(j.name, target.K8sServiceName),
			DisplayName: genJobDisplayName(j.name, target.K8sServiceName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:         j.name,
				"k8s_service_name": target.K8sServiceName,
			},
			JobType: string(config.JobK8sCanaryDeploy),
			Spec: &commonmodels.JobTaskCanaryDeploySpec{
				Namespace:        j.jobSpec.Namespace,
				ClusterID:        j.jobSpec.ClusterID,
				DockerRegistryID: j.jobSpec.DockerRegistryID,
				DeployTimeout:    target.DeployTimeout,
				K8sServiceName:   target.K8sServiceName,
				WorkloadType:     setting.Deployment,
				WorkloadName:     deployment.Name,
				ContainerName:    target.ContainerName,
				CanaryPercentage: target.CanaryPercentage,
				CanaryReplica:    int(canaryReplica),
				Image:            target.Image,
			},
			ErrorPolicy: j.errorPolicy,
		}
		resp = append(resp, task)
	}

	return resp, nil
}

func (j CanaryDeployJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j CanaryDeployJobController) SetRepoCommitInfo() error {
	return nil
}

func (j CanaryDeployJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j CanaryDeployJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j CanaryDeployJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}
