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

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/types"
)

type ApprovalJobController struct {
	*BasicInfo

	jobSpec *commonmodels.ApprovalJobSpec
}

func CreateApprovalJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
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

	return ApprovalJobController{
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

	j.jobSpec.JobName = latestJobSpec.JobName
	j.jobSpec.Type = latestJobSpec.Type
	j.jobSpec.Timeout = latestJobSpec.Timeout
	j.jobSpec.OriginJobName = latestJobSpec.OriginJobName
	j.jobSpec.Source = latestJobSpec.Source
	j.jobSpec.Description = latestJobSpec.Description

	if latestJobSpec.Source != config.SourceFixed {
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
	}

	if latestJobSpec.ApprovalMessageSource == config.SourceFixed {
		j.jobSpec.ApprovalMessage = latestJobSpec.ApprovalMessage
	}

	j.jobSpec.NativeApproval = latestJobSpec.NativeApproval
	j.jobSpec.LarkApproval = latestJobSpec.LarkApproval
	j.jobSpec.DingTalkApproval = latestJobSpec.DingTalkApproval
	j.jobSpec.WorkWXApproval = latestJobSpec.WorkWXApproval
	return nil
}

func (j ApprovalJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j ApprovalJobController) ClearOptions() {
	return
}

func (j ApprovalJobController) ClearSelection() {
	return
}

func (j ApprovalJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	nativeApproval := j.jobSpec.NativeApproval
	if nativeApproval != nil && j.jobSpec.Source != config.SourceFromJob {
		approvalUser, _ := util.GeneFlatUsers(nativeApproval.ApproveUsers)
		nativeApproval.ApproveUsers = approvalUser
	}

	jobSpec := &commonmodels.JobTaskApprovalSpec{
		Timeout:          j.jobSpec.Timeout,
		Type:             j.jobSpec.Type,
		Description:      j.jobSpec.Description,
		NativeApproval:   nativeApproval,
		LarkApproval:     j.jobSpec.LarkApproval,
		DingTalkApproval: j.jobSpec.DingTalkApproval,
		WorkWXApproval:   j.jobSpec.WorkWXApproval,
		ApprovalMessage:  j.jobSpec.ApprovalMessage,
	}
	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType:     string(config.JobApproval),
		Spec:        jobSpec,
		Timeout:     j.jobSpec.Timeout,
		ErrorPolicy: j.errorPolicy,
	}

	if j.jobSpec.Source == config.SourceFromJob {
		// adapt to the front end, use the direct quoted job name
		if j.jobSpec.OriginJobName != "" {
			j.jobSpec.JobName = j.jobSpec.OriginJobName
		}

		serviceReferredJob := getOriginJobName(j.workflow, j.jobSpec.JobName)
		originJobSpec, err := j.getOriginReferredJobSpec(serviceReferredJob)
		if err != nil {
			return nil, fmt.Errorf("failed to get origin refered job: %s", err)
		}

		if originJobSpec.Type != jobSpec.Type {
			return nil, fmt.Errorf("origin refered %s's job type %s is different from current %s's job type %s", serviceReferredJob, originJobSpec.Type, j.jobSpec.JobName, j.jobSpec.Type)
		}

		switch originJobSpec.Type {
		case config.NativeApproval:
			approvalUser, _ := util.GeneFlatUsers(originJobSpec.NativeApproval.ApproveUsers)
			jobSpec.NativeApproval.ApproveUsers = approvalUser
		case config.LarkApproval, config.LarkApprovalIntl:
			if originJobSpec.LarkApproval == nil {
				return nil, fmt.Errorf("%s lark approval not found", serviceReferredJob)
			}

			if originJobSpec.LarkApproval.ID != jobSpec.LarkApproval.ID {
				return nil, fmt.Errorf("origin refered %s's lark id is different from current %s's lark id", serviceReferredJob, j.jobSpec.JobName)
			}

			jobSpec.LarkApproval.ApprovalNodes = originJobSpec.LarkApproval.ApprovalNodes
		case config.DingTalkApproval:
			if originJobSpec.DingTalkApproval == nil {
				return nil, fmt.Errorf("%s's dingtalk approval not found", serviceReferredJob)
			}

			if originJobSpec.DingTalkApproval.ID != jobSpec.DingTalkApproval.ID {
				return nil, fmt.Errorf("origin refered %s's dingtalk id is different from current %s's dingtalk id", serviceReferredJob, j.jobSpec.JobName)
			}

			jobSpec.DingTalkApproval.ApprovalNodes = originJobSpec.DingTalkApproval.ApprovalNodes
		case config.WorkWXApproval:
			if originJobSpec.WorkWXApproval == nil {
				return nil, fmt.Errorf("%s's workwx approval not found", serviceReferredJob)
			}

			if originJobSpec.WorkWXApproval.ID != jobSpec.WorkWXApproval.ID {
				return nil, fmt.Errorf("origin refered %s's workwx id is different from current %s's workwx id", serviceReferredJob, j.jobSpec.JobName)
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
	case config.LarkApproval, config.LarkApprovalIntl:
		if jobSpec.LarkApproval == nil {
			return nil, fmt.Errorf("lark approval not found")
		}

		if len(jobSpec.LarkApproval.ApprovalNodes) == 0 {
			return nil, fmt.Errorf("num of approval-node is 0")
		}

		for i, node := range jobSpec.LarkApproval.ApprovalNodes {
			if node.ApproveNodeType == lark.ApproveNodeTypeUser {
				if node.Type == lark.ApproveTypeStart || node.Type == lark.ApproveTypeEnd {
					continue
				}

				if len(node.ApproveUsers) == 0 {
					return nil, fmt.Errorf("num of approval-node %d approver is 0", i)
				}
			} else if node.ApproveNodeType == lark.ApproveNodeTypeUserGroup {
				if node.Type != lark.ApproveTypeStart && node.Type != lark.ApproveTypeEnd {
					if len(node.ApproveGroups) == 0 {
						return nil, fmt.Errorf("num of approval-node %d approver is 0", i)
					}
				}

				users, err := convertLarkUserGroupToUser(j.jobSpec.LarkApproval.ID, node.ApproveGroups)
				if err != nil {
					return nil, fmt.Errorf("failed to convert lark user group to user: %s", err)
				}

				approveUsers := make([]*commonmodels.LarkApprovalUser, 0)
				for _, user := range users {
					approveUsers = append(approveUsers, &commonmodels.LarkApprovalUser{
						UserInfo: *user,
					})
				}
				node.ApproveUsers = approveUsers

				users, err = convertLarkUserGroupToUser(j.jobSpec.LarkApproval.ID, node.CcGroups)
				if err != nil {
					return nil, fmt.Errorf("failed to convert lark user group to user: %s", err)
				}
				node.CcUsers = users

				jobSpec.LarkApproval.ApprovalNodes[i] = node
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

func convertLarkUserGroupToUser(larkApprovalID string, groups []*commonmodels.LarkApprovalGroup) ([]*lark.UserInfo, error) {
	userSet := sets.NewString()
	users := make([]*lark.UserInfo, 0)
	for _, group := range groups {
		userGroup, err := larkservice.GetLarkUserGroup(larkApprovalID, group.GroupID)
		if err != nil {
			return nil, fmt.Errorf("failed to get lark user group: %s", err)
		}

		if userGroup.MemberUserCount > 0 {
			userInfos, err := larkservice.GetLarkUserGroupMembersInfo(larkApprovalID, group.GroupID, "user", setting.LarkUserOpenID, "")
			if err != nil {
				return nil, fmt.Errorf("failed to get lark department user infos: %s", err)
			}

			for _, user := range userInfos {
				if !userSet.Has(user.ID) {
					users = append(users, user)
					userSet.Insert(user.ID)
				}
			}
		}

		if userGroup.MemberDepartmentCount > 0 {
			userInfos, err := larkservice.GetLarkUserGroupMembersInfo(larkApprovalID, group.GroupID, "department", setting.LarkDepartmentID, "")
			if err != nil {
				return nil, fmt.Errorf("failed to get lark department user infos: %s", err)
			}

			for _, user := range userInfos {
				if !userSet.Has(user.ID) {
					users = append(users, user)
					userSet.Insert(user.ID)
				}
			}
		}
	}
	return users, nil
}

func (j ApprovalJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j ApprovalJobController) SetRepoCommitInfo() error {
	return nil
}

func (j ApprovalJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	return make([]*commonmodels.KeyVal, 0), nil
}

func (j ApprovalJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j ApprovalJobController) getOriginReferredJobSpec(jobName string) (*commonmodels.ApprovalJobSpec, error) {
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

func (j ApprovalJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j ApprovalJobController) IsServiceTypeJob() bool {
	return false
}
