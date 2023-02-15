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

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	testingservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/testing/service"
	"github.com/koderover/zadig/pkg/setting"
)

func TriggerTestByGithubEvent(event interface{}, requestID string, log *zap.SugaredLogger) error {
	//1.find configured testing
	testingList, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{})
	if err != nil {
		log.Errorf("failed to list testing %v", err)
		return err
	}
	mErr := &multierror.Error{}
	diffSrv := func(pullRequestEvent *github.PullRequestEvent, codehostId int) ([]string, error) {
		return findChangedFilesOfPullRequest(pullRequestEvent, codehostId)
	}
	for _, testing := range testingList {
		if testing.HookCtl != nil && testing.HookCtl.Enabled {
			for _, item := range testing.HookCtl.Items {
				if item.TestArgs == nil {
					continue
				}
				matcher := createGithubEventMatcherForTesting(event, diffSrv, testing, log)
				if matcher == nil {
					continue
				}
				if matches, err := matcher.Match(item.MainRepo); err != nil {
					mErr = multierror.Append(err)
				} else if matches {
					log.Infof("event match hook %v of %s", item.MainRepo, testing.Name)
					var mergeRequestID, commitID, ref, eventType string
					autoCancelOpt := &AutoCancelOpt{
						TaskType: config.TestType,
						MainRepo: item.MainRepo,
						TestArgs: item.TestArgs,
					}
					switch ev := event.(type) {
					case *github.PullRequestEvent:
						eventType = EventTypePR
						if ev.PullRequest != nil && ev.PullRequest.Number != nil && ev.PullRequest.Head != nil && ev.PullRequest.Head.SHA != nil {
							mergeRequestID = strconv.Itoa(*ev.PullRequest.Number)
							commitID = *ev.PullRequest.Head.SHA
							autoCancelOpt.MergeRequestID = mergeRequestID
							autoCancelOpt.Type = eventType
						}
					case *github.PushEvent:
						if ev.GetRef() != "" && ev.GetHeadCommit().GetID() != "" {
							eventType = EventTypePush
							ref = ev.GetRef()
							commitID = ev.GetHeadCommit().GetID()
							autoCancelOpt.Type = eventType
							autoCancelOpt.Ref = ref
							autoCancelOpt.CommitID = commitID
						}
					case *github.CreateEvent:
						eventType = EventTypeTag
					}
					if autoCancelOpt.Type != "" {
						err := AutoCancelTask(autoCancelOpt, log)
						if err != nil {
							log.Errorf("failed to auto cancel workflow task when receive event due to %v ", err)
							mErr = multierror.Append(mErr, err)
						}
					}

					args := matcher.UpdateTaskArgs(item.TestArgs, requestID)
					args.MergeRequestID = mergeRequestID
					args.Ref = ref
					args.EventType = eventType
					args.CommitID = commitID
					args.Source = setting.SourceFromGithub
					args.CodehostID = item.MainRepo.CodehostID
					args.RepoOwner = item.MainRepo.RepoOwner
					args.RepoName = item.MainRepo.RepoName
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

type githubPushEventMatcherForTesting struct {
	log     *zap.SugaredLogger
	testing *commonmodels.Testing
	event   *github.PushEvent
}

type githubMergeEventMatcherForTesting struct {
	diffFunc githubPullRequestDiffFunc
	log      *zap.SugaredLogger
	testing  *commonmodels.Testing
	event    *github.PullRequestEvent
}

func (gpem *githubPushEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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
		if matched, _ := regexp.MatchString(hookRepo.Branch, getBranchFromRef(*ev.Ref)); !matched {
			return false, nil
		}
	}
	hookRepo.Branch = getBranchFromRef(*ev.Ref)
	var changedFiles []string
	for _, commit := range ev.Commits {
		changedFiles = append(changedFiles, commit.Added...)
		changedFiles = append(changedFiles, commit.Removed...)
		changedFiles = append(changedFiles, commit.Modified...)
	}

	return MatchChanges(hookRepo, changedFiles), nil
}

func (gpem *githubPushEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gpem.testing,
		reqID:   requestID,
	}

	factory.Update(args)
	return args
}

type githubTagEventMatcherForTesting struct {
	log     *zap.SugaredLogger
	testing *commonmodels.Testing
	event   *github.CreateEvent
}

func (gtem githubTagEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gtem.event
	if !checkRepoNamespaceMatch(hookRepo, *ev.Repo.FullName) {
		return false, nil
	}
	if !EventConfigured(hookRepo, config.HookEventTag) {
		return false, nil
	}
	isRegular := hookRepo.IsRegular
	if !isRegular && hookRepo.Branch != *ev.Repo.DefaultBranch {
		return false, nil
	}

	if isRegular {
		if matched, _ := regexp.MatchString(hookRepo.Branch, *ev.Repo.DefaultBranch); !matched {
			return false, nil
		}
	}
	hookRepo.Branch = *ev.Repo.DefaultBranch

	return true, nil
}

func (gtem githubTagEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gtem.testing,
		reqID:   requestID,
	}

	factory.Update(args)
	return args
}

func createGithubEventMatcherForTesting(
	event interface{}, diffSrv githubPullRequestDiffFunc, testing *commonmodels.Testing, log *zap.SugaredLogger,
) gitEventMatcherForTesting {
	switch evt := event.(type) {
	case *github.PushEvent:
		return &githubPushEventMatcherForTesting{
			testing: testing,
			log:     log,
			event:   evt,
		}
	case *github.PullRequestEvent:
		return &githubMergeEventMatcherForTesting{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			testing:  testing,
		}
	case *github.CreateEvent:
		return &githubTagEventMatcherForTesting{
			testing: testing,
			log:     log,
			event:   evt,
		}
	}

	return nil
}

func (gmem *githubMergeEventMatcherForTesting) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
	ev := gmem.event
	// TODO: match codehost

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

func (gmem *githubMergeEventMatcherForTesting) UpdateTaskArgs(args *commonmodels.TestTaskArgs, requestID string) *commonmodels.TestTaskArgs {
	factory := &testArgsFactory{
		testing: gmem.testing,
		reqID:   requestID,
	}

	args = factory.Update(args)

	return args
}
