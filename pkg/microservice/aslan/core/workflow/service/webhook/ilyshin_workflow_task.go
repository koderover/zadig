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
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/tool/ilyshin"
	"github.com/koderover/zadig/pkg/types"
)

type ilyshinMergeRequestDiffFunc func(event *ilyshin.MergeEvent, id int) ([]string, error)

type ilyshinMergeEventMatcher struct {
	diffFunc ilyshinMergeRequestDiffFunc
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *ilyshin.MergeEvent
}

func (gmem *ilyshinMergeEventMatcher) Match(hookRepo commonmodels.MainHookRepo) (bool, error) {
	ev := gmem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.ObjectAttributes.Target.PathWithNamespace {
		if EventConfigured(hookRepo, config.HookEventPr) && (hookRepo.Branch == ev.ObjectAttributes.TargetBranch) {
			if ev.ObjectAttributes.State == "opened" {
				var changedFiles []string
				changedFiles, err := gmem.diffFunc(ev, hookRepo.CodehostID)
				if err != nil {
					gmem.log.Warnf("failed to get changes of event %v, err:%v", ev, err)
					return false, err
				}
				gmem.log.Debugf("succeed to get %d changes in merge event", len(changedFiles))

				return MatchChanges(hookRepo, changedFiles), nil
			}
		}
	}
	return false, nil
}

func (gmem *ilyshinMergeEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo commonmodels.MainHookRepo, requestID string,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gmem.workflow,
		reqID:    requestID,
	}

	args = factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
		PR:         gmem.event.ObjectAttributes.IID,
	})

	return args
}

func createIlyshinEventMatcher(
	event interface{}, diffSrv ilyshinMergeRequestDiffFunc, workflow *commonmodels.Workflow, log *zap.SugaredLogger,
) gitEventMatcher {
	switch evt := event.(type) {
	case *ilyshin.PushEvent:
		return &ilyshinPushEventMatcher{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	case *ilyshin.MergeEvent:
		return &ilyshinMergeEventMatcher{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			workflow: workflow,
		}
	}

	return nil
}

type ilyshinPushEventMatcher struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *ilyshin.PushEvent
}

func (gpem *ilyshinPushEventMatcher) Match(hookRepo commonmodels.MainHookRepo) (bool, error) {
	ev := gpem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Project.PathWithNamespace {
		if hookRepo.Branch == getBranchFromRef(ev.Ref) && EventConfigured(hookRepo, config.HookEventPush) {
			var changedFiles []string

			detail, err := codehost.GetCodehostDetail(hookRepo.CodehostID)
			if err != nil {
				gpem.log.Errorf("GetCodehostDetail error: %v", err)
				return false, err
			}

			client := ilyshin.NewClient(detail.Address, detail.OauthToken)
			// compare接口获取两个commit之间的最终的改动
			diffs, err := client.Compare(ev.ProjectID, ev.Before, ev.After)
			if err != nil {
				gpem.log.Errorf("Failed to get push event diffs, error: %v", err)
				return false, err
			}
			for _, diff := range diffs {
				changedFiles = append(changedFiles, diff.NewPath)
				changedFiles = append(changedFiles, diff.OldPath)
			}

			return MatchChanges(hookRepo, changedFiles), nil
		}
	}

	return false, nil
}

func (gpem *ilyshinPushEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo commonmodels.MainHookRepo, requestID string,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gpem.workflow,
		reqID:    requestID,
	}

	factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
	})

	return args
}
