package workflow

import (
	"context"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflowcontroller"
)

func CreateWorkflowV4(user string, workflowTask *commonmodels.WorkflowTask, logger *zap.SugaredLogger) error {
	ctl := workflowcontroller.NewWorkflowController(workflowTask, logger)
	ctx := context.Background()
	go ctl.Run(ctx, 2)
	return nil
}
