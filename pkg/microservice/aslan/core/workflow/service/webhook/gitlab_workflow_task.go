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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/scmnotify"
	environmentservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	gitlabtool "github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

type gitlabMergeRequestDiffFunc func(event *gitlab.MergeEvent, id int) ([]string, error)

type gitlabMergeEventMatcher struct {
	diffFunc           gitlabMergeRequestDiffFunc
	log                *zap.SugaredLogger
	workflow           *commonmodels.Workflow
	event              *gitlab.MergeEvent
	trigger            *TriggerYaml
	isYaml             bool
	yamlServiceChanged []BuildServices
}

func (gmem *gitlabMergeEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gmem *gitlabMergeEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string,
) *commonmodels.WorkflowTaskArgs {
	if gmem.isYaml {
		var targets []*commonmodels.TargetArgs
		for _, target := range args.Target {
			for _, bs := range gmem.yamlServiceChanged {
				if target.Name == bs.ServiceModule && target.ServiceName == bs.Name {
					targets = append(targets, target)
					break
				}
			}
		}
		args.Target = targets
	}
	factory := &workflowArgsFactory{
		workflow: gmem.workflow,
		reqID:    requestID,
		IsYaml:   gmem.isYaml,
	}

	args = factory.Update(product, args, &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		PR:            gmem.event.ObjectAttributes.IID,
	})

	return args
}

func createGitlabEventMatcher(
	event interface{}, diffSrv gitlabMergeRequestDiffFunc, workflow *commonmodels.Workflow, isyaml bool, trigger *TriggerYaml, log *zap.SugaredLogger,
) gitEventMatcher {
	switch evt := event.(type) {
	case *gitlab.PushEvent:
		return &gitlabPushEventMatcher{
			workflow: workflow,
			log:      log,
			event:    evt,
			trigger:  trigger,
			isYaml:   isyaml,
		}
	case *gitlab.MergeEvent:
		return &gitlabMergeEventMatcher{
			diffFunc: diffSrv,
			log:      log,
			event:    evt,
			workflow: workflow,
			trigger:  trigger,
			isYaml:   isyaml,
		}
	case *gitlab.TagEvent:
		return &gitlabTagEventMatcher{
			workflow: workflow,
			log:      log,
			event:    evt,
			trigger:  trigger,
			isYaml:   isyaml,
		}
	}

	return nil
}

type gitlabPushEventMatcher struct {
	log                *zap.SugaredLogger
	workflow           *commonmodels.Workflow
	event              *gitlab.PushEvent
	trigger            *TriggerYaml
	isYaml             bool
	yamlServiceChanged []BuildServices
}

func (gpem *gitlabPushEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gpem *gitlabPushEventMatcher) UpdateTaskArgs(
	product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string,
) *commonmodels.WorkflowTaskArgs {

	if gpem.isYaml {
		var targets []*commonmodels.TargetArgs
		for _, target := range args.Target {
			for _, bs := range gpem.yamlServiceChanged {
				if target.Name == bs.ServiceModule && target.ServiceName == bs.Name {
					targets = append(targets, target)
				}
			}
		}
		args.Target = targets
	}
	factory := &workflowArgsFactory{
		workflow: gpem.workflow,
		reqID:    requestID,
		IsYaml:   gpem.isYaml,
	}

	factory.Update(product, args, &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
	})

	return args
}

type gitlabTagEventMatcher struct {
	log                *zap.SugaredLogger
	workflow           *commonmodels.Workflow
	event              *gitlab.TagEvent
	trigger            *TriggerYaml
	isYaml             bool
	yamlServiceChanged []BuildServices
}

func (gtem gitlabTagEventMatcher) Match(hookRepo *commonmodels.MainHookRepo) (bool, error) {
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

func (gtem gitlabTagEventMatcher) UpdateTaskArgs(product *commonmodels.Product, args *commonmodels.WorkflowTaskArgs, hookRepo *commonmodels.MainHookRepo, requestID string) *commonmodels.WorkflowTaskArgs {
	if gtem.isYaml {
		var targets []*commonmodels.TargetArgs
		for _, target := range args.Target {
			targets = append(targets, target)
		}
		args.Target = targets
	}
	factory := &workflowArgsFactory{
		workflow: gtem.workflow,
		reqID:    requestID,
		IsYaml:   gtem.isYaml,
	}

	factory.Update(product, args, &types.Repository{
		CodehostID:    hookRepo.CodehostID,
		RepoName:      hookRepo.RepoName,
		RepoOwner:     hookRepo.RepoOwner,
		RepoNamespace: hookRepo.GetRepoNamespace(),
		Branch:        hookRepo.Branch,
		Tag:           hookRepo.Tag,
	})

	return args
}

func UpdateWorkflowTaskArgs(triggerYaml *TriggerYaml, workflow *commonmodels.Workflow, workFlowArgs *commonmodels.WorkflowTaskArgs, item *commonmodels.WorkflowHook, branref string, prId int) error {
	svcType, err := getServiceTypeByProject(workflow.ProductTmplName)
	if err != nil {
		return fmt.Errorf("getServiceTypeByProduct ProductTmplName:%s err:%s", workflow.ProductTmplName, err)
	}

	ch, err := systemconfig.New().GetCodeHost(item.MainRepo.CodehostID)
	if err != nil {
		return fmt.Errorf("GetCodeHost codehostId:%d err:%s", item.MainRepo.CodehostID, err)
	}
	cli, err := gitlabtool.NewClient(ch.ID, ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy, ch.DisableSSL)
	if err != nil {
		return fmt.Errorf("gitlabtool.NewClient codehostId:%d err:%s", item.MainRepo.CodehostID, err)
	}
	zadigTriggerYamls, err := cli.GetYAMLContents(item.MainRepo.RepoOwner, item.MainRepo.RepoName, item.YamlPath, branref, false, false)
	if err != nil {
		return fmt.Errorf("GetYAMLContents repoowner:%s reponame:%s ref:%s triggeryaml:%s err:%s", item.MainRepo.RepoOwner, item.MainRepo.RepoName, item.YamlPath, branref, err)
	}
	if len(zadigTriggerYamls) == 0 {
		return fmt.Errorf("GetYAMLContents repoowner:%s reponame:%s ref:%s triggeryaml:%s ;content is empty", item.MainRepo.RepoOwner, item.MainRepo.RepoName, item.YamlPath, branref)
	}
	log.Infof("zadig-Trigger Yaml info:%s", zadigTriggerYamls[0])
	err = yaml.Unmarshal([]byte(zadigTriggerYamls[0]), triggerYaml)
	if err != nil {
		return fmt.Errorf("yaml.Unmarshal err:%s", err)
	}
	triggerYamlByt, err := json.Marshal(triggerYaml)
	if err != nil {
		return err
	}
	log.Infof("triggerYaml struct info:%s", string(triggerYamlByt))
	err = checkTriggerYamlParams(triggerYaml)
	if err != nil {
		return fmt.Errorf("checkTriggerYamlParams yamlPath:%s err:%s", item.YamlPath, err)
	}
	deployed := existStage(StageDeploy, triggerYaml)
	if svcType == setting.BasicFacilityCVM {
		deployed = true
	}
	workFlowArgs.WorkflowName = workflow.Name
	workFlowArgs.ProductTmplName = workflow.ProductTmplName
	if triggerYaml.CacheSet != nil {
		workFlowArgs.IgnoreCache = triggerYaml.CacheSet.IgnoreCache
		workFlowArgs.ResetCache = triggerYaml.CacheSet.ResetCache
	}
	if triggerYaml.Deploy != nil {
		workFlowArgs.Namespace = strings.Join(triggerYaml.Deploy.Envsname, ",")
		workFlowArgs.BaseNamespace = triggerYaml.Deploy.BaseNamespace
		workFlowArgs.EnvRecyclePolicy = string(triggerYaml.Deploy.EnvRecyclePolicy)
		workFlowArgs.EnvUpdatePolicy = string(triggerYaml.Deploy.Strategy)
	}
	item.MainRepo.Events = triggerYaml.Rules.Events
	if triggerYaml.Rules.Strategy != nil {
		item.AutoCancel = triggerYaml.Rules.Strategy.AutoCancel
	}
	//test
	tests := make([]*commonmodels.TestArgs, 0)
	for _, test := range triggerYaml.Test {
		moduleTest, err := commonrepo.NewTestingColl().Find(test.Name, workflow.ProductTmplName)
		if err != nil {
			log.Errorf("fail to find test TestModuleName:%s, workflowname:%s,productTmplName:%s,error:%v", test.Name, workflow.Name, workflow.ProductTmplName, err)
			if commonrepo.IsErrNoDocuments(err) {
				continue
			}
			return fmt.Errorf("fail to find test TestModuleName:%s, workflowname:%s,productTmplName:%s,error:%s", test.Name, workflow.Name, workflow.ProductTmplName, err)
		}
		envs := make([]*commonmodels.KeyVal, 0)
		for _, env := range test.Variables {
			envElem := &commonmodels.KeyVal{
				Key:   env.Name,
				Value: env.Value,
			}
			envs = append(envs, envElem)
		}
		testArg := &commonmodels.TestArgs{
			TestModuleName: test.Name,
			Envs:           envs,
		}
		if test.Repo.Strategy == TestRepoStrategyCurrentRepo {
			for _, repo := range moduleTest.Repos {
				if repo.RepoName == item.MainRepo.RepoName && repo.RepoOwner == item.MainRepo.RepoOwner {
					repo.Branch = branref
					repo.PR = prId
				}
			}
		}
		testArg.Builds = moduleTest.Repos
		tests = append(tests, testArg)
	}
	workFlowArgs.Tests = tests
	testsRepo, err := json.Marshal(workFlowArgs.Tests)
	if err != nil {
		log.Errorf("json.Marshal workflowname:%s,productTmplName:%s,error:%s", workflow.Name, workflow.ProductTmplName, err)
		return fmt.Errorf("json.Marshal workflowname:%s,productTmplName:%s,error:%s", workflow.Name, workflow.ProductTmplName, err)
	}
	log.Infof("moduleTests workflowname:%s,productTmplName:%s,info:%s", workflow.Name, workflow.ProductTmplName, string(testsRepo))
	//target
	targets := make([]*commonmodels.TargetArgs, 0)
	for _, svr := range triggerYaml.Rules.MatchFolders.MatchFoldersTree {
		targetElem := &commonmodels.TargetArgs{
			Name:        svr.ServiceModule,
			ProductName: workflow.ProductTmplName,
			ServiceName: svr.Name,
			ServiceType: svcType,
		}
		opt := &commonrepo.BuildFindOption{
			ServiceName: svr.Name,
			ProductName: workflow.ProductTmplName,
			Targets:     []string{svr.ServiceModule},
		}
		resp, err := commonrepo.NewBuildColl().Find(opt)
		if err != nil {
			log.Errorf("[Build.Find] serviceName: %s productName:%s serviceModule:%s error: %s", svr.Name, workflow.ProductTmplName, svr.ServiceModule, err)
			if commonrepo.IsErrNoDocuments(err) {
				continue
			}
			return fmt.Errorf("[Build.Find] serviceName: %s productName:%s serviceModule:%s error: %s", svr.Name, workflow.ProductTmplName, svr.ServiceModule, err)
		}

		repos := commonservice.FindReposByTarget(targetElem.ProductName, targetElem.ServiceName, targetElem.Name, resp)
		for _, repo := range repos {
			if repo.RepoName == item.MainRepo.RepoName && repo.RepoOwner == item.MainRepo.RepoOwner {
				repo.Branch = branref
				repo.PR = prId
			}
		}
		targetElem.Build = &commonmodels.BuildArgs{Repos: repos}

		targetElem.Deploy = make([]commonmodels.DeployEnv, 0)
		if deployed {
			targetElem.Deploy = append(targetElem.Deploy, commonmodels.DeployEnv{Env: svr.Name + "/" + svr.ServiceModule, Type: targetElem.ServiceType})
		}
		var envs []*commonmodels.KeyVal
		for _, bsvr := range triggerYaml.Build {
			if bsvr.Name == svr.Name && bsvr.ServiceModule == svr.ServiceModule {
				for _, env := range bsvr.Variables {
					envElem := &commonmodels.KeyVal{
						Key:   env.Name,
						Value: env.Value,
					}
					envs = append(envs, envElem)
				}
			}
		}
		targetElem.Envs = envs
		targets = append(targets, targetElem)
	}
	workFlowArgs.Target = targets
	return nil
}

func findChangedFilesOfMergeRequest(event *gitlab.MergeEvent, codehostID int) ([]string, error) {
	detail, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return nil, fmt.Errorf("failed to find codehost %d: %v", codehostID, err)
	}

	client, err := gitlabtool.NewClient(detail.ID, detail.Address, detail.AccessToken, config.ProxyHTTPSAddr(), detail.EnableProxy, detail.DisableSSL)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
	}

	return client.ListChangedFiles(event)
}

func WaitEnvCreate(timeoutSeconds int, envName string, workflowArgs *commonmodels.WorkflowTaskArgs, log *zap.SugaredLogger) error {
	timeout := false
	go func() {
		<-time.After(time.Duration(timeoutSeconds) * time.Second)
		timeout = true
	}()

	for {
		if timeout {
			return fmt.Errorf("WaitEnvCreate %s wait create envName:%s timeout in %d seconds", workflowArgs.ProductTmplName, envName, timeoutSeconds)
		}

		productResp, err := environmentservice.GetProduct(setting.SystemUser, envName, workflowArgs.ProductTmplName, log)
		if err != nil {
			log.Errorf("WaitEnvCreate Product find err:%v ", err)
			time.Sleep(time.Second)
			continue
		}
		prTaskInfo := &commonmodels.PrTaskInfo{
			ProductName:      workflowArgs.ProductTmplName,
			EnvStatus:        productResp.Status,
			EnvName:          envName,
			EnvRecyclePolicy: workflowArgs.EnvRecyclePolicy,
		}

		if err = scmnotify.NewService().UpdateEnvAndTaskWebhookComment(workflowArgs, prTaskInfo, log); err != nil {
			log.Errorf("WaitEnvCreate create product UpdateEnvAndTaskWebhookComment err:%v", err)
		}

		if productResp.Status == setting.PodRunning || productResp.Status == setting.PodUnstable || productResp.Status == setting.ClusterUnknown {
			break
		} else {
			time.Sleep(time.Second)
		}
	}
	return nil
}

func WaitEnvDelete(timeoutSeconds int, envName string, workflowArgs *commonmodels.WorkflowTaskArgs, log *zap.SugaredLogger) error {
	timeout := false
	go func() {
		<-time.After(time.Duration(timeoutSeconds) * time.Second)
		timeout = true
	}()
	for {
		if timeout {
			return fmt.Errorf("WaitEnvDelete %s wait delete envName:%s timeout in %d seconds", workflowArgs.ProductTmplName, envName, timeoutSeconds)
		}

		prTaskInfo := &commonmodels.PrTaskInfo{
			ProductName:      workflowArgs.ProductTmplName,
			EnvName:          envName,
			EnvRecyclePolicy: workflowArgs.EnvRecyclePolicy,
		}
		productResp, err := environmentservice.GetProduct(setting.SystemUser, envName, workflowArgs.ProductTmplName, log)
		if err != nil {
			log.Errorf("WaitEnvDelete GetProduct err:%v ", err)
			prTaskInfo.EnvStatus = "Completed"
			if err = scmnotify.NewService().UpdateEnvAndTaskWebhookComment(workflowArgs, prTaskInfo, log); err != nil {
				log.Errorf("WaitEnvDelete delete product UpdateEnvAndTaskWebhookComment1 err:%v", err)
			}
			break
		}
		prTaskInfo.EnvStatus = productResp.Status
		if err = scmnotify.NewService().UpdateEnvAndTaskWebhookComment(workflowArgs, prTaskInfo, log); err != nil {
			log.Errorf("WaitEnvDelete delete product UpdateEnvAndTaskWebhookComment2 err:%v", err)
		}
		time.Sleep(time.Second)
	}
	return nil
}
