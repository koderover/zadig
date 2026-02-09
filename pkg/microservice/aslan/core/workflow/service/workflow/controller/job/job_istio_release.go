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
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/types"
)

type IstioReleaseJobController struct {
	*BasicInfo

	jobSpec *commonmodels.IstioJobSpec
}

func CreateIstioReleaseJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.IstioJobSpec)
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

	return IstioReleaseJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j IstioReleaseJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j IstioReleaseJobController) GetSpec() interface{} {
	return j.jobSpec
}

type lintIstioReleaseJob struct {
	jobName string
	weight  int64
}

func (j IstioReleaseJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	if j.jobSpec.Weight > 100 {
		return fmt.Errorf("istio release job: [%s] weight cannot be more than 100", j.name)
	}

	//from job was empty means it is the first deploy job.
	if j.jobSpec.FromJob == "" {
		jobRankmap := GetJobRankMap(j.workflow.Stages)
		releaseJobs := make([]*lintIstioReleaseJob, 0)
		for _, stage := range j.workflow.Stages {
			for _, job := range stage.Jobs {
				if job.JobType != config.JobIstioRelease {
					continue
				}
				jobSpec := &commonmodels.IstioJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, jobSpec); err != nil {
					return err
				}
				if jobSpec.FromJob != j.name {
					continue
				}
				releaseJobs = append(releaseJobs, &lintIstioReleaseJob{jobName: job.Name, weight: jobSpec.Weight})
			}
		}
		if len(releaseJobs) == 0 {
			return fmt.Errorf("no release job found for job [%s]", j.name)
		}
		for i, releaseJob := range releaseJobs {
			if jobRankmap[j.name] >= jobRankmap[releaseJob.jobName] {
				return fmt.Errorf("istio release job: [%s] must be run before [%s]", j.name, releaseJob.jobName)
			}
			if i < len(releaseJobs)-1 && releaseJob.weight >= 100 {
				return fmt.Errorf("istio release job: [%s] cannot full release in the middle", releaseJob.jobName)
			}
			if i == len(releaseJobs)-1 && releaseJob.weight != 100 {
				return fmt.Errorf("istio last release job: [%s] must be full released", releaseJob.jobName)
			}
		}
		return nil
	}

	var quoteJobSpec *commonmodels.IstioJobSpec
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobIstioRelease || job.Name != j.jobSpec.FromJob {
				continue
			}
			quoteJobSpec = &commonmodels.IstioJobSpec{}
			if err := commonmodels.IToiYaml(job.Spec, quoteJobSpec); err != nil {
				return err
			}
			break
		}
	}

	if quoteJobSpec == nil {
		return fmt.Errorf("[%s] quote istio relase job: [%s] not found", j.name, j.jobSpec.FromJob)
	}
	if quoteJobSpec.FromJob != "" {
		return fmt.Errorf("[%s] cannot quote a non-first-release job [%s]", j.name, j.jobSpec.FromJob)
	}
	return nil
}

func (j IstioReleaseJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.IstioJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}
	j.errorPolicy = currJob.ErrorPolicy
	j.executePolicy = currJob.ExecutePolicy

	j.jobSpec.ClusterID = currJobSpec.ClusterID
	j.jobSpec.ClusterSource = currJobSpec.ClusterSource
	j.jobSpec.FromJob = currJobSpec.FromJob
	j.jobSpec.RegistryID = currJobSpec.RegistryID
	j.jobSpec.Namespace = currJobSpec.Namespace
	j.jobSpec.Timeout = currJobSpec.Timeout
	j.jobSpec.ReplicaPercentage = currJobSpec.ReplicaPercentage
	j.jobSpec.Weight = currJobSpec.Weight
	j.jobSpec.TargetOptions = currJobSpec.TargetOptions

	if !useUserInput {
		j.jobSpec.Targets = currJobSpec.Targets
	}

	return nil
}

func (j IstioReleaseJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j IstioReleaseJobController) ClearOptions() {
	return
}

func (j IstioReleaseJobController) ClearSelection() {
	return
}

func (j IstioReleaseJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	// if from job is empty, it was the first deploy Job.
	firstJob := false
	if j.jobSpec.FromJob != "" {
		if j.jobSpec.Weight > 100 {
			return resp, fmt.Errorf("istio release job: %s release percentage cannot largger than 100", j.name)
		}
		found := false
		for _, stage := range j.workflow.Stages {
			for _, job := range stage.Jobs {
				if job.Name != j.jobSpec.FromJob || job.JobType != config.JobIstioRelease {
					continue
				}
				found = true
				fromJobSpec := &commonmodels.IstioJobSpec{}
				if err := commonmodels.IToi(job.Spec, fromJobSpec); err != nil {
					return resp, err
				}
				j.jobSpec.ClusterID = fromJobSpec.ClusterID
				j.jobSpec.Namespace = fromJobSpec.Namespace
				j.jobSpec.RegistryID = fromJobSpec.RegistryID
				j.jobSpec.Targets = fromJobSpec.Targets
			}
		}
		if !found {
			return resp, fmt.Errorf("gray release job: %s not found", j.jobSpec.FromJob)
		}
	} else {
		firstJob = true
		if j.jobSpec.Weight >= 100 {
			return resp, fmt.Errorf("the first istio release job: %s cannot be released in full", j.name)
		}
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
		target.CurrentReplica = int(*deployment.Spec.Replicas)
	}

	cluster, err := commonrepo.NewK8SClusterColl().Get(j.jobSpec.ClusterID)
	if err != nil {
		return resp, fmt.Errorf("cluster id: %s not found", j.jobSpec.ClusterID)
	}

	for _, target := range j.jobSpec.Targets {
		newReplicaCount := math.Ceil(float64(target.CurrentReplica) * (float64(j.jobSpec.ReplicaPercentage) / 100))
		jobTask := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, 0),
			Key:         genJobKey(j.name, target.WorkloadName),
			DisplayName: genJobDisplayName(j.name, target.WorkloadName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:      j.name,
				"workload_name": target.WorkloadName,
			},
			JobType: string(config.JobIstioRelease),
			Spec: &commonmodels.JobIstioReleaseSpec{
				FirstJob:          firstJob,
				ClusterID:         j.jobSpec.ClusterID,
				ClusterName:       cluster.Name,
				Namespace:         j.jobSpec.Namespace,
				Weight:            j.jobSpec.Weight,
				Timeout:           j.jobSpec.Timeout,
				ReplicaPercentage: j.jobSpec.ReplicaPercentage,
				Replicas:          int64(newReplicaCount),
				Targets:           target,
			},
			ErrorPolicy:   j.errorPolicy,
			ExecutePolicy: j.executePolicy,
		}
		resp = append(resp, jobTask)
	}

	return resp, nil
}

func (j IstioReleaseJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j IstioReleaseJobController) SetRepoCommitInfo() error {
	return nil
}

func (j IstioReleaseJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)
	if getRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
	}
	return resp, nil
}

func (j IstioReleaseJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j IstioReleaseJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j IstioReleaseJobController) IsServiceTypeJob() bool {
	return false
}
