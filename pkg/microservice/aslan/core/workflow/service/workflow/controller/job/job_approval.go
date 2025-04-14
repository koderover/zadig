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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type ApprovalJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ApprovalJobSpec
}

func CreateApprovalJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (*ApprovalJobController, error) {
	spec := new(commonmodels.ApprovalJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create apollo job controller, error: %s", err)
	}

	basicInfo := &BasicInfo{
		name:        job.Name,
		jobType:     job.JobType,
		errorPolicy: job.ErrorPolicy,
		workflow:    workflow,
	}

	return &ApprovalJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j ApprovalJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j ApprovalJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j ApprovalJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.ApprovalJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode approval job spec, error: %s", err)
	}

	if err := util.CheckZadigProfessionalLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}

	return nil
}

func (j ApprovalJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	latestJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	latestJobSpec := new(commonmodels.ApprovalJobSpec)
	if err := commonmodels.IToi(latestJob.Spec, latestJobSpec); err != nil {
		return fmt.Errorf("failed to decode approval job spec, error: %s", err)
	}

	if latestJobSpec.NativeApproval != nil && j.jobSpec.NativeApproval != nil {
		latestJobSpec.NativeApproval.ApproveUsers = j.jobSpec.NativeApproval.ApproveUsers
	}
	if latestJobSpec.LarkApproval != nil && j.jobSpec.LarkApproval != nil {
		latestJobSpec.LarkApproval.ApprovalNodes = j.jobSpec.LarkApproval.ApprovalNodes
	}
	if latestJobSpec.DingTalkApproval != nil && j.jobSpec.DingTalkApproval != nil {
		latestJobSpec.DingTalkApproval.ApprovalNodes = j.jobSpec.DingTalkApproval.ApprovalNodes
	}
	if latestJobSpec.WorkWXApproval != nil && j.jobSpec.WorkWXApproval != nil {
		latestJobSpec.WorkWXApproval.ApprovalNodes = j.jobSpec.WorkWXApproval.ApprovalNodes
	}

	return nil
}
