/*
Copyright 2022 The KodeRover Authors.

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

package workflowcontroller

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
)

var reg = regexp.MustCompile(`{{\.job\.([\p{L}\p{N}]+)\.([a-zA-Z0-9_-]+)\.([a-zA-Z0-9_-]+)\.output\.IMAGE}}`)

type CustomStageCtl struct {
	stage       *commonmodels.StageTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	ack         func()
}

func NewCustomStageCtl(stage *commonmodels.StageTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger, ack func()) *CustomStageCtl {
	return &CustomStageCtl{
		stage:       stage,
		logger:      logger,
		workflowCtx: workflowCtx,
		ack:         ack,
	}
}

func (c *CustomStageCtl) Run(ctx context.Context, concurrency int) {
	var workerConcurrency = 1
	if c.stage.Parallel {
		if len(c.stage.Jobs) > int(concurrency) {
			workerConcurrency = int(concurrency)
		} else {
			workerConcurrency = len(c.stage.Jobs)
		}

	}
	jobcontroller.RunJobs(ctx, c.stage.Jobs, c.workflowCtx, workerConcurrency, c.logger, c.ack)
}

func (c *CustomStageCtl) AfterRun() {
	// set IMAGES workflow variable
	// set after a stage has been done for build and some other type job maybe split to many job tasks in one stage
	// after stage run, concurrent competition of workflowCtx.GlobalContext is not exist

	jobImages := map[string][]string{}
	for k, v := range c.workflowCtx.GlobalContextGetAll() {
		list := reg.FindStringSubmatch(k)
		if len(list) > 0 {
			jobImages[list[1]] = append(jobImages[list[1]], v)
		}
	}
	for jobName, images := range jobImages {
		key := fmt.Sprintf("{{.job.%s.IMAGES}}", jobName)
		if _, ok := c.workflowCtx.GlobalContextGet(key); !ok {
			c.workflowCtx.GlobalContextSet(key, strings.Join(images, ","))
		}
	}

	c.setVMDeployPkgFiles()
}

// setVMDeployPkgFiles sets the PKG_FILES variable of vm deploy jobs. When the
// artifacts come from an upstream build job, the package names are only known
// at runtime, so the variable cannot be rendered at task creation.
func (c *CustomStageCtl) setVMDeployPkgFiles() {
	jobNames := make([]string, 0)
	jobPkgFiles := map[string][]string{}
	for _, job := range c.stage.Jobs {
		if job == nil || job.JobType != string(config.JobZadigVMDeploy) {
			continue
		}

		spec := &commonmodels.JobTaskFreestyleSpec{}
		if err := commonmodels.IToi(job.Spec, spec); err != nil {
			c.logger.Errorf("failed to decode vm deploy job %s spec: %v", job.Name, err)
			continue
		}

		pkgFile := ""
		for _, env := range spec.Properties.Envs {
			if env != nil && env.Key == "PKG_FILE" {
				pkgFile = env.GetValue()
				break
			}
		}
		if pkgFile == "" || strings.HasPrefix(pkgFile, "{{.") {
			continue
		}

		if _, ok := jobPkgFiles[job.OriginName]; !ok {
			jobNames = append(jobNames, job.OriginName)
		}
		jobPkgFiles[job.OriginName] = append(jobPkgFiles[job.OriginName], pkgFile)
	}

	// always overwrite: a retried stage must refresh the value with the
	// packages of job tasks that only ran in the retry.
	for _, jobName := range jobNames {
		key := fmt.Sprintf("{{.job.%s.PKG_FILES}}", jobName)
		c.workflowCtx.GlobalContextSet(key, strings.Join(jobPkgFiles[jobName], ","))
	}
}
