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

	jenkins "github.com/bndr/gojenkins"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
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
	c.job.Status = config.StatusRunning
	c.ack()

	info, err := mongodb.NewJenkinsIntegrationColl().Get(c.jobTaskSpec.JenkinsID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return
	}

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

	for _, parameter := range c.jobTaskSpec.Job.Parameter {
		params[parameter.Name] = parameter.Value
	}

	var offset int64 = 0

	queueid, err := job.InvokeSimple(context.TODO(), params)

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
