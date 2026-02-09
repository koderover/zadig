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

// TODO: jobs => job_options

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/types"
)

type JenkinsJobController struct {
	*BasicInfo

	jobSpec *commonmodels.JenkinsJobSpec
}

func CreateJenkinsJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.JenkinsJobSpec)
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

	return JenkinsJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j JenkinsJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j JenkinsJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j JenkinsJobController) Validate(isExecution bool) error {
	if _, err := mongodb.NewCICDToolColl().Get(j.jobSpec.ID); err != nil {
		return fmt.Errorf("not found Jenkins in mongo, err: %v", err)
	}

	return nil
}

func (j JenkinsJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.JenkinsJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}
	j.errorPolicy = currJob.ErrorPolicy
	j.executePolicy = currJob.ExecutePolicy

	j.jobSpec.ID = currJobSpec.ID
	if useUserInput {
		newJobs := make([]*commonmodels.JenkinsJobInfo, 0)
		configuredJobMap := make(map[string]*commonmodels.JenkinsJobInfo)
		for _, option := range currJobSpec.JobOptions {
			configuredJobMap[option.JobName] = option
		}

		for _, selectedJob := range j.jobSpec.Jobs {
			if configuredJob, ok := configuredJobMap[selectedJob.JobName]; !ok {
				continue
			} else {
				newJobs = append(newJobs, &commonmodels.JenkinsJobInfo{
					JobName:    selectedJob.JobName,
					Parameters: util.ApplyJenkinsParameter(configuredJob.Parameters, selectedJob.Parameters),
				})
			}
		}

		j.jobSpec.Jobs = newJobs
	}

	return nil
}

func (j JenkinsJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j JenkinsJobController) ClearOptions() {
	return
}

func (j JenkinsJobController) ClearSelection() {
	j.jobSpec.Jobs = make([]*commonmodels.JenkinsJobInfo, 0)
}

func (j JenkinsJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	if len(j.jobSpec.Jobs) == 0 {
		return nil, fmt.Errorf("jenkins job list is empty")
	}
	for _, job := range j.jobSpec.Jobs {
		resp = append(resp, &commonmodels.JobTask{
			Name:        GenJobName(j.workflow, j.name, 0),
			Key:         genJobKey(j.name, job.JobName),
			DisplayName: genJobDisplayName(j.name, job.JobName),
			OriginName:  j.name,
			JobInfo: map[string]string{
				JobNameKey:         j.name,
				"jenkins_job_name": job.JobName,
			},
			JobType: string(config.JobJenkins),
			Spec: &commonmodels.JobTaskJenkinsSpec{
				ID: j.jobSpec.ID,
				Job: commonmodels.JobTaskJenkinsJobInfo{
					JobName:    job.JobName,
					Parameters: job.Parameters,
				},
			},
			Timeout:       0,
			ErrorPolicy:   j.errorPolicy,
			ExecutePolicy: j.executePolicy,
		})
	}

	return resp, nil
}

func (j JenkinsJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j JenkinsJobController) SetRepoCommitInfo() error {
	return nil
}

func (j JenkinsJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
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

func (j JenkinsJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j JenkinsJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j JenkinsJobController) IsServiceTypeJob() bool {
	return false
}
