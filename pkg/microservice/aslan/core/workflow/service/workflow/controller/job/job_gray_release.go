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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/types"
)

// TODO: target -> target_options

type GrayReleaseJobController struct {
	*BasicInfo

	jobSpec *commonmodels.GrayReleaseJobSpec
}

func CreateGrayReleaseJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.GrayReleaseJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return GrayReleaseJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j GrayReleaseJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j GrayReleaseJobController) GetSpec() interface{} {
	return j.jobSpec
}

type validateGrayReleaseJob struct {
	jobName   string
	GrayScale int
}

func (j GrayReleaseJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	// lint the first release job
	if j.jobSpec.FromJob == "" {
		jobRankMap := GetJobRankMap(j.workflow.Stages)
		releaseJobs := make([]*validateGrayReleaseJob, 0)
		for _, stage := range j.workflow.Stages {
			for _, job := range stage.Jobs {
				if job.JobType != config.JobK8sGrayRelease {
					continue
				}
				jobSpec := &commonmodels.GrayReleaseJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, jobSpec); err != nil {
					return err
				}
				if jobSpec.FromJob != j.name {
					continue
				}
				releaseJobs = append(releaseJobs, &validateGrayReleaseJob{jobName: job.Name, GrayScale: jobSpec.GrayScale})
			}
		}
		if len(releaseJobs) == 0 {
			return fmt.Errorf("no release job found for job [%s]", j.name)
		}
		for i, releaseJob := range releaseJobs {
			if jobRankMap[j.name] >= jobRankMap[releaseJob.jobName] {
				return fmt.Errorf("release job: [%s] must be run before [%s]", j.name, releaseJob.jobName)
			}
			if i < len(releaseJobs)-1 && releaseJob.GrayScale >= 100 {
				return fmt.Errorf("release job: [%s] cannot full release in the middle", releaseJob.jobName)
			}
			if i == len(releaseJobs)-1 && releaseJob.GrayScale != 100 {
				return fmt.Errorf("last release job: [%s] must be full released", releaseJob.jobName)
			}
		}

		return nil
	}

	var quoteJobSpec *commonmodels.GrayReleaseJobSpec
	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != config.JobK8sGrayRelease || job.Name != j.jobSpec.FromJob {
				continue
			}
			quoteJobSpec = &commonmodels.GrayReleaseJobSpec{}
			if err := commonmodels.IToiYaml(job.Spec, quoteJobSpec); err != nil {
				return err
			}
			break
		}
	}
	if quoteJobSpec == nil {
		return fmt.Errorf("[%s] quote release job: [%s] not found", j.name, j.jobSpec.FromJob)
	}
	if quoteJobSpec.FromJob != "" {
		return fmt.Errorf("[%s] cannot quote a non-first-release job [%s]", j.name, j.jobSpec.FromJob)
	}

	return nil
}

func (j GrayReleaseJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.GrayReleaseJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.ClusterID = currJobSpec.ClusterID
	j.jobSpec.Namespace = currJobSpec.Namespace
	j.jobSpec.DockerRegistryID = currJobSpec.DockerRegistryID
	j.jobSpec.FromJob = currJobSpec.FromJob
	j.jobSpec.DeployTimeout = currJobSpec.DeployTimeout
	j.jobSpec.GrayScale = currJobSpec.GrayScale
	j.jobSpec.TargetOptions = currJobSpec.TargetOptions
	return nil
}

func (j GrayReleaseJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j GrayReleaseJobController) ClearOptions() {
	return
}

func (j GrayReleaseJobController) ClearSelection() {
	j.jobSpec.Targets = make([]*commonmodels.GrayReleaseTarget, 0)
	return
}

func (j GrayReleaseJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)
	firstJob := false
	if j.jobSpec.FromJob != "" {
		if j.jobSpec.GrayScale > 100 {
			return resp, fmt.Errorf("release job: %s release percentage cannot largger than 100", j.name)
		}
		found := false
		for _, stage := range j.workflow.Stages {
			for _, job := range stage.Jobs {
				if job.Name != j.jobSpec.FromJob || job.JobType != config.JobK8sGrayRelease {
					continue
				}
				found = true
				fromJobSpec := &commonmodels.GrayReleaseJobSpec{}
				if err := commonmodels.IToi(job.Spec, fromJobSpec); err != nil {
					return resp, err
				}
				j.jobSpec.ClusterID = fromJobSpec.ClusterID
				j.jobSpec.Namespace = fromJobSpec.Namespace
				j.jobSpec.DockerRegistryID = fromJobSpec.DockerRegistryID
				j.jobSpec.Targets = fromJobSpec.Targets
			}
		}
		if !found {
			return resp, fmt.Errorf("gray release job: %s not found", j.jobSpec.FromJob)
		}
	} else {
		firstJob = true
		if j.jobSpec.GrayScale >= 100 {
			return resp, fmt.Errorf("the first release job: %s cannot be released in full", j.name)
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
			target.Replica = int(*deployment.Spec.Replicas)
		}
	}

	cluster, err := commonrepo.NewK8SClusterColl().Get(j.jobSpec.ClusterID)
	if err != nil {
		return resp, fmt.Errorf("cluster id: %s not found", j.jobSpec.ClusterID)
	}

	for _, target := range j.jobSpec.Targets {
		grayReplica := math.Ceil(float64(*&target.Replica) * (float64(j.jobSpec.GrayScale) / 100))
		jobTask := &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, 0),
			Key:         genJobKey(j.name, target.WorkloadName),
			DisplayName: genJobDisplayName(j.name, target.WorkloadName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:      j.name,
				"workload_name": target.WorkloadName,
			},
			JobType: string(config.JobK8sGrayRelease),
			Spec: &commonmodels.JobTaskGrayReleaseSpec{
				ClusterID:        j.jobSpec.ClusterID,
				ClusterName:      cluster.Name,
				Namespace:        j.jobSpec.Namespace,
				WorkloadType:     target.WorkloadType,
				WorkloadName:     target.WorkloadName,
				ContainerName:    target.ContainerName,
				FirstJob:         firstJob,
				GrayWorkloadName: target.WorkloadName + config.GrayDeploymentSuffix,
				Image:            target.Image,
				DeployTimeout:    j.jobSpec.DeployTimeout,
				GrayScale:        j.jobSpec.GrayScale,
				TotalReplica:     target.Replica,
				GrayReplica:      int(grayReplica),
			},
			ErrorPolicy: j.errorPolicy,
		}
		resp = append(resp, jobTask)
	}
	return resp, nil
}

func (j GrayReleaseJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j GrayReleaseJobController) SetRepoCommitInfo() error {
	return nil
}

func (j GrayReleaseJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j GrayReleaseJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j GrayReleaseJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j GrayReleaseJobController) IsServiceTypeJob() bool {
	return false
}
