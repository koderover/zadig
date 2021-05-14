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
	"github.com/hashicorp/go-multierror"
	"github.com/xanzy/go-gitlab"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/scmnotify"
	workflowservice "github.com/koderover/zadig/lib/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types"
)

type pipelineGitEventMatcher interface {
	Match(*commonmodels.MainHookRepo) (*commonmodels.TaskArgs, error)
}

type pipelineGitlabPushEventMatcher struct {
	log      *xlog.Logger
	pipeline *commonmodels.Pipeline
	event    *gitlab.PushEvent
}

func (pgpem *pipelineGitlabPushEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (*commonmodels.TaskArgs, error) {
	ev := pgpem.event
	var (
		changedFiles = make([]string, 0)
	)
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Project.PathWithNamespace {
		if hookRepo.Branch == getBranchFromRef(ev.Ref) && EventConfigured(*hookRepo, config.HookEventPush) {
			for _, commit := range ev.Commits {
				changedFiles = append(changedFiles, commit.Added...)
				changedFiles = append(changedFiles, commit.Removed...)
				changedFiles = append(changedFiles, commit.Modified...)
			}
		}
	}
	if MatchChanges(*hookRepo, changedFiles) {
		var (
			branch        = getBranchFromRef(ev.Ref)
			ref           = ev.Ref
			commitID      = ev.Commits[0].ID
			commitMessage = ev.Commits[0].Message
		)

		eventRepo := &types.Repository{
			RepoOwner:     hookRepo.RepoOwner,
			RepoName:      hookRepo.RepoName,
			Branch:        branch,
			CommitID:      commitID,
			CommitMessage: commitMessage,
			IsPrimary:     true,
		}

		ptargs := &commonmodels.TaskArgs{
			PipelineName: pgpem.pipeline.Name,
			TaskCreator:  setting.WebhookTaskCreator,
			ReqID:        pgpem.log.ReqID(),
			HookPayload: &commonmodels.HookPayload{
				Owner:  hookRepo.RepoOwner,
				Repo:   hookRepo.RepoName,
				Branch: branch,
				Ref:    ref,
				IsPr:   false,
			},
			Builds: []*types.Repository{eventRepo},
			Test: commonmodels.TestArgs{
				Builds: []*types.Repository{eventRepo},
			},
		}
		return ptargs, nil
	}

	return nil, nil
}

type pipelineGitlabMergeEventMatcher struct {
	diffFunc gitlabMergeRequestDiffFunc
	log      *xlog.Logger
	pipeline *commonmodels.Pipeline
	event    *gitlab.MergeEvent
}

func (pgmem *pipelineGitlabMergeEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (*commonmodels.TaskArgs, error) {
	ev := pgmem.event
	var (
		err          error
		changedFiles = make([]string, 0)
	)
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.ObjectAttributes.Target.PathWithNamespace {
		if EventConfigured(*hookRepo, config.HookEventPr) && (hookRepo.Branch == ev.ObjectAttributes.TargetBranch) {
			if ev.ObjectAttributes.State == "opened" {
				changedFiles, err = pgmem.diffFunc(ev, hookRepo.CodehostID)
				if err != nil {
					pgmem.log.Warnf("failed to get changes of event %v , err:%v", ev, err)
					return nil, err
				}
				pgmem.log.Debugf("succeed to get %d changes in merge event", len(changedFiles))
			}
		}
	}
	if MatchChanges(*hookRepo, changedFiles) {
		var (
			branch        = ev.ObjectAttributes.TargetBranch
			commitID      = ev.ObjectAttributes.LastCommit.ID
			commitMessage = ev.ObjectAttributes.LastCommit.Message
		)

		eventRepo := &types.Repository{
			RepoOwner:     hookRepo.RepoOwner,
			RepoName:      hookRepo.RepoName,
			PR:            ev.ObjectAttributes.IID,
			Branch:        branch,
			CommitID:      commitID,
			CommitMessage: commitMessage,
			IsPrimary:     true,
		}

		ptargs := &commonmodels.TaskArgs{
			PipelineName: pgmem.pipeline.Name,
			TaskCreator:  setting.WebhookTaskCreator,
			ReqID:        pgmem.log.ReqID(),
			HookPayload: &commonmodels.HookPayload{
				Owner:  hookRepo.RepoOwner,
				Repo:   hookRepo.RepoName,
				Branch: branch,
				Ref:    commitID,
				IsPr:   true,
			},
			Builds: []*types.Repository{eventRepo},
			Test: commonmodels.TestArgs{
				Builds: []*types.Repository{eventRepo},
			},
		}
		return ptargs, nil
	}
	return nil, nil
}

func pipelineCreateGitlabEventMatcher(
	event interface{}, diffSrv gitlabMergeRequestDiffFunc, pipeline *commonmodels.Pipeline, log *xlog.Logger,
) pipelineGitEventMatcher {
	switch e := event.(type) {
	case *gitlab.PushEvent:
		return &pipelineGitlabPushEventMatcher{
			pipeline: pipeline,
			log:      log,
			event:    e,
		}
	case *gitlab.MergeEvent:
		return &pipelineGitlabMergeEventMatcher{
			diffFunc: diffSrv,
			log:      log,
			event:    e,
			pipeline: pipeline,
		}
	}

	return nil
}

func TriggerPipelineByGitlabEvent(event interface{}, baseUri string, log *xlog.Logger) error {
	pipelineList, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{})
	if err != nil {
		log.Errorf("list PipelineV2 error: %v", err)
		return e.ErrListPipeline
	}

	mErr := &multierror.Error{}
	diffSrv := func(mergeEvent *gitlab.MergeEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfMergeRequest(mergeEvent, codehostId)
	}

	var notification *commonmodels.Notification

	for _, pipelineObject := range pipelineList {
		if pipelineObject.Hook != nil && pipelineObject.Hook.Enabled {
			log.Debugf("find %d hooks in pipeline %s", len(pipelineObject.Hook.GitHooks), pipelineObject.Name)
			for _, item := range pipelineObject.Hook.GitHooks {
				matcher := pipelineCreateGitlabEventMatcher(event, diffSrv, pipelineObject, log)
				if matcher == nil {
					continue
				}
				hookRepo := new(commonmodels.MainHookRepo)
				hookRepo.RepoOwner = item.Owner
				hookRepo.RepoName = item.Repo
				hookRepo.Branch = item.Branch
				hookRepo.MatchFolders = item.MatchFolders
				HookEvents := make([]config.HookEventType, 0)
				for _, event := range item.Events {
					if config.HookEventPush == config.HookEventType(event) {
						HookEvents = append(HookEvents, config.HookEventPush)
					} else if config.HookEventPr == config.HookEventType(event) {
						HookEvents = append(HookEvents, config.HookEventPr)
					}
				}
				hookRepo.Events = HookEvents
				hookRepo.CodehostID = item.CodehostId
				taskargs, err := matcher.Match(hookRepo)
				if taskargs == nil || err != nil {
					log.Infof("[Webhook] %s Match none , [task] : %s , [error] : %v", pipelineObject.Name, pipelineObject.Name, err)
					continue
				}
				log.Infof("TriggerPipelineByGitlabEvent event match hook, pipelineObject.Name:%s\n", pipelineObject.Name)
				if taskargs.HookPayload.IsPr {
					hookRepo.RepoOwner = taskargs.HookPayload.Owner
					if notification == nil {
						notification, _ = scmnotify.NewService().SendInitWebhookComment(
							hookRepo, taskargs.Builds[0].PR, baseUri, true, false, log,
						)
					}
				}
				if notification != nil {
					taskargs.NotificationId = notification.ID.Hex()
				}
				resp, err1 := workflowservice.CreatePipelineTask(taskargs, log)
				if err1 != nil {
					log.Errorf("[Webhook] CreatePipelineTask task %s error: %v", pipelineObject.Name, err)
					continue
				}
				log.Infof("[Webhook] triggered task %s:%d", pipelineObject.Name, resp.TaskID)
			}
		}
	}

	return mErr.ErrorOrNil()
}
