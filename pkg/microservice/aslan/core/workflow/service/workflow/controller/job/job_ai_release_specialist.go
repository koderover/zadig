package job

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	runtimeJobController "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
)

const (
	AIReleaseSpecialistOutputResultJSON           = "RESULT_JSON"
	AIReleaseSpecialistOutputConclusion           = "CONCLUSION"
	AIReleaseSpecialistOutputSummary              = "SUMMARY"
	AIReleaseSpecialistOutputCheckCount           = "CHECK_COUNT"
	AIReleaseSpecialistOutputCheckDetailsMarkdown = "CHECK_DETAILS_MARKDOWN"
)

type AIReleaseSpecialistJobController struct {
	*BasicInfo

	jobSpec *commonmodels.AIReleaseSpecialistJobSpec
}

func CreateAIReleaseSpecialistJobController(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (Job, error) {
	spec := new(commonmodels.AIReleaseSpecialistJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return nil, fmt.Errorf("failed to create ai release specialist job controller, error: %s", err)
	}
	spec.SystemPrompt = runtimeJobController.NormalizeAIReleaseSpecialistSystemPromptForStorage(spec.SystemPrompt)

	basicInfo := &BasicInfo{
		name:          job.Name,
		jobType:       job.JobType,
		errorPolicy:   job.ErrorPolicy,
		executePolicy: job.ExecutePolicy,
		workflow:      workflow,
	}

	return AIReleaseSpecialistJobController{
		BasicInfo: basicInfo,
		jobSpec:   spec,
	}, nil
}

func (j AIReleaseSpecialistJobController) SetWorkflow(wf *commonmodels.WorkflowV4) {
	j.workflow = wf
}

func (j AIReleaseSpecialistJobController) GetSpec() interface{} {
	return j.jobSpec
}

func (j AIReleaseSpecialistJobController) Validate(isExecution bool) error {
	if err := util.CheckZadigEnterpriseLicense(); err != nil {
		return e.ErrLicenseInvalid.AddDesc("")
	}
	if j.jobSpec.RequireManualConfirm && len(j.jobSpec.ConfirmUsers) == 0 {
		return fmt.Errorf("confirm users cannot be empty when manual confirm is enabled")
	}
	for _, user := range j.jobSpec.ConfirmUsers {
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

func (j AIReleaseSpecialistJobController) Update(useUserInput bool, ticket *commonmodels.ApprovalTicket) error {
	currJob, err := j.workflow.FindJob(j.name, j.jobType)
	if err != nil {
		return err
	}

	currJobSpec := new(commonmodels.AIReleaseSpecialistJobSpec)
	if err := commonmodels.IToi(currJob.Spec, currJobSpec); err != nil {
		return fmt.Errorf("failed to decode ai release specialist job spec, error: %s", err)
	}

	j.errorPolicy = currJob.ErrorPolicy
	j.executePolicy = currJob.ExecutePolicy
	j.jobSpec.Timeout = currJobSpec.Timeout
	j.jobSpec.PromptTemplate = currJobSpec.PromptTemplate
	j.jobSpec.RulePlan = currJobSpec.RulePlan
	j.jobSpec.RequireManualConfirm = currJobSpec.RequireManualConfirm
	j.jobSpec.ConfirmUsers = currJobSpec.ConfirmUsers
	j.jobSpec.SystemPrompt = runtimeJobController.NormalizeAIReleaseSpecialistSystemPromptForStorage(currJobSpec.SystemPrompt)
	return nil
}

func (j AIReleaseSpecialistJobController) SetOptions(ticket *commonmodels.ApprovalTicket) error {
	return nil
}

func (j AIReleaseSpecialistJobController) ClearOptions() {}

func (j AIReleaseSpecialistJobController) ClearSelection() {}

func (j AIReleaseSpecialistJobController) ToTask(taskID int64) ([]*commonmodels.JobTask, error) {
	timeout := j.getTimeout()
	spec := &commonmodels.JobTaskAIReleaseSpecialistSpec{
		Timeout:              timeout,
		PromptTemplate:       j.jobSpec.PromptTemplate,
		RulePlan:             j.jobSpec.RulePlan,
		RequireManualConfirm: j.jobSpec.RequireManualConfirm,
		ConfirmUsers:         j.jobSpec.ConfirmUsers,
		SystemPrompt:         j.jobSpec.SystemPrompt,
	}
	if j.jobSpec.RequireManualConfirm {
		spec.NativeApproval = &commonmodels.NativeApproval{
			ApproveUsers:    j.jobSpec.ConfirmUsers,
			NeededApprovers: 1,
			Timeout:         int(timeout),
		}
	}

	jobTask := &commonmodels.JobTask{
		Name:        GenJobName(j.workflow, j.name, 0),
		Key:         genJobKey(j.name),
		DisplayName: genJobDisplayName(j.name),
		OriginName:  j.name,
		JobInfo: map[string]string{
			JobNameKey: j.name,
		},
		JobType:       string(config.JobAIReleaseSpecialist),
		Spec:          spec,
		Timeout:       timeout,
		ErrorPolicy:   j.errorPolicy,
		ExecutePolicy: j.executePolicy,
		Outputs: []*commonmodels.Output{
			{Name: AIReleaseSpecialistOutputResultJSON, Description: "AI 发布专员结构化结果 JSON"},
			{Name: AIReleaseSpecialistOutputConclusion, Description: "AI 发布专员结论"},
			{Name: AIReleaseSpecialistOutputSummary, Description: "AI 发布专员摘要"},
			{Name: AIReleaseSpecialistOutputCheckCount, Description: "AI 发布专员检测项数量"},
			{Name: AIReleaseSpecialistOutputCheckDetailsMarkdown, Description: "AI 发布专员检测项 Markdown"},
		},
	}

	return []*commonmodels.JobTask{jobTask}, nil
}

func (j AIReleaseSpecialistJobController) getTimeout() int64 {
	if j.jobSpec.Timeout > 0 {
		return j.jobSpec.Timeout
	}
	return config.AIReleaseSpecialistDefaultTimeoutMinutes
}

func (j AIReleaseSpecialistJobController) SetRepo(repo *types.Repository) error {
	return nil
}

func (j AIReleaseSpecialistJobController) SetRepoCommitInfo() error {
	return nil
}

func (j AIReleaseSpecialistJobController) GetVariableList(jobName string, getAggregatedVariables, getRuntimeVariables, getPlaceHolderVariables, getServiceSpecificVariables, useUserInputValue bool) ([]*commonmodels.KeyVal, error) {
	resp := make([]*commonmodels.KeyVal, 0)
	if getRuntimeVariables {
		resp = append(resp, &commonmodels.KeyVal{
			Key:          strings.Join([]string{"job", j.name, "status"}, "."),
			Value:        "",
			Type:         "string",
			IsCredential: false,
		})
		outputs := []string{
			AIReleaseSpecialistOutputResultJSON,
			AIReleaseSpecialistOutputConclusion,
			AIReleaseSpecialistOutputSummary,
			AIReleaseSpecialistOutputCheckCount,
			AIReleaseSpecialistOutputCheckDetailsMarkdown,
		}
		for _, output := range outputs {
			resp = append(resp, &commonmodels.KeyVal{
				Key:          strings.Join([]string{"job", j.name, "output", output}, "."),
				Value:        "",
				Type:         "string",
				IsCredential: false,
			})
		}
	}
	return resp, nil
}

func (j AIReleaseSpecialistJobController) GetUsedRepos() ([]*types.Repository, error) {
	return make([]*types.Repository, 0), nil
}

func (j AIReleaseSpecialistJobController) RenderDynamicVariableOptions(key string, option *RenderDynamicVariableValue) ([]string, error) {
	return nil, fmt.Errorf("invalid job type: %s to render dynamic variable", j.name)
}

func (j AIReleaseSpecialistJobController) IsServiceTypeJob() bool {
	return false
}
