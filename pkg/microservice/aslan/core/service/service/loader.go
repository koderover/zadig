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
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/codehub"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/gerrit"
	gitlab2 "github.com/koderover/zadig/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/pkg/tool/gitee"
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

func PreloadServiceFromCodeHost(codehostID int, repoOwner, repoName, repoUUID, branchName, remoteName, path string, isDir bool, log *zap.SugaredLogger) ([]string, error) {
	var ret []string
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		log.Errorf("Failed to load codehost for preload service list, the error is: %+v", err)
		return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
	}
	switch ch.Type {
	case setting.SourceFromGithub, setting.SourceFromGitlab:
		ret, err = preloadService(ch, repoOwner, repoName, branchName, path, isDir, log)
	case setting.SourceFromGerrit:
		ret, err = preloadGerritService(ch, repoName, branchName, remoteName, path, isDir)
	case setting.SourceFromCodeHub:
		ret, err = preloadCodehubService(ch, repoName, repoUUID, branchName, path, isDir)
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		ret, err = preloadGiteeService(ch, repoOwner, repoName, branchName, remoteName, path, isDir)
	default:
		return nil, e.ErrPreloadServiceTemplate.AddDesc("Not supported code source")
	}

	return ret, err
}

// LoadServiceFromCodeHost 根据提供的codehost信息加载服务
func LoadServiceFromCodeHost(username string, codehostID int, repoOwner, namespace, repoName, repoUUID, branchName, remoteName string, args *LoadServiceReq, force bool, log *zap.SugaredLogger) error {
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		log.Errorf("Failed to load codehost for preload service list, the error is: %+v", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	switch ch.Type {
	case setting.SourceFromGithub, setting.SourceFromGitlab:
		return loadService(username, ch, repoOwner, namespace, repoName, branchName, args, force, log)
	case setting.SourceFromGerrit:
		return loadGerritService(username, ch, repoOwner, repoName, branchName, remoteName, args, force, log)
	case setting.SourceFromCodeHub:
		return loadCodehubService(username, ch, repoOwner, repoName, repoUUID, branchName, args, force, log)
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		return loadGiteeService(username, ch, repoOwner, repoName, branchName, remoteName, args, force, log)
	default:
		return e.ErrLoadServiceTemplate.AddDesc("unsupported code source")
	}
}

// ValidateServiceUpdate 根据服务名和提供的加载信息确认是否可以更新服务加载地址
func ValidateServiceUpdate(codehostID int, serviceName, repoOwner, repoName, repoUUID, branchName, remoteName, path string, isDir bool, log *zap.SugaredLogger) error {
	detail, err := systemconfig.New().GetCodeHost(codehostID)
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
	case setting.SourceFromCodeHub:
		return validateServiceUpdateCodehub(detail, serviceName, repoName, repoUUID, branchName, path, isDir)
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		return validateServiceUpdateGitee(detail, serviceName, repoOwner, repoName, branchName, remoteName, path, isDir)
	default:
		return e.ErrValidateServiceUpdate.AddDesc("Not supported code source")
	}
}

// 根据repo信息获取gerrit可以加载的服务列表
func preloadGerritService(detail *systemconfig.CodeHost, repoName, branchName, remoteName, loadPath string, isDir bool) ([]string, error) {
	ret := make([]string, 0)

	base := path.Join(config.S3StoragePath(), repoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = command.RunGitCmds(detail, setting.GerritDefaultOwner, setting.GerritDefaultOwner, repoName, branchName, remoteName)
		if err != nil {
			return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
	}

	filePath := path.Join(base, loadPath)

	if !isDir {
		if !isYaml(loadPath) {
			log.Error("trying to preload a non-yaml file")
			return nil, e.ErrPreloadServiceTemplate.AddDesc("Non-yaml service loading is not supported")
		}
		pathSegment := strings.Split(loadPath, "/")
		fileName := pathSegment[len(pathSegment)-1]
		ret = append(ret, getFileName(fileName))
	} else {
		fileInfos, err := ioutil.ReadDir(filePath)
		if err != nil {
			log.Error("Failed to read directory info of path: %s, the error is: %+v", filePath, err)
			return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
		if isValidServiceDir(fileInfos) {
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
				if isValidServiceDir(subtree) {
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

// 根据 repo 信息获取 codehub 可以加载的服务列表
func preloadCodehubService(detail *systemconfig.CodeHost, repoName, repoUUID, branchName, path string, isDir bool) ([]string, error) {
	var ret []string

	codeHubClient := codehub.NewCodeHubClient(detail.AccessKey, detail.SecretKey, detail.Region, config.ProxyHTTPSAddr(), detail.EnableProxy)
	// 非文件夹情况下直接获取文件信息
	if !isDir {
		if !isYaml(path) {
			return ret, e.ErrPreloadServiceTemplate.AddDesc("File is not of type yaml or yml, select again")
		}
		fileInfo, err := codeHubClient.FileContent(repoUUID, branchName, path)
		if err != nil {
			log.Errorf("Failed to get file info from codehub with path: %s, the error is %+v", path, err)
			return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
		fileName := getFileName(fileInfo.FileName)
		ret = append(ret, fileName)
		return ret, nil
	}

	treeInfo, err := codeHubClient.FileTree(repoUUID, branchName, path)
	if err != nil {
		log.Errorf("Failed to get dir content from codehub with path: %s, the error is: %+v", path, err)
		return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
	}
	if isValidCodehubServiceDir(treeInfo) {
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
	for _, entry := range treeInfo {
		if entry.Type == "tree" {
			subTreeInfo, err := codeHubClient.FileTree(repoUUID, branchName, entry.Path)
			if err != nil {
				log.Errorf("Failed to get dir content from codehub with path: %s, the error is: %+v", path, err)
				return ret, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
			}
			if isValidCodehubServiceDir(subTreeInfo) {
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

// Get a list of services that gitee can load based on repo information
func preloadGiteeService(detail *systemconfig.CodeHost, repoOwner, repoName, branchName, remoteName, loadPath string, isDir bool) ([]string, error) {
	ret := make([]string, 0)

	base := path.Join(config.S3StoragePath(), repoName)
	if exist, err := util.PathExists(base); !exist {
		log.Warnf("path does not exist,err:%s", err)
		err = command.RunGitCmds(detail, repoOwner, repoOwner, repoName, branchName, remoteName)
		if err != nil {
			return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
	}

	filePath := path.Join(base, loadPath)

	if !isDir {
		if !isYaml(loadPath) {
			log.Error("trying to preload a non-yaml file")
			return nil, e.ErrPreloadServiceTemplate.AddDesc("Non-yaml service loading is not supported")
		}
		pathSegment := strings.Split(loadPath, "/")
		fileName := pathSegment[len(pathSegment)-1]
		ret = append(ret, getFileName(fileName))
	} else {
		fileInfos, err := ioutil.ReadDir(filePath)
		if err != nil {
			log.Errorf("Failed to read directory info of path: %s, the error is: %s", filePath, err)
			return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
		if isValidServiceDir(fileInfos) {
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
					log.Errorf("Failed to get subdir content from gitee with path: %s, the error is: %s", subDirPath, err)
					return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
				}
				if isValidServiceDir(subtree) {
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
func loadGerritService(username string, ch *systemconfig.CodeHost, repoOwner, repoName, branchName, remoteName string, args *LoadServiceReq, force bool, log *zap.SugaredLogger) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	base := path.Join(config.S3StoragePath(), repoName)
	// we should not use the cache stored on local disk since files stored in remove gerrit server may be changed
	_ = os.RemoveAll(base)
	// TODO there may by concurrency issues since same directory are used by multiple requests
	err := command.RunGitCmds(ch, repoOwner, repoOwner, repoName, branchName, remoteName)
	if err != nil {
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	//if _, err := os.Stat(base); os.IsNotExist(err) {
	//	err = command.RunGitCmds(ch, repoOwner, repoOwner, repoName, branchName, remoteName)
	//	if err != nil {
	//		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	//	}
	//}

	gerritCli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
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
			CodehostID:       ch.ID,
			RepoName:         repoName,
			RepoOwner:        repoOwner,
			RepoNamespace:    repoOwner, // TODO gerrit need set namespace?
			BranchName:       branchName,
			LoadPath:         args.LoadPath,
			LoadFromDir:      args.LoadFromDir,
			GerritBranchName: branchName,
			GerritCodeHostID: ch.ID,
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
		_, err = CreateServiceTemplate(username, createSvcArgs, force, log)
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
	if isValidServiceDir(fileInfos) {
		return loadServiceFromGerrit(fileInfos, ch.ID, username, branchName, args.LoadPath, filePath, repoOwner, remoteName, repoName, args, commitInfo, force, log)
	}
	for _, entry := range fileInfos {
		subtreeLoadPath := fmt.Sprintf("%s/%s", args.LoadPath, entry.Name())
		subtreePath := fmt.Sprintf("%s/%s", filePath, entry.Name())
		subtreeInfo, err := ioutil.ReadDir(subtreePath)
		if err != nil {
			log.Errorf("Failed to read subdir info from gerrit package of path: %s, the error is: %+v", subtreePath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
		if isValidServiceDir(subtreeInfo) {
			if err := loadServiceFromGerrit(subtreeInfo, ch.ID, username, branchName, subtreeLoadPath, subtreePath, repoOwner, remoteName, repoName, args, commitInfo, force, log); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadServiceFromGerrit(tree []os.FileInfo, id int, username, branchName, loadPath, path, repoOwner, remoteName, repoName string, args *LoadServiceReq, commit *models.Commit, force bool, log *zap.SugaredLogger) error {
	pathList := strings.Split(path, "/")
	var splittedYaml []string
	fileName := pathList[len(pathList)-1]
	serviceName := getFileName(fileName)
	yamlList, err := extractYamls(path, tree)
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

	_, err = CreateServiceTemplate(username, createSvcArgs, force, log)
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

// load codehub service
func loadCodehubService(username string, ch *systemconfig.CodeHost, repoOwner, repoName, repoUUID, branchName string, args *LoadServiceReq, force bool, log *zap.SugaredLogger) error {
	codeHubClient := codehub.NewCodeHubClient(ch.AccessKey, ch.SecretKey, ch.Region, config.ProxyHTTPSAddr(), ch.EnableProxy)

	if !args.LoadFromDir {
		yamls, err := codeHubClient.GetYAMLContents(repoUUID, branchName, args.LoadPath, args.LoadFromDir, true)
		if err != nil {
			log.Errorf("Failed to get yamls under path %s, error: %s", args.LoadPath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}

		commit, err := codeHubClient.GetLatestRepositoryCommit(repoOwner, repoName, branchName)
		if err != nil {
			log.Errorf("Failed to get latest commit under path %s, error: %s", args.LoadPath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}

		srcPath := fmt.Sprintf("%s/%s/%s/blob/%s/%s", ch.Address, repoOwner, repoName, branchName, args.LoadPath)
		createSvcArgs := &models.Service{
			CodehostID:    ch.ID,
			RepoOwner:     repoOwner,
			RepoNamespace: repoOwner, // TODO codehub need fill namespace?
			RepoName:      repoName,
			RepoUUID:      repoUUID,
			BranchName:    branchName,
			SrcPath:       srcPath,
			LoadPath:      args.LoadPath,
			LoadFromDir:   args.LoadFromDir,
			KubeYamls:     yamls,
			CreateBy:      username,
			ServiceName:   getFileName(args.LoadPath),
			Type:          args.Type,
			ProductName:   args.ProductName,
			Source:        ch.Type,
			Yaml:          util.CombineManifests(yamls),
			Commit:        &models.Commit{SHA: commit.ID, Message: commit.Message},
			Visibility:    args.Visibility,
		}
		if _, err = CreateServiceTemplate(username, createSvcArgs, force, log); err != nil {
			log.Errorf("Failed to create service template, serviceName:%s error: %s", createSvcArgs.ServiceName, err)
			_, messageMap := e.ErrorMessage(err)
			if description, ok := messageMap["description"]; ok {
				return e.ErrLoadServiceTemplate.AddDesc(description.(string))
			}
			return e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
		}
	}
	treeNodes, err := codeHubClient.FileTree(repoUUID, branchName, args.LoadPath)
	if err != nil {
		log.Errorf("Failed to get dir content from codehub with path: %s, the error is: %s", args.LoadPath, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	if isValidCodehubServiceDir(treeNodes) {
		return loadServiceFromCodehub(codeHubClient, treeNodes, ch, username, repoOwner, repoName, repoUUID, branchName, args.LoadPath, args, force, log)
	}

	for _, treeNode := range treeNodes {
		if treeNode.Type == "tree" {
			subtree, err := codeHubClient.FileTree(repoUUID, branchName, treeNode.Path)
			if err != nil {
				log.Errorf("Failed to get dir content from codehub with path: %s, the error is %s", treeNode.Path, err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}
			if isValidCodehubServiceDir(subtree) {
				if err := loadServiceFromCodehub(codeHubClient, subtree, ch, username, repoOwner, repoName, repoUUID, branchName, treeNode.Path, args, force, log); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func loadServiceFromCodehub(client *codehub.CodeHubClient, tree []*codehub.TreeNode, ch *systemconfig.CodeHost, username, repoOwner, repoName, repoUUID, branchName, path string, args *LoadServiceReq, force bool, log *zap.SugaredLogger) error {
	pathList := strings.Split(path, "/")
	var splittedYaml []string
	serviceName := pathList[len(pathList)-1]
	repoInfo := fmt.Sprintf("%s/%s", repoOwner, repoName)
	yamlList, err := extractCodehubYamls(client, tree, repoUUID, branchName)
	if err != nil {
		log.Errorf("Failed to extract yamls from codehub, the error is: %s", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}

	for _, yamlEntry := range yamlList {
		splittedYaml = append(splittedYaml, SplitYaml(yamlEntry)...)
	}
	yml := joinYamls(yamlList)

	commit, err := client.GetLatestRepositoryCommit(repoOwner, repoName, branchName)
	if err != nil {
		log.Errorf("Failed to get latest commit under path %s, error: %s", args.LoadPath, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}

	srcPath := fmt.Sprintf("%s/%s/tree/%s/%s", ch.Address, repoInfo, branchName, path)
	createSvcArgs := &models.Service{
		CodehostID:  ch.ID,
		RepoOwner:   repoOwner,
		RepoName:    repoName,
		BranchName:  branchName,
		RepoUUID:    repoUUID,
		LoadPath:    path,
		LoadFromDir: args.LoadFromDir,
		KubeYamls:   splittedYaml,
		SrcPath:     srcPath,
		CreateBy:    username,
		ServiceName: serviceName,
		Type:        args.Type,
		ProductName: args.ProductName,
		Source:      setting.SourceFromCodeHub,
		Yaml:        yml,
		Commit:      &models.Commit{SHA: commit.ID, Message: commit.Message},
		Visibility:  args.Visibility,
	}

	if _, err = CreateServiceTemplate(username, createSvcArgs, force, log); err != nil {
		_, messageMap := e.ErrorMessage(err)
		if description, ok := messageMap["description"]; ok {
			err = e.ErrLoadServiceTemplate.AddDesc(description.(string))
		} else {
			err = e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
		}
	}
	return err
}

// Load services from gitee based on repo information
func loadGiteeService(username string, ch *systemconfig.CodeHost, repoOwner, repoName, branchName, remoteName string, args *LoadServiceReq, force bool, log *zap.SugaredLogger) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	base := path.Join(config.S3StoragePath(), repoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = command.RunGitCmds(ch, repoOwner, repoName, repoName, branchName, remoteName)
		if err != nil {
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
	}

	giteeCli := gitee.NewClient(ch.ID, ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
	branch, err := giteeCli.GetSingleBranch(ch.Address, ch.AccessToken, repoOwner, repoName, branchName)
	if err != nil {
		log.Errorf("Failed to get latest commit info from repo: %s, the error is: %s", repoName, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	commitInfo := &models.Commit{
		SHA:     branch.Commit.Sha,
		Message: branch.Commit.Commit.Message,
	}

	filePath := path.Join(base, args.LoadPath)
	if !args.LoadFromDir {
		contentBytes, err := ioutil.ReadFile(path.Join(base, args.LoadPath))
		if err != nil {
			log.Errorf("Failed to read file of path: %s, the error is: %s", args.LoadPath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
		pathSegments := strings.Split(args.LoadPath, "/")
		fileName := pathSegments[len(pathSegments)-1]
		svcName := getFileName(fileName)
		splittedYaml := SplitYaml(string(contentBytes))
		srcPath := fmt.Sprintf("%s/%s/%s/blob/%s/%s", ch.Address, repoOwner, repoName, branchName, args.LoadPath)
		createSvcArgs := &models.Service{
			CodehostID:  ch.ID,
			RepoName:    repoName,
			RepoOwner:   repoOwner,
			BranchName:  branchName,
			SrcPath:     srcPath,
			LoadPath:    args.LoadPath,
			LoadFromDir: args.LoadFromDir,
			KubeYamls:   splittedYaml,
			CreateBy:    username,
			ServiceName: svcName,
			Type:        args.Type,
			ProductName: args.ProductName,
			Source:      setting.SourceFromGitee,
			Yaml:        string(contentBytes),
			Commit:      commitInfo,
			Visibility:  args.Visibility,
		}
		_, err = CreateServiceTemplate(username, createSvcArgs, force, log)
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
		log.Errorf("Failed to read directory info of path: %s, the error is: %s", filePath, err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}
	if isValidServiceDir(fileInfos) {
		return loadServiceFromGitee(fileInfos, ch, username, branchName, args.LoadPath, filePath, repoOwner, remoteName, repoName, args, commitInfo, force, log)
	}
	for _, entry := range fileInfos {
		subtreeLoadPath := fmt.Sprintf("%s/%s", args.LoadPath, entry.Name())
		subtreePath := fmt.Sprintf("%s/%s", filePath, entry.Name())
		subtreeInfo, err := ioutil.ReadDir(subtreePath)
		if err != nil {
			log.Errorf("Failed to read subdir info from gitee package of path: %s, the error is: %s", subtreePath, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}
		if isValidServiceDir(subtreeInfo) {
			if err := loadServiceFromGitee(subtreeInfo, ch, username, branchName, subtreeLoadPath, subtreePath, repoOwner, remoteName, repoName, args, commitInfo, force, log); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadServiceFromGitee(tree []os.FileInfo, ch *systemconfig.CodeHost, username, branchName, loadPath, path, repoOwner, remoteName, repoName string, args *LoadServiceReq, commit *models.Commit, force bool, log *zap.SugaredLogger) error {
	pathList := strings.Split(path, "/")
	var splittedYaml []string
	fileName := pathList[len(pathList)-1]
	serviceName := getFileName(fileName)
	repoInfo := fmt.Sprintf("%s/%s", repoOwner, repoName)
	yamlList, err := extractYamls(path, tree)
	if err != nil {
		log.Errorf("Failed to extract yamls from gitee package, the error is: %+v", err)
		return err
	}
	for _, yamlEntry := range yamlList {
		splittedYaml = append(splittedYaml, SplitYaml(yamlEntry)...)
	}
	yml := joinYamls(yamlList)
	srcPath := fmt.Sprintf("%s/%s/tree/%s/%s", ch.Address, repoInfo, branchName, loadPath)
	createSvcArgs := &models.Service{
		CodehostID:  ch.ID,
		BranchName:  branchName,
		RepoName:    repoName,
		RepoOwner:   repoOwner,
		LoadPath:    loadPath,
		SrcPath:     srcPath,
		LoadFromDir: args.LoadFromDir,
		KubeYamls:   splittedYaml,
		CreateBy:    username,
		ServiceName: serviceName,
		Type:        args.Type,
		ProductName: args.ProductName,
		Source:      setting.SourceFromGitee,
		Yaml:        yml,
		Commit:      commit,
		Visibility:  args.Visibility,
	}

	_, err = CreateServiceTemplate(username, createSvcArgs, force, log)
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
func validateServiceUpdateGithub(detail *systemconfig.CodeHost, serviceName, repoOwner, repoName, branchName, path string, isDir bool) error {
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
func validateServiceUpdateGitlab(detail *systemconfig.CodeHost, serviceName, repoOwner, repoName, branchName, path string, isDir bool) error {
	repoInfo := fmt.Sprintf("%s/%s", repoOwner, repoName)

	newToken, err := gitlab2.UpdateGitlabToken(detail.ID, detail.AccessToken)
	if err != nil {
		log.Errorf("failed to update gitlab token, err: %s", err)
		return err
	}
	gitlabClient, err := gitlab.NewOAuthClient(newToken, gitlab.WithBaseURL(detail.Address))
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
func validateServiceUpdateGerrit(detail *systemconfig.CodeHost, serviceName, repoName, branchName, remoteName, loadPath string, isDir bool) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	base := path.Join(config.S3StoragePath(), repoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = command.RunGitCmds(detail, setting.GerritDefaultOwner, setting.GerritDefaultOwner, repoName, branchName, remoteName)
		if err != nil {
			return e.ErrValidateServiceUpdate.AddDesc(err.Error())
		}
	}

	filePath := path.Join(base, loadPath)
	if !isDir {
		if !isYaml(loadPath) {
			log.Error("trying to preload a non-yaml file")
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
	if isValidServiceDir(fileInfos) {
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

func validateServiceUpdateCodehub(detail *systemconfig.CodeHost, serviceName, repoName, repoUUID, branchName, loadPath string, isDir bool) error {
	codeHubClient := codehub.NewCodeHubClient(detail.AccessKey, detail.SecretKey, detail.Region, config.ProxyHTTPSAddr(), detail.EnableProxy)
	// 非文件夹情况下直接获取文件信息
	if !isDir {
		if !isYaml(loadPath) {
			return e.ErrValidateServiceUpdate.AddDesc("File is not of type yaml or yml, select again")
		}
		fileInfo, err := codeHubClient.FileContent(repoUUID, branchName, loadPath)
		if err != nil {
			log.Errorf("Failed to get file info from codehub with path: %s, the error is %s", loadPath, err)
			return e.ErrValidateServiceUpdate.AddDesc(err.Error())
		}
		if getFileName(fileInfo.FileName) != serviceName {
			log.Errorf("The loaded file name [%s] is the same as the service to be updated: [%s]", fileInfo.FileName, serviceName)
			return e.ErrValidateServiceUpdate.AddDesc("文件名称和服务名称不一致")
		}
		return nil
	}

	treeInfo, err := codeHubClient.FileTree(repoUUID, branchName, loadPath)
	if err != nil {
		log.Errorf("Failed to get dir content from codehub with path: %s, the error is: %+v", loadPath, err)
		return e.ErrValidateServiceUpdate.AddDesc(err.Error())
	}
	if isValidCodehubServiceDir(treeInfo) {
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

// Determine whether the service can update the repo address according to the gitee repo
func validateServiceUpdateGitee(detail *systemconfig.CodeHost, serviceName, repoOwner, repoName, branchName, remoteName, loadPath string, isDir bool) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	base := path.Join(config.S3StoragePath(), repoName)
	if exist, err := util.PathExists(base); !exist {
		log.Warnf("path does not exist,err:%s", err)
		err := command.RunGitCmds(detail, repoOwner, repoOwner, repoName, branchName, remoteName)
		if err != nil {
			return e.ErrValidateServiceUpdate.AddDesc(err.Error())
		}
	}

	filePath := path.Join(base, loadPath)
	if !isDir {
		if !isYaml(loadPath) {
			log.Error("trying to preload a non-yaml file")
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
	if isValidServiceDir(fileInfos) {
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

func isValidServiceDir(child []os.FileInfo) bool {
	for _, file := range child {
		if !file.IsDir() && isYaml(file.Name()) {
			return true
		}
	}
	return false
}

func isValidCodehubServiceDir(child []*codehub.TreeNode) bool {
	for _, entry := range child {
		if entry.Type == "blob" && isYaml(entry.Name) {
			return true
		}
	}
	return false
}

func extractCodehubYamls(client *codehub.CodeHubClient, tree []*codehub.TreeNode, repoUUID, branchName string) ([]string, error) {
	var ret []string
	for _, entry := range tree {
		if isYaml(entry.Name) {
			fileInfo, err := client.FileContent(repoUUID, branchName, entry.Path)
			if err != nil {
				log.Errorf("Failed to download codehub file: %s, the error is: %s", entry.Path, err)
				return nil, err
			}
			contentByte, err := base64.StdEncoding.DecodeString(fileInfo.Content)
			if err != nil {
				log.Errorf("Failed to decode content from the given file of path: %s, the error is: %s", entry.Path, err)
				return nil, err
			}
			ret = append(ret, string(contentByte))
		}
	}
	return ret, nil
}

func extractYamls(basePath string, tree []os.FileInfo) ([]string, error) {
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
