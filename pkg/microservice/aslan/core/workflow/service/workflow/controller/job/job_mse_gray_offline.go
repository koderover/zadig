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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

type MseGrayOfflineJobController struct {
	*BasicInfo

	jobSpec *commonmodels.MseGrayOfflineJobSpec
}

func CreateMseGrayOfflineJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.MseGrayOfflineJobSpec)
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

	return MseGrayOfflineJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j MseGrayOfflineJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j MseGrayOfflineJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j MseGrayOfflineJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	if j.jobSpec.GrayTag == types.ZadigReleaseVersionOriginal {
		return fmt.Errorf("gray tag must not be 'original'")
	}

	return nil
}

func (j MseGrayOfflineJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.MseGrayOfflineJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode apollo job spec, error: %s", err)
	}

	j.jobSpec.Source = currJobSpec.Source
	j.jobSpec.Production = currJobSpec.Production
	return nil
}

func (j MseGrayOfflineJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j MseGrayOfflineJobController) ClearOptions() {
	return
}

func (j MseGrayOfflineJobController) ClearSelection() {
	j.jobSpec.GrayTag = ""
}

func (j MseGrayOfflineJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := make([]*commonmodels.JobTask, 0)

	resp = append(resp, &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType: string(config.JobMseGrayOffline),
		Spec: commonmodels.JobTaskMseGrayOfflineSpec{
			Env:        j.jobSpec.Env,
			GrayTag:    j.jobSpec.GrayTag,
			Production: j.jobSpec.Production,
		},
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
	})

	return resp, nil
}

func (j MseGrayOfflineJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j MseGrayOfflineJobController) SetRepoCommitInfo() error {
	return nil
}

func (j MseGrayOfflineJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
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

func (j MseGrayOfflineJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j MseGrayOfflineJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j MseGrayOfflineJobController) IsServiceTypeJob() bool {
	return false
}
