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

package bundle

import (
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"

	"github.com/koderover/zadig/pkg/tool/log"
)

const UpdateKey = "opa_bundle"

var q = workqueue.New()

func RefreshOPABundle() {
	q.Add(UpdateKey)
}

func NewBundleController() *controller {
	return &controller{logger: log.Logger()}
}

type controller struct {
	logger *zap.Logger
}

// Run starts the controller and blocks until receiving signal from stopCh.
// Since the merge of aslan and policy, the worker parameter has to be added.
// It won't affect how the old logic works.
func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	c.logger.Info("Starting bundle controller")
	defer c.logger.Info("Shutting down bundle controller")

	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue. It returns false when it's time to quit.
func (c *controller) processNextWorkItem() bool {
	t, shutdown := q.Get()
	if shutdown {
		return false
	}
	defer q.Done(t)

	if t.(string) != UpdateKey {
		return true
	}

	err := GenerateOPABundle()
	if err != nil {
		c.logger.With(zap.Error(err)).Error("Failed to generate OPA bundle")
	}

	return true
}
