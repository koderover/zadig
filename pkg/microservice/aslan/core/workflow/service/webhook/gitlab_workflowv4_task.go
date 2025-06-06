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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	gitlabtool "github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/v2/pkg/types"
)

type gitlabMergeEventMatcherForWorkflowV4 struct {
	diffFunc           gitlabMergeRequestDiffFunc
	log                *zap.SugaredLogger
	workflow           *commonmodels.WorkflowV4
	event              *gitlab.MergeEvent
	trigger            *TriggerYaml
	isYaml             bool
	yamlServiceChanged []BuildServices
}

func (gmem *gitlabMergeEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gmem.event
	// TODO: match codehost
	if !checkRepoNamespaceMatch(hookRepo, ev.ObjectAttributes.Target.PathWithNamespace) {
		return false, nil
	}
	if !EventConfigured(hookRepo, config.HookEventPr) {
		return false, nil
	}
	if gmem.isYaml {
		refFlag := false
		for _, ref := range gmem.trigger.Rules.Branchs {
			if matched, _ := regexp.MatchString(ref, getBranchFromRef(ev.ObjectAttributes.TargetBranch)); matched {
				refFlag = true
				break
			}
		}
		if !refFlag {
			return false, nil
		}
	} else {
		isRegular := hookRepo.IsRegular
		if !isRegular && hookRepo.Branch != ev.ObjectAttributes.TargetBranch {
			return false, nil
		}
		if isRegular {
			if matched, _ := regexp.MatchString(hookRepo.Branch, ev.ObjectAttributes.TargetBranch); !matched {
				return false, nil
			}
		}
	}
	hookRepo.Branch = ev.ObjectAttributes.TargetBranch
	hookRepo.Committer = ev.User.Username
	if ev.ObjectAttributes.State == "opened" {
		var changedFiles []string
		changedFiles, err := gmem.diffFunc(ev, hookRepo.CodehostID)
		if err != nil {
			gmem.log.Warnf("failed to get changes of event %v, err:%s", ev, err)
			return false, err
		}
		gmem.log.Debugf("succeed to get %d changes in merge event", len(changedFiles))
		if gmem.isYaml {
			serviceChangeds := ServicesMatchChangesFiles(gmem.trigger.Rules.MatchFolders, changedFiles)
			gmem.yamlServiceChanged = serviceChangeds
			return len(serviceChangeds) != 0, nil
		}
		return MatchChanges(hookRepo, changedFiles), nil
	}
	return false, nil
}

func (gmem *gitlabMergeEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		PR:            gmem.event.ObjectAttributes.IID,
		Source:        hookRepo.Source,
	}
}

func createGitlabEventMatcherForWorkflowV4(
	event interface{}, diffSrv gitlabMergeRequestDiffFunc, workflow *commonmodels.WorkflowV4, log *zap.SugaredLogger,
) gitEventMatcherForWorkflowV4 {
	switch evt := event.(type) {
	case *gitlab.PushEvent:
		return &gitlabPushEventMatcherForWorkflowV4{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	case *gitlab.MergeEvent:
		return &gitlabMergeEventMatcherForWorkflowV4{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			workflow: workflow,
		}
	case *gitlab.TagEvent:
		return &gitlabTagEventMatcherForWorkflowV4{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	}

	return nil
}

type gitlabPushEventMatcherForWorkflowV4 struct {
	log                *zap.SugaredLogger
	workflow           *commonmodels.WorkflowV4
	event              *gitlab.PushEvent
	trigger            *TriggerYaml
	isYaml             bool
	yamlServiceChanged []BuildServices
}

func (gpem *gitlabPushEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gpem.event
	if !checkRepoNamespaceMatch(hookRepo, ev.Project.PathWithNamespace) {
		return false, nil
	}
	if !EventConfigured(hookRepo, config.HookEventPush) {
		return false, nil
	}
	if gpem.isYaml {
		refFlag := false
		for _, ref := range gpem.trigger.Rules.Branchs {
			if matched, _ := regexp.MatchString(ref, getBranchFromRef(ev.Ref)); matched {
				refFlag = true
				break
			}
		}
		if !refFlag {
			return false, nil
		}
	} else {
		isRegular := hookRepo.IsRegular
		if !isRegular && hookRepo.Branch != getBranchFromRef(ev.Ref) {
			return false, nil
		}
		if isRegular {
			if matched, _ := regexp.MatchString(hookRepo.Branch, getBranchFromRef(ev.Ref)); !matched {
				return false, nil
			}
		}
	}

	hookRepo.Branch = getBranchFromRef(ev.Ref)
	hookRepo.Committer = ev.UserUsername
	var changedFiles []string
	detail, err := systemconfig.New().GetCodeHost(hookRepo.CodehostID)
	if err != nil {
		gpem.log.Errorf("GetCodehostDetail error: %s", err)
		return false, err
	}

	client, err := gitlabtool.NewClient(detail.ID, detail.Address, detail.AccessToken, config.ProxyHTTPSAddr(), detail.EnableProxy, detail.DisableSSL)
	if err != nil {
		gpem.log.Errorf("NewClient error: %s", err)
		return false, err
	}

	// When push a new branch, ev.Before will be a lot of "0"
	// So we should not use Compare
	if strings.Count(ev.Before, "0") == len(ev.Before) {
		for _, commit := range ev.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Removed...)
			changedFiles = append(changedFiles, commit.Modified...)
		}
	} else {
		// compare接口获取两个commit之间的最终的改动
		diffs, err := client.Compare(ev.ProjectID, ev.Before, ev.After)
		if err != nil {
			gpem.log.Errorf("Failed to get push event diffs, error: %s", err)
			return false, err
		}
		for _, diff := range diffs {
			changedFiles = append(changedFiles, diff.NewPath)
			changedFiles = append(changedFiles, diff.OldPath)
		}
	}
	if gpem.isYaml {
		serviceChangeds := ServicesMatchChangesFiles(gpem.trigger.Rules.MatchFolders, changedFiles)
		gpem.yamlServiceChanged = serviceChangeds
		return len(serviceChangeds) != 0, nil
	}
	return MatchChanges(hookRepo, changedFiles), nil
}

func (gpem *gitlabPushEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		Source:        hookRepo.Source,
	}
}

type gitlabTagEventMatcherForWorkflowV4 struct {
	log                *zap.SugaredLogger
	workflow           *commonmodels.WorkflowV4
	event              *gitlab.TagEvent
	trigger            *TriggerYaml
	isYaml             bool
	yamlServiceChanged []BuildServices
}

func (gtem gitlabTagEventMatcherForWorkflowV4) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gtem.event

	if !checkRepoNamespaceMatch(hookRepo, ev.Project.PathWithNamespace) {
		return false, nil
	}

	if !EventConfigured(hookRepo, config.HookEventTag) {
		return false, nil
	}

	hookRepo.Committer = ev.UserName
	hookRepo.Tag = getTagFromRef(ev.Ref)

	return true, nil
}

func (gpem *gitlabTagEventMatcherForWorkflowV4) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
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

func TriggerWorkflowV4ByGitlabEvent(event interface{}, baseURI, requestID string, log *zap.SugaredLogger) error {
	// TODO: cache workflow
	// 1. find configured workflow
	workflows, _, err := commonrepo.NewWorkflowV4Coll().List(&commonrepo.ListWorkflowV4Option{}, 0, 0)
	if err != nil {
		errMsg := fmt.Sprintf("list workflow v4 error: %v", err)
		log.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	mErr := &multierror.Error{}
	diffSrv := func(mergeEvent *gitlab.MergeEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfMergeRequest(mergeEvent, codehostId)
	}
	var notification *commonmodels.Notification
	var hookPayload *commonmodels.HookPayload

	for _, workflow := range workflows {
		if workflow.HookCtls == nil {
			continue
		}

		for _, item := range workflow.HookCtls {
			if !item.Enabled {
				continue
			}
			var pushEvent *gitlab.PushEvent
			var mergeEvent *gitlab.MergeEvent
			var tagEvent *gitlab.TagEvent
			switch evt := event.(type) {
			case *gitlab.PushEvent:
				pushEvent = evt
				if !checkRepoNamespaceMatch(item.MainRepo, pushEvent.Project.PathWithNamespace) {
					log.Debugf("event not matches repo: %v", item.MainRepo)
					continue
				}
			case *gitlab.MergeEvent:
				mergeEvent = evt
				if !checkRepoNamespaceMatch(item.MainRepo, mergeEvent.ObjectAttributes.Target.PathWithNamespace) {
					log.Debugf("event not matches repo: %v", item.MainRepo)
					continue
				}
			case *gitlab.TagEvent:
				tagEvent = evt
				if !checkRepoNamespaceMatch(item.MainRepo, tagEvent.Project.PathWithNamespace) {
					log.Debugf("event not matches repo: %v", item.MainRepo)
					continue
				}
			}
			matcher := createGitlabEventMatcherForWorkflowV4(event, diffSrv, workflow, log)
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
			case *gitlab.MergeEvent:
				eventType = EventTypePR
				mergeRequestID = strconv.Itoa(ev.ObjectAttributes.IID)
				commitID = ev.ObjectAttributes.LastCommit.ID
				prID = ev.ObjectAttributes.IID
				autoCancelOpt.Type = eventType
				autoCancelOpt.MergeRequestID = mergeRequestID
				autoCancelOpt.CommitID = commitID
				hookPayload = &commonmodels.HookPayload{
					Owner:          eventRepo.RepoOwner,
					Repo:           eventRepo.RepoName,
					Branch:         eventRepo.Branch,
					IsPr:           true,
					MergeRequestID: mergeRequestID,
					CommitID:       commitID,
					CodehostID:     eventRepo.CodehostID,
					EventType:      eventType,
				}
			case *gitlab.PushEvent:
				eventType = EventTypePush
				ref = ev.Ref
				commitID = ev.After
				autoCancelOpt.Type = EventTypePush
				autoCancelOpt.Ref = ref
				autoCancelOpt.CommitID = commitID
				hookPayload = &commonmodels.HookPayload{
					Owner:      eventRepo.RepoOwner,
					Repo:       eventRepo.RepoName,
					Branch:     eventRepo.Branch,
					Ref:        ref,
					IsPr:       false,
					CommitID:   commitID,
					CodehostID: eventRepo.CodehostID,
					EventType:  eventType,
				}
			case *gitlab.TagEvent:
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
					notification, _ = scmnotify.NewService().SendInitWebhookComment(
						item.MainRepo, prID, baseURI, false, false, false, true, log,
					)
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
