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
	"strconv"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

type GrayRollbackJobController struct {
	*BasicInfo

	jobSpec *commonmodels.GrayRollbackJobSpec
}

func CreateGrayRollbackJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.GrayRollbackJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return GrayRollbackJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j GrayRollbackJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j GrayRollbackJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j GrayRollbackJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j GrayRollbackJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.GrayRollbackJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.ClusterID = currJobSpec.ClusterID
	j.jobSpec.Namespace = currJobSpec.Namespace
	j.jobSpec.RollbackTimeout = currJobSpec.RollbackTimeout

	return nil
}

func (j GrayRollbackJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(j.jobSpec.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kube client, err: %v", err)
	}
	newTargets := make([]*commonmodels.GrayRollbackTarget, 0)
	deployments, err := getter.ListDeployments(j.jobSpec.Namespace, nil, kubeClient)
	if err != nil {
		return err
	}
	for _, deployment := range deployments {
		rollbackInfo, err := getGrayRollbackInfoFromAnnotations(deployment.GetAnnotations())
		if err != nil {
			log.Warnf("deployment %s get gray rollback info failed: %v", deployment.Name, err)
			continue
		}
		target := &commonmodels.GrayRollbackTarget{
			WorkloadType:  "Deployment",
			WorkloadName:  deployment.Name,
			OriginImage:   rollbackInfo.image,
			OriginReplica: rollbackInfo.replica,
		}

		newTargets = append(newTargets, target)
	}

	j.jobSpec.TargetOptions = newTargets

	return nil
}

func (j GrayRollbackJobController) ClearOptions() {
	return
}

func (j GrayRollbackJobController) ClearSelection() {
	j.jobSpec.Targets = make([]*commonmodels.GrayRollbackTarget, 0)
}

func (j GrayRollbackJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	cluster, err := commonrepo.NewK8SClusterColl().Get(j.jobSpec.ClusterID)
	if err != nil {
		return resp, fmt.Errorf("cluster id: %s not found", j.jobSpec.ClusterID)
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(j.jobSpec.ClusterID)
	if err != nil {
		return resp, fmt.Errorf("failed to get kube client, err: %v", err)
	}
	for _, target := range j.jobSpec.Targets {
		deployment, found, err := getter.GetDeployment(j.jobSpec.Namespace, target.WorkloadName, kubeClient)
		if err != nil || !found {
			return resp, fmt.Errorf("deployment %s not found in namespace: %s", target.WorkloadName, j.jobSpec.Namespace)
		}
		rollbackInfo, err := getGrayRollbackInfoFromAnnotations(deployment.GetAnnotations())
		if err != nil {
			return resp, fmt.Errorf("deployment %s get gray rollback info failed: %v", target.WorkloadName, err)
		}
		jobTask := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, 0),
			Key:         genJobKey(j.name, target.WorkloadName),
			DisplayName: genJobDisplayName(j.name, target.WorkloadName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:      j.name,
				"workload_name": target.WorkloadName,
			},
			JobType: string(config.JobK8sGrayRollback),
			Spec: &commonmodels.JobTaskGrayRollbackSpec{
				ClusterID:        j.jobSpec.ClusterID,
				ClusterName:      cluster.Name,
				Namespace:        j.jobSpec.Namespace,
				WorkloadType:     target.WorkloadType,
				WorkloadName:     target.WorkloadName,
				ContainerName:    rollbackInfo.containerName,
				GrayWorkloadName: target.WorkloadName + config.GrayDeploymentSuffix,
				Image:            rollbackInfo.image,
				RollbackTimeout:  j.jobSpec.RollbackTimeout,
				TotalReplica:     rollbackInfo.replica,
			},
			ErrorPolicy:   j.errorPolicy,
			ExecutePolicy: j.executePolicy,
		}
		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j GrayRollbackJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j GrayRollbackJobController) SetRepoCommitInfo() error {
	return nil
}

func (j GrayRollbackJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j GrayRollbackJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j GrayRollbackJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j GrayRollbackJobController) IsServiceTypeJob() bool {
	return false
}

type grayRollbackInfo struct {
	image         string
	replica       int
	containerName string
}

func getGrayRollbackInfoFromAnnotations(annotations map[string]string) (*grayRollbackInfo, error) {
	image, ok := annotations[config.GrayImageAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("deployment annotations has no zadig gray image info")
	}
	containerName, ok := annotations[config.GrayContainerAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("deployment annotations has no zadig gray container info")
	}
	replicaStr, ok := annotations[config.GrayReplicaAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("deployment annotations has no zadig gray replica info")
	}
	replica, err := strconv.Atoi(replicaStr)
	if err != nil {
		return nil, fmt.Errorf("annotation replica: %s is not a number", replicaStr)
	}

	return &grayRollbackInfo{
		image:         image,
		replica:       replica,
		containerName: containerName,
	}, nil
}
