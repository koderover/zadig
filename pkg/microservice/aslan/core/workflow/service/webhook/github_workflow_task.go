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
	"strconv"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	git "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/types"
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

	isRegular := hookRepo.IsRegular
	if !isRegular && hookRepo.Branch != *ev.Repo.DefaultBranch {
		return false, nil
	}
	if isRegular {
		// Do not use regexp.MustCompile to avoid panic
		if matched, _ := regexp.MatchString(hookRepo.Branch, *ev.Repo.DefaultBranch); !matched {
			return false, nil
		}
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

func TriggerWorkflowByGithubEvent(event interface{}, baseURI, deliveryID, requestID string, log *zap.SugaredLogger) error {
	workflowList, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
	if err != nil {
		log.Errorf("failed to list workflow %v", err)
		return err
	}

	mErr := &multierror.Error{}
	diffSrv := func(pullRequestEvent *github.PullRequestEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfPullRequest(pullRequestEvent, codehostId)
	}

	for _, workflow := range workflowList {
		if workflow.HookCtl != nil && workflow.HookCtl.Enabled {
			log.Debugf("find %d hooks in workflow %s", len(workflow.HookCtl.Items), workflow.Name)
			for _, item := range workflow.HookCtl.Items {
				if item.WorkflowArgs == nil {
					continue
				}

				matcher := createGithubEventMatcher(event, diffSrv, workflow, log)
				if matcher == nil {
					continue
				}

				if matches, err := matcher.Match(item.MainRepo); err != nil {
					mErr = multierror.Append(mErr, err)
				} else if matches {
					log.Infof("event match hook %v of %s", item.MainRepo, workflow.Name)
					namespace := strings.Split(item.WorkflowArgs.Namespace, ",")[0]
					opt := &commonrepo.ProductFindOptions{Name: workflow.ProductTmplName, EnvName: namespace}
					var prod *commonmodels.Product
					if prod, err = commonrepo.NewProductColl().Find(opt); err != nil {
						log.Warnf("can't find environment %s-%s", item.WorkflowArgs.Namespace, workflow.ProductTmplName)
						continue
					}

					var mergeRequestID, commitID, ref, eventType string
					var hookPayload *commonmodels.HookPayload
					autoCancelOpt := &AutoCancelOpt{
						TaskType:     config.WorkflowType,
						MainRepo:     item.MainRepo,
						WorkflowArgs: item.WorkflowArgs,
					}
					switch ev := event.(type) {
					case *github.PullRequestEvent:
						eventType = EventTypePR
						// 如果是merge request，且该webhook触发器配置了自动取消，
						// 则需要确认该merge request在本次commit之前的commit触发的任务是否处理完，没有处理完则取消掉。
						if ev.PullRequest != nil && ev.PullRequest.Number != nil && ev.PullRequest.Head != nil && ev.PullRequest.Head.SHA != nil {
							mergeRequestID = strconv.Itoa(*ev.PullRequest.Number)
							commitID = *ev.PullRequest.Head.SHA
							autoCancelOpt.MergeRequestID = mergeRequestID
							autoCancelOpt.Type = eventType
						}
						hookPayload = &commonmodels.HookPayload{
							Owner:      *ev.Repo.Owner.Login,
							Repo:       *ev.Repo.Name,
							Branch:     *ev.PullRequest.Base.Ref,
							Ref:        *ev.PullRequest.Head.SHA,
							IsPr:       true,
							DeliveryID: deliveryID,
							EventType:  eventType,
						}
					case *github.PushEvent:
						// if event type is push，and this webhook trigger enabled auto cancel
						// should cancel tasks triggered by the same git branch push event.
						if ev.GetRef() != "" && ev.GetHeadCommit().GetID() != "" {
							eventType = EventTypePush
							ref = ev.GetRef()
							commitID = ev.GetHeadCommit().GetID()
							autoCancelOpt.Ref = ref
							autoCancelOpt.CommitID = commitID
							autoCancelOpt.Type = eventType
							hookPayload = &commonmodels.HookPayload{
								Owner:      *ev.Repo.Owner.Login,
								Repo:       *ev.Repo.Name,
								Branch:     ref,
								Ref:        commitID,
								IsPr:       false,
								DeliveryID: deliveryID,
								EventType:  eventType,
							}
						}
					case *github.CreateEvent:
						eventType = EventTypeTag
					}
					// if event type is not PR or push, skip
					if autoCancelOpt.Type != "" {
						err := AutoCancelTask(autoCancelOpt, log)
						if err != nil {
							log.Errorf("failed to auto cancel workflow task when receive event due to %v ", err)
							mErr = multierror.Append(mErr, err)
						}
					}

					args := matcher.UpdateTaskArgs(prod, item.WorkflowArgs, item.MainRepo, requestID)
					args.MergeRequestID = mergeRequestID
					args.Ref = ref
					args.EventType = eventType
					args.CommitID = commitID
					args.Source = setting.SourceFromGithub
					args.CodehostID = item.MainRepo.CodehostID
					args.RepoOwner = item.MainRepo.RepoOwner
					args.RepoNamespace = item.MainRepo.GetRepoNamespace()
					args.RepoName = item.MainRepo.RepoName
					args.Committer = item.MainRepo.Committer
					args.HookPayload = hookPayload

					// 3. create task with args
					if resp, err := workflowservice.CreateWorkflowTask(args, setting.WebhookTaskCreator, log); err != nil {
						log.Errorf("failed to create workflow task when receive push event due to %v ", err)
						mErr = multierror.Append(mErr, err)
					} else {
						log.Infof("succeed to create task %v", resp)
					}
				} else {
					log.Debugf("event not matches %v", item.MainRepo)
				}
			}
		}
	}

	return mErr.ErrorOrNil()
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
