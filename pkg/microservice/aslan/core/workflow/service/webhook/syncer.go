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
			err := mongodb.NewWorkflowColl().Replace(workflow)
			if err != nil {
				log.Errorf("Failed to update workflow, err: %s", err)
				continue
			}
		}

		err := commonservice.ProcessWebhook(workflow.HookCtl.Items, nil, webhook.WorkflowPrefix+workflow.Name, logger)
		if err != nil {
			log.Errorf("Failed to process webhook, err: %s", err)
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
			err := mongodb.NewPipelineColl().Upsert(pipeline)
			if err != nil {
				log.Errorf("Failed to update pipeline, err: %s", err)
				continue
			}
		}

		err := commonservice.ProcessWebhook(pipeline.Hook.GitHooks, nil, webhook.PipelinePrefix+pipeline.Name, logger)
		if err != nil {
			log.Errorf("Failed to process webhook, err: %s", err)
		}
	}
}
