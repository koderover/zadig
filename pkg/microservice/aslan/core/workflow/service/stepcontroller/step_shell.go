package stepcontroller

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type shellStep struct {
	stepTask   *StepTask
	stepSpec   *ShellSpec
	kubeClient client.Client
	clientset  kubernetes.Interface
	restConfig *rest.Config
	logger     *zap.SugaredLogger

	ack func() error
}

type ShellSpec struct {
	Script string `bson:"script"            yaml:"script"`
}

func NewShellStepController(stepTask *StepTask, ack func() error, logger *zap.SugaredLogger) *shellStep {
	shellStep := &shellStep{
		stepTask:   stepTask,
		kubeClient: krkubeclient.Client(),
		clientset:  krkubeclient.Clientset(),
		restConfig: krkubeclient.RESTConfig(),
		ack:        ack,
		logger:     logger,
	}
	b, err := json.Marshal(stepTask.Spec)
	if err != nil {
		logger.Errorf("marshal shell spec error: %s", err)
		return shellStep
	}
	shellSpec := ShellSpec{}
	if err := json.Unmarshal(b, &shellSpec); err != nil {
		logger.Errorf("unmarshal shell spec error: %s", err)
	}
	shellStep.stepSpec = &shellSpec
	return shellStep
}

func (s *shellStep) stepType() config.StepType {
	return config.StepShell
}

func (s *shellStep) status() config.Status {
	return s.stepTask.Status
}

func (s *shellStep) outputVariables() map[string]string {
	return map[string]string{}

}

func (s *shellStep) run(ctx context.Context) {
	r := rand.Intn(10)
	time.Sleep(time.Duration(r) * time.Second)
	s.stepTask.Status = config.StatusPassed
}
