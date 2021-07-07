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

package workflow

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

func CreateGerritWebhook(workflow *commonmodels.Workflow, log *zap.SugaredLogger) error {
	if workflow != nil && workflow.HookCtl != nil && workflow.HookCtl.Enabled {
		for _, workflowWebhook := range workflow.HookCtl.Items {
			if workflowWebhook == nil {
				continue
			}

			opt := &codehost.Option{
				CodeHostID: workflowWebhook.MainRepo.CodehostID,
			}
			detail, err := codehost.GetCodeHostInfo(opt)
			if err != nil {
				return err
			}

			if detail.Type != setting.SourceFromGerrit {
				continue
			}

			cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
			webhookURL := fmt.Sprintf("/%s/%s/%s/%s", "a/config/server/webhooks~projects", gerrit.Escape(workflowWebhook.MainRepo.RepoName), "remotes", workflow.Name)
			if _, err := cl.Get(webhookURL); err != nil {
				log.Errorf("CreateGerritWebhook getGerritWebhook err:%v", err)
				//创建webhook
				gerritWebhook := &gerrit.Webhook{
					URL:       fmt.Sprintf("%s?name=%s", config.WebHookURL(), workflow.Name),
					MaxTries:  setting.MaxTries,
					SslVerify: false,
				}
				for _, event := range workflowWebhook.MainRepo.Events {
					gerritWebhook.Events = append(gerritWebhook.Events, string(event))
				}

				_, err = cl.Put(webhookURL, httpclient.SetBody(gerritWebhook))
				if err != nil {
					log.Errorf("CreateGerritWebhook addGerritWebhook err:%v", err)
					return err
				}
			}
		}
	}

	return nil
}

// UpdateGerritWebhook 更新gerrit webhook
func UpdateGerritWebhook(currentWorkflow *commonmodels.Workflow, log *zap.SugaredLogger) error {
	oldWorkflow, err := FindWorkflow(currentWorkflow.Name, log)
	if err != nil {
		log.Errorf("UpdateGerritWebhook get workflow err:%v", err)
		return err
	}

	if oldWorkflow != nil && oldWorkflow.HookCtl != nil {
		for _, oldWorkflowWebhook := range oldWorkflow.HookCtl.Items {
			if oldWorkflowWebhook == nil {
				continue
			}

			opt := &codehost.Option{
				CodeHostID: oldWorkflowWebhook.MainRepo.CodehostID,
			}
			detail, err := codehost.GetCodeHostInfo(opt)
			if err != nil {
				return err
			}

			if detail.Type == setting.SourceFromGerrit {
				cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
				webhookURLPrefix := fmt.Sprintf("/%s/%s/%s", "a/config/server/webhooks~projects", gerrit.Escape(oldWorkflowWebhook.MainRepo.RepoName), "remotes")
				_, _ = cl.Delete(fmt.Sprintf("%s/%s", webhookURLPrefix, gerrit.RemoteName))
				_, err = cl.Delete(fmt.Sprintf("%s/%s", webhookURLPrefix, oldWorkflow.Name))
				if err != nil {
					log.Errorf("UpdateGerritWebhook err:%v", err)
				}
			}
		}
	}
	if currentWorkflow != nil && currentWorkflow.HookCtl != nil && currentWorkflow.HookCtl.Enabled {
		for _, workflowWebhook := range currentWorkflow.HookCtl.Items {
			if workflowWebhook == nil {
				continue
			}

			opt := &codehost.Option{
				CodeHostID: workflowWebhook.MainRepo.CodehostID,
			}
			detail, err := codehost.GetCodeHostInfo(opt)
			if err != nil {
				return err
			}

			if detail.Type != setting.SourceFromGerrit {
				continue
			}

			cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
			webhookURL := fmt.Sprintf("/%s/%s/%s/%s", "a/config/server/webhooks~projects", gerrit.Escape(workflowWebhook.MainRepo.RepoName), "remotes", currentWorkflow.Name)
			//创建webhook
			gerritWebhook := &gerrit.Webhook{
				URL:       fmt.Sprintf("%s?name=%s", config.WebHookURL(), currentWorkflow.Name),
				MaxTries:  setting.MaxTries,
				SslVerify: false,
			}
			for _, event := range workflowWebhook.MainRepo.Events {
				gerritWebhook.Events = append(gerritWebhook.Events, string(event))
			}

			_, err = cl.Put(webhookURL, httpclient.SetBody(gerritWebhook))
			if err != nil {
				log.Errorf("UpdateGerritWebhook addGerritWebhook err:%v", err)
				return err
			}
		}
	}
	return nil
}

// DeleteGerritWebhook 删除gerrit webhook
func DeleteGerritWebhook(workflow *commonmodels.Workflow, log *zap.SugaredLogger) error {
	if workflow != nil && workflow.HookCtl != nil {
		for _, workflowWebhook := range workflow.HookCtl.Items {
			if workflowWebhook == nil {
				continue
			}

			opt := &codehost.Option{
				CodeHostID: workflowWebhook.MainRepo.CodehostID,
			}
			detail, err := codehost.GetCodeHostInfo(opt)
			if err != nil {
				return err
			}

			if detail.Type == setting.SourceFromGerrit {
				cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
				webhookURLPrefix := fmt.Sprintf("/%s/%s/%s", "a/config/server/webhooks~projects", gerrit.Escape(workflowWebhook.MainRepo.RepoName), "remotes")
				_, _ = cl.Delete(fmt.Sprintf("%s/%s", webhookURLPrefix, gerrit.RemoteName))
				_, err = cl.Delete(fmt.Sprintf("%s/%s", webhookURLPrefix, workflow.Name))
				if err != nil {
					log.Errorf("DeleteGerritWebhook err:%v", err)
				}
			}
		}
	}

	return nil
}
