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
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/hashicorp/go-multierror"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	scanningservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	"github.com/koderover/zadig/pkg/types"
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

	for _, scanning := range scanningList {
		if scanning.AdvancedSetting.HookCtl != nil && scanning.AdvancedSetting.HookCtl.Enabled {
			log.Infof("find %d hooks in scanning %s", len(scanning.AdvancedSetting.HookCtl.Items), scanning.Name)
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
					var mergeRequestID int
					if ev, isPr := event.(*gitlab.MergeEvent); isPr {
						// 如果是merge request，且该webhook触发器配置了自动取消，
						// 则需要确认该merge request在本次commit之前的commit触发的任务是否处理完，没有处理完则取消掉。
						mergeRequestID = ev.ObjectAttributes.IID

						if notification == nil {
							mainRepo := ConvertScanningHookToMainHookRepo(item)
							notification, _ = scmnotify.NewService().SendInitWebhookComment(
								mainRepo, ev.ObjectAttributes.IID, baseURI, false, false, true, log,
							)
						}
					}

					triggerRepoInfo := make([]*scanningservice.ScanningRepoInfo, 0)
					// for now only one repo is supported
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

					if resp, err := scanningservice.CreateScanningTask(scanning.ID.Hex(), triggerRepoInfo, notificationID, "webhook", log); err != nil {
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

func (gpem *gitlabPushEventMatcherForScanning) Match(hookRepo *types.ScanningHook) (bool, error) {
	ev := gpem.event
	if hookRepo == nil {
		return false, nil
	}
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Project.PathWithNamespace {
		matchRepo := ConvertScanningHookToMainHookRepo(hookRepo)

		if !EventConfigured(matchRepo, config.HookEventPush) {
			return false, nil
		}

		if hookRepo.Branch != getBranchFromRef(ev.Ref) {
			return false, nil
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

func (gmem *gitlabMergeEventMatcherForScanning) Match(hookRepo *types.ScanningHook) (bool, error) {
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

func (gtem *gitlabTagEventMatcherForScanning) Match(hookRepo *types.ScanningHook) (bool, error) {
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
