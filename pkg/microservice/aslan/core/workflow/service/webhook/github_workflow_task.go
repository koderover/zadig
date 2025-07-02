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
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-github/v35/github"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	git "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/types"
)

const SplitSymbol = "&"

type githubPullRequestDiffFunc func(event *github.PullRequestEvent, id int) ([]string, error)

type gitEventMatcher interface {
	Match(*commonmodels.MainHookRepo) (bool, error)
	UpdateTaskArgs(*commonmodels.Product, *commonmodels.WorkflowTaskArgs, *commonmodels.MainHookRepo, string) *commonmodels.WorkflowTaskArgs
}

type githubPushEventMatcher struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *github.PushEvent
}

func (gpem *githubPushEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gpem.event

	if !checkRepoNamespaceMatch(hookRepo, *ev.Repo.FullName) {
		return false, nil
	}

	if !EventConfigured(hookRepo, config.HookEventPush) {
		return false, nil
	}

	isRegular := hookRepo.IsRegular
	if !isRegular && hookRepo.Branch != getBranchFromRef(*ev.Ref) {
		return false, nil
	}
	if isRegular {
		// Do not use regexp.MustCompile to avoid panic
		if matched, _ := regexp.MatchString(hookRepo.Branch, getBranchFromRef(*ev.Ref)); !matched {
			return false, nil
		}
	}
	hookRepo.Branch = getBranchFromRef(*ev.Ref)
	hookRepo.Committer = *ev.Pusher.Name
	var changedFiles []string
	for _, commit := range ev.Commits {
		changedFiles = append(changedFiles, commit.Added...)
		changedFiles = append(changedFiles, commit.Removed...)
		changedFiles = append(changedFiles, commit.Modified...)
	}
	return MatchChanges(hookRepo, changedFiles), nil
}

func getBranchFromRef(ref string) string {
	prefix := "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}

	return ref
}

func getTagFromRef(ref string) string {
	prefix := "refs/tags/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}

	return ref
}
func (gpem *githubPushEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gpem.workflow,
		reqID:    requestID,
	}

	factory.Update(product, args, &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
	})

	return args
}

type githubMergeEventMatcher struct {
	diffFunc githubPullRequestDiffFunc
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *github.PullRequestEvent
}

func (gmem *githubMergeEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gmem.event

	if !checkRepoNamespaceMatch(hookRepo, *ev.PullRequest.Base.Repo.FullName) {
		return false, nil
	}

	if !EventConfigured(hookRepo, config.HookEventPr) {
		return false, nil
	}

	isRegular := hookRepo.IsRegular
	if !isRegular && hookRepo.Branch != *ev.PullRequest.Base.Ref {
		return false, nil
	}
	if isRegular {
		if matched, _ := regexp.MatchString(hookRepo.Branch, *ev.PullRequest.Base.Ref); !matched {
			return false, nil
		}
	}
	hookRepo.Branch = *ev.PullRequest.Base.Ref
	hookRepo.Committer = *ev.PullRequest.User.Login
	if *ev.PullRequest.State == "open" {
		var changedFiles []string
		changedFiles, err := gmem.diffFunc(ev, hookRepo.CodehostID)
		if err != nil {
			gmem.log.Warnf("failed to get changes of event %v", ev)
			return false, err
		}
		gmem.log.Debugf("succeed to get %d changes in merge event", len(changedFiles))

		return MatchChanges(hookRepo, changedFiles), nil
	}

	return false, nil
}

func (gmem *githubMergeEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gmem.workflow,
		reqID:    requestID,
	}

	args = factory.Update(product, args, &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		PR:            *gmem.event.PullRequest.Number,
	})

	return args
}

type workflowArgsFactory struct {
	workflow *commonmodels.Workflow
	reqID    string
	IsYaml   bool
}

func (waf *workflowArgsFactory) Update(product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, repo *types.Repository) *commonmodels.WorkflowTaskArgs {
	workflow := waf.workflow
	args.WorkflowName = workflow.Name
	args.WorkflowTaskCreator = setting.WebhookTaskCreator
	args.ProductTmplName = workflow.ProductTmplName
	args.ReqID = waf.reqID
	var targetMap map[string][]commonmodels.DeployEnv

	// 构建和测试中都有可能存在变更对应的repo
	for _, target := range args.Target {
		if target.Build == nil {
			target.Build = &commonmodels.BuildArgs{}
		}

		target.Build.Repos = append(target.Build.Repos, repo)
		if len(target.Deploy) == 0 {
			if targetMap == nil {
				targetMap, _ = commonservice.GetProductTargetMap(product)
			}
			if waf.IsYaml {
				targetMap = make(map[string][]commonmodels.DeployEnv)
			}
			serviceModuleTarget := fmt.Sprintf("%s%s%s%s%s", args.ProductTmplName, SplitSymbol, target.ServiceName, SplitSymbol, target.Name)
			target.Deploy = targetMap[serviceModuleTarget]
		}
	}

	for _, target := range args.Tests {
		target.Builds = append(target.Builds, repo)
	}

	if !waf.IsYaml && workflow.TestStage != nil && workflow.TestStage.Enabled {
		testArgs := make([]*commonmodels.TestArgs, 0)
		for _, testName := range workflow.TestStage.TestNames {
			testArgs = append(testArgs, &commonmodels.TestArgs{
				TestModuleName: testName,
				Namespace:      args.Namespace,
			})
		}

		for _, testEntity := range workflow.TestStage.Tests {
			testArgs = append(testArgs, &commonmodels.TestArgs{
				TestModuleName: testEntity.Name,
				Namespace:      args.Namespace,
			})
		}

		args.Tests = testArgs
	}

	return args
}

type githubTagEventMatcher struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *github.CreateEvent
}

func (gtem githubTagEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gtem.event

	if !checkRepoNamespaceMatch(hookRepo, *ev.Repo.FullName) {
		return false, nil
	}

	if !EventConfigured(hookRepo, config.HookEventTag) {
		return false, nil
	}

	hookRepo.Tag = getTagFromRef(*ev.Ref)
	if ev.Sender.Name != nil {
		hookRepo.Committer = *ev.Sender.Name
	}

	return true, nil
}

func (gtem githubTagEventMatcher) UpdateTaskArgs(product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gtem.workflow,
		reqID:    requestID,
	}

	factory.Update(product, args, &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		Tag:           hookRepo.Tag,
	})

	return args
}

func createGithubEventMatcher(
	event interface{}, diffSrv githubPullRequestDiffFunc, workflow *commonmodels.Workflow, log *zap.SugaredLogger,
) gitEventMatcher {
	switch evt := event.(type) {
	case *github.PushEvent:
		return &githubPushEventMatcher{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	case *github.PullRequestEvent:
		return &githubMergeEventMatcher{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			workflow: workflow,
		}
	case *github.CreateEvent:
		return &githubTagEventMatcher{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	}

	return nil
}

func findChangedFilesOfPullRequest(event *github.PullRequestEvent, codehostID int) ([]string, error) {
	detail, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return nil, fmt.Errorf("failed to find codehost %d: %v", codehostID, err)
	}
	//pullrequest文件修改
	githubCli := git.NewClient(detail.AccessToken, config.ProxyHTTPSAddr(), detail.EnableProxy)
	commitComparison, _, err := githubCli.Repositories.CompareCommits(context.Background(), *event.PullRequest.Base.Repo.Owner.Login, *event.PullRequest.Base.Repo.Name, *event.PullRequest.Base.SHA, *event.PullRequest.Head.SHA)
	if err != nil {
		return nil, fmt.Errorf("failed to get changes from github, err: %v", err)
	}

	changeFiles := make([]string, 0)
	for _, commitFile := range commitComparison.Files {
		changeFiles = append(changeFiles, *commitFile.Filename)
	}
	return changeFiles, nil
}
