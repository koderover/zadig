/*
Copyright 2026 The KodeRover Authors.

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
	"context"
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

type AIJobController struct {
	*BasicInfo
	jobSpec *commonmodels.AIJobSpec
}

func CreateAIJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.AIJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create ai job controller, error: %s", err)
	}
	return AIJobController{
		BasicInfo: &BasicInfo{
			name:          job.Name,
			jobType:       job.JobType,
			errorPolicy:   job.ErrorPolicy,
			executePolicy: job.ExecutePolicy,
			workflow:      workflow,
		},
		jobSpec: spec,
	}, nil
}

func (j AIJobController) SetWorkflow(wf *commonmodels.WorkflowV4) { j.workflow = wf }

func (j AIJobController) GetSpec() interface{} { return j.jobSpec }

func (j AIJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigEnterpriseLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}
	if err := validateAIJobSpec(j.jobSpec); err != nil {
		return err
	}
	if j.jobSpec.TargetType == config.AITargetTypeAgent {
		if _, err := commonrepo.NewAgentIntegrationColl().FindByID(context.Background(), j.jobSpec.TargetID); err != nil {
			return fmt.Errorf("failed to find agent integration %s, error: %s", j.jobSpec.TargetID, err)
		}
	}
	return nil
}

func validateAIJobSpec(spec *commonmodels.AIJobSpec) error {
	if spec == nil {
		return fmt.Errorf("ai job spec is required")
	}
	if spec.TargetType != config.AITargetTypeModel && spec.TargetType != config.AITargetTypeAgent {
		return fmt.Errorf("target_type %q is not supported", spec.TargetType)
	}
	if strings.TrimSpace(spec.TargetID) == "" {
		return fmt.Errorf("target_id is required")
	}
	if strings.TrimSpace(spec.Prompt) == "" {
		return fmt.Errorf("prompt is required")
	}
	if spec.RequireManualConfirm && len(spec.ConfirmUsers) == 0 {
		return fmt.Errorf("confirm users cannot be empty when manual confirm is enabled")
	}
	for _, user := range spec.ConfirmUsers {
		if user == nil {
			return fmt.Errorf("confirm user cannot be nil")
		}
		switch user.Type {
		case "", setting.UserTypeUser:
			if user.UserID == "" {
				return fmt.Errorf("confirm user id cannot be empty")
			}
		case setting.UserTypeGroup:
			if user.GroupID == "" {
				return fmt.Errorf("confirm group id cannot be empty")
			}
		case setting.UserTypeTaskCreator:
		default:
			return fmt.Errorf("confirm user type %s is not supported", user.Type)
		}
	}
	return nil
}

// Update always takes the saved workflow settings. useUserInput is ignored because
// none of the AI job fields are editable at execution time.
func (j AIJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	current, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}
	currentSpec := new(commonmodels.AIJobSpec)
	if err := commonmodels.IToi(current.Spec, currentSpec); err != nil {
		return fmt.Errorf("failed to decode ai job spec, error: %s", err)
	}
	j.errorPolicy = current.ErrorPolicy
	j.executePolicy = current.ExecutePolicy
	*j.jobSpec = *currentSpec
	return nil
}

func (j AIJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error { return nil }
func (j AIJobController) ClearOptions()                                        {}
func (j AIJobController) ClearSelection()                                      {}

func (j AIJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	if err := validateAIJobSpec(j.jobSpec); err != nil {
		return nil, err
	}
	timeout := j.jobSpec.Timeout
	if timeout <= 0 {
		timeout = config.AIDefaultTimeoutMinutes
	}
	spec := &commonmodels.JobTaskAISpec{
		TargetType:           j.jobSpec.TargetType,
		TargetID:             j.jobSpec.TargetID,
		TargetName:           getAITargetName(j.jobSpec.TargetType, j.jobSpec.TargetID),
		Prompt:               j.jobSpec.Prompt,
		RequireManualConfirm: j.jobSpec.RequireManualConfirm,
		ConfirmUsers:         j.jobSpec.ConfirmUsers,
	}
	return []*commonmodels.JobTask{{
		Name:          GenJobName(j.workflow, j.name, 0),
		Key:           genJobKey(j.name),
		DisplayName:   genJobDisplayName(j.name),
		OriginName:    j.name,
		JobInfo:       map[string]string{JobNameKey: j.name},
		JobType:       string(config.JobAI),
		Spec:          spec,
		Timeout:       timeout,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
		Outputs: []*commonmodels.Output{{
			Name: config.AIOutputResult, Description: "AI 任务输出结果",
		}},
	}}, nil
}

// getAITargetName resolves the display name of the configured model or agent so
// the task detail does not depend on the integration still existing afterwards.
func getAITargetName(targetType, targetID string) string {
	ctx := context.Background()
	switch targetType {
	case config.AITargetTypeModel:
		integration, err := commonrepo.NewLLMIntegrationColl().FindByID(ctx, targetID)
		if err != nil {
			log.Warnf("failed to find model integration %s, error: %s", targetID, err)
			return ""
		}
		return integration.Name
	case config.AITargetTypeAgent:
		integration, err := commonrepo.NewAgentIntegrationColl().FindByID(ctx, targetID)
		if err != nil {
			log.Warnf("failed to find agent integration %s, error: %s", targetID, err)
			return ""
		}
		return integration.Name
	default:
		return ""
	}
}

func (j AIJobController) SetRepo(repo *types.Repository) error { return nil }
func (j AIJobController) SetRepoCommitInfo() error             { return nil }

func (j AIJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	if !getRuntimeVariables {
		return []*commonmodels.KeyVal{}, nil
	}
	return []*commonmodels.KeyVal{
		{Key: strings.Join([]string{"job", j.name, "status"}, "."), Type: "string"},
		{Key: strings.Join([]string{"job", j.name, "output", config.AIOutputResult}, "."), Type: "string"},
	}, nil
}

func (j AIJobController) GetUsedRepos() ([]*types.Repository, error) {
	return []*types.Repository{}, nil
}
func (j AIJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}
func (j AIJobController) IsServiceTypeJob() bool { return false }
