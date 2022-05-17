package stepcontroller

import (
	"context"
	"sync"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/tool/log"
	"go.uber.org/zap"
)

type Step interface {
	// Type ...
	stepType() config.StepType

	// Status ...
	status() config.Status

	// outputVariables
	outputVariables() map[string]string

	// Run ...
	run(ctx context.Context)
}

type StepTask commonmodels.StepTask

// render context key value to step spec
func (s *StepTask) renderSpec(globalContext, jobContext *sync.Map) {
}

// render context key value to step spec
func (s *StepTask) writeOutputToContext(globalContext, jobContext *sync.Map, outputMap map[string]string) {
}

func (s *StepTask) Run(ctx context.Context, globalContext, jobContext *sync.Map, ack func() error, logger *zap.SugaredLogger) {
	done := make(chan struct{}, 1)
	go func() {
		s.run(ctx, globalContext, jobContext, ack, logger)
		done <- struct{}{}
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
		s.Status = config.StatusCancelled
		return
	}
}

func (s *StepTask) run(ctx context.Context, globalContext, jobContext *sync.Map, ack func() error, logger *zap.SugaredLogger) {
	s.Status = config.StatusRunning
	ack()
	s.renderSpec(globalContext, jobContext)
	var stepController Step
	switch s.StepType {
	case config.StepShell:
		stepController = NewShellStepController(s, ack, logger)
	default:
		log.Errorf("no matched step type found: %s", s.StepType)
	}
	stepController.run(ctx)
	output := stepController.outputVariables()
	s.writeOutputToContext(globalContext, jobContext, output)
}
