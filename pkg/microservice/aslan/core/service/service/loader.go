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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
	"github.com/koderover/zadig/v2/pkg/util"
)

type LoadServiceReq struct {
	Type         string            `json:"type"`
	ProductName  string            `json:"product_name"`
	ServicePaths []LoadServicePath `json:"service_paths"`
}

type PreLoadServicePath struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func PreloadServiceFromCodeHost(codehostID int, repoOwner, repoName, repoUUID, branchName, remoteName string, paths []PreLoadServicePath, log *zap.SugaredLogger) ([]LoadServicePath, error) {
	var ret []LoadServicePath
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		log.Errorf("Failed to load codehost for preload service list, the error is: %+v", err)
		return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
	}
	switch ch.Type {
	case setting.SourceFromGithub, setting.SourceFromGitlab:
		ret, err = preloadService(ch, repoOwner, repoName, branchName, paths, log)
	case setting.SourceFromGerrit:
		ret, err = preloadGerritService(ch, repoName, branchName, remoteName, paths, log)
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		ret, err = preloadGiteeService(ch, repoOwner, repoName, branchName, remoteName, paths, log)
	default:
		return nil, e.ErrPreloadServiceTemplate.AddDesc("Not supported code source")
	}

	return ret, err
}

// LoadServiceFromCodeHost 根据提供的codehost信息加载服务
func LoadServiceFromCodeHost(username string, codehostID int, repoOwner, namespace, repoName, repoUUID, branchName, remoteName string, args *LoadServiceReq, isSync, force, production bool, log *zap.SugaredLogger) error {
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		log.Errorf("Failed to load codehost for preload service list, the error is: %+v", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}

	if isSync {
		service, err := repository.QueryTemplateService(&mongodb.ServiceFindOption{
			ProductName: args.ProductName,
			ServiceName: args.ServicePaths[0].ServiceName,
		}, production)
		if err != nil {
			log.Errorf("Failed to query template service, the error is: %+v", err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}

		if service.ServiceName != args.ServicePaths[0].ServiceName {
			return e.ErrLoadServiceTemplate.AddDesc("service name mismatch")
		}
	}

	switch ch.Type {
	case setting.SourceFromGithub, setting.SourceFromGitlab:
		return loadService(username, ch, repoOwner, namespace, repoName, branchName, args, force, production, log)
	case setting.SourceFromGerrit:
		return loadGerritService(username, ch, repoOwner, repoName, branchName, remoteName, args, force, production, log)
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		return loadGiteeService(username, ch, repoOwner, repoName, branchName, remoteName, args, force, production, log)
	default:
		return e.ErrLoadServiceTemplate.AddDesc("unsupported code source")
	}
}

// 根据repo信息获取gerrit可以加载的服务列表
func preloadGerritService(detail *systemconfig.CodeHost, repoName, branchName, remoteName string, paths []PreLoadServicePath, log *zap.SugaredLogger) ([]LoadServicePath, error) {
	ret := make([]LoadServicePath, 0)

	if remoteName == "" {
		remoteName = "origin"
	}

	base := path.Join(config.S3StoragePath(), strings.Replace(repoName, "/", "-", -1))

	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = command.RunGitCmds(detail, setting.GerritDefaultOwner, setting.GerritDefaultOwner, repoName, branchName, remoteName)
		if err != nil {
			return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
		}
	}

	for _, loadPath := range paths {
		filePath := path.Join(base, loadPath.Path)

		if !loadPath.IsDir {
			if !isYaml(loadPath.Path) {
				log.Error("trying to preload a non-yaml file")
				return nil, e.ErrPreloadServiceTemplate.AddDesc("Non-yaml service loading is not supported")
			}
			pathSegment := strings.Split(loadPath.Path, "/")
			fileName := pathSegment[len(pathSegment)-1]
			ret = append(ret, LoadServicePath{
				ServiceName: getFileName(fileName),
				Path:        loadPath.Path,
				IsDir:       loadPath.IsDir,
			})
		} else {
			fileInfos, err := ioutil.ReadDir(filePath)
			if err != nil {
				log.Error("Failed to read directory info of path: %s, the error is: %+v", filePath, err)
				return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
			}
			if isValidServiceDir(fileInfos) {
				svcName := loadPath.Path
				if loadPath.Path == "" {
					svcName = repoName
				}
				pathList := strings.Split(svcName, "/")
				folderName := pathList[len(pathList)-1]
				ret = append(ret, LoadServicePath{
					ServiceName: folderName,
					Path:        loadPath.Path,
					IsDir:       loadPath.IsDir,
				})
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
						ret = append(ret, LoadServicePath{
							ServiceName: getFileName(file.Name()),
							Path:        loadPath.Path,
							IsDir:       loadPath.IsDir,
						})
						isGrandParent = true
					}
				}
			}
			if !isGrandParent {
				log.Errorf("invalid folder selected since no yaml is presented in path: %s", filePath)
				return ret, e.ErrPreloadServiceTemplate.AddDesc("所选路径下没有yaml，请重新选择")
			}
		}
	}

	return ret, nil
}

// Get a list of services that gitee can load based on repo information
func preloadGiteeService(detail *systemconfig.CodeHost, repoOwner, repoName, branchName, remoteName string, loadPaths []PreLoadServicePath, log *zap.SugaredLogger) ([]LoadServicePath, error) {
	ret := make([]LoadServicePath, 0)

	if remoteName == "" {
		remoteName = "origin"
	}

	base := path.Join(config.S3StoragePath(), strings.Replace(repoName, "/", "-", -1))

	if exist, err := util.PathExists(base); !exist {
		log.Warnf("path does not exist,err:%s", err)
		err = command.RunGitCmds(detail, repoOwner, repoOwner, repoName, branchName, remoteName)
		if err != nil {
			return nil, e.ErrPreloadServiceTemplate.AddDesc(fmt.Sprintf("failed to clone code, err: %s", err.Error()))
		}
	}

	for _, loadPath := range loadPaths {
		filePath := path.Join(base, loadPath.Path)

		if !loadPath.IsDir {
			if !isYaml(loadPath.Path) {
				log.Error("trying to preload a non-yaml file")
				return nil, e.ErrPreloadServiceTemplate.AddDesc("Non-yaml service loading is not supported")
			}
			pathSegment := strings.Split(loadPath.Path, "/")
			fileName := pathSegment[len(pathSegment)-1]
			ret = append(ret, LoadServicePath{
				ServiceName: getFileName(fileName),
				Path:        loadPath.Path,
				IsDir:       loadPath.IsDir,
			})
		} else {
			fileInfos, err := ioutil.ReadDir(filePath)
			if err != nil {
				log.Errorf("Failed to read directory info of path: %s, the error is: %s", filePath, err)
				os.RemoveAll(base)
				return nil, e.ErrPreloadServiceTemplate.AddDesc(err.Error())
			}
			if isValidServiceDir(fileInfos) {
				svcName := loadPath.Path
				if loadPath.Path == "" {
					svcName = repoName
				}
				pathList := strings.Split(svcName, "/")
				folderName := pathList[len(pathList)-1]
				ret = append(ret, LoadServicePath{
					ServiceName: folderName,
					Path:        loadPath.Path,
					IsDir:       loadPath.IsDir,
				})
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
						ret = append(ret, LoadServicePath{
							ServiceName: getFileName(file.Name()),
							Path:        loadPath.Path,
							IsDir:       loadPath.IsDir,
						})
						isGrandParent = true
					}
				}
			}
			if !isGrandParent {
				log.Errorf("invalid folder selected since no yaml is presented in path: %s", filePath)
				return ret, e.ErrPreloadServiceTemplate.AddDesc("所选路径下没有yaml，请重新选择")
			}
		}
	}

	return ret, nil
}

// 根据repo信息从gerrit加载服务
func loadGerritService(username string, ch *systemconfig.CodeHost, repoOwner, repoName, branchName, remoteName string, args *LoadServiceReq, force, production bool, log *zap.SugaredLogger) error {
	if remoteName == "" {
		remoteName = "origin"
	}
	base := path.Join(config.S3StoragePath(), strings.Replace(repoName, "/", "-", -1))
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

	for _, loadPath := range args.ServicePaths {
		filePath := path.Join(base, loadPath.Path)
		if !loadPath.IsDir {
			contentBytes, err := ioutil.ReadFile(path.Join(base, loadPath.Path))
			if err != nil {
				log.Errorf("Failed to read file of path: %s, the error is: %+v", loadPath.Path, err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}
			splittedYaml := util.SplitYaml(string(contentBytes))
			// FIXME：gerrit原先有字段保存codehost信息，保存两份，兼容性
			createSvcArgs := &models.Service{
				CodehostID:       ch.ID,
				RepoName:         repoName,
				RepoOwner:        repoOwner,
				RepoNamespace:    repoOwner, // TODO gerrit need set namespace?
				BranchName:       branchName,
				LoadPath:         loadPath.Path,
				LoadFromDir:      loadPath.IsDir,
				GerritBranchName: branchName,
				GerritCodeHostID: ch.ID,
				GerritPath:       filePath,
				GerritRemoteName: remoteName,
				GerritRepoName:   repoName,
				KubeYamls:        splittedYaml,
				CreateBy:         username,
				ServiceName:      loadPath.ServiceName,
				Type:             args.Type,
				ProductName:      args.ProductName,
				Source:           setting.SourceFromGerrit,
				Yaml:             string(contentBytes),
				Commit:           commitInfo,
			}
			_, err = CreateServiceTemplate(username, createSvcArgs, force, production, log)
			if err != nil {
				_, messageMap := e.ErrorMessage(err)
				if description, ok := messageMap["description"]; ok {
					return e.ErrLoadServiceTemplate.AddDesc(description.(string))
				}
				return e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
			}
		} else {
			fileInfos, err := ioutil.ReadDir(filePath)
			if err != nil {
				log.Errorf("Failed to read directory info of path: %s, the error is: %+v", filePath, err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}
			if isValidServiceDir(fileInfos) {
				return loadServiceFromGerrit(fileInfos, ch.ID, username, branchName, loadPath.Path, loadPath.ServiceName, filePath, repoOwner, remoteName, repoName, args.Type, args.ProductName, loadPath.IsDir, commitInfo, force, production, log)
			} else {
				return e.ErrLoadServiceTemplate.AddDesc(fmt.Sprintf("%s 路径下没有yaml文件，请重新选择", loadPath.Path))
			}
			// for _, entry := range fileInfos {
			// 	subtreeLoadPath := fmt.Sprintf("%s/%s", loadPath.Path, entry.Name())
			// 	subtreePath := fmt.Sprintf("%s/%s", filePath, entry.Name())
			// 	subtreeInfo, err := ioutil.ReadDir(subtreePath)
			// 	if err != nil {
			// 		log.Errorf("Failed to read subdir info from gerrit package of path: %s, the error is: %+v", subtreePath, err)
			// 		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			// 	}
			// 	if isValidServiceDir(subtreeInfo) {
			// 		if err := loadServiceFromGerrit(subtreeInfo, ch.ID, username, branchName, subtreeLoadPath, subtreePath, repoOwner, remoteName, repoName, args, commitInfo, force, production, log); err != nil {
			// 			return err
			// 		}
			// 	}
			// }
		}
	}
	return nil
}

func loadServiceFromGerrit(tree []os.FileInfo, id int, username, branchName, loadPath, serviceName, path, repoOwner, remoteName, repoName, svcType, projectName string, isDir bool, commit *models.Commit, force, production bool, log *zap.SugaredLogger) error {
	var splittedYaml []string
	yamlList, err := extractYamls(path, tree)
	if err != nil {
		log.Errorf("Failed to extract yamls from gerrit package, the error is: %+v", err)
		return err
	}
	for _, yamlEntry := range yamlList {
		splittedYaml = append(splittedYaml, util.SplitYaml(yamlEntry)...)
	}
	yml := util.JoinYamls(yamlList)
	createSvcArgs := &models.Service{
		CodehostID:       id,
		BranchName:       branchName,
		RepoName:         repoName,
		RepoOwner:        repoOwner,
		LoadPath:         loadPath,
		LoadFromDir:      isDir,
		GerritRepoName:   repoName,
		GerritCodeHostID: id,
		GerritRemoteName: remoteName,
		GerritPath:       path,
		GerritBranchName: branchName,
		KubeYamls:        splittedYaml,
		CreateBy:         username,
		ServiceName:      serviceName,
		Type:             svcType,
		ProductName:      projectName,
		Source:           setting.SourceFromGerrit,
		Yaml:             yml,
		Commit:           commit,
	}

	_, err = CreateServiceTemplate(username, createSvcArgs, force, production, log)
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

// Load services from gitee based on repo information
func loadGiteeService(username string, ch *systemconfig.CodeHost, repoOwner, repoName, branchName, remoteName string, args *LoadServiceReq, force, production bool, log *zap.SugaredLogger) error {
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

	for _, loadPath := range args.ServicePaths {
		filePath := path.Join(base, loadPath.Path)
		if !loadPath.IsDir {
			contentBytes, err := ioutil.ReadFile(path.Join(base, loadPath.Path))
			if err != nil {
				log.Errorf("Failed to read file of path: %s, the error is: %s", loadPath.Path, err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}
			splittedYaml := util.SplitYaml(string(contentBytes))
			srcPath := fmt.Sprintf("%s/%s/%s/blob/%s/%s", ch.Address, repoOwner, repoName, branchName, loadPath.Path)
			createSvcArgs := &models.Service{
				CodehostID:  ch.ID,
				RepoName:    repoName,
				RepoOwner:   repoOwner,
				BranchName:  branchName,
				SrcPath:     srcPath,
				LoadPath:    loadPath.Path,
				LoadFromDir: loadPath.IsDir,
				KubeYamls:   splittedYaml,
				CreateBy:    username,
				ServiceName: loadPath.ServiceName,
				Type:        args.Type,
				ProductName: args.ProductName,
				Source:      setting.SourceFromGitee,
				Yaml:        string(contentBytes),
				Commit:      commitInfo,
			}
			_, err = CreateServiceTemplate(username, createSvcArgs, force, production, log)
			if err != nil {
				_, messageMap := e.ErrorMessage(err)
				if description, ok := messageMap["description"]; ok {
					return e.ErrLoadServiceTemplate.AddDesc(description.(string))
				}
				return e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
			}
		} else {
			fileInfos, err := ioutil.ReadDir(filePath)
			if err != nil {
				log.Errorf("Failed to read directory info of path: %s, the error is: %s", filePath, err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}
			if isValidServiceDir(fileInfos) {
				return loadServiceFromGitee(fileInfos, ch, username, branchName, loadPath.Path, loadPath.ServiceName, filePath, repoOwner, remoteName, repoName, args.Type, args.ProductName, loadPath.IsDir, commitInfo, force, production, log)
			} else {
				return e.ErrLoadServiceTemplate.AddDesc(fmt.Sprintf("%s 路径下没有yaml文件，请重新选择", loadPath.Path))
			}

			// for _, entry := range fileInfos {
			// 	subtreeLoadPath := fmt.Sprintf("%s/%s", args.LoadPath, entry.Name())
			// 	subtreePath := fmt.Sprintf("%s/%s", filePath, entry.Name())
			// 	subtreeInfo, err := ioutil.ReadDir(subtreePath)
			// 	if err != nil {
			// 		log.Errorf("Failed to read subdir info from gitee package of path: %s, the error is: %s", subtreePath, err)
			// 		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			// 	}
			// 	if isValidServiceDir(subtreeInfo) {
			// 		if err := loadServiceFromGitee(subtreeInfo, ch, username, branchName, subtreeLoadPath, subtreePath, repoOwner, remoteName, repoName, args, commitInfo, force, production, log); err != nil {
			// 			return err
			// 		}
			// 	}
			// }
		}
	}
	return nil
}

func loadServiceFromGitee(tree []os.FileInfo, ch *systemconfig.CodeHost, username, branchName, loadPath, serviceName, path, repoOwner, remoteName, repoName, svcType, projectName string, isDir bool, commit *models.Commit, force, production bool, log *zap.SugaredLogger) error {
	var splittedYaml []string
	repoInfo := fmt.Sprintf("%s/%s", repoOwner, repoName)
	yamlList, err := extractYamls(path, tree)
	if err != nil {
		log.Errorf("Failed to extract yamls from gitee package, the error is: %+v", err)
		return err
	}
	for _, yamlEntry := range yamlList {
		splittedYaml = append(splittedYaml, util.SplitYaml(yamlEntry)...)
	}
	yml := util.JoinYamls(yamlList)
	srcPath := fmt.Sprintf("%s/%s/tree/%s/%s", ch.Address, repoInfo, branchName, loadPath)
	createSvcArgs := &models.Service{
		CodehostID:  ch.ID,
		BranchName:  branchName,
		RepoName:    repoName,
		RepoOwner:   repoOwner,
		LoadPath:    loadPath,
		SrcPath:     srcPath,
		LoadFromDir: isDir,
		KubeYamls:   splittedYaml,
		CreateBy:    username,
		ServiceName: serviceName,
		Type:        svcType,
		ProductName: projectName,
		Source:      setting.SourceFromGitee,
		Yaml:        yml,
		Commit:      commit,
	}

	_, err = CreateServiceTemplate(username, createSvcArgs, force, production, log)
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
