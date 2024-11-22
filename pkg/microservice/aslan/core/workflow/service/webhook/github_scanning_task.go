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
	"regexp"
	"strconv"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	scanningservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/testing/service"
	"github.com/koderover/zadig/v2/pkg/types"
)

type gitEventMatcherForScanning interface {
	Match(repository *commonmodels.ScanningHook) (bool, error)
	GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository
}

func TriggerScanningByGithubEvent(event interface{}, requestID string, log *zap.SugaredLogger) error {
	//1.find configured testing
	scanningList, _, err := commonrepo.NewScanningColl().List(nil, 0, 0)
	if err != nil {
		log.Errorf("failed to list scanning %v", err)
		return err
	}

	mErr := &multierror.Error{}
	diffSrv := func(pullRequestEvent *github.PullRequestEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfPullRequest(pullRequestEvent, codehostId)
	}
	hookPayload := &commonmodels.HookPayload{}

	log.Infof("Matching scanning list to find matched task to run.")
	for _, scanning := range scanningList {
		if scanning.AdvancedSetting.HookCtl != nil && scanning.AdvancedSetting.HookCtl.Enabled {
			for _, item := range scanning.AdvancedSetting.HookCtl.Items {
				matcher := createGithubEventMatcherForScanning(event, diffSrv, scanning, log)
				if matcher == nil {
					log.Infof("got a nil matcher for trigger: %s/%s, stopping...", item.RepoOwner, item.RepoName)
					continue
				}
				if matches, err := matcher.Match(item); err != nil {
					mErr = multierror.Append(err)
				} else if matches {
					log.Infof("event match hook %v of %s", item, scanning.Name)

					var commitID, ref, eventType string
					var prID, mergeRequestID int

					mainRepo := ConvertScanningHookToMainHookRepo(item)
					autoCancelOpt := &AutoCancelOpt{
						WorkflowName: commonutil.GenScanningWorkflowName(scanning.ID.Hex()),
						TaskType:     config.ScanningType,
						MainRepo:     mainRepo,
						AutoCancel:   item.AutoCancel,
					}

					switch ev := event.(type) {
					case *github.PullRequestEvent:
						eventType = EventTypePR
						if ev.PullRequest != nil && ev.PullRequest.Number != nil && ev.PullRequest.Head != nil && ev.PullRequest.Head.SHA != nil {
							mergeRequestID = *ev.PullRequest.Number
							commitID = *ev.PullRequest.Head.SHA
							prID = mergeRequestID
							autoCancelOpt.MergeRequestID = strconv.Itoa(mergeRequestID)
							autoCancelOpt.Type = eventType
						}

						hookPayload = &commonmodels.HookPayload{
							Owner:          *ev.Repo.Owner.Login,
							Repo:           *ev.Repo.Name,
							Branch:         *ev.PullRequest.Base.Ref,
							Ref:            *ev.PullRequest.Head.SHA,
							IsPr:           true,
							CodehostID:     mainRepo.CodehostID,
							MergeRequestID: strconv.Itoa(mergeRequestID),
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
								CommitID:   commitID,
								EventType:  eventType,
								CodehostID: mainRepo.CodehostID,
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
							log.Errorf("failed to auto cancel scanning task when receive event %v due to %v ", event, err)
							mErr = multierror.Append(mErr, err)
						}
					}

					triggerRepoInfo := make([]*scanningservice.ScanningRepoInfo, 0)
					for _, scanningRepo := range scanning.Repos {
						// if this is the triggering repo, we simply skip it and add it later with correct info
						if scanningRepo.CodehostID == item.CodehostID && scanningRepo.RepoOwner == item.RepoOwner && scanningRepo.RepoName == item.RepoName {
							continue
						}
						triggerRepoInfo = append(triggerRepoInfo, &scanningservice.ScanningRepoInfo{
							CodehostID: scanningRepo.CodehostID,
							Source:     scanningRepo.Source,
							RepoOwner:  scanningRepo.RepoOwner,
							RepoName:   scanningRepo.RepoName,
							Branch:     scanningRepo.Branch,
						})
					}

					repoInfo := &scanningservice.ScanningRepoInfo{
						CodehostID: item.CodehostID,
						Source:     item.Source,
						RepoOwner:  item.RepoOwner,
						RepoName:   item.RepoName,
						Branch:     item.Branch,
						PR:         prID,
					}

					triggerRepoInfo = append(triggerRepoInfo, repoInfo)

					if resp, err := scanningservice.CreateScanningTaskV2(scanning.ID.Hex(), "webhook", "", "", &scanningservice.CreateScanningTaskReq{
						Repos:       triggerRepoInfo,
						HookPayload: hookPayload,
					}, "", log); err != nil {
						log.Errorf("failed to create testing task when receive event %v due to %v ", event, err)
						mErr = multierror.Append(mErr, err)
					} else {
						log.Infof("succeed to create task %v", resp)
					}
				} else {
					log.Debugf("event not matches %v", item)
				}
			}
		}
	}
	return mErr.ErrorOrNil()
}

type githubPushEventMatcherForScanning struct {
	log      *zap.SugaredLogger
	scanning *commonmodels.Scanning
	event    *github.PushEvent
}

func (gpem *githubPushEventMatcherForScanning) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
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

func (gpem githubPushEventMatcherForScanning) Match(hookRepo *commonmodels.ScanningHook) (bool, error) {
	ev := gpem.event
	if hookRepo == nil {
		return false, nil
	}
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == *ev.Repo.FullName {
		matchRepo := ConvertScanningHookToMainHookRepo(hookRepo)

		if !EventConfigured(matchRepo, config.HookEventPush) {
			return false, nil
		}

		if hookRepo.Branch != getBranchFromRef(*ev.Ref) {
			return false, nil
		}

		hookRepo.Branch = getBranchFromRef(*ev.Ref)
		var changedFiles []string
		for _, commit := range ev.Commits {
			changedFiles = append(changedFiles, commit.Added...)
			changedFiles = append(changedFiles, commit.Removed...)
			changedFiles = append(changedFiles, commit.Modified...)
		}

		return MatchChanges(matchRepo, changedFiles), nil
	}

	return false, nil
}

type githubMergeEventMatcherForScanning struct {
	diffFunc githubPullRequestDiffFunc
	log      *zap.SugaredLogger
	scanning *commonmodels.Scanning
	event    *github.PullRequestEvent
}

func (gmem *githubMergeEventMatcherForScanning) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
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

func (gmem githubMergeEventMatcherForScanning) Match(hookRepo *commonmodels.ScanningHook) (bool, error) {
	ev := gmem.event
	if hookRepo == nil {
		return false, nil
	}
	// TODO: match codehost
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == *ev.PullRequest.Base.Repo.FullName {
		matchRepo := ConvertScanningHookToMainHookRepo(hookRepo)
		if !EventConfigured(matchRepo, config.HookEventPr) {
			return false, nil
		}

		hookRepo.Branch = *ev.PullRequest.Base.Ref

		isRegExp := matchRepo.IsRegular

		if !isRegExp {
			if *ev.PullRequest.Base.Ref != hookRepo.Branch {
				return false, nil
			}
		} else {
			if matched, _ := regexp.MatchString(hookRepo.Branch, *ev.PullRequest.Base.Ref); !matched {
				return false, nil
			}
		}

		if *ev.PullRequest.State == "open" {
			var changedFiles []string
			changedFiles, err := gmem.diffFunc(ev, hookRepo.CodehostID)
			if err != nil {
				gmem.log.Warnf("failed to get changes of event %v", ev)
				return false, err
			}
			gmem.log.Debugf("succeed to get %d changes in merge event", len(changedFiles))

			return MatchChanges(matchRepo, changedFiles), nil
		}

	}
	return false, nil
}

type githubTagEventMatcherForScanning struct {
	log      *zap.SugaredLogger
	scanning *commonmodels.Scanning
	event    *github.CreateEvent
}

func (gtem *githubTagEventMatcherForScanning) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
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

func (gtem githubTagEventMatcherForScanning) Match(hookRepo *commonmodels.ScanningHook) (bool, error) {
	ev := gtem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == *ev.Repo.FullName {
		hookInfo := ConvertScanningHookToMainHookRepo(hookRepo)
		if !EventConfigured(hookInfo, config.HookEventTag) {
			return false, nil
		}

		hookRepo.Branch = *ev.Repo.DefaultBranch

		return true, nil
	}

	return false, nil
}

func createGithubEventMatcherForScanning(
	event interface{}, diffSrv githubPullRequestDiffFunc, scanning *commonmodels.Scanning, log *zap.SugaredLogger,
) gitEventMatcherForScanning {
	switch evt := event.(type) {
	case *github.PushEvent:
		return &githubPushEventMatcherForScanning{
			scanning: scanning,
			log:      log,
			event:    evt,
		}
	case *github.PullRequestEvent:
		return &githubMergeEventMatcherForScanning{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			scanning: scanning,
		}
	case *github.CreateEvent:
		return &githubTagEventMatcherForScanning{
			scanning: scanning,
			log:      log,
			event:    evt,
		}
	}

	return nil
}
