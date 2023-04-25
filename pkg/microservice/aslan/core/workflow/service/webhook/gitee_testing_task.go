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

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	testingservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	"github.com/koderover/zadig/pkg/tool/gitee"
)

type giteePushEventMatcherForTesting struct {
	log     *zap.SugaredLogger
	testing *commonmodels.Testing
	event   *gitee.PushEvent
}

func (gpem *giteePushEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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
			matched, err := regexp.MatchString(hookRepo.Branch, getBranchFromRef(ev.Ref))
			if err != nil || !matched {
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

		return MatchChanges(hookRepo, changedFiles), nil
	}

	return false, nil
}

func (gpem *giteePushEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gpem.testing,
		reqID:   requestID,
	}

	factory.Update(args)
	return args
}

type giteeTagEventMatcherForTesting struct {
	log     *zap.SugaredLogger
	testing *commonmodels.Testing
	event   *gitee.TagPushEvent
}

func (gtem giteeTagEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gtem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Project.PathWithNamespace {
		if !EventConfigured(hookRepo, config.HookEventTag) {
			return false, nil
		}
		isRegular := hookRepo.IsRegular
		if !isRegular && hookRepo.Branch != ev.Project.DefaultBranch {
			return false, nil
		}

		if isRegular {
			matched, err := regexp.MatchString(hookRepo.Branch, ev.Project.DefaultBranch)
			if err != nil || !matched {
				return false, nil
			}
		}
		hookRepo.Branch = ev.Project.DefaultBranch
		return true, nil
	}

	return false, nil
}

func (gtem giteeTagEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gtem.testing,
		reqID:   requestID,
	}

	factory.Update(args)
	return args
}

type giteeMergeEventMatcherForTesting struct {
	diffFunc giteePullRequestDiffFunc
	log      *zap.SugaredLogger
	testing  *commonmodels.Testing
	event    *gitee.PullRequestEvent
}

func (gmem *giteeMergeEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gmem.event
	// TODO: match codehost
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

func (gmem *giteeMergeEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gmem.testing,
		reqID:   requestID,
	}

	args = factory.Update(args)

	return args
}

func createGiteeEventMatcherForTesting(event interface{}, diffSrv giteePullRequestDiffFunc, testing *commonmodels.Testing, log *zap.SugaredLogger) gitEventMatcherForTesting {
	switch evt := event.(type) {
	case *gitee.PushEvent:
		return &giteePushEventMatcherForTesting{
			testing: testing,
			log:     log,
			event:   evt,
		}
	case *gitee.PullRequestEvent:
		return &giteeMergeEventMatcherForTesting{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			testing:  testing,
		}
	case *gitee.TagPushEvent:
		return &giteeTagEventMatcherForTesting{
			testing: testing,
			log:     log,
			event:   evt,
		}
	}

	return nil
}

func TriggerTestByGiteeEvent(event interface{}, baseURI, requestID string, log *zap.SugaredLogger) error {
	// 1. find configured testing
	testingList, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{})
	if err != nil {
		log.Errorf("failed to list testing,err: %s", err)
		return err
	}

	mErr := &multierror.Error{}
	diffSrv := func(PullRequestEvent *gitee.PullRequestEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfPullRequestEvent(PullRequestEvent, codehostId)
	}

	var notification *commonmodels.Notification
	for _, testing := range testingList {
		if testing.HookCtl != nil && testing.HookCtl.Enabled {
			log.Infof("find %d hooks in testing %s", len(testing.HookCtl.Items), testing.Name)
			for _, item := range testing.HookCtl.Items {
				if item.TestArgs == nil {
					continue
				}

				// 2. match webhook
				matcher := createGiteeEventMatcherForTesting(event, diffSrv, testing, log)
				if matcher == nil {
					continue
				}

				if matches, err := matcher.Match(item.MainRepo); err != nil {
					mErr = multierror.Append(mErr, err)
				} else if matches {
					log.Infof("event match hook %v of %s", item.MainRepo, testing.Name)
					var mergeRequestID, commitID, ref, eventType string
					prID := 0
					autoCancelOpt := &AutoCancelOpt{
						TaskType: config.TestType,
						MainRepo: item.MainRepo,
						TestArgs: item.TestArgs,
					}
					switch ev := event.(type) {
					case *gitee.PullRequestEvent:
						eventType = EventTypePR
						if ev.PullRequest != nil && ev.PullRequest.Number != 0 && ev.PullRequest.Head != nil && ev.PullRequest.Head.Sha != "" {
							mergeRequestID = strconv.Itoa(ev.PullRequest.Number)
							commitID = ev.PullRequest.Head.Sha
							autoCancelOpt.MergeRequestID = mergeRequestID
							autoCancelOpt.CommitID = commitID
							autoCancelOpt.Type = EventTypePR
							prID = ev.PullRequest.Number
						}
					case *gitee.PushEvent:
						eventType = EventTypePush
						ref = ev.Ref
						commitID = ev.After
						autoCancelOpt.Ref = ref
						autoCancelOpt.CommitID = commitID
						autoCancelOpt.Type = EventTypePush
					case *gitee.TagPushEvent:
						eventType = EventTypeTag
					}

					if autoCancelOpt.Type != "" {
						// If it is a merge request, and the webhook trigger is configured with automatic cancellation,
						// It is necessary to confirm whether the task triggered by the commit of the merge request before this commit has been processed, and it will be cancelled if it is not processed.
						err := AutoCancelTask(autoCancelOpt, log)
						if err != nil {
							log.Errorf("failed to auto cancel testing task when receive event %v due to %s ", event, err)
							mErr = multierror.Append(mErr, err)
						}

						if notification == nil {
							notification, err = scmnotify.NewService().SendInitWebhookComment(
								item.MainRepo, prID, baseURI, false, true, false, false, log,
							)
							if err != nil {
								log.Errorf("failed to init webhook comment due to %s", err)
								mErr = multierror.Append(mErr, err)
							}
						}
					}

					if notification != nil {
						item.TestArgs.NotificationID = notification.ID.Hex()
					}

					args := matcher.UpdateTaskArgs(item.TestArgs, requestID)
					args.Ref = ref
					args.EventType = eventType
					args.MergeRequestID = mergeRequestID
					args.CommitID = commitID
					args.Source = item.MainRepo.Source
					args.CodehostID = item.MainRepo.CodehostID
					args.RepoOwner = item.MainRepo.RepoOwner
					args.RepoName = item.MainRepo.RepoName

					// 3. create task with args
					if resp, err := testingservice.CreateTestTask(args, log); err != nil {
						log.Errorf("failed to create testing task when receive event %v due to %s ", event, err)
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
