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

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
	"github.com/koderover/zadig/v2/pkg/types"
)

type giteeEventMatcherForWorkflowV4 interface {
	Match(*commonmodels.MainHookRepo) (bool, error)
	GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository
}

type giteePushEventMatcherForWorkflowV4 struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.WorkflowV4
	event    *gitee.PushEvent
}

func (gpem *giteePushEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gpem *giteePushEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		RepoOwner:     hookRepo.RepoOwner,
		Branch:        hookRepo.Branch,
		Source:        hookRepo.Source,
	}
}

type giteeMergeEventMatcherForWorkflowV4 struct {
	diffFunc giteePullRequestDiffFunc
	log      *zap.SugaredLogger
	workflow *commonmodels.WorkflowV4
	event    *gitee.PullRequestEvent
}

func (gmem *giteeMergeEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gmem *giteeMergeEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		PR:            gmem.event.PullRequest.Number,
		Source:        hookRepo.Source,
	}
}

type giteeTagEventMatcherForWorkflowV4 struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.WorkflowV4
	event    *gitee.TagPushEvent
}

func (gtem giteeTagEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gtem *giteeTagEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		Tag:           hookRepo.Tag,
		Source:        hookRepo.Source,
	}
}

func createGiteeEventMatcherForWorkflowV4(
	event interface{}, diffSrv giteePullRequestDiffFunc, workflow *commonmodels.WorkflowV4, log *zap.SugaredLogger,
) giteeEventMatcherForWorkflowV4 {
	switch evt := event.(type) {
	case *gitee.PushEvent:
		return &giteePushEventMatcherForWorkflowV4{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	case *gitee.PullRequestEvent:
		return &giteeMergeEventMatcherForWorkflowV4{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			workflow: workflow,
		}
	case *gitee.TagPushEvent:
		return &giteeTagEventMatcherForWorkflowV4{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	}

	return nil
}

func TriggerWorkflowV4ByGiteeEvent(event interface{}, baseURI, requestID string, log *zap.SugaredLogger) error {
	workflows, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{}, 0, 0)
	if err != nil {
		errMsg := fmt.Sprintf("list workflow v4 error: %v", err)
		log.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	mErr := &multierror.Error{}
	diffSrv := func(pullRequestEvent *gitee.PullRequestEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfPullRequestEvent(pullRequestEvent, codehostId)
	}
	var hookPayload *commonmodels.HookPayload
	var notification *commonmodels.Notification

	for _, workflow := range workflows {
		gitHooks, err := commonrepo.NewWorkflowV4GitHookColl().List(internalhandler.NewBackgroupContext(), workflow.Name)
		if err != nil {
			log.Errorf("list workflow v4 git hook error: %v", err)
			return fmt.Errorf("list workflow v4 git hook error: %v", err)
		}
		if len(gitHooks) == 0 {
			continue
		}
		for _, item := range gitHooks {
			if !item.Enabled {
				continue
			}

			matcher := createGiteeEventMatcherForWorkflowV4(event, diffSrv, workflow, log)
			if matcher == nil {
				errMsg := fmt.Sprintf("merge webhook repo info to workflowargs error: %v", err)
				log.Error(errMsg)
				mErr = multierror.Append(mErr, fmt.Errorf(errMsg))
				continue
			}
			matches, err := matcher.Match(item.MainRepo)
			if err != nil {
				mErr = multierror.Append(mErr, err)
			}
			if !matches {
				continue
			}

			log.Infof("event match hook %v of %s", item.MainRepo, workflow.Name)

			eventRepo := matcher.GetHookRepo(item.MainRepo)

			autoCancelOpt := &AutoCancelOpt{
				TaskType:     config.WorkflowType,
				MainRepo:     item.MainRepo,
				AutoCancel:   item.AutoCancel,
				WorkflowName: workflow.Name,
			}
			var mergeRequestID, commitID, ref, eventType string
			var prID int
			switch ev := event.(type) {
			case *gitee.PullRequestEvent:
				eventType = EventTypePR
				mergeRequestID = strconv.Itoa(ev.PullRequest.Number)
				commitID = ev.PullRequest.Head.Sha
				prID = ev.PullRequest.Number
				autoCancelOpt.Type = eventType
				autoCancelOpt.CommitID = commitID
				autoCancelOpt.MergeRequestID = mergeRequestID
				hookPayload = &commonmodels.HookPayload{
					Owner:          eventRepo.RepoOwner,
					Repo:           eventRepo.RepoName,
					CodehostID:     item.MainRepo.CodehostID,
					Branch:         eventRepo.Branch,
					IsPr:           true,
					MergeRequestID: mergeRequestID,
					CommitID:       commitID,
					EventType:      eventType,
				}
			case *gitee.PushEvent:
				eventType = EventTypePush
				ref = ev.Ref
				commitID = ev.After
				autoCancelOpt.Type = eventType
				autoCancelOpt.CommitID = commitID
				autoCancelOpt.Ref = ref
				hookPayload = &commonmodels.HookPayload{
					Owner:      eventRepo.RepoOwner,
					Repo:       eventRepo.RepoName,
					CodehostID: item.MainRepo.CodehostID,
					Branch:     eventRepo.Branch,
					Ref:        ref,
					IsPr:       false,
					CommitID:   commitID,
					EventType:  eventType,
				}
			case *gitee.TagPushEvent:
				eventType = EventTypeTag
				hookPayload = &commonmodels.HookPayload{
					EventType: eventType,
				}
			}
			if autoCancelOpt.Type != "" {
				err := AutoCancelWorkflowV4Task(autoCancelOpt, log)
				if err != nil {
					log.Errorf("failed to auto cancel workflowV4 task when receive event %v due to %v ", event, err)
					mErr = multierror.Append(mErr, err)
				}

				if autoCancelOpt.Type == EventTypePR && notification == nil {
					notification, err = scmnotify.NewService().SendInitWebhookComment(
						item.MainRepo, prID, baseURI, false, false, false, true, log,
					)
					if err != nil {
						log.Errorf("failed to init webhook comment due to %s", err)
						mErr = multierror.Append(mErr, err)
					}
				}
			}
			workflowController := controller.CreateWorkflowController(item.WorkflowArg)
			if err := workflowController.UpdateWithLatestWorkflow(nil); err != nil {
				errMsg := fmt.Sprintf("merge workflow args error: %v", err)
				log.Error(errMsg)
				mErr = multierror.Append(mErr, fmt.Errorf(errMsg))
				continue
			}
			if err := workflowController.SetRepo(eventRepo); err != nil {
				errMsg := fmt.Sprintf("merge webhook repo info to workflowargs error: %v", err)
				log.Error(errMsg)
				mErr = multierror.Append(mErr, fmt.Errorf(errMsg))
				continue
			}
			if notification != nil {
				workflowController.NotificationID = notification.ID.Hex()
			}
			workflowController.HookPayload = hookPayload
			if resp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
				Name: setting.WebhookTaskCreator,
			}, workflowController.WorkflowV4, log); err != nil {
				errMsg := fmt.Sprintf("failed to create workflow task when receive push event due to %v ", err)
				log.Error(errMsg)
				mErr = multierror.Append(mErr, fmt.Errorf(errMsg))
			} else {
				log.Infof("succeed to create task %v", resp)
			}
		}
	}
	return mErr.ErrorOrNil()
}
