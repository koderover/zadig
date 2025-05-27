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

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

type gitEventMatcherForWorkflowV4 interface {
	Match(*commonmodels.MainHookRepo) (bool, error)
	GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository
}

type githubPushEventMatcheForWorkflowV4 struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.WorkflowV4
	event    *github.PushEvent
}

func (gpem *githubPushEventMatcheForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gpem *githubPushEventMatcheForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		CommitID:      *gpem.event.HeadCommit.ID,
		CommitMessage: *gpem.event.HeadCommit.Message,
		Source:        hookRepo.Source,
	}
}

type githubMergeEventMatcherForWorkflowV4 struct {
	diffFunc githubPullRequestDiffFunc
	log      *zap.SugaredLogger
	workflow *commonmodels.WorkflowV4
	event    *github.PullRequestEvent
}

func (gmem *githubMergeEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gmem *githubMergeEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		PR:            *gmem.event.PullRequest.Number,
		CommitID:      *gmem.event.PullRequest.Head.SHA,
		CommitMessage: *gmem.event.PullRequest.Title,
		Source:        hookRepo.Source,
	}
}

type githubTagEventMatcherForWorkflowV4 struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.WorkflowV4
	event    *github.CreateEvent
}

func (gtem githubTagEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gtem *githubTagEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
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

func createGithubEventMatcherForWorkflowV4(
	event interface{}, diffSrv githubPullRequestDiffFunc, workflow *commonmodels.WorkflowV4, log *zap.SugaredLogger,
) gitEventMatcherForWorkflowV4 {
	switch evt := event.(type) {
	case *github.PushEvent:
		return &githubPushEventMatcheForWorkflowV4{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	case *github.PullRequestEvent:
		return &githubMergeEventMatcherForWorkflowV4{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			workflow: workflow,
		}
	case *github.CreateEvent:
		return &githubTagEventMatcherForWorkflowV4{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	}

	return nil
}

func TriggerWorkflowV4ByGithubEvent(event interface{}, baseURI, deliveryID, requestID string, log *zap.SugaredLogger) error {
	workflows, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{}, 0, 0)
	if err != nil {
		errMsg := fmt.Sprintf("list workflow v4 error: %v", err)
		log.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	mErr := &multierror.Error{}
	diffSrv := func(pullRequestEvent *github.PullRequestEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfPullRequest(pullRequestEvent, codehostId)
	}
	hookPayload := &commonmodels.HookPayload{}

	for _, workflow := range workflows {
		gitHooks, err := commonrepo.NewWorkflowV4GitHookColl().List(internalhandler.NewBackgroupContext(), workflow.Name)
		if err != nil {
			log.Errorf("list workflow v4 git hook error: %v", err)
			continue
		}
		if len(gitHooks) == 0 {
			continue
		}

		for _, item := range gitHooks {
			if !item.Enabled {
				continue
			}
			matcher := createGithubEventMatcherForWorkflowV4(event, diffSrv, workflow, log)
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

			autoCancelOpt := &AutoCancelOpt{
				TaskType:     config.WorkflowType,
				MainRepo:     item.MainRepo,
				AutoCancel:   item.AutoCancel,
				WorkflowName: workflow.Name,
			}
			var mergeRequestID, commitID, ref, eventType string
			switch ev := event.(type) {
			case *github.PullRequestEvent:
				eventType = EventTypePR
				mergeRequestID = strconv.Itoa(*ev.PullRequest.Number)
				commitID = *ev.PullRequest.Head.SHA
				autoCancelOpt.Type = eventType
				autoCancelOpt.MergeRequestID = mergeRequestID
				autoCancelOpt.CommitID = commitID
				hookPayload = &commonmodels.HookPayload{
					Owner:          *ev.Repo.Owner.Login,
					Repo:           *ev.Repo.Name,
					Branch:         *ev.PullRequest.Base.Ref,
					Ref:            *ev.PullRequest.Head.SHA,
					IsPr:           true,
					CodehostID:     item.MainRepo.CodehostID,
					DeliveryID:     deliveryID,
					MergeRequestID: mergeRequestID,
					CommitID:       commitID,
					EventType:      eventType,
				}
			case *github.PushEvent:
				if ev.GetRef() != "" && ev.GetHeadCommit().GetID() != "" {
					eventType = EventTypePush
					ref = ev.GetRef()
					commitID = ev.GetHeadCommit().GetID()
					autoCancelOpt.Type = eventType
					autoCancelOpt.Ref = ref
					autoCancelOpt.CommitID = commitID
					hookPayload = &commonmodels.HookPayload{
						Owner:      *ev.Repo.Owner.Login,
						Repo:       *ev.Repo.Name,
						Ref:        ref,
						IsPr:       false,
						CodehostID: item.MainRepo.CodehostID,
						DeliveryID: deliveryID,
						CommitID:   commitID,
						EventType:  eventType,
					}
				}
			case *github.CreateEvent:
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
			}

			log.Infof("event match hook %v of %s", item.MainRepo, workflow.Name)

			eventRepo := matcher.GetHookRepo(item.MainRepo)
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
			workflowController.HookPayload = hookPayload
			if resp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
				Name: setting.WebhookTaskCreator,
			}, workflowController.WorkflowV4, log); err != nil {
				errMsg := fmt.Sprintf("failed to create workflow task when receive push event due to %v ", err)
				log.Error(errMsg)
				mErr = multierror.Append(mErr, fmt.Errorf(errMsg))
			} else {
				if workflowController.HookPayload.IsPr {
					// Updating the comment in the git repository, this will not cause the function to return error if this function call fails
					if err := scmnotify.NewService().CreateGitCheckForWorkflowV4(workflow, resp.TaskID, log); err != nil {
						log.Warnf("Failed to create github check status for custom workflow %s, taskID: %d the error is: %s", workflowController.Name, resp.TaskID, err)
					}
				}
				log.Infof("succeed to create task %v", resp)
			}
		}
	}
	return mErr.ErrorOrNil()
}
