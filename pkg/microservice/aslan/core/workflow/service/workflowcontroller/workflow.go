package workflowcontroller

import (
	"context"
	"sync"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/jobcontroller"
	"go.uber.org/zap"
)

var approveChannelMap sync.Map
var cancelChannelMap sync.Map

type workflowCtl struct {
	workflow      *commonmodels.WorkflowV4
	workflowTask  *commonmodels.WorkflowTask
	globalContext *sync.Map
	logger        *zap.SugaredLogger
	ack           func() error
}

func NewWorkflowController(workflow *commonmodels.WorkflowV4, workflowTask *commonmodels.WorkflowTask, logger *zap.SugaredLogger) *workflowCtl {
	ctl := &workflowCtl{
		workflow:     workflow,
		workflowTask: workflowTask,
		logger:       logger,
	}
	ctl.ack = ctl.updateWorkflowTask
	return ctl
}

func (c *workflowCtl) Run(ctx context.Context) error {
	defer c.ack()
	defer updateworkflowStatus(c.workflowTask)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cancelChannelMap.Store(c.workflowTask.TaskID, cancel)
	for _, stage := range c.workflowTask.Stages {
		c.runStage(ctx, stage)
	}
	return nil
}

func (c *workflowCtl) runStage(ctx context.Context, stage *commonmodels.StageTask) {
	defer c.ack()
	defer updateStageStatus(stage)
	switch stage.StageType {
	case "approve":
		//TODO wait for approve
	default:
		// TODO run job in parallel
		for _, job := range stage.Jobs {
			jobCtl := jobcontroller.NewJobCtl(job, c.globalContext, c.ack, c.logger)
			jobCtl.Run(ctx)
		}
	}
}

func updateStageStatus(stage *commonmodels.StageTask) {}

func updateworkflowStatus(workflow *commonmodels.WorkflowTask) {}

func (c *workflowCtl) updateWorkflowTask() error {
	// TODO update workflow task
	return nil
}
