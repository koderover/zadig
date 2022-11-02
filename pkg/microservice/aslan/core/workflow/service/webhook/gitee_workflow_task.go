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
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/koderover/zadig/pkg/types"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/gitee"
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

		isRegular := hookRepo.IsRegular
		if !isRegular && hookRepo.Branch != ev.Repository.DefaultBranch {
			return false, nil
		}
		if isRegular {
			// Do not use regexp.MustCompile to avoid panic
			matched, err := regexp.MatchString(hookRepo.Branch, ev.Repository.DefaultBranch)
			if err != nil || !matched {
				return false, nil
			}
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

func TriggerWorkflowByGiteeEvent(event interface{}, baseURI, requestID string, log *zap.SugaredLogger) error {
	workflowList, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
	if err != nil {
		log.Errorf("failed to list workflow %v", err)
		return err
	}

	mErr := &multierror.Error{}
	diffSrv := func(pullRequestEvent *gitee.PullRequestEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfPullRequestEvent(pullRequestEvent, codehostId)
	}

	var notification *commonmodels.Notification
	for _, workflow := range workflowList {
		if workflow.HookCtl != nil && workflow.HookCtl.Enabled {
			log.Debugf("find %d hooks in workflow %s", len(workflow.HookCtl.Items), workflow.Name)
			for _, item := range workflow.HookCtl.Items {
				if item.WorkflowArgs == nil {
					continue
				}

				matcher := createGiteeEventMatcher(event, diffSrv, workflow, log)
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
					if ev, isPr := event.(*gitee.PullRequestEvent); isPr {
						if ev.PullRequest != nil && ev.PullRequest.Number != 0 && ev.PullRequest.Head != nil && ev.PullRequest.Head.Sha != "" {
							mergeRequestID = strconv.Itoa(ev.PullRequest.Number)
							commitID = ev.PullRequest.Head.Sha
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

							if notification == nil {
								notification, err = scmnotify.NewService().SendInitWebhookComment(
									item.MainRepo, ev.PullRequest.Number, baseURI, false, false, false, false, log,
								)
								if err != nil {
									log.Errorf("failed to init webhook comment due to %s", err)
									mErr = multierror.Append(mErr, err)
								}
							}
						}

						if notification != nil {
							item.WorkflowArgs.NotificationID = notification.ID.Hex()
						}

						hookPayload = &commonmodels.HookPayload{
							Owner:  ev.Repository.Owner.Login,
							Repo:   ev.Repository.Name,
							Branch: ev.PullRequest.Base.Ref,
							Ref:    ev.PullRequest.Head.Sha,
							IsPr:   true,
						}
					}

					args := matcher.UpdateTaskArgs(prod, item.WorkflowArgs, item.MainRepo, requestID)
					args.MergeRequestID = mergeRequestID
					args.CommitID = commitID
					args.Source = setting.SourceFromGitee
					args.CodehostID = item.MainRepo.CodehostID
					args.RepoOwner = item.MainRepo.RepoOwner
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
	for _, commitFile := range commitComparison.Files {
		changeFiles = append(changeFiles, commitFile.Filename)
	}
	return changeFiles, nil
}
