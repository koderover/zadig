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
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gitee"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gitlab"
	codehostdb "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
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
			queue:  make(chan *task, 500),
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
		zap.String("namespace", t.namespace),
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

func ensureSafeDelete(ref, repoName, repoOwner, repoAddress string) (bool, error) {
	// only handle webhooks created by service
	if !strings.HasPrefix(ref, ServicePrefix) || !strings.HasSuffix(ref, "trigger") {
		return true, nil
	}

	serviceName := strings.TrimPrefix(ref, ServicePrefix)
	serviceName = strings.TrimSuffix(serviceName, "-trigger")

	// find existing service with same name
	services, err := mongodb.NewServiceColl().ListMaxRevisionsByProject(serviceName, setting.K8SDeployType)
	if err != nil {
		return false, err
	}

	// check if the webhook reference points to other existing service
	for _, service := range services {
		if service.RepoName != repoName || service.RepoOwner != repoOwner {
			continue
		}
		codeHostInfo, err := codehostdb.NewCodehostColl().GetCodeHostByID(service.CodehostID, false)
		if err == nil {
			if codeHostInfo.Address != repoAddress {
				continue
			} else {
				return false, nil
			}
		}
	}
	return true, nil
}

func removeWebhook(t *task, logger *zap.Logger) {
	coll := mongodb.NewWebHookColl()
	var cl hookCreateDeleter
	var err error

	switch t.from {
	case setting.SourceFromGithub:
		cl = github.NewClient(t.token, config.ProxyHTTPSAddr(), t.enableProxy)
	case setting.SourceFromGitlab:
		cl, err = gitlab.NewClient(t.ID, t.address, t.token, config.ProxyHTTPSAddr(), t.enableProxy, t.disbaleSSL)
		if err != nil {
			t.err = err
			t.doneCh <- struct{}{}
			return
		}
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		cl = gitee.NewClient(t.ID, t.token, config.ProxyHTTPSAddr(), t.enableProxy, t.address)
	default:
		t.err = fmt.Errorf("invaild source: %s", t.from)
		t.doneCh <- struct{}{}
		return
	}

	repoNamespace := t.namespace
	if repoNamespace == "" {
		repoNamespace = t.owner
	}

	webhook, err := coll.Find(repoNamespace, t.repo, t.address)
	if err != nil {
		t.err = err
		t.doneCh <- struct{}{}
		return
	}

	logger = logger.With(zap.String("hookID", webhook.HookID))
	logger.Info("Removing webhook")

	//ensure safe remove, same reference may be used by multiple services
	safe, err := ensureSafeDelete(t.ref, t.repo, repoNamespace, t.address)
	if err != nil {
		t.err = err
		t.doneCh <- struct{}{}
		return
	}

	if safe {
		updated, err := coll.RemoveReference(repoNamespace, t.repo, t.address, t.ref)
		if err != nil {
			t.err = err
			t.doneCh <- struct{}{}
			return
		}

		if len(updated.References) == 0 {
			if !t.isManual {
				logger.Info("Deleting webhook")
				err = cl.DeleteWebHook(repoNamespace, t.repo, webhook.HookID)
				if err != nil {
					logger.Error("Failed to delete webhook", zap.Error(err))
					t.err = err
					t.doneCh <- struct{}{}
					return
				}
			}

			err = coll.Delete(repoNamespace, t.repo, t.address)
			if err != nil {
				logger.Error("Failed to delete webhook record in db", zap.Error(err))
				t.err = err
			}
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
		cl = github.NewClient(t.token, config.ProxyHTTPSAddr(), t.enableProxy)
	case setting.SourceFromGitlab:
		cl, err = gitlab.NewClient(t.ID, t.address, t.token, config.ProxyHTTPSAddr(), t.enableProxy, t.disbaleSSL)
		if err != nil {
			t.err = err
			t.doneCh <- struct{}{}
			return
		}
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		cl = gitee.NewClient(t.ID, t.token, config.ProxyHTTPSAddr(), t.enableProxy, t.address)
	default:
		t.err = fmt.Errorf("invaild source: %s", t.from)
		t.doneCh <- struct{}{}
		return
	}

	repoNamespace := t.namespace
	if repoNamespace == "" {
		repoNamespace = t.owner
	}

	logger.Info("Adding webhook")
	created, err := coll.AddReferenceOrCreate(repoNamespace, t.repo, t.address, t.ref)
	if err != nil || !created {
		t.err = err
		t.doneCh <- struct{}{}
		return
	}

	if !t.isManual {
		logger.Info("Creating webhook")
		hookID, err = cl.CreateWebHook(repoNamespace, t.repo)
		if err != nil {
			t.err = err
			logger.Error("Failed to create webhook", zap.Error(err))
			if err = coll.Delete(repoNamespace, t.repo, t.address); err != nil {
				logger.Error("Failed to delete webhook record in db", zap.Error(err))
			}
		} else {
			if hookID != "" {
				if err = coll.Update(repoNamespace, t.repo, t.address, hookID); err != nil {
					t.err = err
					logger.Error("Failed to update webhook", zap.Error(err))
				}
			}
		}
	}

	t.doneCh <- struct{}{}
}
