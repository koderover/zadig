/*
Copyright 2024 The KodeRover Authors.

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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ApprovalJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ApprovalJobSpec
}

func (j *ApprovalJob) Instantiate() error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ApprovalJob) SetPreset() error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	if j.spec.Source == config.SourceFromJob {
		j.spec.OriginJobName = j.spec.JobName
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ApprovalJob) SetOptions(approvalTicket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j *ApprovalJob) ClearSelectionField() error {
	return nil
}

func (j *ApprovalJob) ClearOptions() error {
	return nil
}

func (j *ApprovalJob) UpdateWithLatestSetting() error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	latestWorkflow, err := mongodb.NewWorkflowV4Coll().Find(j.workflow.Name)
	if err != nil {
		log.Errorf("Failed to find original workflow to set options, error: %s", err)
		return err
	}

	latestSpec := new(commonmodels.ApprovalJobSpec)
	found := false
	for _, stage := range latestWorkflow.Stages {
		if !found {
			for _, job := range stage.Jobs {
				if job.Name == j.job.Name && job.JobType == j.job.JobType {
					if err := commonmodels.IToi(job.Spec, latestSpec); err != nil {
						return err
					}
					found = true
					break
				}
			}
		} else {
			break
		}
	}

	if !found {
		return fmt.Errorf("failed to find the original workflow: %s", j.workflow.Name)
	}

	if latestSpec.NativeApproval != nil && j.spec.NativeApproval != nil {
		latestSpec.NativeApproval.ApproveUsers = j.spec.NativeApproval.ApproveUsers
	}
	if latestSpec.LarkApproval != nil && j.spec.LarkApproval != nil {
		latestSpec.LarkApproval.ApprovalNodes = j.spec.LarkApproval.ApprovalNodes
	}
	if latestSpec.DingTalkApproval != nil && j.spec.DingTalkApproval != nil {
		latestSpec.DingTalkApproval.ApprovalNodes = j.spec.DingTalkApproval.ApprovalNodes
	}
	if latestSpec.WorkWXApproval != nil && j.spec.WorkWXApproval != nil {
		latestSpec.WorkWXApproval.ApprovalNodes = j.spec.WorkWXApproval.ApprovalNodes
	}

	j.job.Spec = latestSpec
	return nil
}

func (j *ApprovalJob) MergeArgs(args *commonmodels.Job) error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(args.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ApprovalJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return nil, err
	}

	nativeApproval := j.spec.NativeApproval
	if nativeApproval != nil && j.spec.Source != config.SourceFromJob {
		approvalUser, _ := util.GeneFlatUsers(nativeApproval.ApproveUsers)
		nativeApproval.ApproveUsers = approvalUser
	}

	jobSpec := &commonmodels.JobTaskApprovalSpec{
		Timeout:          j.spec.Timeout,
		Type:             j.spec.Type,
		Description:      j.spec.Description,
		NativeApproval:   nativeApproval,
		LarkApproval:     j.spec.LarkApproval,
		DingTalkApproval: j.spec.DingTalkApproval,
		WorkWXApproval:   j.spec.WorkWXApproval,
	}
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.job.Name, 0),
		Key:         genJobKey(j.job.Name),
		DisplayName: genJobDisplayName(j.job.Name),
		OriginName:  j.job.Name,
		JobInfo: map[string]string{
			JobNameKey: j.job.Name,
		},
		JobType:     string(config.JobApproval),
		Spec:        jobSpec,
		Timeout:     j.spec.Timeout,
		ErrorPolicy: j.job.ErrorPolicy,
	}

	if j.spec.Source == config.SourceFromJob {
		// adapt to the front end, use the direct quoted job name
		if j.spec.OriginJobName != "" {
			j.spec.JobName = j.spec.OriginJobName
		}

		serviceReferredJob := getOriginJobName(j.workflow, j.spec.JobName)
		originJobSpec, err := j.getOriginReferedJobSpec(serviceReferredJob)
		if err != nil {
			return nil, fmt.Errorf("failed to get origin refered job: %s", err)
		}

		if originJobSpec.Type != jobSpec.Type {
			return nil, fmt.Errorf("origin refered %s's job type %s is different from current %s's job type %s", serviceReferredJob, originJobSpec.Type, j.spec.JobName, j.spec.Type)
		}

		switch originJobSpec.Type {
		case config.NativeApproval:
			approvalUser, _ := util.GeneFlatUsers(originJobSpec.NativeApproval.ApproveUsers)
			jobSpec.NativeApproval.ApproveUsers = approvalUser
		case config.LarkApproval:
			if originJobSpec.LarkApproval == nil {
				return nil, fmt.Errorf("%s lark approval not found", serviceReferredJob)
			}

			if originJobSpec.LarkApproval.ID != jobSpec.LarkApproval.ID {
				return nil, fmt.Errorf("origin refered %s's lark id is different from current %s's lark id", serviceReferredJob, j.spec.JobName)
			}

			jobSpec.LarkApproval.ApprovalNodes = originJobSpec.LarkApproval.ApprovalNodes
		case config.DingTalkApproval:
			if originJobSpec.DingTalkApproval == nil {
				return nil, fmt.Errorf("%s's dingtalk approval not found", serviceReferredJob)
			}

			if originJobSpec.DingTalkApproval.ID != jobSpec.DingTalkApproval.ID {
				return nil, fmt.Errorf("origin refered %s's dingtalk id is different from current %s's dingtalk id", serviceReferredJob, j.spec.JobName)
			}

			jobSpec.DingTalkApproval.ApprovalNodes = originJobSpec.DingTalkApproval.ApprovalNodes
		case config.WorkWXApproval:
			if originJobSpec.WorkWXApproval == nil {
				return nil, fmt.Errorf("%s's workwx approval not found", serviceReferredJob)
			}

			if originJobSpec.WorkWXApproval.ID != jobSpec.WorkWXApproval.ID {
				return nil, fmt.Errorf("origin refered %s's workwx id is different from current %s's workwx id", serviceReferredJob, j.spec.JobName)
			}

			jobSpec.WorkWXApproval.ApprovalNodes = originJobSpec.WorkWXApproval.ApprovalNodes
		default:
			return nil, fmt.Errorf("%s's invalid approval type %s's", originJobSpec.Type, serviceReferredJob)
		}
	}

	// check if the approval job is valid
	switch jobSpec.Type {
	case config.NativeApproval:
		if jobSpec.NativeApproval == nil {
			return nil, fmt.Errorf("native approval not found")
		}
		if len(jobSpec.NativeApproval.ApproveUsers) == 0 {
			return nil, fmt.Errorf("num of approve-users is 0")
		}
		if len(jobSpec.NativeApproval.ApproveUsers) < jobSpec.NativeApproval.NeededApprovers {
			return nil, fmt.Errorf("all approve users should not less than needed approvers")
		}
	case config.DingTalkApproval:
		if jobSpec.DingTalkApproval == nil {
			return nil, fmt.Errorf("dingtalk approval not found")
		}
		if len(jobSpec.DingTalkApproval.ApprovalNodes) == 0 {
			return nil, fmt.Errorf("num of approval-node is 0")
		}

		userIDSets := sets.NewString()
		for i, node := range jobSpec.DingTalkApproval.ApprovalNodes {
			if len(node.ApproveUsers) == 0 {
				return nil, fmt.Errorf("num of approval-node %d approver is 0", i)
			}
			for _, user := range node.ApproveUsers {
				if userIDSets.Has(user.ID) {
					return nil, fmt.Errorf("duplicate approvers %s should not appear in a complete approval process", user.Name)
				}
				userIDSets.Insert(user.ID)
			}
			if !lo.Contains([]string{"AND", "OR"}, string(node.Type)) {
				return nil, fmt.Errorf("approval-node %d type should be AND or OR", i)
			}
		}
	case config.LarkApproval:
		if jobSpec.LarkApproval == nil {
			return nil, fmt.Errorf("lark approval not found")
		}
		if len(jobSpec.LarkApproval.ApprovalNodes) == 0 {
			return nil, fmt.Errorf("num of approval-node is 0")
		}
		for i, node := range jobSpec.LarkApproval.ApprovalNodes {
			if len(node.ApproveUsers) == 0 {
				return nil, fmt.Errorf("num of approval-node %d approver is 0", i)
			}
			if !lo.Contains([]string{"AND", "OR"}, string(node.Type)) {
				return nil, fmt.Errorf("approval-node %d type should be AND or OR", i)
			}
		}
	case config.WorkWXApproval:
		if jobSpec.WorkWXApproval == nil {
			return nil, fmt.Errorf("workwx approval not found")
		}
		// if len(jobSpec.WorkWXApproval.ApprovalNodes) == 0 {
		// 	return nil, fmt.Errorf("num of approval-node is 0")
		// }
	default:
		return nil, fmt.Errorf("invalid approval type %s", jobSpec.Type)
	}

	resp := make([]*commonmodels.JobTask, 0)
	resp = append(resp, jobTask)

	return resp, nil
}

func (j *ApprovalJob) LintJob() error {
	j.spec = &commonmodels.ApprovalJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	if err := util.CheckZadigProfessionalLicense(); err != nil {
		if j.spec.Type == config.LarkApproval || j.spec.Type == config.DingTalkApproval || j.spec.Type == config.WorkWXApproval {
			return e.ErrLicenseInvalid.AddDesc("飞书、钉钉、企业微信审批是专业版功能")
		}
	}

	return nil
}

func (j *ApprovalJob) getOriginReferedJobSpec(jobName string) (*commonmodels.ApprovalJobSpec, error) {
	var err error
	resp := &commonmodels.ApprovalJobSpec{}

	for _, stage := range j.workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			if job.JobType != config.JobApproval {
				continue
			}

			approvalSpec := &commonmodels.ApprovalJobSpec{}
			if err = commonmodels.IToi(job.Spec, approvalSpec); err != nil {
				return resp, err
			}

			if approvalSpec.Source == config.SourceFromJob {
				return nil, fmt.Errorf("origin refered %s's source is also from job", jobName)
			}

			resp = approvalSpec
			return resp, nil
		}
	}

	return nil, fmt.Errorf("approval job %s not found", jobName)
}
