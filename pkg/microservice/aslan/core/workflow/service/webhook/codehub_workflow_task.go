package webhook

import (
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	workflowservice "github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/codehub"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/permission"
)

type codehubMergeEventMatcher struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *codehub.MergeEvent
}

func (cmem *codehubMergeEventMatcher) Match(hookRepo commonmodels.MainHookRepo) (bool, error) {
	ev := cmem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.ObjectAttributes.Target.PathWithNamespace {
		if EventConfigured(hookRepo, config.HookEventPr) && (hookRepo.Branch == ev.ObjectAttributes.TargetBranch) {
			if ev.ObjectAttributes.State == "opened" {
				return true, nil
			}
		}
	}
	return false, nil
}

func (cmem *codehubMergeEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo commonmodels.MainHookRepo, requestID string,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: cmem.workflow,
		reqID:    requestID,
	}

	args = factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
		PR:         cmem.event.ObjectAttributes.IID,
	})

	return args
}

type codehubPushEventMatcher struct {
	log      *zap.SugaredLogger
	workflow *commonmodels.Workflow
	event    *codehub.PushEvent
}

func (cpem *codehubPushEventMatcher) Match(hookRepo commonmodels.MainHookRepo) (bool, error) {
	ev := cpem.event
	if (hookRepo.RepoOwner + "/" + hookRepo.RepoName) == ev.Project.PathWithNamespace {
		if hookRepo.Branch == getBranchFromRef(ev.Ref) && EventConfigured(hookRepo, config.HookEventPush) {
			return true, nil
		}
	}

	return false, nil
}

func (cpem *codehubPushEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo commonmodels.MainHookRepo, requestID string,
) *commonmodels.WorkflowTaskArgs {
	factory := &workflowArgsFactory{
		workflow: cpem.workflow,
		reqID:    requestID,
	}

	factory.Update(product, args, &types.Repository{
		CodehostID: hookRepo.CodehostID,
		RepoName:   hookRepo.RepoName,
		RepoOwner:  hookRepo.RepoOwner,
		Branch:     hookRepo.Branch,
	})

	return args
}

func createCodehubEventMatcher(
	event interface{}, workflow *commonmodels.Workflow, log *zap.SugaredLogger,
) gitEventMatcher {
	switch evt := event.(type) {
	case *codehub.PushEvent:
		return &codehubPushEventMatcher{
			workflow: workflow,
			log:      log,
			event:    evt,
		}
	case *codehub.MergeEvent:
		return &codehubMergeEventMatcher{
			log:      log,
			event:    evt,
			workflow: workflow,
		}
	}

	return nil
}

func TriggerWorkflowByCodehubEvent(event interface{}, baseURI, requestID string, log *zap.SugaredLogger) error {
	// 1. find configured workflow
	workflowList, err := commonrepo.NewWorkflowColl().List(&commonrepo.ListWorkflowOption{})
	if err != nil {
		log.Errorf("failed to list workflow %v", err)
		return err
	}

	mErr := &multierror.Error{}
	for _, workflow := range workflowList {
		if workflow.HookCtl == nil || !workflow.HookCtl.Enabled {
			continue
		}

		log.Debugf("find %d hooks in workflow %s", len(workflow.HookCtl.Items), workflow.Name)
		for _, item := range workflow.HookCtl.Items {
			if item.WorkflowArgs == nil {
				continue
			}

			// 2. match webhook
			matcher := createCodehubEventMatcher(event, workflow, log)
			if matcher == nil {
				continue
			}

			matches, err := matcher.Match(item.MainRepo)
			if err != nil {
				mErr = multierror.Append(mErr, err)
				continue
			}

			if !matches {
				log.Debugf("event not matches %v", item.MainRepo)
				continue
			}

			log.Infof("event match hook %v of %s", item.MainRepo, workflow.Name)
			namespace := strings.Split(item.WorkflowArgs.Namespace, ",")[0]
			opt := &commonrepo.ProductFindOptions{Name: workflow.ProductTmplName, EnvName: namespace}
			var prod *commonmodels.Product
			if prod, err = commonrepo.NewProductColl().Find(opt); err != nil {
				log.Warnf("can't find environment %s-%s", item.WorkflowArgs.Namespace, workflow.ProductTmplName)
				continue
			}

			var mergeRequestID, commitID string
			if ev, isPr := event.(*codehub.MergeEvent); isPr {
				// 如果是merge request，且该webhook触发器配置了自动取消，
				// 则需要确认该merge request在本次commit之前的commit触发的任务是否处理完，没有处理完则取消掉。
				mergeRequestID = strconv.Itoa(ev.ObjectAttributes.IID)
				commitID = ev.ObjectAttributes.LastCommit.ID
				autoCancelOpt := &AutoCancelOpt{
					MergeRequestID: mergeRequestID,
					CommitID:       commitID,
					TaskType:       config.WorkflowType,
					MainRepo:       item.MainRepo,
					WorkflowArgs:   item.WorkflowArgs,
				}
				err := AutoCancelTask(autoCancelOpt, log)
				if err != nil {
					log.Errorf("failed to auto cancel workflow task when receive event %v due to %v ", event, err)
					mErr = multierror.Append(mErr, err)
				}
			}

			args := matcher.UpdateTaskArgs(prod, item.WorkflowArgs, item.MainRepo, requestID)
			args.MergeRequestID = mergeRequestID
			args.CommitID = commitID
			args.Source = setting.SourceFromCodeHub
			args.CodehostID = item.MainRepo.CodehostID
			args.RepoOwner = item.MainRepo.RepoOwner
			args.RepoName = item.MainRepo.RepoName
			// 3. create task with args
			if resp, err := workflowservice.CreateWorkflowTask(args, setting.WebhookTaskCreator, permission.AnonymousUserID, false, log); err != nil {
				log.Errorf("failed to create workflow task when receive push event %v due to %v ", event, err)
				mErr = multierror.Append(mErr, err)
			} else {
				log.Infof("succeed to create task %v", resp)
			}
		}
	}

	return mErr.ErrorOrNil()
}
