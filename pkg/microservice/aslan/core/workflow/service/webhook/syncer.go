/*
Copyright 2021 The KodeRover Authors.

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

package webhook

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/pkg/tool/log"
)

func SyncWebHooks() {
	logger := log.SugaredLogger()

	count, _ := mongodb.NewWebHookColl().EstimatedDocumentCount(context.TODO())
	if count > 0 {
		return
	}

	workflows, _ := mongodb.NewWorkflowColl().List(&mongodb.ListWorkflowOption{})
	for _, workflow := range workflows {
		logger.Debugf("Processing workflow %s", workflow.Name)
		if workflow.HookCtl != nil {
			for i, h := range workflow.HookCtl.Items {
				h.MainRepo.Name = fmt.Sprintf("trigger%d", i+1)
				workflow.HookCtl.Items[i] = h
			}
			if err := mongodb.NewWorkflowColl().Replace(workflow); err != nil {
				log.Errorf("Failed to update workflow, err: %s", err)
				continue
			}

			if err := commonservice.ProcessWebhook(workflow.HookCtl.Items, nil, webhook.WorkflowPrefix+workflow.Name, logger); err != nil {
				log.Errorf("Failed to process webhook, err: %s", err)
			}
		}
	}

	pipelines, _ := mongodb.NewPipelineColl().List(&mongodb.PipelineListOption{})
	for _, pipeline := range pipelines {
		logger.Debugf("Processing pipeline %s", pipeline.Name)
		if pipeline.Hook != nil {
			for i, h := range pipeline.Hook.GitHooks {
				h.Name = fmt.Sprintf("trigger%d", i+1)
				pipeline.Hook.GitHooks[i] = h
			}
			if err := mongodb.NewPipelineColl().Upsert(pipeline); err != nil {
				log.Errorf("Failed to update pipeline, err: %s", err)
				continue
			}

			if err := commonservice.ProcessWebhook(pipeline.Hook.GitHooks, nil, webhook.PipelinePrefix+pipeline.Name, logger); err != nil {
				log.Errorf("Failed to process webhook, err: %s", err)
			}
		}
	}
}
