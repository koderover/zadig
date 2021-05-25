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
	"strconv"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	ch "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	git "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types"
	"github.com/koderover/zadig/lib/types/permission"
)

const SplitSymbol = "&"

type githubPullRequestDiffFunc func(event *github.PullRequestEvent, id int) ([]string, error)

type gitEventMatcher interface {
	Match(commonmodels.MainHookRepo) (bool, error)
	UpdateTaskArgs(*commonmodels.Product, *commonmodels.WorkflowTaskArgs, commonmodels.MainHookRepo) *commonmodels.WorkflowTaskArgs
}

type githubPushEventMatcher struct {
	log      *xlog.Logger
	workflow *commonmodels.Workflow
	event    *github.PushEvent
}

func (gpem *githubPushEventMatcher) Match(hookRepo commonmodels.MainHookRepo) (bool, error) {
	ev := gpem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == *ev.Repo.FullName {
		if hookRepo.Branch == getBranchFromRef(*ev.Ref) && EventConfigured(hookRepo, config.HookEventPush) {
			var changedFiles []string
			for _, commit := range ev.Commits {
				changedFiles = append(changedFiles, commit.Added...)
				changedFiles = append(changedFiles, commit.Removed...)
				changedFiles = append(changedFiles, commit.Modified...)
			}
			return MatchChanges(hookRepo, changedFiles), nil
		}
	}

	return false, nil
}

func getBranchFromRef(ref string) string {
	prefix := "refs/heads/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}

	return ref
}

func (gpem *githubPushEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo commonmodels.MainHookRepo,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gpem.workflow,
		reqId:    gpem.log.ReqID(),
	}

	factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
	})

	return args
}

type githubMergeEventMatcher struct {
	diffFunc githubPullRequestDiffFunc
	log      *xlog.Logger
	workflow *commonmodels.Workflow
	event    *github.PullRequestEvent
}

func (gmem *githubMergeEventMatcher) Match(hookRepo commonmodels.MainHookRepo) (bool, error) {
	ev := gmem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == *ev.PullRequest.Base.Repo.FullName {
		if EventConfigured(hookRepo, config.HookEventPr) && (hookRepo.Branch == *ev.PullRequest.Base.Ref) {
			if *ev.PullRequest.State == "open" {
				var changedFiles []string
				changedFiles, err := gmem.diffFunc(ev, hookRepo.CodehostID)
				if err != nil {
					gmem.log.Warnf("failed to get changes of event %v", ev)
					return false, err
				} else {
					gmem.log.Debugf("succeed to get %d changes in merge event", len(changedFiles))
				}

				return MatchChanges(hookRepo, changedFiles), nil
			}
		}
	}
	return false, nil
}

func (gmem *githubMergeEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo commonmodels.MainHookRepo,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: gmem.workflow,
		reqId:    gmem.log.ReqID(),
	}

	args = factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
		PR:         *gmem.event.PullRequest.Number,
	})

	return args
}

type workflowArgsFactory struct {
	workflow *commonmodels.Workflow
	reqId    string
}

func (waf *workflowArgsFactory) Update(product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, repo *types.Repository) *commonmodels.WorkflowTaskArgs {
	workflow := waf.workflow
	args.WorkflowName = workflow.Name
	args.WorklowTaskCreator = setting.WebhookTaskCreator
	args.ProductTmplName = workflow.ProductTmplName
	args.ReqID = waf.reqId
	var targetMap map[string][]commonmodels.DeployEnv

	// 构建和测试中都有可能存在变更对应的repo
	for _, target := range args.Target {
		if target.Build == nil {
			target.Build = &commonmodels.BuildArgs{}
		}

		target.Build.Repos = append(target.Build.Repos, repo)
		if len(target.Deploy) == 0 {
			if targetMap == nil {
				targetMap = getProductTargetMap(product)
			}
			serviceModuleTarget := fmt.Sprintf("%s%s%s%s%s", args.ProductTmplName, SplitSymbol, target.ServiceName, SplitSymbol, target.Name)
			target.Deploy = targetMap[serviceModuleTarget]
		}
	}

	for _, target := range args.Tests {
		target.Builds = append(target.Builds, repo)
	}

	if workflow.TestStage != nil && workflow.TestStage.Enabled {
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

func createGithubEventMatcher(
	event interface{}, diffSrv githubPullRequestDiffFunc, workflow *commonmodels.Workflow, log *xlog.Logger,
) gitEventMatcher {
	switch event.(type) {
	case *github.PushEvent:
		return &githubPushEventMatcher{
			workflow: workflow,
			log:      log,
			event:    event.(*github.PushEvent),
		}
	case *github.PullRequestEvent:
		return &githubMergeEventMatcher{
			diffFunc: diffSrv,
			log:      log,
			event:    event.(*github.PullRequestEvent),
			workflow: workflow,
		}
	}

	return nil
}

func TriggerWorkflowByGithubEvent(event interface{}, baseUri, deliveryID string, log *xlog.Logger) error {
	workflowList, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
	if err != nil {
		log.Errorf("failed to list workflow %v", err)
		return err
	}

	mErr := &multierror.Error{}
	diffSrv := func(pullRequestEvent *github.PullRequestEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfPullRequest(pullRequestEvent, codehostId)
	}

	var notification *commonmodels.Notification

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

					var mergeRequestID, commitID string
					var hookPayload *commonmodels.HookPayload
					if ev, isPr := event.(*github.PullRequestEvent); isPr {
						// 如果是merge request，且该webhook触发器配置了自动取消，
						// 则需要确认该merge request在本次commit之前的commit触发的任务是否处理完，没有处理完则取消掉。
						if ev.PullRequest != nil && ev.PullRequest.Number != nil && ev.PullRequest.Head != nil && ev.PullRequest.Head.SHA != nil {
							mergeRequestID = strconv.Itoa(*ev.PullRequest.Number)
							commitID = *ev.PullRequest.Head.SHA
							autoCancelOpt := &AutoCancelOpt{
								MergeRequestID: mergeRequestID,
								CommitID:       commitID,
								TaskType:       config.WorkflowType,
								MainRepo:       item.MainRepo,
								WorkflowArgs:   item.WorkflowArgs,
							}
							err := AutoCancelTask(autoCancelOpt, log)
							if err != nil {
								log.Errorf("failed to auto cancel workflow task when receive event due to %v ", err)
								mErr = multierror.Append(mErr, err)
							}
						}

						if notification == nil {
							notification, _ = scmnotify.NewService().SendInitWebhookComment(
								&item.MainRepo, *ev.PullRequest.Number, baseUri, false, false, log,
							)
						}

						hookPayload = &commonmodels.HookPayload{
							Owner:      *ev.Repo.Owner.Login,
							Repo:       *ev.Repo.Name,
							Branch:     *ev.PullRequest.Base.Ref,
							Ref:        *ev.PullRequest.Head.SHA,
							IsPr:       true,
							DeliveryID: deliveryID,
						}
					}

					if notification != nil {
						item.WorkflowArgs.NotificationId = notification.ID.Hex()
					}

					args := matcher.UpdateTaskArgs(prod, item.WorkflowArgs, item.MainRepo)
					args.MergeRequestID = mergeRequestID
					args.CommitID = commitID
					args.Source = setting.SourceFromGithub
					args.CodehostID = item.MainRepo.CodehostID
					args.RepoOwner = item.MainRepo.RepoOwner
					args.RepoName = item.MainRepo.RepoName
					args.HookPayload = hookPayload

					// 3. create task with args
					if resp, err := workflowservice.CreateWorkflowTask(args, setting.WebhookTaskCreator, permission.AnonymousUserID, false, log); err != nil {
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

func findChangedFilesOfPullRequest(event *github.PullRequestEvent, codehostId int) ([]string, error) {
	if detail, err := ch.GetCodehostDetail(codehostId); err != nil {
		return nil, fmt.Errorf("failed to find codehost %d: %v", codehostId, err)
	} else {
		//pullrequest文件修改
		githubCli := git.NewGithubAppClient(detail.OauthToken, githubAPIServer, config.ProxyHTTPSAddr())
		if commitComparison, _, err := githubCli.Repositories.CompareCommits(context.Background(), *event.PullRequest.Base.Repo.Owner.Login, *event.PullRequest.Base.Repo.Name, *event.PullRequest.Base.SHA, *event.PullRequest.Head.SHA); err != nil {
			return nil, fmt.Errorf("failed to get changes from github, err: %v", err)
		} else {
			changeFiles := make([]string, 0)
			for _, commitFile := range commitComparison.Files {
				changeFiles = append(changeFiles, *commitFile.Filename)
			}
			return changeFiles, nil
		}
	}
}
