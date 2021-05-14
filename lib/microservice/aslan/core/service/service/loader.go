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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/lib/microservice/aslan/internal/log"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/gerrit"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type LoadServiceReq struct {
	Type        string `json:"type"`
	ProductName string `json:"product_name"`
	Visibility  string `json:"visibility"`
	LoadFromDir bool   `json:"is_dir"`
	LoadPath    string `json:"path"`
}

func PreloadServiceFromCodeHost(codehostID int, repoOwner, repoName, branchName, remoteName, path string, isDir bool, log *xlog.Logger) ([]string, error) {
	ret := make([]string, 0)
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
	default:
		return nil, e.ErrPreloadServiceTemplate.AddDesc("Not supported code source")
	}

	return ret, err
}

// 根据提供的codehost信息加载服务
func LoadServiceFromCodeHost(username string, codehostID int, repoOwner, repoName, branchName, remoteName string, args *LoadServiceReq, log *xlog.Logger) error {
	detail, err := codehost.GetCodehostDetail(codehostID)
	if err != nil {
		log.Errorf("Failed to load codehost for preload service list, the error is: %+v", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	switch detail.Source {
	case setting.SourceFromGithub:
		return loadGithubService(username, detail, repoOwner, repoName, branchName, args, log)
	case setting.SourceFromGitlab:
		return loadGitlabService(username, detail, repoOwner, repoName, branchName, args, log)
	case setting.SourceFromGerrit:
		return loadGerritService(username, detail, repoOwner, repoName, branchName, remoteName, args, log)
	default:
		return e.ErrLoadServiceTemplate.AddDesc("unsupported code source")
	}
}

// 根据服务名和提供的加载信息确认是否可以更新服务加载地址
func ValidateServiceUpdate(codehostID int, serviceName, repoOwner, repoName, branchName, remoteName, path string, isDir bool, log *xlog.Logger) error {
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
func preloadGithubService(detail *codehost.CodeHost, repoOwner, repoName, branchName, path string, isDir bool) ([]string, error) {
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
func preloadGitlabService(detail *codehost.CodeHost, repoOwner, repoName, branchName, path string, isDir bool) ([]string, error) {
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
	} else {
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
	}
	return ret, nil
}

// 根据repo信息获取gerrit可以加载的服务列表
func preloadGerritService(detail *codehost.CodeHost, repoName, branchName, remoteName, loadPath string, isDir bool) ([]string, error) {
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

// 根据repo信息从github加载服务
func loadGithubService(username string, detail *codehost.Detail, repoOwner, repoName, branchName string, args *LoadServiceReq, log *xlog.Logger) error {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: detail.OauthToken},
	)
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(tokenClient)

	log.Infof("Request username is: %s", username)

	// 如果是单个文件导入，直接读取文件内容进行保存
	if !args.LoadFromDir {
		fileContent, _, _, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, args.LoadPath, &github.RepositoryContentGetOptions{Ref: branchName})
		if err != nil {
			log.Errorf("failed to get file info for path: %s from github, the error is: %+v", args.LoadPath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
		//commitInfo, _, err := githubClient.Repositories.GetCommit(ctx, repoOwner, repoName, fileContent.GetSHA())
		//if err != nil {
		//	log.Errorf("Failed to get commit info for path: %s, the error is: %+v", args.LoadPath, err)
		//	return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		//}
		srcPath := fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", repoOwner, repoName, branchName, args.LoadPath)
		svcContent, _ := fileContent.GetContent()
		splittedYaml := SplitYaml(svcContent)
		createSvcArgs := &models.Service{
			CodehostID:  detail.ID,
			RepoName:    repoName,
			RepoOwner:   repoOwner,
			BranchName:  branchName,
			LoadPath:    args.LoadPath,
			LoadFromDir: args.LoadFromDir,
			KubeYamls:   splittedYaml,
			SrcPath:     srcPath,
			CreateBy:    username,
			ServiceName: getFileName(fileContent.GetName()),
			Type:        args.Type,
			ProductName: args.ProductName,
			Source:      setting.SourceFromGithub,
			Yaml:        svcContent,
			Commit:      &models.Commit{SHA: fileContent.GetSHA()},
			Visibility:  args.Visibility,
		}
		_, err = CreateServiceTemplate(username, createSvcArgs, log)
		if err != nil {
			_, messageMap := e.ErrorMessage(err)
			if description, ok := messageMap["description"]; ok {
				return e.ErrLoadServiceTemplate.AddDesc(description.(string))
			}
			return e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
		}
		return err
	}
	_, dirInfo, _, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, args.LoadPath, &github.RepositoryContentGetOptions{Ref: branchName})
	if err != nil {
		log.Errorf("failed to get file info for path: %s from github, the error is: %+v", args.LoadPath, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	// 如果从文件夹载入，判断该目录是否含有yaml
	if isValidGithubServiceDir(dirInfo) {
		return loadServiceFromGithub(ctx, detail.ID, githubClient, dirInfo, username, repoOwner, repoName, branchName, args.LoadPath, args, log)
	}
	// 如果载入目录无yaml, 循环查询子目录是否含有yaml，并载入
	for _, entry := range dirInfo {
		if entry.GetType() == "dir" {
			_, subtree, _, err := githubClient.Repositories.GetContents(ctx, repoOwner, repoName, entry.GetPath(), &github.RepositoryContentGetOptions{Ref: branchName})
			if err != nil {
				log.Errorf("Failed to get dir content from github with path: %s, the error is: %+v", entry.GetPath(), err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}
			if isValidGithubServiceDir(subtree) {
				if err := loadServiceFromGithub(ctx, detail.ID, githubClient, subtree, username, repoOwner, repoName, branchName, entry.GetPath(), args, log); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func loadServiceFromGithub(ctx context.Context, codehostID int, client *github.Client, tree []*github.RepositoryContent, username, owner, repoName, branchName, path string, args *LoadServiceReq, log *xlog.Logger) error {
	pathList := strings.Split(path, "/")
	splittedYaml := []string{}
	serviceName := pathList[len(pathList)-1]
	yamlList, err := extractGithubYamls(ctx, client, tree, owner, repoName, branchName)
	if err != nil {
		log.Errorf("Failed to extract yamls from github, the error is: %+v", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	for _, yamlEntry := range yamlList {
		splittedYaml = append(splittedYaml, SplitYaml(yamlEntry)...)
	}
	yml := joinYamls(yamlList)
	commitInfo, _, err := client.Repositories.ListCommits(ctx, owner, repoName, &github.CommitsListOptions{Path: path})
	if err != nil {
		log.Errorf("Failed to get commit info for path: %s, the error is: %+v", path, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	srcPath := fmt.Sprintf("https://github.com/%s/%s/tree/%s/%s", owner, repoName, branchName, path)
	createSvcArgs := &models.Service{
		CodehostID:  codehostID,
		RepoName:    repoName,
		RepoOwner:   owner,
		BranchName:  branchName,
		LoadPath:    path,
		LoadFromDir: args.LoadFromDir,
		KubeYamls:   splittedYaml,
		SrcPath:     srcPath,
		CreateBy:    username,
		ServiceName: serviceName,
		Type:        args.Type,
		ProductName: args.ProductName,
		Source:      setting.SourceFromGithub,
		Yaml:        yml,
		Commit:      &models.Commit{SHA: commitInfo[0].GetSHA(), Message: commitInfo[0].GetCommit().GetMessage()},
		Visibility:  args.Visibility,
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

// 根据repo信息从gitlab加载服务
func loadGitlabService(username string, detail *codehost.Detail, repoOwner, repoName, branchName string, args *LoadServiceReq, log *xlog.Logger) error {
	gitlabClient, err := gitlab.NewOAuthClient(detail.OauthToken, gitlab.WithBaseURL(detail.Address))
	if err != nil {
		log.Errorf("failed to prepare gitlab client, the error is:%+v", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}

	repoInfo := fmt.Sprintf("%s/%s", repoOwner, repoName)

	if !args.LoadFromDir {
		fileContent, _, err := gitlabClient.RepositoryFiles.GetFile(repoInfo, args.LoadPath, &gitlab.GetFileOptions{Ref: gitlab.String(branchName)})
		if err != nil {
			log.Errorf("Failed to get file info for path: %s from gitlab, the error is: %+v", args.LoadPath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
		decodedContent, err := base64.StdEncoding.DecodeString(fileContent.Content)
		if err != nil {
			log.Errorf("Failed to decode file, the error is: %+v", err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
		srcPath := fmt.Sprintf("%s/%s/blob/%s/%s", detail.Address, repoInfo, branchName, args.LoadPath)
		splittedYaml := SplitYaml(string(decodedContent))
		createSvcArgs := &models.Service{
			CodehostID:  detail.ID,
			RepoOwner:   repoOwner,
			RepoName:    repoName,
			BranchName:  branchName,
			LoadPath:    args.LoadPath,
			LoadFromDir: args.LoadFromDir,
			KubeYamls:   splittedYaml,
			SrcPath:     srcPath,
			CreateBy:    username,
			ServiceName: getFileName(fileContent.FileName),
			Type:        args.Type,
			ProductName: args.ProductName,
			Source:      setting.SourceFromGitlab,
			Yaml:        string(decodedContent),
			Commit:      &models.Commit{SHA: fileContent.CommitID},
			Visibility:  args.Visibility,
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
	opt := &gitlab.ListTreeOptions{
		Path:      gitlab.String(args.LoadPath),
		Ref:       gitlab.String(branchName),
		Recursive: gitlab.Bool(false),
	}
	treeInfo, _, err := gitlabClient.Repositories.ListTree(repoInfo, opt)
	if err != nil {
		log.Errorf("Failed to get dir content from gitlab with path: %s, the error is: %+v", args.LoadPath, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	if isValidGitlabServiceDir(treeInfo) {
		return loadServiceFromGitlab(gitlabClient, treeInfo, detail, username, repoOwner, repoName, branchName, args.LoadPath, args, log)
	}
	for _, treeNode := range treeInfo {
		if treeNode.Type == "tree" {
			subOpt := &gitlab.ListTreeOptions{
				Path:      gitlab.String(treeNode.Path),
				Ref:       gitlab.String(branchName),
				Recursive: gitlab.Bool(false),
			}
			subtree, _, err := gitlabClient.Repositories.ListTree(repoInfo, subOpt)
			if err != nil {
				log.Errorf("Failed to get dir content from gitlab with path: %s, the error is %+v", treeNode.Path, err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}
			if isValidGitlabServiceDir(subtree) {
				if err := loadServiceFromGitlab(gitlabClient, subtree, detail, username, repoOwner, repoName, branchName, treeNode.Path, args, log); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func loadServiceFromGitlab(client *gitlab.Client, tree []*gitlab.TreeNode, detail *codehost.Detail, username, repoOwner, repoName, branchName, path string, args *LoadServiceReq, log *xlog.Logger) error {
	pathList := strings.Split(path, "/")
	splittedYaml := []string{}
	serviceName := pathList[len(pathList)-1]
	repoInfo := fmt.Sprintf("%s/%s", repoOwner, repoName)
	yamlList, sha, err := extractGitlabYamls(client, tree, repoInfo, branchName)
	if err != nil {
		log.Error("Failed to extract yamls from gitlab, the error is: %+v", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	for _, yamlEntry := range yamlList {
		splittedYaml = append(splittedYaml, SplitYaml(yamlEntry)...)
	}
	yml := joinYamls(yamlList)
	srcPath := fmt.Sprintf("%s/%s/tree/%s/%s", detail.Address, repoInfo, branchName, path)
	createSvcArgs := &models.Service{
		CodehostID:  detail.ID,
		RepoOwner:   repoOwner,
		RepoName:    repoName,
		BranchName:  branchName,
		LoadPath:    path,
		LoadFromDir: args.LoadFromDir,
		KubeYamls:   splittedYaml,
		SrcPath:     srcPath,
		CreateBy:    username,
		ServiceName: serviceName,
		Type:        args.Type,
		ProductName: args.ProductName,
		Source:      setting.SourceFromGitlab,
		Yaml:        yml,
		Commit:      &models.Commit{SHA: sha},
		Visibility:  args.Visibility,
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

// 根据repo信息从gerrit加载服务
func loadGerritService(username string, detail *codehost.Detail, repoOwner, repoName, branchName, remoteName string, args *LoadServiceReq, log *xlog.Logger) error {
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
		splittedYaml := SplitYaml(string(contentBytes))
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
			KubeYamls:        splittedYaml,
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

func loadServiceFromGerrit(tree []os.FileInfo, id int, username, branchName, loadPath, path, repoOwner, remoteName, repoName string, args *LoadServiceReq, commit *models.Commit, log *xlog.Logger) error {
	pathList := strings.Split(path, "/")
	splittedYaml := []string{}
	fileName := pathList[len(pathList)-1]
	serviceName := getFileName(fileName)
	yamlList, err := extractGerritYamls(path, tree)
	if err != nil {
		log.Errorf("Failed to extract yamls from gerrit package, the error is: %+v", err)
		return err
	}
	for _, yamlEntry := range yamlList {
		splittedYaml = append(splittedYaml, SplitYaml(yamlEntry)...)
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
		KubeYamls:        splittedYaml,
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
func validateServiceUpdateGithub(detail *codehost.CodeHost, serviceName, repoOwner, repoName, branchName, path string, isDir bool) error {
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
func validateServiceUpdateGitlab(detail *codehost.CodeHost, serviceName, repoOwner, repoName, branchName, path string, isDir bool) error {
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
func validateServiceUpdateGerrit(detail *codehost.CodeHost, serviceName, repoName, branchName, remoteName, loadPath string, isDir bool) error {
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
	extension := filepath.Ext(fullName)
	return fullName[0:(len(fullName) - len(extension))]
}

func extractGithubYamls(ctx context.Context, client *github.Client, tree []*github.RepositoryContent, owner, repoName, branchName string) ([]string, error) {
	ret := []string{}
	for _, entry := range tree {
		if isYaml(entry.GetPath()) {
			fileInfo, _, _, err := client.Repositories.GetContents(ctx, owner, repoName, entry.GetPath(), &github.RepositoryContentGetOptions{Ref: branchName})
			if err != nil {
				log.Errorf("Failed to download github file: %s, the error is: %+v", entry.GetPath(), err)
				return nil, err
			}
			yamlInfo, err := fileInfo.GetContent()
			if err != nil {
				log.Errorf("Cannot get content from the downloaded github file: %s, the error is: %+v", entry.GetPath(), err)
				return nil, err
			}
			ret = append(ret, yamlInfo)
		}
	}
	return ret, nil
}

func extractGitlabYamls(client *gitlab.Client, tree []*gitlab.TreeNode, repoName, branchName string) ([]string, string, error) {
	ret := []string{}
	var sha string
	for _, entry := range tree {
		if isYaml(entry.Name) {
			fileInfo, _, err := client.RepositoryFiles.GetFile(repoName, entry.Path, &gitlab.GetFileOptions{Ref: gitlab.String(branchName)})
			if err != nil {
				log.Errorf("Failed to download gitlab file: %s, the error is: %+v", entry.Path, err)
				return nil, "", err
			}
			decodedFile, err := base64.StdEncoding.DecodeString(fileInfo.Content)
			if err != nil {
				log.Errorf("Failed to decode content from the given file of path: %s, the error is: %+v", entry.Path, err)
				return nil, "", err
			}
			ret = append(ret, string(decodedFile))
			sha = fileInfo.CommitID
		}
	}
	return ret, sha, nil
}

func extractGerritYamls(basePath string, tree []os.FileInfo) ([]string, error) {
	ret := []string{}
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
