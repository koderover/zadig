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

package workflow

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/llmservice"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"github.com/koderover/zadig/v2/pkg/types"
)

type DebugAIReleaseSpecialistPromptRequest struct {
	WorkflowName         string `json:"workflow_name"`
	TaskID               int64  `json:"task_id"`
	JobName              string `json:"job_name"`
	PromptTemplate       string `json:"prompt_template"`
	SystemPromptOverride string `json:"system_prompt_override"`
	Execute              bool   `json:"execute"`
}

type DebugAIReleaseSpecialistPromptResponse struct {
	WorkflowName          string                                  `json:"workflow_name,omitempty"`
	TaskID                int64                                   `json:"task_id,omitempty"`
	JobName               string                                  `json:"job_name,omitempty"`
	PromptTemplate        string                                  `json:"prompt_template,omitempty"`
	EffectiveInput        *commonmodels.AIReleaseSpecialistInput  `json:"effective_input,omitempty"`
	EffectiveSystemPrompt string                                  `json:"effective_system_prompt"`
	FinalPrompt           string                                  `json:"final_prompt"`
	PromptTokens          int                                     `json:"prompt_tokens"`
	PromptTooLarge        bool                                    `json:"prompt_too_large"`
	Model                 string                                  `json:"model,omitempty"`
	RawResponse           string                                  `json:"raw_response,omitempty"`
	ParsedResult          *commonmodels.AIReleaseSpecialistResult `json:"parsed_result,omitempty"`
	ParseError            string                                  `json:"parse_error,omitempty"`
	LLMError              string                                  `json:"llm_error,omitempty"`
}

func DebugAIReleaseSpecialistPrompt(ctx *handler.Context, req *DebugAIReleaseSpecialistPromptRequest, logger *zap.SugaredLogger) (*DebugAIReleaseSpecialistPromptResponse, error) {
	if req == nil {
		return nil, e.ErrInvalidParam.AddDesc("request cannot be nil")
	}
	if err := validateAIReleaseSpecialistDebugRequest(req); err != nil {
		return nil, e.ErrInvalidParam.AddDesc(err.Error())
	}

	workflowName := strings.TrimSpace(req.WorkflowName)
	taskID := req.TaskID
	jobName := strings.TrimSpace(req.JobName)

	task, err := commonrepo.NewworkflowTaskv4Coll().Find(workflowName, taskID)
	if err != nil {
		logger.Errorf("find workflow task failed, workflow: %s, taskID: %d, err: %v", workflowName, taskID, err)
		return nil, e.ErrFindWorkflow.AddErr(err)
	}
	if err := ensureWorkflowPermission(ctx, task.ProjectName, workflowName, req.Execute); err != nil {
		return nil, err
	}

	input, err := jobcontroller.BuildAIReleaseSpecialistInputFromTask(task, jobName)
	if err != nil {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("build ai release specialist input failed: %v", err))
	}
	promptTemplate, err := getAIReleaseSpecialistPromptTemplate(workflowName, jobName, req.PromptTemplate, logger)
	if err != nil {
		return nil, err
	}

	debugResult, err := jobcontroller.BuildAIReleaseSpecialistPromptForDebug(promptTemplate, req.SystemPromptOverride, input)
	if err != nil {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("build prompt failed: %v", err))
	}

	resp := &DebugAIReleaseSpecialistPromptResponse{
		WorkflowName:          workflowName,
		TaskID:                taskID,
		JobName:               jobName,
		PromptTemplate:        promptTemplate,
		EffectiveInput:        input,
		EffectiveSystemPrompt: debugResult.SystemPrompt,
		FinalPrompt:           debugResult.Prompt,
		PromptTokens:          debugResult.PromptTokens,
		PromptTooLarge:        debugResult.PromptTooLarge,
	}

	if !req.Execute {
		return resp, nil
	}

	llmClient, err := llmservice.GetDefaultLLMClient(ctx.Context)
	if err != nil {
		resp.LLMError = err.Error()
		return resp, nil
	}

	options := []llm.ParamOption{
		llm.WithTemperature(0.1),
		llm.WithMaxTokens(3000),
	}
	if llmClient.GetModel() != "" {
		resp.Model = llmClient.GetModel()
		options = append(options, llm.WithModel(llmClient.GetModel()))
	}

	answer, err := llmClient.GetCompletion(ctx.Context, debugResult.Prompt, options...)
	if err != nil {
		resp.LLMError = err.Error()
		return resp, nil
	}
	resp.RawResponse = answer

	parsedResult, err := jobcontroller.ParseAIReleaseSpecialistResult(answer)
	if err != nil {
		resp.ParseError = err.Error()
		return resp, nil
	}
	resp.ParsedResult = parsedResult
	return resp, nil
}

func validateAIReleaseSpecialistDebugRequest(req *DebugAIReleaseSpecialistPromptRequest) error {
	if strings.TrimSpace(req.WorkflowName) == "" || req.TaskID <= 0 || strings.TrimSpace(req.JobName) == "" {
		return fmt.Errorf("workflow_name, task_id and job_name are required")
	}
	return nil
}

func getAIReleaseSpecialistPromptTemplate(workflowName, jobName, override string, logger *zap.SugaredLogger) (string, error) {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override), nil
	}

	workflow, err := FindWorkflowV4Raw(workflowName, logger)
	if err != nil {
		return "", err
	}
	job, err := workflow.FindJob(jobName, "")
	if err != nil {
		return "", e.ErrFindWorkflow.AddDesc(err.Error())
	}
	if job.JobType != config.JobAIReleaseSpecialist {
		return "", e.ErrInvalidParam.AddDesc(fmt.Sprintf("job %s is not ai release specialist", jobName))
	}

	spec := new(commonmodels.AIReleaseSpecialistJobSpec)
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		return "", e.ErrInvalidParam.AddDesc(fmt.Sprintf("decode ai release specialist job spec failed: %v", err))
	}
	return strings.TrimSpace(spec.PromptTemplate), nil
}

func ensureWorkflowPermission(ctx *handler.Context, projectName, workflowName string, requireEdit bool) error {
	if ctx == nil || ctx.Resources == nil {
		return e.ErrUnauthorized
	}
	if ctx.Resources.IsSystemAdmin {
		return nil
	}

	projectAuth, ok := ctx.Resources.ProjectAuthInfo[projectName]
	if !ok {
		return e.ErrForbidden
	}
	if projectAuth.IsProjectAdmin {
		return nil
	}
	if projectAuth.Workflow != nil {
		if requireEdit && projectAuth.Workflow.Edit {
			return nil
		}
		if !requireEdit && (projectAuth.Workflow.View || projectAuth.Workflow.Edit) {
			return nil
		}
	}

	action := types.WorkflowActionView
	if requireEdit {
		action = types.WorkflowActionEdit
	}
	permitted, err := handler.GetCollaborationModePermission(ctx.UserID, projectName, types.ResourceTypeWorkflow, workflowName, action)
	if err == nil && permitted {
		return nil
	}
	return e.ErrForbidden
}
