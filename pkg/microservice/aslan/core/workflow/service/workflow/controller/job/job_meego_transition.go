/*
 * Copyright 2025 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package job

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/types"
)

type MeegoTransitionJobController struct {
	*BasicInfo

	jobSpec *commonmodels.MeegoTransitionJobSpec
}

func CreateMeegoTransitionJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.MeegoTransitionJobSpec)
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

	return MeegoTransitionJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j MeegoTransitionJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j MeegoTransitionJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j MeegoTransitionJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigEnterpriseLicense(); err != nil {
		return err
	}

	return nil
}

func (j MeegoTransitionJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.MeegoTransitionJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.Source = currJobSpec.Source
	j.jobSpec.ProjectKey = currJobSpec.ProjectKey
	j.jobSpec.ProjectName = currJobSpec.ProjectName
	j.jobSpec.MeegoID = currJobSpec.MeegoID
	j.jobSpec.MeegoSystemIdentity = currJobSpec.MeegoSystemIdentity
	j.jobSpec.MeegoURL = currJobSpec.MeegoURL
	j.jobSpec.WorkItemType = currJobSpec.WorkItemType
	j.jobSpec.WorkItemTypeKey = currJobSpec.WorkItemTypeKey

	return nil
}

func (j MeegoTransitionJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j MeegoTransitionJobController) ClearOptions() {
	return
}

func (j MeegoTransitionJobController) ClearSelection() {
	j.jobSpec.StatusWorkItems = make([]*commonmodels.MeegoWorkItemTransition, 0)
}

func (j MeegoTransitionJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	spec, err := commonrepo.NewProjectManagementColl().GetMeegoSpec(j.jobSpec.MeegoID)
	if err != nil {
		return nil, fmt.Errorf("failed to find meego integration info")
	}
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobMeegoTransition),
		Spec: &commonmodels.MeegoTransitionSpec{
			MeegoID:         j.jobSpec.MeegoID,
			Link:            spec.MeegoHost,
			Source:          j.jobSpec.Source,
			ProjectKey:      j.jobSpec.ProjectKey,
			ProjectName:     j.jobSpec.ProjectName,
			WorkItemType:    j.jobSpec.WorkItemType,
			WorkItemTypeKey: j.jobSpec.WorkItemTypeKey,
			StatusWorkItems: j.jobSpec.StatusWorkItems,
			NodeWorkItems:   j.jobSpec.NodeWorkItems,
		},
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	}
	resp = append(resp, jobTask)

	return resp, nil
}

func (j MeegoTransitionJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j MeegoTransitionJobController) SetRepoCommitInfo() error {
	return nil
}

func (j MeegoTransitionJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
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

func (j MeegoTransitionJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j MeegoTransitionJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j MeegoTransitionJobController) IsServiceTypeJob() bool {
	return false
}
