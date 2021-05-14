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

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	"github.com/koderover/zadig/lib/tool/gerrit"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

func DeleteGerritWebhook(workflow *models.Workflow, log *xlog.Logger) error {
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
				webhookURLPrefix := fmt.Sprintf("%s/%s/%s/%s", detail.Address, "a/config/server/webhooks~projects", gerrit.Escape(workflowWebhook.MainRepo.RepoName), "remotes")
				_, _ = gerrit.Do(fmt.Sprintf("%s/%s", webhookURLPrefix, gerrit.RemoteName), "DELETE", detail.AccessToken, util.GetRequestBody(&gerrit.GerritWebhook{}))
				_, err = gerrit.Do(fmt.Sprintf("%s/%s", webhookURLPrefix, workflow.Name), "DELETE", detail.AccessToken, util.GetRequestBody(&gerrit.GerritWebhook{}))
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
