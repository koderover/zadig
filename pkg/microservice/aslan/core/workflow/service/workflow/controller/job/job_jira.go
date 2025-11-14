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
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/types"
)

type JiraJobController struct {
	*BasicInfo

	jobSpec *commonmodels.JiraJobSpec
}

func CreateJiraJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.JiraJobSpec)
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

	return JiraJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j JiraJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j JiraJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j JiraJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigEnterpriseLicense(); err != nil {
		return err
	}

	return nil
}

func (j JiraJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.JiraJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.ProjectID = currJobSpec.ProjectID
	j.jobSpec.JiraID = currJobSpec.JiraID
	j.jobSpec.JiraSystemIdentity = currJobSpec.JiraSystemIdentity
	j.jobSpec.JiraURL = currJobSpec.JiraURL
	j.jobSpec.QueryType = currJobSpec.QueryType
	j.jobSpec.JQL = currJobSpec.JQL
	j.jobSpec.IssueType = currJobSpec.IssueType
	j.jobSpec.TargetStatus = currJobSpec.TargetStatus

	if !useUserInput {
		j.jobSpec.Issues = currJobSpec.Issues
	}

	return nil
}

func (j JiraJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j JiraJobController) ClearOptions() {
	return
}

func (j JiraJobController) ClearSelection() {
	return
}

func (j JiraJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)
	if len(j.jobSpec.Issues) == 0 {
		return nil, fmt.Errorf("需要指定至少一个 Jira Issue")
	}

	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(j.jobSpec.JiraID)
	if err != nil {
		return nil, fmt.Errorf("get jira spec error: %v", err)
	}
	for _, issue := range j.jobSpec.Issues {
		issue.Link = fmt.Sprintf("%s/browse/%s", spec.JiraHost, issue.Key)
	}
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobJira),
		Spec: &commonmodels.JobTaskJiraSpec{
			ProjectID:    j.jobSpec.ProjectID,
			JiraID:       j.jobSpec.JiraID,
			IssueType:    j.jobSpec.IssueType,
			Issues:       j.jobSpec.Issues,
			TargetStatus: j.jobSpec.TargetStatus,
		},
		Timeout:       0,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}

	resp = append(resp, jobTask)

	return resp, nil
}

func (j JiraJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j JiraJobController) SetRepoCommitInfo() error {
	return nil
}

func (j JiraJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
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

func (j JiraJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j JiraJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j JiraJobController) IsServiceTypeJob() bool {
	return false
}
