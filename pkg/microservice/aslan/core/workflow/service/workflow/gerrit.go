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
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/gerrit"
)

func CreateGerritWebhook(workflow *commonmodels.Workflow, log *zap.SugaredLogger) error {
	if workflow != nil && workflow.HookCtl != nil && workflow.HookCtl.Enabled {
		for _, workflowWebhook := range workflow.HookCtl.Items {
			if workflowWebhook == nil {
				continue
			}
			if err := createGerritWebhook(workflowWebhook.MainRepo, workflow.Name); err != nil {
				log.Errorf("CreateGerritWebhook addGerritWebhook err: %v", err)
				return err
			}
		}
	}
	return nil
}

func createGerritWebhook(mainRepo *commonmodels.MainHookRepo, workflowName string) error {
	detail, err := systemconfig.New().GetCodeHost(mainRepo.CodehostID)
	if err != nil {
		return err
	}

	if detail.Type != setting.SourceFromGerrit {
		return nil
	}

	cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
	events := []string{}
	for _, event := range mainRepo.Events {
		events = append(events, string(event))
	}
	if err := cl.UpsertWebhook(mainRepo.RepoName, workflowName, events); err != nil {
		return err
	}
	return nil
}

func deleteGerritWebhook(mainRepo *commonmodels.MainHookRepo, workflowName string) error {
	detail, err := systemconfig.New().GetCodeHost(mainRepo.CodehostID)
	if err != nil {
		return err
	}

	if detail.Type == setting.SourceFromGerrit {
		cl := gerrit.NewHTTPClient(detail.Address, detail.AccessToken)
		if err := cl.DeleteWebhook(mainRepo.RepoName, workflowName); err != nil {
			return err
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
			if err := deleteGerritWebhook(oldWorkflowWebhook.MainRepo, oldWorkflow.Name); err != nil {
				log.Errorf("UpdateGerritWebhook delete webhook err:%v", err)
			}
		}
	}
	if currentWorkflow != nil && currentWorkflow.HookCtl != nil && currentWorkflow.HookCtl.Enabled {
		for _, workflowWebhook := range currentWorkflow.HookCtl.Items {
			if workflowWebhook == nil {
				continue
			}
			if err := createGerritWebhook(workflowWebhook.MainRepo, currentWorkflow.Name); err != nil {
				log.Errorf("UpdateGerritWebhook addGerritWebhook err: %v", err)
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

			if err := deleteGerritWebhook(workflowWebhook.MainRepo, workflow.Name); err != nil {
				log.Errorf("UpdateGerritWebhook delete webhook err:%v", err)
			}
		}
	}
	return nil
}
