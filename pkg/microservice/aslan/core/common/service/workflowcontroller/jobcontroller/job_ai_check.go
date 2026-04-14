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

package jobcontroller

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	approvalservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/approval"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
)

type AICheckJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskAICheckSpec
	ack         func()
}

func NewAICheckJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *AICheckJobCtl {
	jobTaskSpec := &commonmodels.JobTaskAICheckSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &AICheckJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *AICheckJobCtl) Clean(ctx context.Context) {}

func (c *AICheckJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	// Simulate AI detection process with a brief delay
	select {
	case <-ctx.Done():
		c.job.Status = config.StatusCancelled
		c.job.Error = "workflow was canceled"
		return
	case <-time.After(3 * time.Second):
	}

	// Populate mock detection results
	c.jobTaskSpec.CheckResult = buildMockCheckResult()
	c.ack()

	if !c.jobTaskSpec.ManualConfirm {
		c.job.Status = config.StatusPassed
		return
	}

	// Manual confirmation required: wait for an authorized user to approve/reject
	status, err := c.waitForManualConfirm(ctx)
	c.job.Status = status
	if err != nil {
		c.job.Error = err.Error()
	}
}

func (c *AICheckJobCtl) waitForManualConfirm(ctx context.Context) (config.Status, error) {
	flatUsers, _ := util.GeneFlatUsers(c.jobTaskSpec.ManualConfirmUsers)
	if len(flatUsers) == 0 {
		return config.StatusFailed, fmt.Errorf("ai-check manual confirm: no valid users configured")
	}

	timeout := c.jobTaskSpec.Timeout
	if timeout == 0 {
		timeout = 60
	}

	nativeApproval := &commonmodels.NativeApproval{
		Timeout:         int(timeout),
		ApproveUsers:    flatUsers,
		NeededApprovers: 1,
	}

	approveKey := fmt.Sprintf("%s-%s-%d", c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID)
	approvalservice.GlobalApproveMap.SetApproval(approveKey, nativeApproval)
	defer approvalservice.GlobalApproveMap.DeleteApproval(approveKey)

	c.job.Status = config.StatusWaitingApprove
	c.ack()

	timeoutChan := time.After(time.Duration(timeout) * time.Minute)
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			return config.StatusCancelled, fmt.Errorf("workflow was canceled")
		case <-timeoutChan:
			return config.StatusTimeout, fmt.Errorf("workflow timeout")
		default:
			approved, rejected, latestApproval, err := approvalservice.GlobalApproveMap.IsApproval(approveKey)
			if err != nil {
				return config.StatusFailed, fmt.Errorf("get approval status error: %s", err)
			}

			if latestApproval != nil {
				for _, latestUser := range latestApproval.ApproveUsers {
					for _, user := range c.jobTaskSpec.ManualConfirmUsers {
						if latestUser.UserID == user.UserID {
							user.RejectOrApprove = latestUser.RejectOrApprove
							user.Comment = latestUser.Comment
							user.OperationTime = latestUser.OperationTime
						}
					}
				}
			}
			c.ack()

			if rejected {
				return config.StatusReject, nil
			}
			if approved {
				return config.StatusPassed, nil
			}
		}
	}
}

func (c *AICheckJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(ctx, &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	})
}

// buildMockCheckResult returns fixed mock detection results for demo purposes.
func buildMockCheckResult() *commonmodels.AICheckResult {
	return &commonmodels.AICheckResult{
		OverallStatus: "warning",
		Summary:       "警示 — 无硬门禁失败，但存在需关注项，建议评估后继续执行。",
		Items: []*commonmodels.AICheckItem{
			{
				Name:    "代码扫描",
				Status:  "passed",
				Summary: "敏感信息扫描通过；依赖漏洞无高危及以上。",
				Detail:  "指标：CVE 严重+高危数量 = 0。",
			},
			{
				Name:    "测试报告",
				Status:  "warning",
				Summary: "单测覆盖率 76%，接近阈值（>=75%）。",
				Detail:  "建议动作：补充关键链路单测后再进入高峰窗口发布。",
			},
			{
				Name:    "运行时服务状态",
				Status:  "passed",
				Summary: "关联服务健康检查均为 Healthy。",
				Detail:  "时间范围：近 10 分钟；就绪成功率 100%。",
			},
			{
				Name:    "业务日志（最近 10 条）",
				Status:  "warning",
				Summary: "无 ERROR，但 WARN 达 4 条（阈值 3）。",
				Detail:  "异常点：依赖下游重试抖动；建议动作：观察 15 分钟。",
			},
			{
				Name:    "基础设施资源",
				Status:  "passed",
				Summary: "CPU / 内存 / 磁盘占用在建议范围内。",
				Detail:  "阈值：CPU<75%，Mem<80%，Disk<85%。",
			},
			{
				Name:    "监控与告警",
				Status:  "passed",
				Summary: "近 1 小时无 P0/P1 告警。",
				Detail:  "数据源：Prometheus + 告警平台。",
			},
			{
				Name:    "流量与发布窗口",
				Status:  "warning",
				Summary: "接近业务流量上升窗口，需关注。",
				Detail:  "建议动作：发布后安排值班观察。",
			},
		},
	}
}
