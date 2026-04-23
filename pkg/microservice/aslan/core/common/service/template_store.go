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

package service

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"go.uber.org/zap"
)

func GetDockerfileTemplateContent(id string) (string, error) {
	dockerfileTemplate, err := mongodb.NewDockerfileTemplateColl().GetById(id)
	if err != nil {
		return "", err
	}
	return dockerfileTemplate.Content, nil
}

func ProcessYamlTemplateWebhook(updated, current *models.YamlTemplate, logger *zap.SugaredLogger) error {
	var updatedHooks, currentHooks []*webhook.WebHook
	if current != nil {
		namespace := current.Namespace
		if namespace == "" {
			namespace = current.RepoOwner
		}
		switch current.Source {
		case setting.SourceFromGithub, setting.SourceFromGitlab, setting.SourceFromGitee, setting.SourceFromGiteeEE:
			currentHooks = append(currentHooks, &webhook.WebHook{
				Owner:      current.RepoOwner,
				Namespace:  namespace,
				Repo:       current.RepoName,
				Name:       "trigger",
				CodeHostID: current.CodeHostID,
			})
		case setting.SourceFromGerrit:
			detail, err := systemconfig.New().GetCodeHost(current.CodeHostID)
			if err != nil {
				return err
			}

			cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
			if err := cl.DeleteWebhook(current.RepoName, webhook.YamlTemplatePrefix+current.Name); err != nil {
				logger.Errorf("failed to delete gerrit webhook for yaml template %s, err: %s", current.Name, err)
			}
		}
	}
	if updated != nil {
		namespace := updated.Namespace
		if namespace == "" {
			namespace = updated.RepoOwner
		}
		switch updated.Source {
		case setting.SourceFromGithub, setting.SourceFromGitlab, setting.SourceFromGitee, setting.SourceFromGiteeEE:
			updatedHooks = append(updatedHooks, &webhook.WebHook{
				Owner:      updated.RepoOwner,
				Namespace:  namespace,
				Repo:       updated.RepoName,
				Name:       "trigger",
				CodeHostID: updated.CodeHostID,
			})
		case setting.SourceFromGerrit:
			detail, err := systemconfig.New().GetCodeHost(updated.CodeHostID)
			if err != nil {
				return err
			}

			cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
			webhookName := webhook.YamlTemplatePrefix + updated.Name
			webhookURL := fmt.Sprintf("/%s/%s/%s/%s", "a/config/server/webhooks~projects", gerrit.Escape(updated.RepoName), "remotes", webhookName)
			if _, err := cl.Get(webhookURL); err != nil {
				gerritWebhook := &gerrit.Webhook{
					URL:       fmt.Sprintf("%s?name=%s", WebHookURL(), webhookName),
					MaxTries:  setting.MaxTries,
					SslVerify: false,
				}
				if _, err = cl.Put(webhookURL, httpclient.SetBody(gerritWebhook)); err != nil {
					return err
				}
			}
		}
	}

	name := ""
	if updated != nil {
		name = webhook.YamlTemplatePrefix + updated.Name
	} else if current != nil {
		name = webhook.YamlTemplatePrefix + current.Name
	}
	if len(updatedHooks) == 0 && len(currentHooks) == 0 {
		return nil
	}

	return ProcessWebhook(updatedHooks, currentHooks, name, logger)
}
