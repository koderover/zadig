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

package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/webhook"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
)

func ProcessGitWebHook(c *gin.Context) {
	ctx := internalhandler.NewContext(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	payload, err := c.GetRawData()
	if err != nil {
		ctx.RespErr = err
		return
	}

	if github.WebHookType(c.Request) != "" {
		ctx.RespErr = processGithub(payload, c.Request, ctx.RequestID, ctx.Logger)
	} else if gitlab.HookEventType(c.Request) != "" {
		ctx.RespErr = webhook.ProcessGitlabHook(payload, c.Request, ctx.RequestID, ctx.Logger)
	} else if gitee.HookEventType(c.Request) != "" {
		ctx.RespErr = webhook.ProcessGiteeHook(payload, c.Request, ctx.RequestID, ctx.Logger)
	} else {
		ctx.RespErr = webhook.ProcessGerritHook(payload, c.Request, ctx.RequestID, ctx.Logger)
	}
}

func processGithub(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	errs := &multierror.Error{}

	// trigger classic pipeline
	//_, err := webhook.ProcessGithubHook(payload, req, requestID, log)
	//if err != nil {
	//	log.Errorf("error happens to trigger classic pipeline %v", err)
	//	errs = multierror.Append(errs, err)
	//}

	// trigger workflow
	// err := webhook.ProcessGithubWebHook(payload, req, requestID, log)

	// if err != nil {
	// 	log.Errorf("error happens to trigger workflow %v", err)
	// 	errs = multierror.Append(errs, err)
	// }
	//测试管理webhook
	err := webhook.ProcessGithubWebHookForTest(payload, req, requestID, log)
	if err != nil {
		log.Errorf("error happens to trigger ProcessGithubWebHookForTest %v", err)
		errs = multierror.Append(errs, err)
	}
	// webhooks for scanning task
	err = webhook.ProcessGithubWebhookForScanning(payload, req, requestID, log)
	if err != nil {
		log.Errorf("error happens to trigger Scanning for github %v", err)
		errs = multierror.Append(errs, err)
	}
	// webhooks for workflow v4
	err = webhook.ProcessGithubWebHookForWorkflowV4(payload, req, requestID, log)
	if err != nil {
		log.Errorf("error happens to trigger workflowV4 for github %v", err)
		errs = multierror.Append(errs, err)
	}
	return errs.ErrorOrNil()
}
