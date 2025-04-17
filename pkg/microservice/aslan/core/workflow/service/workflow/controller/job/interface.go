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

	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

const (
	JobNameKey = "job_name"
)

// Job is the interface to deal with the workflow's job spec
type Job interface {
	// SetWorkflow sets the workflow into the context to render the required spec
	SetWorkflow(workflow *commonmodels.WorkflowV4)
	// GetSpec gets the modified spec to use for workflow execution/update
	GetSpec() interface{}
	// Validate checks if the given spec is a valid one (good for execution or saving)
	// @param isExecution indicates if the check is for generating workflow task.
	Validate(isExecution bool) error
	// Update function merge the given spec with the JobControllers workflow, giving an error if the merge is not possible
	// @param useUserInput will use some of the field in the spec over the workflow's spec
	Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error
	// SetOptions sets all option field to the spec, giving the frontend options to display
	SetOptions(ticket *commonmodels.ApprovalTicket) error
	// ClearOptions clears the options field, mainly for db
	ClearOptions()
	// ClearSelection clears the field that represents the user's selection
	ClearSelection()
	// ToTask creates tasks for workflow controller to run
	ToTask(taskID int64) ([]*commonmodels.JobTask, error)
	// SetRepo sets the given repo info to the spec (if applicable)
	SetRepo(repo *types.Repository) error
	// SetRepoCommitInfo ..
	SetRepoCommitInfo() error
	// GetVariableList gets the variable list by the given flags, note that most of the runtime variables will not have values so use this function with care
	GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, getReferredKeyValVariables bool) ([]*commonmodels.KeyVal, error)
	// GetUsedRepos returns the all the repos a job used
	GetUsedRepos() ([]*types.Repository, error)
	// RenderDynamicVariableOptions renders a key's value option based on given values (and parameters)
	RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error)
}

type BasicInfo struct {
	name        string
	jobType     config.JobType
	errorPolicy *commonmodels.JobErrorPolicy

	workflow *commonmodels.WorkflowV4
}

func CreateJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	switch job.JobType {
	case config.JobApollo:
		return CreateApolloJobController(job, workflow)
	case config.JobApproval:
		return CreateApprovalJobController(job, workflow)
	case config.JobK8sBlueGreenDeploy:
		return CreateBlueGreenDeployJobController(job, workflow)
	case config.JobK8sBlueGreenRelease:
		return CreateBlueGreenReleaseJobController(job, workflow)
	case config.JobBlueKing:
		return CreateBlueKingJobController(job, workflow)
	case config.JobK8sCanaryDeploy:
		return CreateCanaryDeployJobController(job, workflow)
	case config.JobK8sCanaryRelease:
		return CreateCanaryReleaseJobController(job, workflow)
	case config.JobZadigBuild:
		return CreateBuildJobController(job, workflow)
	case config.JobCustomDeploy:
		return CreateCustomDeployJobController(job, workflow)
	case config.JobZadigDeploy:
		return CreateDeployJobController(job, workflow)
	case config.JobZadigDistributeImage:
		return CreateDistributeImageJobController(job, workflow)
	case config.JobFreestyle:
		return CreateFreestyleJobController(job, workflow)
	case config.JobGrafana:
		return CreateGrafanaJobJobController(job, workflow)
	case config.JobK8sGrayRelease:
		return CreateGrayReleaseJobController(job, workflow)
	case config.JobK8sGrayRollback:
		return CreateGrayRollbackJobController(job, workflow)
	case config.JobZadigHelmChartDeploy:
		return CreateHelmChartDeployJobController(job, workflow)
	case config.JobIstioRelease:
		return CreateIstioReleaseJobController(job, workflow)
	case config.JobIstioRollback:
		return CreateIstioRollbackJobController(job, workflow)
	case config.JobJenkins:
		return CreateJenkinsJobController(job, workflow)
	case config.JobJira:
		return CreateJiraJobController(job, workflow)
	case config.JobZadigScanning:
		return CreateScanningJobController(job, workflow)
	case config.JobZadigTesting:
		return CreateTestingJobController(job, workflow)
	case config.JobZadigVMDeploy:
		return CreateVMDeployJobController(job, workflow)
	default:
		return nil, fmt.Errorf("job type not supported")
	}
}
