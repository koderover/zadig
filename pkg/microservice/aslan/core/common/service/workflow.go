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
	"encoding/json"
	"sync"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/msg_queue"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
)

func DisableCronjobForWorkflow(workflow *models.Workflow) error {
	disableIDList := make([]string, 0)
	payload := &CronjobPayload{
		Name:    workflow.Name,
		JobType: setting.WorkflowCronjob,
		Action:  setting.TypeEnableCronjob,
	}
	if workflow.ScheduleEnabled {
		jobList, err := mongodb.NewCronjobColl().List(&mongodb.ListCronjobParam{
			ParentName: workflow.Name,
			ParentType: setting.WorkflowCronjob,
		})
		if err != nil {
			return err
		}

		for _, job := range jobList {
			disableIDList = append(disableIDList, job.ID.Hex())
		}
		payload.DeleteList = disableIDList
	}

	pl, _ := json.Marshal(payload)
	return mongodb.NewMsgQueueCommonColl().Create(&msg_queue.MsgQueueCommon{
		Payload:   string(pl),
		QueueType: setting.TopicCronjob,
	})
}

func ProcessWebhook(updatedHooks, currentHooks interface{}, name string, logger *zap.SugaredLogger) error {
	currentSet := toHookSet(currentHooks)
	updatedSet := toHookSet(updatedHooks)
	hooksToRemove := currentSet.Difference(updatedSet)
	hooksToAdd := updatedSet.Difference(currentSet)

	if hooksToRemove.Len() > 0 {
		logger.Debugf("Going to remove webhooks %+v", hooksToRemove)
	}
	if hooksToAdd.Len() > 0 {
		logger.Debugf("Going to add webhooks %+v", hooksToAdd)
	}

	var errs *multierror.Error
	var wg sync.WaitGroup

	for _, h := range hooksToRemove {
		wg.Add(1)
		go func(wh hookItem) {
			defer wg.Done()
			ch, err := systemconfig.New().GetRawCodeHost(wh.codeHostID)
			if err != nil {
				logger.Errorf("Failed to get codeHost by id %d, err: %s", wh.codeHostID, err)
				errs = multierror.Append(errs, err)
				return
			}

			switch ch.Type {
			case setting.SourceFromGithub, setting.SourceFromGitlab, setting.SourceFromGitee, setting.SourceFromGiteeEE:
				err = webhook.NewClient().RemoveWebHook(&webhook.TaskOption{
					ID:          ch.ID,
					Name:        wh.name,
					Owner:       wh.owner,
					Namespace:   wh.namespace,
					Repo:        wh.repo,
					Address:     ch.Address,
					Token:       ch.AccessToken,
					AK:          ch.AccessKey,
					SK:          ch.SecretKey,
					Region:      ch.Region,
					EnableProxy: ch.EnableProxy,
					Ref:         name,
					From:        ch.Type,
					IsManual:    wh.IsManual,
				})
				if err != nil {
					logger.Errorf("Failed to remove %s webhook %+v, err: %s", ch.Type, wh, err)
					errs = multierror.Append(errs, err)
					return
				}
			}
		}(h)
	}

	for _, h := range hooksToAdd {
		wg.Add(1)
		go func(wh hookItem) {
			defer wg.Done()
			ch, err := systemconfig.New().GetCodeHost(wh.codeHostID)
			if err != nil {
				logger.Errorf("Failed to get codeHost by id %d, err: %s", wh.codeHostID, err)
				errs = multierror.Append(errs, err)
				return
			}

			switch ch.Type {
			case setting.SourceFromGithub, setting.SourceFromGitlab, setting.SourceFromGitee, setting.SourceFromGiteeEE:
				err = webhook.NewClient().AddWebHook(&webhook.TaskOption{
					ID:        ch.ID,
					Name:      wh.name,
					Owner:     wh.owner,
					Namespace: wh.namespace,
					Repo:      wh.repo,
					Address:   ch.Address,
					Token:     ch.AccessToken,
					Ref:       name,
					AK:        ch.AccessKey,
					SK:        ch.SecretKey,
					Region:    ch.Region,
					From:      ch.Type,
					IsManual:  wh.IsManual,
				})
				if err != nil {
					logger.Errorf("Failed to add %s webhook %+v, err: %s", ch.Type, wh, err)
					errs = multierror.Append(errs, err)
					return
				}
			}
		}(h)
	}

	wg.Wait()

	return errs.ErrorOrNil()
}

func toHookSet(hooks interface{}) HookSet {
	res := NewHookSet()
	switch hs := hooks.(type) {
	case []*models.WorkflowHook:
		// deprecated, for old custom workflow
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.MainRepo.Name,
					owner:     h.MainRepo.RepoOwner,
					namespace: h.MainRepo.GetRepoNamespace(),
					repo:      h.MainRepo.RepoName,
					source:    h.MainRepo.Source,
				},
				codeHostID: h.MainRepo.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	case []models.GitHook:
		// for pipline workflow
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.Name,
					owner:     h.Owner,
					namespace: h.Owner, // webhooks for pipelines, no need to handler anymore
					repo:      h.Repo,
				},
				codeHostID: h.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	case []*webhook.WebHook:
		// for template service sync
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.Name,
					owner:     h.Owner,
					namespace: h.Namespace,
					repo:      h.Repo,
				},
				codeHostID: h.CodeHostID,
				IsManual:   h.IsManual,
			})
		}
	case []*models.TestingHook:
		// for testing
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.MainRepo.Name,
					owner:     h.MainRepo.RepoOwner,
					namespace: h.MainRepo.GetRepoNamespace(),
					repo:      h.MainRepo.RepoName,
				},
				codeHostID: h.MainRepo.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	case []*models.ScanningHook:
		// for scanning
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:  h.RepoName,
					owner: h.RepoOwner,
					repo:  h.RepoName,
				},
				codeHostID: h.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	case []*models.WorkflowV4GitHook:
		// for custom workflow
		for _, h := range hs {
			res.Insert(hookItem{
				hookUniqueID: hookUniqueID{
					name:      h.Name,
					owner:     h.MainRepo.RepoOwner,
					namespace: h.MainRepo.GetRepoNamespace(),
					repo:      h.MainRepo.RepoName,
					source:    h.MainRepo.Source,
				},
				codeHostID: h.MainRepo.CodehostID,
				IsManual:   h.IsManual,
			})
		}
	}

	return res
}
