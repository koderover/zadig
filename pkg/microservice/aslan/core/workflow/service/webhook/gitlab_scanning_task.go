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

	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/hashicorp/go-multierror"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	scanningservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/testing/service"
	"github.com/koderover/zadig/v2/pkg/types"
)

func TriggerScanningByGitlabEvent(event interface{}, baseURI, requestID string, log *zap.SugaredLogger) error {
	// 1. find configured testing
	scanningList, _, err := commonrepo.NewScanningColl().List(nil, 0, 0)
	if err != nil {
		log.Errorf("failed to list testing %v", err)
		return err
	}

	mErr := &multierror.Error{}
	diffSrv := func(mergeEvent *gitlab.MergeEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfMergeRequest(mergeEvent, codehostId)
	}

	var notification *commonmodels.Notification
	var hookPayload *commonmodels.HookPayload

	for _, scanning := range scanningList {
		if scanning.AdvancedSetting.HookCtl != nil && scanning.AdvancedSetting.HookCtl.Enabled {
			log.Debugf("find %d hooks in scanning %s", len(scanning.AdvancedSetting.HookCtl.Items), scanning.Name)
			for _, item := range scanning.AdvancedSetting.HookCtl.Items {
				// 2. match webhook
				matcher := createGitlabEventMatcherForScanning(event, diffSrv, scanning, log)
				if matcher == nil {
					continue
				}

				if matches, err := matcher.Match(item); err != nil {
					mErr = multierror.Append(mErr, err)
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

					eventRepo := matcher.GetHookRepo(mainRepo)

					switch ev := event.(type) {
					case *gitlab.MergeEvent:
						eventType = EventTypePR
						mergeRequestID = ev.ObjectAttributes.IID
						commitID = ev.ObjectAttributes.LastCommit.ID
						prID = ev.ObjectAttributes.IID
						autoCancelOpt.MergeRequestID = strconv.Itoa(mergeRequestID)
						autoCancelOpt.CommitID = commitID
						autoCancelOpt.Type = eventType

						hookPayload = &commonmodels.HookPayload{
							Owner:          eventRepo.RepoOwner,
							Repo:           eventRepo.RepoName,
							Branch:         eventRepo.Branch,
							IsPr:           true,
							MergeRequestID: strconv.Itoa(mergeRequestID),
							CommitID:       commitID,
							CodehostID:     eventRepo.CodehostID,
							EventType:      eventType,
						}
					case *gitlab.PushEvent:
						eventType = EventTypePush
						ref = ev.Ref
						commitID = ev.After
						autoCancelOpt.Ref = ref
						autoCancelOpt.CommitID = commitID
						autoCancelOpt.Type = eventType

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
						err = AutoCancelWorkflowV4Task(autoCancelOpt, log)
						if err != nil {
							log.Errorf("failed to auto cancel scanning task when receive event %v due to %v ", event, err)
							mErr = multierror.Append(mErr, err)
						}
						// 发送本次commit的通知
						if autoCancelOpt.Type == EventTypePR && notification == nil {
							notification, _ = scmnotify.NewService().SendInitWebhookComment(
								mainRepo, prID, baseURI, false, false, true, false, log,
							)
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
					}
					triggerRepoInfo = append(triggerRepoInfo, repoInfo)

					repoInfo.PR = mergeRequestID

					notificationID := ""
					if notification != nil {
						notificationID = notification.ID.Hex()
					}

					if resp, err := scanningservice.CreateScanningTaskV2(scanning.ID.Hex(), "webhook", "", "", &scanningservice.CreateScanningTaskReq{
						Repos:       triggerRepoInfo,
						HookPayload: hookPayload,
					}, notificationID, log); err != nil {
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

func createGitlabEventMatcherForScanning(
	event interface{}, diffSrv gitlabMergeRequestDiffFunc, scanning *commonmodels.Scanning, log *zap.SugaredLogger,
) gitEventMatcherForScanning {
	switch evt := event.(type) {
	case *gitlab.PushEvent:
		return &gitlabPushEventMatcherForScanning{
			scanning: scanning,
			log:      log,
			event:    evt,
		}
	case *gitlab.MergeEvent:
		return &gitlabMergeEventMatcherForScanning{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			scanning: scanning,
		}
	case *gitlab.TagEvent:
		return &gitlabTagEventMatcherForScanning{
			scanning: scanning,
			log:      log,
			event:    evt,
		}
	}

	return nil
}

type gitlabPushEventMatcherForScanning struct {
	log      *zap.SugaredLogger
	scanning *commonmodels.Scanning
	event    *gitlab.PushEvent
}

func (gmem *gitlabPushEventMatcherForScanning) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		Source:        hookRepo.Source,
	}
}

func (gpem *gitlabPushEventMatcherForScanning) Match(hookRepo *commonmodels.ScanningHook) (bool, error) {
	ev := gpem.event
	if hookRepo == nil {
		return false, nil
	}
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Project.PathWithNamespace {
		matchRepo := ConvertScanningHookToMainHookRepo(hookRepo)

		if !EventConfigured(matchRepo, config.HookEventPush) {
			return false, nil
		}

		if !matchRepo.IsRegular {
			if getBranchFromRef(ev.Ref) != hookRepo.Branch {
				return false, nil
			}
		} else {
			if matched, _ := regexp.MatchString(hookRepo.Branch, getBranchFromRef(ev.Ref)); !matched {
				return false, nil
			}
		}

		hookRepo.Branch = getBranchFromRef(ev.Ref)
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

type gitlabMergeEventMatcherForScanning struct {
	diffFunc gitlabMergeRequestDiffFunc
	log      *zap.SugaredLogger
	scanning *commonmodels.Scanning
	event    *gitlab.MergeEvent
}

func (gmem *gitlabMergeEventMatcherForScanning) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		Source:        hookRepo.Source,
		PR:            gmem.event.ObjectAttributes.IID,
	}
}

func (gmem *gitlabMergeEventMatcherForScanning) Match(hookRepo *commonmodels.ScanningHook) (bool, error) {
	ev := gmem.event
	if hookRepo == nil {
		return false, nil
	}
	// TODO: match codehost
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.ObjectAttributes.Target.PathWithNamespace {
		matchRepo := ConvertScanningHookToMainHookRepo(hookRepo)
		if !EventConfigured(matchRepo, config.HookEventPr) {
			return false, nil
		}

		isRegExp := matchRepo.IsRegular

		if !isRegExp {
			if ev.ObjectAttributes.TargetBranch != hookRepo.Branch {
				return false, nil
			}
		} else {
			if matched, _ := regexp.MatchString(hookRepo.Branch, ev.ObjectAttributes.TargetBranch); !matched {
				return false, nil
			}
		}

		hookRepo.Branch = ev.ObjectAttributes.TargetBranch

		if ev.ObjectAttributes.State == "opened" {
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

type gitlabTagEventMatcherForScanning struct {
	log      *zap.SugaredLogger
	scanning *commonmodels.Scanning
	event    *gitlab.TagEvent
}

func (gmem *gitlabTagEventMatcherForScanning) GetHookRepo(hookRepo *commonmodels.MainHookRepo) *types.Repository {
	return &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		Source:        hookRepo.Source,
	}
}

func (gtem *gitlabTagEventMatcherForScanning) Match(hookRepo *commonmodels.ScanningHook) (bool, error) {
	ev := gtem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Project.PathWithNamespace {
		hookInfo := ConvertScanningHookToMainHookRepo(hookRepo)
		if !EventConfigured(hookInfo, config.HookEventTag) {
			return false, nil
		}

		hookRepo.Branch = ev.Project.DefaultBranch
		return true, nil
	}

	return false, nil
}
