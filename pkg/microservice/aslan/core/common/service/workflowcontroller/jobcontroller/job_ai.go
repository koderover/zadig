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

package jobcontroller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/llmservice"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	runtimejob "github.com/koderover/zadig/v2/pkg/types/job"
)

type AIJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskAISpec
	ack         func()
}

func NewAIJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *AIJobCtl {
	spec := &commonmodels.JobTaskAISpec{}
	if err := commonmodels.IToi(job.Spec, spec); err != nil {
		logger.Error(err)
	}
	job.Spec = spec
	return &AIJobCtl{job: job, workflowCtx: workflowCtx, logger: logger, jobTaskSpec: spec, ack: ack}
}

func (c *AIJobCtl) Clean(ctx context.Context) {}

func (c *AIJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()
	start := time.Now()
	totalTimeout := c.getJobTimeout()
	jobCtx, cancel := context.WithTimeout(ctx, time.Duration(totalTimeout)*time.Minute)
	defer cancel()

	var client llm.ILLM
	var err error
	switch c.jobTaskSpec.TargetType {
	case config.AITargetTypeModel:
		integration, findErr := commonrepo.NewLLMIntegrationColl().FindByID(jobCtx, c.jobTaskSpec.TargetID)
		if findErr != nil {
			c.fail(fmt.Errorf("find model integration failed: %w", findErr))
			return
		}
		client, err = llmservice.NewLLMClient(integration)
	case config.AITargetTypeAgent:
		integration, findErr := commonrepo.NewAgentIntegrationColl().FindByID(jobCtx, c.jobTaskSpec.TargetID)
		if findErr != nil {
			c.fail(fmt.Errorf("find agent integration failed: %w", findErr))
			return
		}
		client, err = llmservice.NewAgentClient(integration)
	default:
		c.fail(fmt.Errorf("ai target type %q is not supported", c.jobTaskSpec.TargetType))
		return
	}
	if err != nil {
		c.fail(fmt.Errorf("create ai client failed: %w", err))
		return
	}
	deadline, _ := jobCtx.Deadline()
	result, err := client.GetCompletion(
		jobCtx,
		c.jobTaskSpec.Prompt,
		llm.WithErrorOnMaxTokens(),
		llm.WithRequestTimeout(time.Until(deadline)),
	)
	if err != nil {
		if errors.Is(jobCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			c.timeout()
		} else if errors.Is(jobCtx.Err(), context.Canceled) {
			c.job.Status = config.StatusCancelled
			c.job.Error = "workflow was canceled"
			c.ack()
		} else {
			c.fail(fmt.Errorf("ai completion failed: %w", err))
		}
		return
	}
	if strings.TrimSpace(result) == "" {
		c.fail(errors.New("ai completion returned empty response"))
		return
	}

	// Persist the candidate before approval so the task detail can display it while waiting.
	c.jobTaskSpec.Result = result
	c.ack()
	if !c.jobTaskSpec.RequireManualConfirm {
		c.publishResult()
		c.job.Status = config.StatusPassed
		c.ack()
		return
	}

	users, err := c.getRuntimeConfirmUsers()
	if err != nil {
		c.fail(err)
		return
	}
	remaining := c.getRemainingTimeout(start)
	if remaining <= 0 {
		c.timeout()
		return
	}
	c.jobTaskSpec.ConfirmUsers = users
	c.jobTaskSpec.NativeApproval = &commonmodels.NativeApproval{
		ApproveUsers: users, NeededApprovers: 1, Timeout: int(remaining),
	}
	approvalSpec := &commonmodels.JobTaskApprovalSpec{
		Timeout: remaining, Type: config.NativeApproval, NativeApproval: c.jobTaskSpec.NativeApproval,
	}
	c.job.Status = config.StatusWaitingApprove
	c.ack()
	sendJobNotifications(c.workflowCtx, c.job, config.StatusWaitingApprove, c.logger)
	status, err := waitForNativeApprove(jobCtx, approvalSpec, c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID, c.ack)
	if errors.Is(jobCtx.Err(), context.DeadlineExceeded) {
		c.timeout()
		return
	}
	c.job.Status = status
	if err != nil {
		c.job.Error = err.Error()
	} else if status == config.StatusPassed {
		c.publishResult()
	}
	c.ack()
}

func (c *AIJobCtl) publishResult() {
	c.workflowCtx.GlobalContextSet(runtimejob.GetJobOutputKey(c.job.Key, config.AIOutputResult), c.jobTaskSpec.Result)
}

func (c *AIJobCtl) fail(err error) {
	c.job.Status = config.StatusFailed
	c.job.Error = err.Error()
	c.ack()
}

func (c *AIJobCtl) timeout() {
	c.job.Status = config.StatusTimeout
	c.job.Error = "ai task timeout"
	c.ack()
}

func (c *AIJobCtl) SaveInfo(ctx context.Context) error {
	return commonrepo.NewJobInfoColl().Create(ctx, &commonmodels.JobInfo{
		Type: c.job.JobType, WorkflowName: c.workflowCtx.WorkflowName, WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID: c.workflowCtx.TaskID, ProductName: c.workflowCtx.ProjectName, StartTime: c.job.StartTime, EndTime: c.job.EndTime,
		Duration: c.job.EndTime - c.job.StartTime, Status: string(c.job.Status),
	})
}

func (c *AIJobCtl) getJobTimeout() int64 {
	if c.job.Timeout > 0 {
		return c.job.Timeout
	}
	return config.AIDefaultTimeoutMinutes
}

func (c *AIJobCtl) getRemainingTimeout(start time.Time) int64 {
	return remainingApprovalTimeout(c.getJobTimeout(), start)
}

// remainingApprovalTimeout returns the whole minutes left before an AI job's
// total timeout elapses, rounded up, or 0 once it is already exhausted.
func remainingApprovalTimeout(totalTimeoutMinutes int64, start time.Time) int64 {
	remaining := time.Duration(totalTimeoutMinutes)*time.Minute - time.Since(start)
	if remaining <= 0 {
		return 0
	}
	return int64(math.Ceil(remaining.Minutes()))
}

func (c *AIJobCtl) getRuntimeConfirmUsers() ([]*commonmodels.User, error) {
	// GeneFlatUsersWithCaller flattens groups and the task creator into plain users.
	users, _ := commonutil.GeneFlatUsersWithCaller(c.jobTaskSpec.ConfirmUsers, c.workflowCtx.WorkflowTaskCreatorUserID)
	if len(users) == 0 {
		return nil, errors.New("confirm users are empty")
	}
	return users, nil
}
