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
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	jenkins "github.com/bndr/gojenkins"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type JenkinsJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	jobTaskSpec *commonmodels.JobTaskJenkinsSpec
	ack         func()
}

func NewJenkinsJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *JenkinsJobCtl {
	jobTaskSpec := &commonmodels.JobTaskJenkinsSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &JenkinsJobCtl{
		job:         job,
		workflowCtx: workflowCtx,
		logger:      logger,
		ack:         ack,
		jobTaskSpec: jobTaskSpec,
	}
}

func (c *JenkinsJobCtl) Clean(ctx context.Context) {}

func (c *JenkinsJobCtl) Run(ctx context.Context) {
	c.job.Status = config.StatusPrepare
	c.ack()

	info, err := mongodb.NewJenkinsIntegrationColl().Get(c.jobTaskSpec.ID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}
	c.jobTaskSpec.Host = info.URL

	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: transport}
	jenkinsClient, err := jenkins.CreateJenkins(client, info.URL, info.Username, info.Password).Init(context.Background())

	if err != nil {
		logError(c.job, fmt.Sprintf("failed to create jenkins client, error is: %s", err), c.logger)
		return
	}

	job, err := jenkinsClient.GetJob(context.TODO(), c.jobTaskSpec.Job.JobName)
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to get jenkins job, error is: %s", err), c.logger)
		return
	}

	params := make(map[string]string)
	for _, parameter := range c.jobTaskSpec.Job.Parameters {
		fmt.Printf("parameter: %v\n", parameter)
		params[parameter.Name] = parameter.Value
	}

	queueid, err := job.InvokeSimple(context.TODO(), params)
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to invoke jenkins job, error is: %s", err), c.logger)
		return
	}

	build, err := jenkinsClient.GetBuildFromQueueID(context.TODO(), queueid)
	if err != nil {
		logError(c.job, fmt.Sprintf("failed to get jenkins build, error is: %s", err), c.logger)
		return
	}

	// frontend will try to get log when job is running, so we need to set running status after setting job id
	c.jobTaskSpec.Job.JobID = int(build.GetBuildNumber())
	c.job.Status = config.StatusRunning
	c.ack()
	for build.IsRunning(context.TODO()) {
		select {
		case <-ctx.Done():
			if _, err := build.Stop(context.Background()); err != nil {
				log.Warnf("job jenkins failed to stop jenkins job, error: %s", err)
			}
			c.job.Status = config.StatusCancelled
			consoleOutput, err := build.GetConsoleOutputFromIndex(context.TODO(), 0)
			if err != nil {
				log.Warnf("job jenkins failed to get logs from jenkins job, error: %s", err)
			}
			c.jobTaskSpec.Job.JobOutput = consoleOutput.Content
			return
		default:
			time.Sleep(time.Second)
			build.Poll(context.TODO())
		}
	}

	consoleOutput, err := build.GetConsoleOutputFromIndex(context.TODO(), 0)
	if err != nil {
		log.Warnf("job jenkins failed to get logs from jenkins job, error: %s", err)
	}
	c.jobTaskSpec.Job.JobOutput = consoleOutput.Content

	if !build.IsGood(context.TODO()) {
		c.job.Status = config.StatusFailed
		return
	}

	c.job.Status = config.StatusPassed
	return
}

func (c *JenkinsJobCtl) SaveInfo(ctx context.Context) error {
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
