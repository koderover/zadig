/*
 * Copyright 2023 The KodeRover Authors.
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
	"net/url"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/grafana"
)

type GrafanaJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskGrafanaSpec
	ack         func()
}

func NewGrafanaJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *GrafanaJobCtl {
	jobTaskSpec := &commonmodels.JobTaskGrafanaSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &GrafanaJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *GrafanaJobCtl) Clean(ctx context.Context) {}

func (c *GrafanaJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	info, err := mongodb.NewObservabilityColl().GetByID(context.Background(), c.jobTaskSpec.ID)
	if err != nil {
		logError(c.job, fmt.Sprintf("get observability info error: %v", err), c.logger)
		return
	}
	link := func(alert string) string {
		return info.Host + "/alerting/list?search=" + url.QueryEscape(alert)
	}

	client := grafana.NewClient(info.Host, info.GrafanaToken)
	timeout := time.After(time.Duration(c.jobTaskSpec.CheckTime) * time.Minute)

	alertMap := make(map[string]*commonmodels.GrafanaAlert)
	for _, alert := range c.jobTaskSpec.Alerts {
		alertMap[alert.ID] = alert
		alert.Status = StatusChecking
	}
	c.ack()

	check := func() (bool, error) {
		triggered := false
		resp, err := client.ListAlertInstance()
		if err != nil {
			return false, err
		}

		for _, eventResp := range resp {
			if eventResp.Labels.AlertRuleUID == "" {
				return false, errors.New("alert uid not found")
			}
			if alert, ok := alertMap[eventResp.Labels.AlertRuleUID]; ok {
				// alert has been triggered if url not empty, ignore it
				if alert.Url == "" {
					alert.Status = StatusAbnormal
					alert.Url = link(alert.Name)
					triggered = true
				}
			}
		}
		return triggered, nil
	}
	setNoEventAlertStatusUnfinished := func() {
		for _, alert := range c.jobTaskSpec.Alerts {
			if alert.Url == "" {
				alert.Status = StatusUnfinished
			}
		}
	}
	isAllAlertHasEvent := func() bool {
		for _, alert := range c.jobTaskSpec.Alerts {
			if alert.Url == "" {
				return false
			}
		}
		return true
	}
	isNoAlertHasEvent := func() bool {
		for _, alert := range c.jobTaskSpec.Alerts {
			if alert.Url != "" {
				return false
			}
		}
		return true
	}
	for {
		c.ack()
		time.Sleep(time.Second * 5)

		triggered, err := check()
		if err != nil {
			logError(c.job, fmt.Sprintf("check error: %v", err), c.logger)
			return
		}
		switch c.jobTaskSpec.CheckMode {
		case "trigger":
			if triggered {
				setNoEventAlertStatusUnfinished()
				c.job.Status = config.StatusFailed
				return
			}
		case "alert":
			if isAllAlertHasEvent() {
				c.job.Status = config.StatusFailed
				return
			}
		default:
			logError(c.job, fmt.Sprintf("invalid check mode: %s", c.jobTaskSpec.CheckMode), c.logger)
			return
		}
		select {
		case <-ctx.Done():
			c.job.Status = config.StatusCancelled
			return
		case <-timeout:
			if isNoAlertHasEvent() {
				c.job.Status = config.StatusPassed
			} else {
				c.job.Status = config.StatusFailed
			}
			// no event triggered in check time
			for _, alert := range c.jobTaskSpec.Alerts {
				if alert.Url == "" {
					alert.Status = StatusNormal
				}
			}
			return
		default:
		}
	}
}

func (c *GrafanaJobCtl) SaveInfo(ctx context.Context) error {
	return mongodb.NewJobInfoColl().Create(context.TODO(), &commonmodels.JobInfo{
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
