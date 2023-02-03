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
	"regexp"
	"strconv"

	multierror "github.com/hashicorp/go-multierror"
	gitlab "github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	testingservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	"github.com/koderover/zadig/pkg/setting"
)

type gitEventMatcherForTesting interface {
	Match(*commonmodels.MainHookRepo) (bool, error)
	UpdateTaskArgs(*commonmodels.TestTaskArgs, string) *commonmodels.TestTaskArgs
}

type testArgsFactory struct {
	testing *commonmodels.Testing
	reqID   string
}

func (waf *testArgsFactory) Update(args *commonmodels.TestTaskArgs) *commonmodels.TestTaskArgs {
	test := waf.testing
	args.TestName = test.Name
	args.TestTaskCreator = setting.WebhookTaskCreator
	args.ProductName = test.ProductName

	return args
}

type gitlabPushEventMatcherForTesting struct {
	log     *zap.SugaredLogger
	testing *commonmodels.Testing
	event   *gitlab.PushEvent
}

func (gpem *gitlabPushEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gpem.event
	if !checkRepoNamespaceMatch(hookRepo, ev.Project.PathWithNamespace) {
		return false, nil
	}
	if !EventConfigured(hookRepo, config.HookEventPush) {
		return false, nil
	}
	isRegular := hookRepo.IsRegular
	if !isRegular && hookRepo.Branch != getBranchFromRef(ev.Ref) {
		return false, nil
	}

	if isRegular {
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
	return MatchChanges(hookRepo, changedFiles), nil
}

func (gpem *gitlabPushEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gpem.testing,
		reqID:   requestID,
	}

	factory.Update(args)
	return args
}

type gitlabTagEventMatcherForTesting struct {
	log     *zap.SugaredLogger
	testing *commonmodels.Testing
	event   *gitlab.TagEvent
}

func (gtem gitlabTagEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gtem.event

	if !checkRepoNamespaceMatch(hookRepo, ev.Project.PathWithNamespace) {
		return false, nil
	}

	if !EventConfigured(hookRepo, config.HookEventTag) {
		return false, nil
	}
	isRegular := hookRepo.IsRegular
	if !isRegular && hookRepo.Branch != ev.Project.DefaultBranch {
		return false, nil
	}

	if isRegular {
		if matched, _ := regexp.MatchString(hookRepo.Branch, ev.Project.DefaultBranch); !matched {
			return false, nil
		}
	}
	hookRepo.Branch = ev.Project.DefaultBranch
	return true, nil
}

func (gtem gitlabTagEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gtem.testing,
		reqID:   requestID,
	}

	factory.Update(args)
	return args
}

// TriggerTestByGitlabEvent 测试管理模块的触发器任务
func TriggerTestByGitlabEvent(event interface{}, baseURI, requestID string, log *zap.SugaredLogger) error {
	// 1. find configured testing
	testingList, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{})
	if err != nil {
		log.Errorf("failed to list testing %v", err)
		return err
	}

	mErr := &multierror.Error{}
	diffSrv := func(mergeEvent *gitlab.MergeEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfMergeRequest(mergeEvent, codehostId)
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
				matcher := createGitlabEventMatcherForTesting(event, diffSrv, testing, log)
				if matcher == nil {
					continue
				}

				if matches, err := matcher.Match(item.MainRepo); err != nil {
					mErr = multierror.Append(mErr, err)
				} else if matches {
					log.Infof("event match hook %v of %s", item.MainRepo, testing.Name)
					var mergeRequestID, commitID, ref, eventType string
					var prID int
					autoCancelOpt := &AutoCancelOpt{
						TaskType: config.TestType,
						MainRepo: item.MainRepo,
						TestArgs: item.TestArgs,
					}
					switch ev := event.(type) {
					case *gitlab.MergeEvent:
						eventType = EventTypePR
						mergeRequestID = strconv.Itoa(ev.ObjectAttributes.IID)
						commitID = ev.ObjectAttributes.LastCommit.ID
						prID = ev.ObjectAttributes.IID
						autoCancelOpt.MergeRequestID = mergeRequestID
						autoCancelOpt.CommitID = commitID
						autoCancelOpt.Type = eventType
					case *gitlab.PushEvent:
						eventType = EventTypePush
						ref = ev.Ref
						commitID = ev.After
						autoCancelOpt.Ref = ref
						autoCancelOpt.CommitID = commitID
						autoCancelOpt.Type = eventType
					case *gitlab.TagEvent:
						eventType = EventTypeTag
					}
					if autoCancelOpt.Type != "" {
						err := AutoCancelTask(autoCancelOpt, log)
						if err != nil {
							log.Errorf("failed to auto cancel testing task when receive event %v due to %v ", event, err)
							mErr = multierror.Append(mErr, err)
						}
						// 发送本次commit的通知
						if autoCancelOpt.Type == EventTypePR && notification == nil {
							notification, _ = scmnotify.NewService().SendInitWebhookComment(
								item.MainRepo, prID, baseURI, false, true, false, false, log,
							)
						}
					}

					if notification != nil {
						item.TestArgs.NotificationID = notification.ID.Hex()
					}

					args := matcher.UpdateTaskArgs(item.TestArgs, requestID)
					args.MergeRequestID = mergeRequestID
					args.Ref = ref
					args.EventType = eventType
					args.CommitID = commitID
					args.Source = setting.SourceFromGitlab
					args.CodehostID = item.MainRepo.CodehostID
					args.RepoOwner = item.MainRepo.RepoOwner
					args.RepoNamespace = item.MainRepo.GetRepoNamespace()
					args.RepoName = item.MainRepo.RepoName

					// 3. create task with args
					if resp, err := testingservice.CreateTestTask(args, log); err != nil {
						log.Errorf("failed to create testing task when receive event %v due to %v ", event, err)
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

func createGitlabEventMatcherForTesting(
	event interface{}, diffSrv gitlabMergeRequestDiffFunc, testing *commonmodels.Testing, log *zap.SugaredLogger,
) gitEventMatcherForTesting {
	switch evt := event.(type) {
	case *gitlab.PushEvent:
		return &gitlabPushEventMatcherForTesting{
			testing: testing,
			log:     log,
			event:   evt,
		}
	case *gitlab.MergeEvent:
		return &gitlabMergeEventMatcherForTesting{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			testing:  testing,
		}
	case *gitlab.TagEvent:
		return &gitlabTagEventMatcherForTesting{
			testing: testing,
			log:     log,
			event:   evt,
		}
	}

	return nil
}

type gitlabMergeEventMatcherForTesting struct {
	diffFunc gitlabMergeRequestDiffFunc
	log      *zap.SugaredLogger
	testing  *commonmodels.Testing
	event    *gitlab.MergeEvent
}

func (gmem *gitlabMergeEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gmem.event
	// TODO: match codehost
	if !checkRepoNamespaceMatch(hookRepo, ev.ObjectAttributes.Target.PathWithNamespace) {
		return false, nil
	}

	if !EventConfigured(hookRepo, config.HookEventPr) {
		return false, nil
	}

	isRegular := hookRepo.IsRegular
	if !isRegular && hookRepo.Branch != ev.ObjectAttributes.TargetBranch {
		return false, nil
	}

	if isRegular {
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

		return MatchChanges(hookRepo, changedFiles), nil
	}
	return false, nil
}

func (gmem *gitlabMergeEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gmem.testing,
		reqID:   requestID,
	}

	args = factory.Update(args)

	return args
}
