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

package gerrit

import (
	"fmt"
	"os"
	"path"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/tool/gerrit"
)

func DeleteGerritWebhook(workflow *models.Workflow, log *zap.SugaredLogger) error {
	if workflow != nil && workflow.HookCtl != nil {
		for _, workflowWebhook := range workflow.HookCtl.Items {
			if workflowWebhook == nil {
				continue
			}

			detail, err := codehost.GetCodeHostInfoByID(workflowWebhook.MainRepo.CodehostID)
			if err != nil {
				log.Errorf("DeleteGerritWebhook GetCodehostDetail err:%v", err)
				continue
			}
			if detail.Type == gerrit.CodehostTypeGerrit {
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

func GetGerritWorkspaceBasePath(repoName string) (string, error) {
	if strings.Contains(repoName, "/") {
		repoName = strings.Replace(repoName, "/", "-", -1)
	}
	base := path.Join(config.S3StoragePath(), repoName)

	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base, err
	}

	return base, nil
}
