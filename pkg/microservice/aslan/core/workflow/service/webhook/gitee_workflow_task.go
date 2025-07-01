/*
Copyright 2022 The KodeRover Authors.

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
	"regexp"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/types"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
)

type giteePullRequestDiffFunc func(event *gitee.PullRequestEvent, id int) ([]string, error)

type giteePushEventMatcher struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *gitee.PushEvent
}

func (gpem *giteePushEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gpem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Repository.FullName {
		if !EventConfigured(hookRepo, config.HookEventPush) {
			return false, nil
		}

		isRegular := hookRepo.IsRegular
		if !isRegular && hookRepo.Branch != getBranchFromRef(ev.Ref) {
			return false, nil
		}
		if isRegular {
			// Do not use regexp.MustCompile to avoid panic
			matched, err := regexp.MatchString(hookRepo.Branch, getBranchFromRef(ev.Ref))
			if err != nil || !matched {
				return false, nil
			}
		}
		hookRepo.Branch = getBranchFromRef(ev.Ref)
		hookRepo.Committer = ev.Pusher.Name
		var changedFiles []string
		for _, commit := range ev.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Removed...)
			changedFiles = append(changedFiles, commit.Modified...)
		}
		return MatchChanges(hookRepo, changedFiles), nil
	}

	return false, nil
}

func (gpem *giteePushEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string,
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

type giteeMergeEventMatcher struct {
	diffFunc giteePullRequestDiffFunc
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *gitee.PullRequestEvent
}

func (gmem *giteeMergeEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gmem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.PullRequest.Base.Repo.FullName {
		if !EventConfigured(hookRepo, config.HookEventPr) {
			return false, nil
		}

		isRegular := hookRepo.IsRegular
		if !isRegular && hookRepo.Branch != ev.PullRequest.Base.Ref {
			return false, nil
		}
		if isRegular {
			matched, err := regexp.MatchString(hookRepo.Branch, ev.PullRequest.Base.Ref)
			if err != nil || !matched {
				return false, nil
			}
		}
		hookRepo.Branch = ev.PullRequest.Base.Ref
		hookRepo.Committer = ev.PullRequest.User.Login
		if ev.PullRequest.State == "open" {
			var changedFiles []string
			changedFiles, err := gmem.diffFunc(ev, hookRepo.CodehostID)
			if err != nil {
				gmem.log.Warnf("failed to get changes of event %v", ev)
				return false, err
			}
			gmem.log.Debugf("succeed to get %d changes in merge event", len(changedFiles))

			return MatchChanges(hookRepo, changedFiles), nil
		}
	}
	return false, nil
}

func (gmem *giteeMergeEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string,
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
		PR:         gmem.event.PullRequest.Number,
	})

	return args
}

type giteeTagEventMatcher struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *gitee.TagPushEvent
}

func (gtem giteeTagEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gtem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Repository.FullName {
		if !EventConfigured(hookRepo, config.HookEventTag) {
			return false, nil
		}

		hookRepo.Tag = getTagFromRef(ev.Ref)
		hookRepo.Committer = ev.Sender.Name

		return true, nil
	}

	return false, nil
}

func (gtem giteeTagEventMatcher) UpdateTaskArgs(product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gtem.workflow,
		reqID:    requestID,
	}

	factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
		Tag:        hookRepo.Tag,
	})

	return args
}

func createGiteeEventMatcher(
	event interface{}, diffSrv giteePullRequestDiffFunc, workflow *commonmodels.Workflow, log *zap.SugaredLogger,
) gitEventMatcher {
	switch evt := event.(type) {
	case *gitee.PushEvent:
		return &giteePushEventMatcher{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	case *gitee.PullRequestEvent:
		return &giteeMergeEventMatcher{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			workflow: workflow,
		}
	case *gitee.TagPushEvent:
		return &giteeTagEventMatcher{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	}

	return nil
}

func findChangedFilesOfPullRequestEvent(event *gitee.PullRequestEvent, codehostID int) ([]string, error) {
	detail, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return nil, fmt.Errorf("failed to find codehost %d: %v", codehostID, err)
	}

	var commitComparison *gitee.Compare

	giteeCli := gitee.NewClient(detail.ID, detail.Address, detail.AccessToken, config.ProxyHTTPSAddr(), detail.EnableProxy)
	if detail.Type == setting.SourceFromGitee {
		commitComparison, err = giteeCli.GetReposOwnerRepoCompareBaseHead(detail.Address, detail.AccessToken, event.Project.Namespace, event.Project.Name, event.PullRequest.Base.Sha, event.PullRequest.Head.Sha)
		if err != nil {
			return nil, fmt.Errorf("failed to get changes from gitee, err: %v", err)
		}
	} else if detail.Type == setting.SourceFromGiteeEE {
		commitComparison, err = giteeCli.GetReposOwnerRepoCompareBaseHeadForEnterprise(detail.Address, detail.AccessToken, event.Project.Namespace, event.Project.Name, event.PullRequest.Base.Sha, event.PullRequest.Head.Sha)
		if err != nil {
			return nil, fmt.Errorf("failed to get changes from gitee enterprise, err: %v", err)
		}
	}

	changeFiles := make([]string, 0)
	if commitComparison.Files == nil {
		return changeFiles, nil
	}
	for _, commitFile := range commitComparison.Files {
		changeFiles = append(changeFiles, commitFile.Filename)
	}
	return changeFiles, nil
}
