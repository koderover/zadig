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

package service

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	githubservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	gitlabservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
)

type LoadServiceReq struct {
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	Visibility  string `json:"visibility"`
	LoadFromDir bool   `json:"is_dir"`
	LoadPath    string `json:"path"`
}

func PreloadServiceFromCodeHost(codehostID int, repoOwner, repoName, branchName, remoteName, path string, isDir bool, log *zap.SugaredLogger) ([]string, error) {
	var ret []string
	detail, err := codehost.GetCodeHostInfoByID(codehostID)
	if err != nil {
		log.Errorf("Failed to load codehost for preload service list, the error is: %+v", err)
		return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
	}
	switch detail.Type {
	case setting.SourceFromGithub:
		ret, err = preloadGithubService(detail, repoOwner, repoName, branchName, path, isDir)
	case setting.SourceFromGitlab:
		ret, err = preloadGitlabService(detail, repoOwner, repoName, branchName, path, isDir)
	case setting.SourceFromGerrit:
		ret, err = preloadGerritService(detail, repoName, branchName, remoteName, path, isDir)
	case setting.SourceFromCodeHub:
		//ret, err = preloadGerritService(detail, repoName, branchName, remoteName, path, isDir)
	default:
		return nil, e.ErrPreloadServiceTemplate.AddDesc("Not supported code source")
	}

	return ret, err
}

// LoadServiceFromCodeHost 根据提供的codehost信息加载服务
func LoadServiceFromCodeHost(username string, codehostID int, repoOwner, repoName, branchName, remoteName string, args *LoadServiceReq, log *zap.SugaredLogger) error {
	detail, err := codehost.GetCodehostDetail(codehostID)
	if err != nil {
		log.Errorf("Failed to load codehost for preload service list, the error is: %+v", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	switch detail.Source {
	case setting.SourceFromGithub, setting.SourceFromGitlab:
		return loadService(username, detail, repoOwner, repoName, branchName, args, log)
	case setting.SourceFromGerrit:
		return loadGerritService(username, detail, repoOwner, repoName, branchName, remoteName, args, log)
	default:
		return e.ErrLoadServiceTemplate.AddDesc("unsupported code source")
	}
}

type yamlLoader interface {
	GetYAMLContents(owner, repo, branch, path string, isDir, split bool) ([]string, error)
	GetLatestRepositoryCommit(owner, repo, branch, path string) (*git.RepositoryCommit, error)
}

func getLoader(ch *codehost.Detail) (yamlLoader, error) {
	switch ch.Source {
	case setting.SourceFromGithub:
		return githubservice.NewClient(ch.OauthToken, config.ProxyHTTPSAddr()), nil
	case setting.SourceFromGitlab:
		return gitlabservice.NewClient(ch.Address, ch.OauthToken)
	default:
		// should not have happened here
		log.DPanicf("invalid source: %s", ch.Source)
		return nil, fmt.Errorf("invalid source: %s", ch.Source)
	}
}

func loadService(username string, detail *codehost.Detail, owner, repo, branch string, args *LoadServiceReq, logger *zap.SugaredLogger) error {
	logger.Infof("Loading service from %s with owner %s, repo %s, branch %s and path %s", detail.Source, owner, repo, branch, args.LoadPath)

	loader, err := getLoader(detail)
	if err != nil {
		logger.Errorf("Failed to create loader client, error: %s", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	yamls, err := loader.GetYAMLContents(owner, repo, branch, args.LoadPath, args.LoadFromDir, true)
	if err != nil {
		logger.Errorf("Failed to get yamls under path %s, error: %s", args.LoadPath, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}

	commit, err := loader.GetLatestRepositoryCommit(owner, repo, branch, args.LoadPath)
	if err != nil {
		logger.Errorf("Failed to get latest commit under path %s, error: %s", args.LoadPath, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}

	pathType := "tree"
	if !args.LoadFromDir {
		pathType = "blob"
	}
	srcPath := fmt.Sprintf("%s/%s/%s/%s/%s/%s", detail.Address, owner, repo, pathType, branch, args.LoadPath)
	createSvcArgs := &models.Service{
		CodehostID:  detail.ID,
		RepoName:    repo,
		RepoOwner:   owner,
		BranchName:  branch,
		LoadPath:    args.LoadPath,
		LoadFromDir: args.LoadFromDir,
		KubeYamls:   yamls,
		SrcPath:     srcPath,
		CreateBy:    username,
		ServiceName: getFileName(args.LoadPath),
		Type:        args.Type,
		ProductName: args.ProductName,
		Source:      detail.Source,
		Yaml:        util.CombineManifests(yamls),
		Commit:      &models.Commit{SHA: commit.SHA, Message: commit.Message},
		Visibility:  args.Visibility,
	}
	_, err = CreateServiceTemplate(username, createSvcArgs, logger)
	if err != nil {
		logger.Errorf("Failed to create service template, error: %s", err)
		_, messageMap := e.ErrorMessage(err)
		if description, ok := messageMap["description"]; ok {
			return e.ErrLoadServiceTemplate.AddDesc(description.(string))
		}
		return e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
	}

	return nil
}

// ValidateServiceUpdate 根据服务名和提供的加载信息确认是否可以更新服务加载地址
func ValidateServiceUpdate(codehostID int, serviceName, repoOwner, repoName, branchName, remoteName, path string, isDir bool, log *zap.SugaredLogger) error {
	detail, err := codehost.GetCodeHostInfoByID(codehostID)
	if err != nil {
		log.Errorf("Failed to load codehost for validate service update, the error is: %+v", err)
		return e.ErrValidateServiceUpdate.AddDesc(err.Error())
	}
	switch detail.Type {
	case setting.SourceFromGithub:
		return validateServiceUpdateGithub(detail, serviceName, repoOwner, repoName, branchName, path, isDir)
	case setting.SourceFromGitlab:
		return validateServiceUpdateGitlab(detail, serviceName, repoOwner, repoName, branchName, path, isDir)
	case setting.SourceFromGerrit:
		return validateServiceUpdateGerrit(detail, serviceName, repoName, branchName, remoteName, path, isDir)
	default:
		return e.ErrValidateServiceUpdate.AddDesc("Not supported code source")
	}
}

// 根据repo信息获取github可以加载的服务列表
func preloadGithubService(detail *poetry.CodeHost, repoOwner, repoName, branchName, path string, isDir bool) ([]string, error) {
	ret := make([]string, 0)

	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: detail.AccessToken},
	)
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(tokenClient)

	if !isDir {
		fileContent, _, _, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, path, &github.RepositoryContentGetOptions{Ref: branchName})
		if err != nil {
			log.Errorf("Failed to get dir content from github with path: %s, the error is: %+v", path, err)
			return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
		if !isYaml(fileContent.GetName()) {
			log.Errorf("trying to preload a non-yaml file, ending ...")
			return ret, e.ErrPreloadServiceTemplate.AddDesc("Non-yaml service loading is not supported")
		}
		serviceName := getFileName(fileContent.GetName())
		ret = append(ret, serviceName)
	} else {
		_, dirContent, _, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, path, &github.RepositoryContentGetOptions{Ref: branchName})
		if err != nil {
			log.Errorf("Failed to get dir content from github with path: %s, the error is: %+v", path, err)
			return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
		if isValidGithubServiceDir(dirContent) {
			svcName := path
			if path == "" {
				svcName = repoName
			}
			pathList := strings.Split(svcName, "/")
			folderName := pathList[len(pathList)-1]
			ret = append(ret, folderName)
			return ret, nil
		}
		isGrandparent := false
		for _, entry := range dirContent {
			if entry.GetType() == "dir" {
				_, subtree, _, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, entry.GetPath(), &github.RepositoryContentGetOptions{Ref: branchName})
				if err != nil {
					log.Errorf("Failed to get dir content from github with path: %s, the error is: %+v", entry.GetPath(), err)
					return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
				}
				if isValidGithubServiceDir(subtree) {
					isGrandparent = true
					ret = append(ret, entry.GetName())
				}
			}
		}
		if !isGrandparent {
			log.Errorf("invalid folder selected since no yaml is presented for path: %s", path)
			return ret, e.ErrPreloadServiceTemplate.AddDesc("所选路径下没有yaml，请重新选择")
		}
	}

	return ret, nil
}

// 根据repo信息获取gitlab可以加载的服务列表
func preloadGitlabService(detail *poetry.CodeHost, repoOwner, repoName, branchName, path string, isDir bool) ([]string, error) {
	ret := make([]string, 0)

	repoInfo := fmt.Sprintf("%s/%s", repoOwner, repoName)

	gitlabClient, err := gitlab.NewOAuthClient(detail.AccessToken, gitlab.WithBaseURL(detail.Address))
	if err != nil {
		log.Errorf("failed to prepare gitlab client, the error is:%+v", err)
		return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
	}

	// 非文件夹情况下直接获取文件信息
	if !isDir {
		if !isYaml(path) {
			return ret, e.ErrPreloadServiceTemplate.AddDesc("File is not of type yaml or yml, select again")
		}
		fileInfo, _, err := gitlabClient.RepositoryFiles.GetFile(repoInfo, path, &gitlab.GetFileOptions{Ref: gitlab.String(branchName)})
		if err != nil {
			log.Errorf("Failed to get file info from gitlab with path: %s, the error is %+v", path, err)
			return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
		extension := filepath.Ext(fileInfo.FileName)
		fileName := fileInfo.FileName[0 : len(fileInfo.FileName)-len(extension)]
		ret = append(ret, fileName)
		return ret, nil
	}

	opt := &gitlab.ListTreeOptions{
		Path:      gitlab.String(path),
		Ref:       gitlab.String(branchName),
		Recursive: gitlab.Bool(false),
	}
	treeInfo, _, err := gitlabClient.Repositories.ListTree(repoInfo, opt)
	if err != nil {
		log.Errorf("Failed to get dir content from gitlab with path: %s, the error is: %+v", path, err)
		return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
	}
	if isValidGitlabServiceDir(treeInfo) {
		svcName := path
		if path == "" {
			svcName = repoInfo
		}
		pathList := strings.Split(svcName, "/")
		folderName := pathList[len(pathList)-1]
		ret = append(ret, folderName)
		return ret, nil
	}
	isGrandparent := false
	for _, entry := range treeInfo {
		if entry.Type == "tree" {
			subtreeOpt := &gitlab.ListTreeOptions{
				Path:      gitlab.String(entry.Path),
				Ref:       gitlab.String(branchName),
				Recursive: gitlab.Bool(false),
			}
			subtreeInfo, _, err := gitlabClient.Repositories.ListTree(repoInfo, subtreeOpt)
			if err != nil {
				log.Errorf("Failed to get dir content from gitlab with path: %s, the error is: %+v", path, err)
				return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
			}
			if isValidGitlabServiceDir(subtreeInfo) {
				isGrandparent = true
				ret = append(ret, entry.Name)
			}
		}
	}
	if !isGrandparent {
		log.Errorf("invalid folder selected since no yaml is presented in path: %s", path)
		return ret, e.ErrPreloadServiceTemplate.AddDesc("所选路径下没有yaml，请重新选择")
	}

	return ret, nil
}

// 根据repo信息获取gerrit可以加载的服务列表
func preloadGerritService(detail *poetry.CodeHost, repoName, branchName, remoteName, loadPath string, isDir bool) ([]string, error) {
	ret := make([]string, 0)

	base := path.Join(config.S3StoragePath(), repoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		chDetail := &codehost.Detail{
			ID:         detail.ID,
			Name:       "",
			Address:    detail.Address,
			Owner:      detail.Namespace,
			Source:     detail.Type,
			OauthToken: detail.AccessToken,
		}
		err = command.RunGitCmds(chDetail, setting.GerritDefaultOwner, repoName, branchName, remoteName)
		if err != nil {
			return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
	}

	filePath := path.Join(base, loadPath)

	if !isDir {
		if !isYaml(loadPath) {
			log.Errorf("trying to preload a non-yaml file")
			return nil, e.ErrPreloadServiceTemplate.AddDesc("Non-yaml service loading is not supported")
		}
		pathSegment := strings.Split(loadPath, "/")
		fileName := pathSegment[len(pathSegment)-1]
		ret = append(ret, getFileName(fileName))
	} else {
		fileInfos, err := ioutil.ReadDir(filePath)
		if err != nil {
			log.Errorf("Failed to read directory info of path: %s, the error is: %+v", filePath, err)
			return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
		if isValidGerritServiceDir(fileInfos) {
			svcName := loadPath
			if loadPath == "" {
				svcName = repoName
			}
			pathList := strings.Split(svcName, "/")
			folderName := pathList[len(pathList)-1]
			ret = append(ret, folderName)
			return ret, nil
		}
		isGrandParent := false
		for _, file := range fileInfos {
			if file.IsDir() {
				subDirPath := fmt.Sprintf("%s/%s", filePath, file.Name())
				subtree, err := ioutil.ReadDir(subDirPath)
				if err != nil {
					log.Errorf("Failed to get subdir content from gerrit with path: %s, the error is: %+v", subDirPath, err)
					return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
				}
				if isValidGerritServiceDir(subtree) {
					ret = append(ret, getFileName(file.Name()))
					isGrandParent = true
				}
			}
		}
		if !isGrandParent {
			log.Errorf("invalid folder selected since no yaml is presented in path: %s", filePath)
			return ret, e.ErrPreloadServiceTemplate.AddDesc("所选路径下没有yaml，请重新选择")
		}
	}
	return ret, nil
}

// 根据repo信息从gerrit加载服务
func loadGerritService(username string, detail *codehost.Detail, repoOwner, repoName, branchName, remoteName string, args *LoadServiceReq, log *zap.SugaredLogger) error {
	base := path.Join(config.S3StoragePath(), repoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = command.RunGitCmds(detail, repoOwner, repoName, branchName, remoteName)
		if err != nil {
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
	}

	gerritCli := gerrit.NewClient(detail.Address, detail.OauthToken)
	commit, err := gerritCli.GetCommitByBranch(repoName, branchName)
	if err != nil {
		log.Errorf("Failed to get latest commit info from repo: %s, the error is: %+v", repoName, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	commitInfo := &models.Commit{
		SHA:     commit.Commit,
		Message: commit.Message,
	}

	filePath := path.Join(base, args.LoadPath)
	if !args.LoadFromDir {
		contentBytes, err := ioutil.ReadFile(path.Join(base, args.LoadPath))
		if err != nil {
			log.Errorf("Failed to read file of path: %s, the error is: %+v", args.LoadPath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
		pathSegments := strings.Split(args.LoadPath, "/")
		fileName := pathSegments[len(pathSegments)-1]
		svcName := getFileName(fileName)
		splitYaml := SplitYaml(string(contentBytes))
		// FIXME：gerrit原先有字段保存codehost信息，保存两份，兼容性
		createSvcArgs := &models.Service{
			CodehostID:       detail.ID,
			RepoName:         repoName,
			RepoOwner:        repoOwner,
			BranchName:       branchName,
			LoadPath:         args.LoadPath,
			LoadFromDir:      args.LoadFromDir,
			GerritBranchName: branchName,
			GerritCodeHostID: detail.ID,
			GerritPath:       filePath,
			GerritRemoteName: remoteName,
			GerritRepoName:   repoName,
			KubeYamls:        splitYaml,
			CreateBy:         username,
			ServiceName:      svcName,
			Type:             args.Type,
			ProductName:      args.ProductName,
			Source:           setting.SourceFromGerrit,
			Yaml:             string(contentBytes),
			Commit:           commitInfo,
			Visibility:       args.Visibility,
		}
		_, err = CreateServiceTemplate(username, createSvcArgs, log)
		if err != nil {
			_, messageMap := e.ErrorMessage(err)
			if description, ok := messageMap["description"]; ok {
				return e.ErrLoadServiceTemplate.AddDesc(description.(string))
			}
			return e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
		}
		return nil
	}
	fileInfos, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Errorf("Failed to read directory info of path: %s, the error is: %+v", filePath, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	if isValidGerritServiceDir(fileInfos) {
		return loadServiceFromGerrit(fileInfos, detail.ID, username, branchName, args.LoadPath, filePath, repoOwner, remoteName, repoName, args, commitInfo, log)
	}
	for _, entry := range fileInfos {
		subtreeLoadPath := fmt.Sprintf("%s/%s", args.LoadPath, entry.Name())
		subtreePath := fmt.Sprintf("%s/%s", filePath, entry.Name())
		subtreeInfo, err := ioutil.ReadDir(subtreePath)
		if err != nil {
			log.Errorf("Failed to read subdir info from gerrit package of path: %s, the error is: %+v", subtreePath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
		if isValidGerritServiceDir(subtreeInfo) {
			if err := loadServiceFromGerrit(subtreeInfo, detail.ID, username, branchName, subtreeLoadPath, subtreePath, repoOwner, remoteName, repoName, args, commitInfo, log); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadServiceFromGerrit(tree []os.FileInfo, id int, username, branchName, loadPath, path, repoOwner, remoteName, repoName string, args *LoadServiceReq, commit *models.Commit, log *zap.SugaredLogger) error {
	pathList := strings.Split(path, "/")
	var splitYaml []string
	fileName := pathList[len(pathList)-1]
	serviceName := getFileName(fileName)
	yamlList, err := extractGerritYamls(path, tree)
	if err != nil {
		log.Errorf("Failed to extract yamls from gerrit package, the error is: %+v", err)
		return err
	}
	for _, yamlEntry := range yamlList {
		splitYaml = append(splitYaml, SplitYaml(yamlEntry)...)
	}
	yml := joinYamls(yamlList)
	createSvcArgs := &models.Service{
		CodehostID:       id,
		BranchName:       branchName,
		RepoName:         repoName,
		RepoOwner:        repoOwner,
		LoadPath:         loadPath,
		LoadFromDir:      args.LoadFromDir,
		GerritRepoName:   repoName,
		GerritCodeHostID: id,
		GerritRemoteName: remoteName,
		GerritPath:       path,
		GerritBranchName: branchName,
		KubeYamls:        splitYaml,
		CreateBy:         username,
		ServiceName:      serviceName,
		Type:             args.Type,
		ProductName:      args.ProductName,
		Source:           setting.SourceFromGerrit,
		Yaml:             yml,
		Commit:           commit,
		Visibility:       args.Visibility,
	}

	_, err = CreateServiceTemplate(username, createSvcArgs, log)
	if err != nil {
		_, messageMap := e.ErrorMessage(err)
		if description, ok := messageMap["description"]; ok {
			err = e.ErrLoadServiceTemplate.AddDesc(description.(string))
		} else {
			err = e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
		}
	}
	return err
}

// 根据github repo决定服务是否可以更新这个repo地址
func validateServiceUpdateGithub(detail *poetry.CodeHost, serviceName, repoOwner, repoName, branchName, path string, isDir bool) error {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: detail.AccessToken},
	)
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(tokenClient)

	if !isDir {
		fileContent, _, _, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, path, &github.RepositoryContentGetOptions{Ref: branchName})
		if err != nil {
			log.Errorf("Failed to get dir content from github with path: %s, the error is: %+v", path, err)
			return e.ErrValidateServiceUpdate.AddDesc(err.Error())
		}
		if !isYaml(fileContent.GetName()) {
			log.Errorf("trying to preload a non-yaml file, ending ...")
			return e.ErrValidateServiceUpdate.AddDesc("Non-yaml service loading is not supported")
		}
		loadedName := getFileName(fileContent.GetName())
		if loadedName != serviceName {
			log.Errorf("The loaded file name [%s] is the same as the service to be updated: [%s]", loadedName, serviceName)
			return e.ErrValidateServiceUpdate.AddDesc("文件名称和服务名称不一致")
		}
		return nil
	}
	_, dirContent, _, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, path, &github.RepositoryContentGetOptions{Ref: branchName})
	if err != nil {
		log.Errorf("Failed to get dir content from github with path: %s, the error is: %+v", path, err)
		return e.ErrValidateServiceUpdate.AddDesc(err.Error())
	}
	if isValidGithubServiceDir(dirContent) {
		svcName := path
		if path == "" {
			svcName = repoName
		}
		pathList := strings.Split(svcName, "/")
		folderName := pathList[len(pathList)-1]
		if folderName != serviceName {
			log.Errorf("The loaded file name [%s] is the same as the service to be updated: [%s]", folderName, serviceName)
			return e.ErrValidateServiceUpdate.AddDesc("文件夹名称和服务名称不一致")
		}
		return nil
	}
	return e.ErrValidateServiceUpdate.AddDesc("所选路径中没有yaml，请重新选择")
}

// 根据gitlab repo决定服务是否可以更新这个repo地址
func validateServiceUpdateGitlab(detail *poetry.CodeHost, serviceName, repoOwner, repoName, branchName, path string, isDir bool) error {
	repoInfo := fmt.Sprintf("%s/%s", repoOwner, repoName)

	gitlabClient, err := gitlab.NewOAuthClient(detail.AccessToken, gitlab.WithBaseURL(detail.Address))
	if err != nil {
		log.Errorf("failed to prepare gitlab client, the error is:%+v", err)
		return e.ErrValidateServiceUpdate.AddDesc(err.Error())
	}

	// 非文件夹情况下直接获取文件信息
	if !isDir {
		if !isYaml(path) {
			return e.ErrValidateServiceUpdate.AddDesc("File is not of type yaml or yml, select again")
		}
		fileInfo, _, err := gitlabClient.RepositoryFiles.GetFile(repoInfo, path, &gitlab.GetFileOptions{Ref: gitlab.String(branchName)})
		if err != nil {
			log.Errorf("Failed to get file info from gitlab with path: %s, the error is %+v", path, err)
			return e.ErrValidateServiceUpdate.AddDesc(err.Error())
		}
		if getFileName(fileInfo.FileName) != serviceName {
			log.Errorf("The loaded file name [%s] is the same as the service to be updated: [%s]", fileInfo.FileName, serviceName)
			return e.ErrValidateServiceUpdate.AddDesc("文件名称和服务名称不一致")
		}
		return nil
	}
	opt := &gitlab.ListTreeOptions{
		Path:      gitlab.String(path),
		Ref:       gitlab.String(branchName),
		Recursive: gitlab.Bool(false),
	}
	treeInfo, _, err := gitlabClient.Repositories.ListTree(repoInfo, opt)
	if err != nil {
		log.Errorf("Failed to get dir content from gitlab with path: %s, the error is: %+v", path, err)
		return e.ErrValidateServiceUpdate.AddDesc(err.Error())
	}
	if isValidGitlabServiceDir(treeInfo) {
		svcName := path
		if path == "" {
			svcName = repoInfo
		}
		pathList := strings.Split(svcName, "/")
		folderName := pathList[len(pathList)-1]
		if folderName != serviceName {
			log.Errorf("The loaded file name [%s] is the same as the service to be updated: [%s]", folderName, serviceName)
			return e.ErrValidateServiceUpdate.AddDesc("文件夹名称和服务名称不一致")
		}
		return nil
	}
	return e.ErrValidateServiceUpdate.AddDesc("所选路径中没有yaml，请重新选择")
}

// 根据gerrit repo决定服务是否可以更新这个repo地址
func validateServiceUpdateGerrit(detail *poetry.CodeHost, serviceName, repoName, branchName, remoteName, loadPath string, isDir bool) error {
	base := path.Join(config.S3StoragePath(), repoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		chDetail := &codehost.Detail{
			ID:         detail.ID,
			Name:       "",
			Address:    detail.Address,
			Owner:      detail.Namespace,
			Source:     detail.Type,
			OauthToken: detail.AccessToken,
		}
		err = command.RunGitCmds(chDetail, setting.GerritDefaultOwner, repoName, branchName, remoteName)
		if err != nil {
			return e.ErrValidateServiceUpdate.AddDesc(err.Error())
		}
	}

	filePath := path.Join(base, loadPath)
	if !isDir {
		if !isYaml(loadPath) {
			log.Errorf("trying to preload a non-yaml file")
			return e.ErrPreloadServiceTemplate.AddDesc("Non-yaml service loading is not supported")
		}
		pathSegment := strings.Split(loadPath, "/")
		fileName := pathSegment[len(pathSegment)-1]
		if getFileName(fileName) != serviceName {
			log.Errorf("The loaded file name [%s] is the same as the service to be updated: [%s]", fileName, serviceName)
			return e.ErrValidateServiceUpdate.AddDesc("文件名称和服务名称不一致")
		}
		return nil
	}
	fileInfos, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Errorf("Failed to read directory info of path: %s, the error is: %+v", filePath, err)
		return e.ErrValidateServiceUpdate.AddDesc(err.Error())
	}
	if isValidGerritServiceDir(fileInfos) {
		svcName := loadPath
		if loadPath == "" {
			svcName = repoName
		}
		pathList := strings.Split(svcName, "/")
		folderName := pathList[len(pathList)-1]
		if folderName != serviceName {
			log.Errorf("The loaded file name [%s] is the same as the service to be updated: [%s]", folderName, serviceName)
			return e.ErrValidateServiceUpdate.AddDesc("文件夹名称和服务名称不一致")
		}
		return nil
	}
	return e.ErrValidateServiceUpdate.AddDesc("所选路径中没有yaml，请重新选择")
}

func isYaml(filename string) bool {
	return strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml")
}

func isValidGithubServiceDir(child []*github.RepositoryContent) bool {
	for _, entry := range child {
		if entry.GetType() == "file" && isYaml(entry.GetName()) {
			return true
		}
	}
	return false
}

func isValidGitlabServiceDir(child []*gitlab.TreeNode) bool {
	for _, entry := range child {
		if entry.Type == "blob" && isYaml(entry.Name) {
			return true
		}
	}
	return false
}

func isValidGerritServiceDir(child []os.FileInfo) bool {
	for _, file := range child {
		if !file.IsDir() && isYaml(file.Name()) {
			return true
		}
	}
	return false
}

func getFileName(fullName string) string {
	name := filepath.Base(fullName)
	ext := filepath.Ext(name)
	return name[0:(len(name) - len(ext))]
}

func extractGerritYamls(basePath string, tree []os.FileInfo) ([]string, error) {
	var ret []string
	for _, entry := range tree {
		if !entry.IsDir() && isYaml(entry.Name()) {
			tmpFilepath := fmt.Sprintf("%s/%s", basePath, entry.Name())
			yamlByte, err := ioutil.ReadFile(tmpFilepath)
			if err != nil {
				return nil, e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}
			ret = append(ret, string(yamlByte))
		}
	}
	return ret, nil
}
