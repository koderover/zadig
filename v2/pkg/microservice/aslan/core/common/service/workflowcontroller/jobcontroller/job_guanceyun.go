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

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/guanceyun"
)

const (
	StatusChecking   = "checking"
	StatusNormal     = "normal"
	StatusAbnormal   = "abnormal"
	StatusUnfinished = "unfinished"
)

type GuanceyunCheckJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskGuanceyunCheckSpec
	ack         func()
}

func NewGuanceyunCheckJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *GuanceyunCheckJobCtl {
	jobTaskSpec := &commonmodels.JobTaskGuanceyunCheckSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &GuanceyunCheckJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *GuanceyunCheckJobCtl) Clean(ctx context.Context) {}

func (c *GuanceyunCheckJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusRunning
	c.ack()

	info, err := mongodb.NewObservabilityColl().GetByID(context.Background(), c.jobTaskSpec.ID)
	if err != nil {
		logError(c.job, fmt.Sprintf("get observability info error: %v", err), c.logger)
		return
	}
	link := func(checker string) string {
		return info.ConsoleHost + "/keyevents/monitorChart?leftActiveKey=Events&activeName=Events&query=df_monitor_checker_name" + url.QueryEscape(`:"`+checker+`"`)
	}

	client := guanceyun.NewClient(info.Host, info.ApiKey)
	timeout := time.After(time.Duration(c.jobTaskSpec.CheckTime) * time.Minute)

	checkArgs := make([]*guanceyun.SearchEventByMonitorArg, 0)
	checkMap := make(map[string]*commonmodels.GuanceyunMonitor)
	for _, monitor := range c.jobTaskSpec.Monitors {
		checkArgs = append(checkArgs, &guanceyun.SearchEventByMonitorArg{
			CheckerName: monitor.Name,
			CheckerID:   monitor.ID,
		})
		checkMap[monitor.ID] = monitor
		monitor.Status = StatusChecking
	}
	c.ack()

	check := func() (bool, error) {
		triggered := false
		resp, err := client.SearchEventByChecker(checkArgs, time.Now().UnixMilli(), time.Now().UnixMilli())
		if err != nil {
			return false, err
		}

		for _, eventResp := range resp {
			if checker, ok := checkMap[eventResp.CheckerID]; ok {
				// checker has been triggered if url not empty, ignore it
				if checker.Url == "" && guanceyun.LevelMap[eventResp.EventLevel] >= guanceyun.LevelMap[checker.Level] {
					checker.Status = StatusAbnormal
					checker.Url = link(eventResp.CheckerName)
					triggered = true
				}
			} else {
				return false, fmt.Errorf("checker %s %s not found", eventResp.CheckerID, eventResp.CheckerName)
			}
		}
		return triggered, nil
	}
	setNoEventMonitorStatusUnfinished := func() {
		for _, monitor := range c.jobTaskSpec.Monitors {
			if monitor.Url == "" {
				monitor.Status = StatusUnfinished
			}
		}
	}
	isAllMonitorHasEvent := func() bool {
		for _, monitor := range c.jobTaskSpec.Monitors {
			if monitor.Url == "" {
				return false
			}
		}
		return true
	}
	isNoMonitorHasEvent := func() bool {
		for _, monitor := range c.jobTaskSpec.Monitors {
			if monitor.Url != "" {
				return false
			}
		}
		return true
	}
	for {
		c.ack()
		// GuanceYun default openapi limit is 20 per minute
		time.Sleep(time.Second * 10)

		triggered, err := check()
		if err != nil {
			logError(c.job, fmt.Sprintf("check error: %v", err), c.logger)
			return
		}
		switch c.jobTaskSpec.CheckMode {
		case "trigger":
			if triggered {
				setNoEventMonitorStatusUnfinished()
				c.job.Status = config.StatusFailed
				return
			}
		case "monitor":
			if isAllMonitorHasEvent() {
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
			if isNoMonitorHasEvent() {
				c.job.Status = config.StatusPassed
			} else {
				c.job.Status = config.StatusFailed
			}
			// no event triggered in check time
			for _, monitor := range c.jobTaskSpec.Monitors {
				if monitor.Url == "" {
					monitor.Status = StatusNormal
				}
			}
			return
		default:
		}
	}
}

func (c *GuanceyunCheckJobCtl) SaveInfo(ctx context.Context) error {
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
