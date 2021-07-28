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
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/codehub"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

type hookCreateDeleter interface {
	CreateWebHook(owner, repo string) (string, error)
	DeleteWebHook(owner, repo string, hookID string) error
}

type controller struct {
	queue chan *task

	logger *zap.Logger
}

var once sync.Once
var c *controller

func webhookController() *controller {
	once.Do(func() {
		c = &controller{
			queue:  make(chan *task, 100),
			logger: log.Logger(),
		}
	})

	return c
}

func NewWebhookController() *controller {
	return webhookController()
}

// Run starts the controller and blocks until receiving signal from stopCh.
func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer close(c.queue)

	if workers > 1 {
		c.logger.Panic("webhook controller only accept one worker")
	}

	c.logger.Info("Starting webhook controller")
	defer c.logger.Info("Shutting down webhook controller")

	//for i := 0; i < workers; i++ {
	//	go wait.Until(c.runWorker, time.Second, stopCh)
	//}
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (c *controller) processNextWorkItem() bool {
	t, ok := <-c.queue
	if !ok {
		return false
	}

	logger := c.logger.With(
		zap.String("owner", t.owner),
		zap.String("repo", t.repo),
		zap.String("address", t.address),
		zap.String("ref", t.ref),
	)

	if t.err != nil {
		logger.Warn(fmt.Sprintf("Task is canceled with reason: %s", t.err))
		return true
	}

	if t.add {
		addWebhook(t, logger)
	} else {
		removeWebhook(t, logger)
	}

	return true
}

func removeWebhook(t *task, logger *zap.Logger) {
	coll := mongodb.NewWebHookColl()
	var cl hookCreateDeleter
	var err error

	switch t.from {
	case setting.SourceFromGithub:
		cl = github.NewClient(t.token, config.ProxyHTTPSAddr())
	case setting.SourceFromGitlab:
		cl, err = gitlab.NewClient(t.address, t.token)
		if err != nil {
			t.err = err
			t.doneCh <- struct{}{}
			return
		}
	case setting.SourceFromCodeHub:
		cl = codehub.NewClient(t.ak, t.sk, t.region)
	default:
		t.err = fmt.Errorf("invaild source: %s", t.from)
		t.doneCh <- struct{}{}
		return
	}

	webhook, err := coll.Find(t.owner, t.repo, t.address)
	if err != nil {
		t.err = err
		t.doneCh <- struct{}{}
		return
	}
	logger.Info("Removing webhook")
	updated, err := coll.RemoveReference(t.owner, t.repo, t.address, t.ref)
	if err != nil {
		t.err = err
		t.doneCh <- struct{}{}
		return
	}

	if len(updated.References) == 0 {
		logger.Info("Deleting webhook")
		err = cl.DeleteWebHook(t.owner, t.repo, webhook.HookID)
		if err != nil {
			logger.Error("Failed to delete webhook", zap.Error(err))
			t.err = err
			t.doneCh <- struct{}{}
			return
		}

		err = coll.Delete(t.owner, t.repo, t.address)
		if err != nil {
			logger.Error("Failed to delete webhook record in db", zap.Error(err))
			t.err = err
		}
	}

	t.doneCh <- struct{}{}
}

func addWebhook(t *task, logger *zap.Logger) {
	coll := mongodb.NewWebHookColl()
	var cl hookCreateDeleter
	var err error
	var hookID string

	switch t.from {
	case setting.SourceFromGithub:
		cl = github.NewClient(t.token, config.ProxyHTTPSAddr())
	case setting.SourceFromGitlab:
		cl, err = gitlab.NewClient(t.address, t.token)
		if err != nil {
			t.err = err
			t.doneCh <- struct{}{}
			return
		}

	case setting.SourceFromCodeHub:
		cl = codehub.NewClient(t.ak, t.sk, t.region)
	default:
		t.err = fmt.Errorf("invaild source: %s", t.from)
		t.doneCh <- struct{}{}
		return
	}

	logger.Info("Adding webhook")
	created, err := coll.AddReferenceOrCreate(t.owner, t.repo, t.address, t.ref)
	if err != nil || !created {
		t.err = err
		t.doneCh <- struct{}{}
		return
	}

	logger.Info("Creating webhook")
	hookID, err = cl.CreateWebHook(t.owner, t.repo)
	if err != nil {
		t.err = err
		logger.Error("Failed to create webhook", zap.Error(err))
		if err = coll.Delete(t.owner, t.repo, t.address); err != nil {
			logger.Error("Failed to delete webhook record in db", zap.Error(err))
		}
	}

	if hookID != "" {
		if err = coll.Update(t.owner, t.repo, t.address, hookID); err != nil {
			t.err = err
			logger.Error("Failed to update webhook", zap.Error(err))
		}
	}

	t.doneCh <- struct{}{}
}
