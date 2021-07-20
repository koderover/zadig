package webhook

import (
	"context"

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
		err := commonservice.ProcessWebhook(workflow.HookCtl.Items, nil, webhook.WorkflowPrefix+workflow.Name, logger)
		if err != nil {
			log.Errorf("Failed to process webhook, err: %s", err)
		}
	}

	pipelines, _ := mongodb.NewPipelineColl().List(&mongodb.PipelineListOption{})
	for _, pipeline := range pipelines {
		logger.Debugf("Processing pipeline %s", pipeline.Name)
		err := commonservice.ProcessWebhook(pipeline.Hook.GitHooks, nil, webhook.PipelinePrefix+pipeline.Name, logger)
		if err != nil {
			log.Errorf("Failed to process webhook, err: %s", err)
		}
	}
}
