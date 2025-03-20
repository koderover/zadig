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
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	githubtool "github.com/koderover/zadig/v2/pkg/tool/git/github"
	gitlabtool "github.com/koderover/zadig/v2/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

func reloadServiceTmplFromGit(svc *commonmodels.Service, log *zap.SugaredLogger) error {
	_, err := service.CreateOrUpdateHelmServiceFromGitRepo(svc.ProductName, &service.HelmServiceCreationArgs{
		HelmLoadSource: service.HelmLoadSource{
			Source: service.LoadFromRepo,
		},
		CreatedBy: svc.CreateBy,
		CreateFrom: &service.CreateFromRepo{
			CodehostID: svc.CodehostID,
			Owner:      svc.RepoOwner,
			Namespace:  svc.GetRepoNamespace(),
			Repo:       svc.RepoName,
			Branch:     svc.BranchName,
			Paths:      []string{svc.LoadPath},
		},
		Production: svc.Production,
	}, true, log)
	return err
}

// fillServiceTmpl 更新服务模板参数
func fillServiceTmpl(userName string, args *commonmodels.Service, log *zap.SugaredLogger) error {
	if args == nil {
		return errors.New("service template arg is null")
	}
	if len(args.ServiceName) == 0 {
		return errors.New("service name is empty")
	}
	if !config.ServiceNameRegex.MatchString(args.ServiceName) {
		return fmt.Errorf("service name must match %s", config.ServiceNameRegexString)
	}
	if args.Type == setting.K8SDeployType {
		if args.Containers == nil {
			args.Containers = make([]*commonmodels.Container, 0)
		}
		// 配置来源为Gitlab，需要从Gitlab同步配置，并设置KubeYamls.
		if args.Source == setting.SourceFromGitlab {
			// Set args.Commit
			if err := syncLatestCommit(args); err != nil {
				log.Errorf("Sync change log from gitlab failed, error: %v", err)
				return err
			}
			// 从Gitlab同步args指定的Commit下，指定目录中对应的Yaml文件
			// Set args.Yaml & args.KubeYamls
			if err := syncContentFromGitlab(userName, args); err != nil {
				log.Errorf("Sync content from gitlab failed, error: %v", err)
				return err
			}
		} else if args.Source == setting.SourceFromGithub {
			err := syncContentFromGithub(args, log)
			if err != nil {
				log.Errorf("Sync content from github failed, error: %v", err)
				return err
			}
		} else {
			// 拆分 all-in-one yaml文件
			// 替换分隔符
			args.Yaml = util.ReplaceWrapLine(args.Yaml)
			// 分隔符为\n---\n
			args.KubeYamls = util.SplitYaml(args.Yaml)
		}

		_, err := service.CreateServiceTemplate(userName, args, true, args.Production, log)
		if err != nil {
			log.Infof("failed to create svc tempalte, err: %s", err)
			return err
		}
	} else if args.Type == setting.HelmDeployType {
		if args.Source == setting.SourceFromGitlab {
			// Set args.Commit
			if err := syncLatestCommit(args); err != nil {
				log.Errorf("Sync change log from gitlab failed, error: %s", err)
				return err
			}

			if err := reloadServiceTmplFromGit(args, log); err != nil {
				log.Errorf("Sync content from gitlab failed, error: %s", err)
				return err
			}
		} else if args.Source == setting.SourceFromGithub {
			if err := reloadServiceTmplFromGit(args, log); err != nil {
				log.Errorf("Sync content from github failed, error: %s", err)
				return err
			}
		}
	}

	return nil
}

func syncLatestCommit(service *commonmodels.Service) error {
	if service.SrcPath == "" {
		return fmt.Errorf("url不能是空的")
	}

	_, owner, repo, branch, path, _, err := GetOwnerRepoBranchPath(service.SrcPath)
	if err != nil {
		return fmt.Errorf("url 必须包含 owner/repo/tree/branch/path，具体请参考 Placeholder 提示")
	}

	client, err := getGitlabClientByCodehostId(service.CodehostID)
	if err != nil {
		return err
	}

	if len(service.BranchName) > 0 {
		branch = service.BranchName
	}
	if len(service.LoadPath) > 0 {
		path = service.LoadPath
	}
	if len(service.GetRepoNamespace()) > 0 {
		owner = service.GetRepoNamespace()
	}
	if len(service.RepoName) > 0 {
		repo = service.RepoName
	}

	commit, err := GitlabGetLatestCommit(client, owner, repo, branch, path)
	if err != nil {
		return err
	}

	if commit != nil {
		service.Commit = &commonmodels.Commit{
			SHA:     commit.ID,
			Message: commit.Message,
		}
	}

	return nil
}

func getGitlabClientByCodehostId(codehostId int) (*gitlabtool.Client, error) {
	codehost, err := systemconfig.New().GetCodeHost(codehostId)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc(fmt.Sprintf("failed to get codehost:%d, err: %s", codehost, err))
	}
	client, err := gitlabtool.NewClient(codehost.ID, codehost.Address, codehost.AccessToken, config.ProxyHTTPSAddr(), codehost.EnableProxy)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
	}

	return client, nil
}

func GitlabGetLatestCommit(client *gitlabtool.Client, owner, repo string, ref, path string) (*gitlab.Commit, error) {
	commit, err := client.GetLatestRepositoryCommit(owner, repo, path, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get lastest commit with project %s/%s, ref: %s, path:%s, error: %v",
			owner, repo, ref, path, err)
	}
	return commit, nil
}

// GitlabGetRawFiles ...
// projectID: identity of project, can be retrieved from s.GitlabGetProjectID(owner, repo)
// ref: branch (e.g. master) or commit (commit id) or tag
// path: file path of raw files, only retrieve leaf node(blob type == file), no recursive get
func GitlabGetRawFiles(client *gitlabtool.Client, owner, repo, ref, path, pathType string) (files []string, err error) {
	files = make([]string, 0)
	var errs *multierror.Error
	if pathType == "tree" {
		nodes, err := client.ListTree(owner, repo, path, ref, false, nil)
		if err != nil {
			return files, err
		}
		for _, node := range nodes {
			// if node type is "tree", it is a directory, skip it for now
			if node.Type == "tree" {
				continue
			}
			fileName := strings.ToLower(node.Name)
			if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
				continue
			}
			// if node type is "blob", it is a file
			// Path is filepath of a node
			content, err := client.GetRawFile(owner, repo, ref, node.Path)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
			contentStr := string(content)
			contentStr = util.ReplaceWrapLine(contentStr)
			files = append(files, contentStr)
		}
		return files, errs.ErrorOrNil()
	}
	content, err := client.GetFileContent(owner, repo, path, ref)
	if err != nil {
		return files, err
	}
	files = append(files, string(content))
	return files, errs.ErrorOrNil()
}

// syncContentFromGitlab ...
// sync content with commit, args.Commit should not be nil
func syncContentFromGitlab(userName string, args *commonmodels.Service) error {
	if args.Commit == nil {
		return nil
	}

	var owner, repo, branch, path string = args.GetRepoNamespace(), args.RepoName, args.BranchName, args.LoadPath
	var pathType = "tree"
	if strings.Contains(args.SrcPath, "blob") {
		pathType = "blob"
	}

	client, err := getGitlabClientByCodehostId(args.CodehostID)
	if err != nil {
		return err
	}

	files, err := GitlabGetRawFiles(client, owner, repo, branch, path, pathType)
	if err != nil {
		return err
	}
	if userName != setting.WebhookTaskCreator {
		if len(files) == 0 {
			return fmt.Errorf("没有检索到yml,yaml类型文件，请检查目录是否正确")
		}
	}
	// KubeYamls field is dynamicly synced.
	// 根据gitlab sync的内容来设置args.KubeYamls
	args.KubeYamls = files
	// 拼装并设置args.Yaml
	args.Yaml = util.JoinYamls(files)
	return nil
}

func syncContentFromGithub(args *commonmodels.Service, log *zap.SugaredLogger) error {
	// 根据pipeline中的filepath获取文件内容
	var owner, repo, branch, path = args.GetRepoNamespace(), args.RepoName, args.BranchName, args.LoadPath

	ch, err := systemconfig.New().GetCodeHost(args.CodehostID)
	if err != nil {
		log.Errorf("failed to getCodeHostInfo, srcPath:%s, err:%s", args.SrcPath, err)
		return err
	}

	gc := githubtool.NewClient(&githubtool.Config{AccessToken: ch.AccessToken, Proxy: config.ProxyHTTPSAddr()})
	fileContent, directoryContent, err := gc.GetContents(context.TODO(), owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		return err
	}
	if fileContent != nil {
		svcContent, _ := fileContent.GetContent()
		splitYaml := util.SplitYaml(svcContent)
		args.KubeYamls = splitYaml
		args.Yaml = svcContent
	} else {
		var files []string
		for _, f := range directoryContent {
			// 排除目录
			if *f.Type != "file" {
				continue
			}
			fileName := strings.ToLower(*f.Path)
			if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
				continue
			}

			file, err := syncSingleFileFromGithub(owner, repo, branch, *f.Path, ch.AccessToken)
			if err != nil {
				log.Errorf("syncSingleFileFromGithub failed, path: %s, err: %v", *f.Path, err)
				continue
			}
			files = append(files, file)
		}

		args.KubeYamls = files
		args.Yaml = util.JoinYamls(files)
	}

	return nil
}

func syncSingleFileFromGithub(owner, repo, branch, path, token string) (string, error) {
	gc := githubtool.NewClient(&githubtool.Config{AccessToken: token, Proxy: config.ProxyHTTPSAddr()})
	fileContent, _, err := gc.GetContents(context.TODO(), owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if fileContent != nil {
		return fileContent.GetContent()
	}

	return "", err
}

type MatchFolders []string

// ContainsFile  "/" 代表全部文件
func ContainsFile(h *commonmodels.GitHook, file string) bool {
	return MatchFolders(h.MatchFolders).ContainsFile(file)
}

func (m MatchFolders) ContainsFile(file string) bool {
	var excludes []string
	var matches []string

	for _, match := range m {
		if strings.HasPrefix(match, "!") {
			excludes = append(excludes, match)
		} else {
			matches = append(matches, match)
		}
	}

	for _, match := range matches {
		if match == "/" || strings.HasPrefix(file, match) {
			// 以!开头的目录或者后缀名为不运行pipeline的过滤条件
			for _, exclude := range excludes {
				// 如果！后面不跟任何目录或者文件，忽略
				if len(exclude) <= 2 {
					return false
				}
				eCheck := exclude[1:]
				if eCheck == "/" || path.Ext(file) == eCheck || strings.HasPrefix(file, eCheck) || strings.HasSuffix(file, eCheck) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func MatchChanges(m *commonmodels.MainHookRepo, files []string) bool {
	// if it is an empty commit, allow triggering workflow tasks
	if len(files) == 0 {
		return true
	}
	mf := MatchFolders(m.MatchFolders)
	for _, file := range files {
		if matches := mf.ContainsFile(file); matches {
			return true
		}
	}
	return false
}

func ConvertScanningHookToMainHookRepo(hook *commonmodels.ScanningHook) *commonmodels.MainHookRepo {
	return &commonmodels.MainHookRepo{
		Source:       hook.Source,
		RepoOwner:    hook.RepoOwner,
		RepoName:     hook.RepoName,
		Branch:       hook.Branch,
		MatchFolders: hook.MatchFolders,
		CodehostID:   hook.CodehostID,
		Events:       hook.Events,
		IsRegular:    hook.IsRegular,
	}
}

func EventConfigured(m *commonmodels.MainHookRepo, event config.HookEventType) bool {
	for _, ev := range m.Events {
		if ev == event {
			return true
		}
	}

	return false
}

func ServicesMatchChangesFiles(mf *MatchFoldersElem, files []string) []BuildServices {
	resMactchSvr := []BuildServices{}
	var wg sync.WaitGroup
	var mutex sync.Mutex
	for _, mftreeElem := range mf.MatchFoldersTree {
		wg.Add(1)
		go func(mftree *MatchFoldersTree) {
			defer wg.Done()
			mf := MatchFolders(mftree.FileTree)
			for _, file := range files {
				if matches := mf.ContainsFile(file); matches {
					mutex.Lock()
					resMactchSvr = append(resMactchSvr, BuildServices{Name: mftree.Name, ServiceModule: mftree.ServiceModule})
					mutex.Unlock()
					break
				}
			}
		}(mftreeElem)
	}
	wg.Wait()
	return resMactchSvr
}

func getServiceTypeByProject(productName string) (string, error) {
	projectInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		return "", err
	}
	projectType := setting.K8SDeployType
	if projectInfo == nil || projectInfo.ProductFeature == nil {
		return projectType, nil
	} else if projectInfo.ProductFeature.BasicFacility == setting.BasicFacilityK8S {
		if projectInfo.ProductFeature.DeployType == setting.HelmDeployType {
			return setting.HelmDeployType, nil
		}
		return projectType, nil
	} else if projectInfo.ProductFeature.BasicFacility == setting.BasicFacilityCVM {
		return setting.PMDeployType, nil
	}
	return projectType, nil
}

func existStage(expectStage Stage, triggerYaml *TriggerYaml) bool {
	for _, stage := range triggerYaml.Stages {
		if stage == expectStage {
			return true
		}
	}
	return false
}

func checkTriggerYamlParams(triggerYaml *TriggerYaml) error {
	//check stages
	for _, stage := range triggerYaml.Stages {
		if stage != StageBuild && stage != StageDeploy && stage != StageTest {
			return fmt.Errorf("stages must %s or %s or %s", StageBuild, StageDeploy, StageTest)
		}
	}

	//check build
	if len(triggerYaml.Build) == 0 {
		return errors.New("build is empty")
	}
	for _, bd := range triggerYaml.Build {
		if bd.Name == "" || bd.ServiceModule == "" {
			return errors.New("build.name or build.service_module is empty")
		}
	}

	//check deploy
	if triggerYaml.Deploy == nil {
		return errors.New("deploy is empty")
	}
	if len(triggerYaml.Deploy.Envsname) == 0 {
		return errors.New("deploy.envs_name is empty")
	}
	if triggerYaml.Deploy.Strategy != DeployStrategySingle && triggerYaml.Deploy.Strategy != DeployStrategyBase && triggerYaml.Deploy.Strategy != DeployStrategyDynamic {
		return fmt.Errorf("deploy.strategy must %s or %s or %s", DeployStrategySingle, DeployStrategyDynamic, DeployStrategyBase)
	}
	if triggerYaml.Deploy.Strategy == DeployStrategyBase {
		if triggerYaml.Deploy.BaseNamespace == "" {
			return errors.New("deploy.base_env is empty")
		}
		if triggerYaml.Deploy.EnvRecyclePolicy != EnvRecyclePolicySuccess && triggerYaml.Deploy.EnvRecyclePolicy != EnvRecyclePolicyAlways && triggerYaml.Deploy.EnvRecyclePolicy != EnvRecyclePolicyNever {
			return errors.New("deploy.env_recycle_policy must success/always/never")
		}
	} else {
		if triggerYaml.Deploy.BaseNamespace != "" {
			return errors.New("deploy.base_env must empty")
		}
	}

	//check test
	if existStage(StageTest, triggerYaml) {
		if len(triggerYaml.Test) == 0 {
			return errors.New("test is empty")
		}
		for _, tt := range triggerYaml.Test {
			if tt.Repo == nil {
				return errors.New("test.repo.strategy must default/currentRepo")
			}
			if tt.Repo.Strategy != TestRepoStrategyDefault && tt.Repo.Strategy != TestRepoStrategyCurrentRepo {
				return errors.New("test.repo.strategy must default/currentRepo")
			}
		}
	}

	//check rule
	if triggerYaml.Rules == nil {
		return errors.New("rules must exist")
	}
	if len(triggerYaml.Rules.Branchs) == 0 {
		return errors.New("rules.branchs must exist")
	}
	for _, ev := range triggerYaml.Rules.Events {
		if ev != "pull_request" && ev != "push" && ev != "tag" {
			return errors.New("rules.event must be pull_request or push or tag")
		}
	}
	if triggerYaml.Rules.MatchFolders == nil {
		return errors.New("rules.match_folders must exist")
	}
	for _, mf := range triggerYaml.Rules.MatchFolders.MatchFoldersTree {
		if mf.Name == "" || mf.ServiceModule == "" {
			return errors.New("match_folders.match_folders_tree.name or match_folders.match_folders_tree.service_module is empty")
		}
		if len(mf.FileTree) == 0 {
			return errors.New("match_folders.match_folders_tree.file_tree is empty")
		}
	}

	return nil
}

func getServiceSrcPath(service *commonmodels.Service) (string, error) {
	if service.LoadPath != "" {
		return service.LoadPath, nil
	}
	_, _, _, _, p, _, err := GetOwnerRepoBranchPath(service.SrcPath)
	return p, err
}

func checkRepoNamespaceMatch(hookRepo *commonmodels.MainHookRepo, pathWithNamespace string) bool {
	return (hookRepo.GetRepoNamespace() + "/" + hookRepo.RepoName) == pathWithNamespace
}

// check if sub path is a part of parent path
// eg: parent: k1/k2   sub: k1/k2/k3  return true
// parent k1/k2-2  sub: k1/k2/k3 return false
func subElem(parent, sub string) bool {
	up := ".." + string(os.PathSeparator)
	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		log.Errorf("failed to check path is relative, parent: %s, sub: %s", parent, sub)
		return false
	}
	if !strings.HasPrefix(rel, up) && rel != ".." {
		return true
	}
	return false
}

func checkBranchMatch(service *commonmodels.Service, production bool, ref string, log *zap.SugaredLogger) bool {
	createFrom := &commonmodels.CreateFromRepo{}
	err := commonmodels.IToi(service.CreateFrom, createFrom)
	if err != nil {
		log.Errorf("cannot convert service.CreateFrom to commonmodels.CreateFromRepo, service name: %s, production: %v, err: %v", service.ServiceName, production, err)
		return false
	}
	if createFrom.GitRepoConfig == nil {
		log.Errorf("service %s, production %v, git repo config is nil", service.ServiceName, production)
		return false
	}
	if !strings.HasPrefix(ref, "refs/heads/") {
		log.Errorf("ref %s is not a branch", ref)
		return false
	}

	ref = strings.TrimPrefix(ref, "refs/heads/")
	if createFrom.GitRepoConfig.Branch != ref {
		// log.Errorf("service %s, production %v, branch %s is not the same as ref %s", service.ServiceName, production, createFrom.GitRepoConfig.Branch, ref)
		return false
	}
	return true
}
