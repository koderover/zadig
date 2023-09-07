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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	"github.com/koderover/zadig/pkg/tool/log"
)

var reg = regexp.MustCompile(`{{\.job\.([a-zA-Z0-9_-]+)\.([a-zA-Z0-9_-]+)\.([a-zA-Z0-9_-]+)\.IMAGES}}`)

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
	// after stage run, concurrent competition not exist

	//TODO DEBUG
	b, _ := json.MarshalIndent(c.workflowCtx.GlobalContextGetAll(), "", "  ")
	log.Debugf("workflow context: %s", string(b))
	buildJobImages := map[string][]string{}
	for k, v := range c.workflowCtx.GlobalContextGetAll() {
		list := reg.FindStringSubmatch(k)
		if len(list) > 0 {
			buildJobImages[list[1]] = append(buildJobImages[list[1]], v)
		}
	}
	for buildName, images := range buildJobImages {
		c.workflowCtx.GlobalContextSet(fmt.Sprintf("{{.job.%s.IMAGES}}", buildName), strings.Join(images, ","))
	}
}
