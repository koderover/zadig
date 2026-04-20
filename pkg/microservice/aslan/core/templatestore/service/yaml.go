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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commmonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/template"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/converter"
	yamlutil "github.com/koderover/zadig/v2/pkg/util/yaml"
)

var DefaultSystemVariable = map[string]string{
	setting.TemplateVariableProduct: setting.TemplateVariableProductDescription,
	setting.TemplateVariableService: setting.TemplateVariableServiceDescription,
}

type LoadYamlTemplateFromCodeHostReq struct {
	Paths []LoadYamlTemplatePath `json:"paths"`
}

type LoadYamlTemplatePath struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type PreloadYamlTemplatePath struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type loadedYamlTemplateContent struct {
	Path    LoadYamlTemplatePath
	Content string
	Commit  *models.Commit
}

func PreloadYamlTemplateFromCodeHost(codehostID int, repoOwner, repoName, repoUUID, branchName, remoteName string, loadPaths []PreloadYamlTemplatePath, logger *zap.SugaredLogger) ([]LoadYamlTemplatePath, error) {
	var templates []LoadYamlTemplatePath

	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		logger.Errorf("failed to get codehost %d for yaml template preload, err: %s", codehostID, err)
		return nil, fmt.Errorf("failed to get codehost %d, err: %w", codehostID, err)
	}

	switch ch.Type {
	case setting.SourceFromGithub, setting.SourceFromGitlab:
		templates, err = preloadYamlTemplatesFromTreeGetter(ch, repoOwner, repoName, branchName, loadPaths, logger)
	case setting.SourceFromGerrit:
		templates, err = preloadYamlTemplatesFromGerrit(ch, repoOwner, repoName, branchName, remoteName, loadPaths, logger)
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		templates, err = preloadYamlTemplatesFromGitee(ch, repoOwner, repoName, branchName, remoteName, loadPaths, logger)
	default:
		logger.Errorf("unsupported code source: %s", ch.Type)
		return nil, fmt.Errorf("unsupported code source: %s", ch.Type)
	}

	return templates, err
}

func preloadYamlTemplatesFromTreeGetter(ch *systemconfig.CodeHost, repoOwner, repoName, branchName string, paths []PreloadYamlTemplatePath, logger *zap.SugaredLogger) ([]LoadYamlTemplatePath, error) {
	logger.Infof("Preloading yaml template from codehost %d with namespace %s, repo %s, branch %s and path %v", ch.ID, repoOwner, repoName, branchName, paths)

	loader, err := commonutil.GetYAMLLoader(ch)
	if err != nil {
		logger.Errorf("Failed to create loader client, err: %s", err)
		return nil, err
	}

	resp := make([]LoadYamlTemplatePath, 0)
	for _, loadPath := range paths {
		if !loadPath.IsDir {
			if !commonutil.IsYaml(loadPath.Path) {
				return nil, fmt.Errorf("file is not of type yaml or yml, select again")
			}

			resp = append(resp, LoadYamlTemplatePath{
				Name:  getFileName(loadPath.Path),
				Path:  loadPath.Path,
				IsDir: loadPath.IsDir,
			})
		} else {
			treeNodes, err := loader.GetTree(repoOwner, repoName, loadPath.Path, branchName)
			if err != nil {
				logger.Errorf("failed to get tree under path %s, err: %s", loadPath.Path, err)
				return nil, fmt.Errorf("failed to get tree under path %s, err: %s", loadPath.Path, err)
			}

			folders, files := commonutil.GetFoldersAndYAMLFiles(treeNodes)

			if len(files) > 0 {
				resp = append(resp, LoadYamlTemplatePath{
					Name:  getFileName(loadPath.Path),
					Path:  loadPath.Path,
					IsDir: loadPath.IsDir,
				})
			} else if len(folders) > 0 {
				for _, f := range folders {
					tns, err := loader.GetTree(repoOwner, repoName, f.FullPath, branchName)
					if err != nil {
						logger.Errorf("Failed to get tree under path %s, err: %s", f.FullPath, err)
						return nil, fmt.Errorf("Failed to get tree under path %s, err: %s", f.FullPath, err)
					}

					if commonutil.HasYAMLFiles(tns) {
						resp = append(resp, LoadYamlTemplatePath{
							Name:  getFileName(f.FullPath),
							Path:  f.FullPath,
							IsDir: f.IsDir,
						})
					}
				}
			}
		}
	}

	if len(resp) == 0 {
		logger.Errorf("no valid yaml is found under paths %v", paths)
		return nil, fmt.Errorf("no valid yaml is found under paths")
	}

	return resp, nil
}

func preloadYamlTemplatesFromGerrit(ch *systemconfig.CodeHost, repoOwner, repoName, branchName, remoteName string, paths []PreloadYamlTemplatePath, logger *zap.SugaredLogger) ([]LoadYamlTemplatePath, error) {
	resp := make([]LoadYamlTemplatePath, 0)

	if remoteName == "" {
		remoteName = "origin"
	}

	base := path.Join(config.S3StoragePath(), strings.Replace(repoName, "/", "-", -1))

	if _, err := os.Stat(base); os.IsNotExist(err) {
		err = command.RunGitCmds(ch, setting.GerritDefaultOwner, setting.GerritDefaultOwner, repoName, branchName, remoteName)
		if err != nil {
			return nil, err
		}
	}

	for _, loadPath := range paths {
		filePath := path.Join(base, loadPath.Path)

		if !loadPath.IsDir {
			if !commonutil.IsYaml(loadPath.Path) {
				logger.Errorf("file is not of type yaml or yml, select again")
				return nil, fmt.Errorf("file is not of type yaml or yml, select again")
			}

			pathSegment := strings.Split(loadPath.Path, "/")
			fileName := pathSegment[len(pathSegment)-1]

			resp = append(resp, LoadYamlTemplatePath{
				Name:  getFileName(fileName),
				Path:  loadPath.Path,
				IsDir: loadPath.IsDir,
			})
		} else {
			fileInfos, err := ioutil.ReadDir(filePath)
			if err != nil {
				logger.Errorf("failed to read yaml directory %s, err: %s", loadPath.Path, err)
				return nil, fmt.Errorf("failed to read yaml directory %s, err: %s", loadPath.Path, err)
			}

			if commonutil.IsValidServiceDir(fileInfos) {
				name := loadPath.Path
				if loadPath.Path == "" {
					name = repoName
				}

				pathList := strings.Split(name, "/")
				folderName := pathList[len(pathList)-1]
				resp = append(resp, LoadYamlTemplatePath{
					Name:  folderName,
					Path:  loadPath.Path,
					IsDir: loadPath.IsDir,
				})
				return resp, nil
			}

			isGrandParent := false
			for _, file := range fileInfos {
				if file.IsDir() {
					subDirPath := fmt.Sprintf("%s/%s", filePath, file.Name())
					subtree, err := ioutil.ReadDir(subDirPath)
					if err != nil {
						logger.Errorf("failed to get subtree fromwith path %s, err: %s", subDirPath, err)
						return nil, fmt.Errorf("failed to get subtree fromwith path %s, err: %s", subDirPath, err)
					}

					if commonutil.IsValidServiceDir(subtree) {
						resp = append(resp, LoadYamlTemplatePath{
							Name:  getFileName(file.Name()),
							Path:  loadPath.Path,
							IsDir: loadPath.IsDir,
						})
						isGrandParent = true
					}
				}
			}

			if !isGrandParent {
				logger.Errorf("no valid yaml is found under directory %s", filePath)
				return nil, fmt.Errorf("no valid yaml is found under directory %s", filePath)
			}
		}
	}

	return resp, nil
}

func preloadYamlTemplatesFromGitee(ch *systemconfig.CodeHost, repoOwner, repoName, branchName, remoteName string, paths []PreloadYamlTemplatePath, logger *zap.SugaredLogger) ([]LoadYamlTemplatePath, error) {
	resp := make([]LoadYamlTemplatePath, 0)

	if remoteName == "" {
		remoteName = "origin"
	}

	base := path.Join(config.S3StoragePath(), strings.Replace(repoName, "/", "-", -1))

	if exist, err := util.PathExists(base); !exist {
		logger.Warnf("path does not exist,err:%s", err)
		err = command.RunGitCmds(ch, repoOwner, repoOwner, repoName, branchName, remoteName)
		if err != nil {
			return nil, fmt.Errorf("failed to clone code, err: %s", err.Error())
		}
	}

	for _, loadPath := range paths {
		filePath := path.Join(base, loadPath.Path)

		if !loadPath.IsDir {
			if !commonutil.IsYaml(loadPath.Path) {
				logger.Errorf("file is not of type yaml or yml, select again")
				return nil, fmt.Errorf("file is not of type yaml or yml, select again")
			}

			pathSegment := strings.Split(loadPath.Path, "/")
			fileName := pathSegment[len(pathSegment)-1]

			resp = append(resp, LoadYamlTemplatePath{
				Name:  getFileName(fileName),
				Path:  loadPath.Path,
				IsDir: loadPath.IsDir,
			})
		} else {
			fileInfos, err := ioutil.ReadDir(filePath)
			if err != nil {
				logger.Errorf("failed to read yaml directory %s, err: %s", loadPath.Path, err)
				os.RemoveAll(base)
				return nil, fmt.Errorf("failed to read yaml directory %s, err: %s", loadPath.Path, err)
			}

			if commonutil.IsValidServiceDir(fileInfos) {
				name := loadPath.Path
				if loadPath.Path == "" {
					name = repoName
				}
				pathList := strings.Split(name, "/")
				folderName := pathList[len(pathList)-1]
				resp = append(resp, LoadYamlTemplatePath{
					Name:  folderName,
					Path:  loadPath.Path,
					IsDir: loadPath.IsDir,
				})
				return resp, nil
			}

			isGrandParent := false
			for _, file := range fileInfos {
				if file.IsDir() {
					subDirPath := fmt.Sprintf("%s/%s", filePath, file.Name())
					subtree, err := ioutil.ReadDir(subDirPath)
					if err != nil {
						logger.Errorf("failed to get subtree fromwith path %s, err: %s", subDirPath, err)
						return nil, fmt.Errorf("failed to get subtree fromwith path %s, err: %s", subDirPath, err)
					}
					if commonutil.IsValidServiceDir(subtree) {
						resp = append(resp, LoadYamlTemplatePath{
							Name:  getFileName(file.Name()),
							Path:  loadPath.Path,
							IsDir: loadPath.IsDir,
						})
						isGrandParent = true
					}
				}
			}
			if !isGrandParent {
				logger.Errorf("no valid yaml is found under directory %s", filePath)
				return nil, fmt.Errorf("no valid yaml is found under directory %s", filePath)
			}
		}
	}

	return resp, nil
}

func getFileName(fullName string) string {
	name := filepath.Base(fullName)
	ext := filepath.Ext(name)
	return name[0:(len(name) - len(ext))]
}

func LoadYamlTemplateFromCodeHost(username string, codehostID int, repoOwner, namespace, repoName, repoUUID, branchName, remoteName string, args *LoadYamlTemplateFromCodeHostReq, isSync bool, logger *zap.SugaredLogger) error {
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		logger.Errorf("failed to get codehost %d, err: %s", codehostID, err)
		return fmt.Errorf("failed to get codehost %d, err: %s", codehostID, err)
	}

	if isSync {
		yamlTemplate, err := commonrepo.NewYamlTemplateColl().GetByName(args.Paths[0].Name)
		if err != nil {
			logger.Errorf("failed to query yaml template, err: %s", err)
			return fmt.Errorf("failed to query yaml template, err: %w", err)
		}

		if yamlTemplate.Name != args.Paths[0].Name {
			return fmt.Errorf("yaml template name mismatch")
		}
	}

	switch ch.Type {
	case setting.SourceFromGithub, setting.SourceFromGitlab:
		return loadYamlTemplateFromTreeGetter(ch, repoOwner, namespace, repoName, branchName, remoteName, args, isSync, logger)
	case setting.SourceFromGerrit:
		return loadYamlTemplateFromGerrit(ch, repoOwner, namespace, repoName, branchName, remoteName, args, isSync, logger)
	case setting.SourceFromGitee, setting.SourceFromGiteeEE:
		return loadYamlTemplateFromGitee(ch, repoOwner, namespace, repoName, branchName, remoteName, args, isSync, logger)
	default:
		logger.Errorf("unsupported code source: %s", ch.Type)
		return fmt.Errorf("unsupported code source: %s", ch.Type)
	}
}

func loadYamlTemplateFromTreeGetter(ch *systemconfig.CodeHost, repoOwner, namespace, repoName, branchName, remoteName string, args *LoadYamlTemplateFromCodeHostReq, isSync bool, logger *zap.SugaredLogger) error {
	logger.Infof("Loading yaml template from codehost %d with owner %s, namespace %s, repo %s, branch %s and path %v", ch.ID, repoOwner, namespace, repoName, branchName, args.Paths)

	loader, err := commonutil.GetYAMLLoader(ch)
	if err != nil {
		logger.Errorf("failed to get yaml loader for codehost %d, err: %s", ch.ID, err)
		return err
	}

	contents := make([]*loadedYamlTemplateContent, 0, len(args.Paths))
	for _, loadPath := range args.Paths {
		if !loadPath.IsDir {
			yamls, err := loader.GetYAMLContents(namespace, repoName, loadPath.Path, branchName, false, false)
			if err != nil {
				logger.Errorf("failed to get yaml content under path %s, err: %s", loadPath.Path, err)
				return err
			}
			contents = append(contents, &loadedYamlTemplateContent{
				Path:    loadPath,
				Content: util.CombineManifests(yamls),
			})
		} else {
			treeNodes, err := loader.GetTree(namespace, repoName, loadPath.Path, branchName)
			if err != nil {
				logger.Errorf("failed to get tree under path %s, err: %s", loadPath.Path, err)
				return err
			}

			_, files := commonutil.GetFoldersAndYAMLFiles(treeNodes)

			if len(files) > 0 {
				yamls := make([]string, 0)
				for _, file := range files {
					res, err := loader.GetYAMLContents(namespace, repoName, file.FullPath, branchName, false, false)
					if err != nil {
						logger.Errorf("failed to get yaml content under path %s, err: %s", file.FullPath, err)
						return err
					}
					yamls = append(yamls, res...)
				}
				contents = append(contents, &loadedYamlTemplateContent{
					Path:    loadPath,
					Content: util.CombineManifests(yamls),
				})
			} else {
				logger.Errorf("no yaml file is found under directory %s", loadPath.Path)
				return fmt.Errorf("no yaml file is found under directory %s", loadPath.Path)
			}
		}
	}

	for _, content := range contents {

		if len(content.Content) == 0 {
			continue
		}

		commit, err := loader.GetLatestRepositoryCommit(namespace, repoName, content.Path.Path, branchName)
		if err != nil {
			logger.Errorf("failed to get latest commit under path %s, err: %s", content.Path.Path, err)
			return err
		}
		commitInfo := &models.Commit{SHA: commit.SHA, Message: commit.Message}

		templateObj := &template.YamlTemplate{
			Source:      ch.Type,
			CodehostID:  ch.ID,
			RepoOwner:   repoOwner,
			Namespace:   namespace,
			RepoName:    repoName,
			BranchName:  branchName,
			RemoteName:  remoteName,
			Name:        content.Path.Name,
			Content:     content.Content,
			Path:        content.Path.Path,
			LoadFromDir: content.Path.IsDir,
			Commit:      commitInfo,
		}

		if isSync {
			origin, err := commonrepo.NewYamlTemplateColl().GetByName(templateObj.Name)
			if err != nil {
				return fmt.Errorf("failed to find template by name: %s, err: %w", templateObj.Name, err)
			}
			if err := UpdateYamlTemplate(origin.ID.Hex(), templateObj, logger); err != nil {
				return err
			}
			continue
		} else {
			if err := CreateYamlTemplate(templateObj, logger); err != nil {
				return err
			}
		}
	}

	return nil
}

func loadYamlTemplateFromGerrit(ch *systemconfig.CodeHost, repoOwner, namespace, repoName, branchName, remoteName string, args *LoadYamlTemplateFromCodeHostReq, isSync bool, logger *zap.SugaredLogger) error {
	if remoteName == "" {
		remoteName = "origin"
	}

	base := path.Join(config.S3StoragePath(), strings.Replace(repoName, "/", "-", -1))
	_ = os.RemoveAll(base)

	if err := command.RunGitCmds(ch, repoOwner, repoOwner, repoName, branchName, remoteName); err != nil {
		logger.Errorf("failed to clone gerrit repo %s branch %s, err: %s", repoName, branchName, err)
		return err
	}

	gerritCli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
	commit, err := gerritCli.GetCommitByBranch(repoName, branchName)
	if err != nil {
		logger.Errorf("failed to get latest commit info from repo %s, err: %s", repoName, err)
		return err
	}
	commitInfo := &models.Commit{
		SHA:     commit.Commit,
		Message: commit.Message,
	}

	for _, loadPath := range args.Paths {
		if !loadPath.IsDir {

			content, err := os.ReadFile(path.Join(base, loadPath.Path))
			if err != nil {
				logger.Errorf("failed to read yaml file %s, err: %s", loadPath.Path, err)
				return err
			}
			templateObj := &template.YamlTemplate{
				Source:      ch.Type,
				CodehostID:  ch.ID,
				RepoOwner:   repoOwner,
				Namespace:   namespace,
				RepoName:    repoName,
				BranchName:  branchName,
				RemoteName:  remoteName,
				Name:        loadPath.Name,
				Content:     string(content),
				Path:        loadPath.Path,
				LoadFromDir: loadPath.IsDir,
				Commit:      commitInfo,
			}

			if isSync {
				origin, err := commonrepo.NewYamlTemplateColl().GetByName(templateObj.Name)
				if err != nil {
					return fmt.Errorf("failed to find template by name: %s, err: %w", templateObj.Name, err)
				}
				if err := UpdateYamlTemplate(origin.ID.Hex(), templateObj, logger); err != nil {
					return err
				}
			} else if err := CreateYamlTemplate(templateObj, logger); err != nil {
				return err
			}

		} else {
			filePath := path.Join(base, loadPath.Path)
			fileInfos, err := ioutil.ReadDir(filePath)
			if err != nil {
				logger.Errorf("failed to read yaml directory %s, err: %s", loadPath.Path, err)
				return err
			}
			if commonutil.IsValidServiceDir(fileInfos) {
				yamls := make([]string, 0)
				for _, fileInfo := range fileInfos {
					if fileInfo.IsDir() || !commonutil.IsYaml(fileInfo.Name()) {
						continue
					}

					content, err := os.ReadFile(path.Join(filePath, fileInfo.Name()))
					if err != nil {
						logger.Errorf("failed to read yaml file %s, err: %s", path.Join(loadPath.Path, fileInfo.Name()), err)
						return err
					}
					yamls = append(yamls, string(content))
				}

				templateObj := &template.YamlTemplate{
					Source:      ch.Type,
					CodehostID:  ch.ID,
					RepoOwner:   repoOwner,
					Namespace:   namespace,
					RepoName:    repoName,
					BranchName:  branchName,
					RemoteName:  remoteName,
					Name:        loadPath.Name,
					Content:     util.CombineManifests(yamls),
					Path:        loadPath.Path,
					LoadFromDir: loadPath.IsDir,
					Commit:      commitInfo,
				}

				if isSync {
					origin, err := commonrepo.NewYamlTemplateColl().GetByName(templateObj.Name)
					if err != nil {
						return fmt.Errorf("failed to find template by name: %s, err: %w", templateObj.Name, err)
					}
					if err := UpdateYamlTemplate(origin.ID.Hex(), templateObj, logger); err != nil {
						return err
					}
				} else if err := CreateYamlTemplate(templateObj, logger); err != nil {
					return err
				}
			} else {
				logger.Errorf("no valid yaml is found under directory %s", loadPath.Path)
				return fmt.Errorf("no valid yaml is found under directory %s", loadPath.Path)
			}
		}
	}
	return nil
}

func loadYamlTemplateFromGitee(ch *systemconfig.CodeHost, repoOwner, namespace, repoName, branchName, remoteName string, args *LoadYamlTemplateFromCodeHostReq, isSync bool, logger *zap.SugaredLogger) error {
	if remoteName == "" {
		remoteName = "origin"
	}

	base := path.Join(config.S3StoragePath(), repoName)
	if _, err := os.Stat(base); os.IsNotExist(err) {
		if err := command.RunGitCmds(ch, repoOwner, repoName, repoName, branchName, remoteName); err != nil {
			logger.Errorf("failed to clone gitee repo %s/%s branch %s, err: %s", repoOwner, repoName, branchName, err)
			return err
		}
	}

	giteeCli := gitee.NewClient(ch.ID, ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
	branch, err := giteeCli.GetSingleBranch(ch.Address, ch.AccessToken, repoOwner, repoName, branchName)
	if err != nil {
		logger.Errorf("failed to get latest commit info from repo %s, err: %s", repoName, err)
		return err
	}
	commitInfo := &models.Commit{
		SHA:     branch.Commit.Sha,
		Message: branch.Commit.Commit.Message,
	}

	for _, loadPath := range args.Paths {
		if !loadPath.IsDir {
			content, err := os.ReadFile(path.Join(base, loadPath.Path))
			if err != nil {
				logger.Errorf("failed to read yaml file %s, err: %s", loadPath.Path, err)
				return err
			}
			templateObj := &template.YamlTemplate{
				Source:      ch.Type,
				CodehostID:  ch.ID,
				RepoOwner:   repoOwner,
				Namespace:   namespace,
				RepoName:    repoName,
				BranchName:  branchName,
				RemoteName:  remoteName,
				Name:        loadPath.Name,
				Content:     string(content),
				Path:        loadPath.Path,
				LoadFromDir: loadPath.IsDir,
				Commit:      commitInfo,
			}

			if isSync {
				origin, err := commonrepo.NewYamlTemplateColl().GetByName(templateObj.Name)
				if err != nil {
					return fmt.Errorf("failed to find template by name: %s, err: %w", templateObj.Name, err)
				}
				if err := UpdateYamlTemplate(origin.ID.Hex(), templateObj, logger); err != nil {
					return err
				}
			} else if err := CreateYamlTemplate(templateObj, logger); err != nil {
				return err
			}

		} else {
			filePath := path.Join(base, loadPath.Path)
			fileInfos, err := ioutil.ReadDir(filePath)
			if err != nil {
				logger.Errorf("failed to read yaml directory %s, err: %s", loadPath.Path, err)
				return err
			}
			if commonutil.IsValidServiceDir(fileInfos) {
				yamls := make([]string, 0)
				for _, fileInfo := range fileInfos {
					if fileInfo.IsDir() || !commonutil.IsYaml(fileInfo.Name()) {
						continue
					}

					content, err := os.ReadFile(path.Join(filePath, fileInfo.Name()))
					if err != nil {
						logger.Errorf("failed to read yaml file %s, err: %s", path.Join(loadPath.Path, fileInfo.Name()), err)
						return err
					}
					yamls = append(yamls, string(content))
				}

				templateObj := &template.YamlTemplate{
					Source:      ch.Type,
					CodehostID:  ch.ID,
					RepoOwner:   repoOwner,
					Namespace:   namespace,
					RepoName:    repoName,
					BranchName:  branchName,
					RemoteName:  remoteName,
					Name:        loadPath.Name,
					Content:     util.CombineManifests(yamls),
					Path:        loadPath.Path,
					LoadFromDir: loadPath.IsDir,
					Commit:      commitInfo,
				}

				if isSync {
					origin, err := commonrepo.NewYamlTemplateColl().GetByName(templateObj.Name)
					if err != nil {
						return fmt.Errorf("failed to find template by name: %s, err: %w", templateObj.Name, err)
					}
					if err := UpdateYamlTemplate(origin.ID.Hex(), templateObj, logger); err != nil {
						return err
					}
				} else if err := CreateYamlTemplate(templateObj, logger); err != nil {
					return err
				}
			} else {
				logger.Errorf("no valid yaml is found under directory %s", loadPath.Path)
				return fmt.Errorf("no valid yaml is found under directory %s", loadPath.Path)
			}
		}
	}

	return nil
}

func CreateYamlTemplate(template *template.YamlTemplate, logger *zap.SugaredLogger) error {
	extractVariableYmal, err := yamlutil.ExtractVariableYaml(template.Content)
	if err != nil {
		return fmt.Errorf("failed to extract variable yaml from service yaml, err: %w", err)
	}
	extractServiceVariableKVs, err := commontypes.YamlToServiceVariableKV(extractVariableYmal, nil)
	if err != nil {
		return fmt.Errorf("failed to convert variable yaml to service variable kv, err: %w", err)
	}

	created := &models.YamlTemplate{
		Name:               template.Name,
		Content:            template.Content,
		Source:             template.Source,
		RepoOwner:          template.RepoOwner,
		Namespace:          template.Namespace,
		RepoName:           template.RepoName,
		Path:               template.Path,
		BranchName:         template.BranchName,
		RemoteName:         template.RemoteName,
		CodeHostID:         template.CodehostID,
		LoadFromDir:        template.LoadFromDir,
		Commit:             template.Commit,
		VariableYaml:       extractVariableYmal,
		ServiceVariableKVs: extractServiceVariableKVs,
	}

	err = commonrepo.NewYamlTemplateColl().Create(created)
	if err != nil {
		logger.Errorf("create dockerfile template error: %s", err)
		return err
	}

	if err := commmonservice.ProcessYamlTemplateWebhook(created, nil, logger); err != nil {
		logger.Errorf("failed to process yaml template webhook for create %s, err: %s", created.Name, err)
	}
	return nil
}

func UpdateYamlTemplate(id string, template *template.YamlTemplate, logger *zap.SugaredLogger) error {
	extractVariableYmal, err := yamlutil.ExtractVariableYaml(template.Content)
	if err != nil {
		return fmt.Errorf("failed to extract variable yaml from service yaml, err: %w", err)
	}
	extractServiceVariableKVs, err := commontypes.YamlToServiceVariableKV(extractVariableYmal, nil)
	if err != nil {
		return fmt.Errorf("failed to convert variable yaml to service variable kv, err: %w", err)
	}

	origin, err := commonrepo.NewYamlTemplateColl().GetById(id)
	if err != nil {
		return fmt.Errorf("failed to find template by id: %s, err: %w", id, err)
	}

	template.VariableYaml, template.ServiceVariableKVs, err = commontypes.MergeServiceVariableKVsIfNotExist(origin.ServiceVariableKVs, extractServiceVariableKVs)
	if err != nil {
		return fmt.Errorf("failed to merge service variables, err %w", err)
	}

	updated := &models.YamlTemplate{
		Name:               template.Name,
		Content:            template.Content,
		Source:             template.Source,
		CodeHostID:         template.CodehostID,
		RepoOwner:          template.RepoOwner,
		Namespace:          template.Namespace,
		RepoName:           template.RepoName,
		BranchName:         template.BranchName,
		RemoteName:         template.RemoteName,
		LoadFromDir:        template.LoadFromDir,
		Path:               template.Path,
		Commit:             template.Commit,
		VariableYaml:       template.VariableYaml,
		ServiceVariableKVs: template.ServiceVariableKVs,
	}

	err = commonrepo.NewYamlTemplateColl().Update(id, updated)
	if err != nil {
		logger.Errorf("update yaml template error: %s", err)
		return err
	}

	if err := commmonservice.ProcessYamlTemplateWebhook(updated, origin, logger); err != nil {
		logger.Errorf("failed to process yaml template webhook for update %s, err: %s", updated.Name, err)
	}
	return nil
}

func UpdateYamlTemplateVariable(id string, template *template.YamlTemplate, logger *zap.SugaredLogger) error {
	origin, err := commonrepo.NewYamlTemplateColl().GetById(id)
	if err != nil {
		return fmt.Errorf("failed to find template by id: %s, err: %w", id, err)
	}

	_, err = commonutil.RenderK8sSvcYamlStrict(origin.Content, "FakeProjectName", template.Name, template.VariableYaml)
	if err != nil {
		return fmt.Errorf("failed to validate variable, err: %s", err)
	}

	err = commonrepo.NewYamlTemplateColl().UpdateVariable(id, template.VariableYaml, template.ServiceVariableKVs)
	if err != nil {
		logger.Errorf("update yaml template variable error: %s", err)
	}
	return err
}

func ListYamlTemplate(pageNum, pageSize int, logger *zap.SugaredLogger) ([]*template.YamlListObject, int, error) {
	resp := make([]*template.YamlListObject, 0)
	templateList, total, err := commonrepo.NewYamlTemplateColl().List(pageNum, pageSize)
	if err != nil {
		logger.Errorf("list yaml template error: %s", err)
		return resp, 0, err
	}
	for _, obj := range templateList {
		resp = append(resp, &template.YamlListObject{
			ID:          obj.ID.Hex(),
			Name:        obj.Name,
			Source:      obj.Source,
			CodehostID:  obj.CodeHostID,
			RepoOwner:   obj.RepoOwner,
			Namespace:   obj.Namespace,
			Repo:        obj.RepoName,
			Path:        obj.Path,
			Branch:      obj.BranchName,
			RemoteName:  obj.RemoteName,
			LoadFromDir: obj.LoadFromDir,
			Commit:      obj.Commit,
		})
	}
	return resp, total, err
}

func GetYamlTemplateDetail(id string, logger *zap.SugaredLogger) (*template.YamlDetail, error) {
	resp := new(template.YamlDetail)
	yamlTemplate, err := commonrepo.NewYamlTemplateColl().GetById(id)
	if err != nil {
		logger.Errorf("Failed to get dockerfile template from id: %s, the error is: %s", id, err)
		return nil, err
	}
	resp.ID = yamlTemplate.ID.Hex()
	resp.Name = yamlTemplate.Name
	resp.Content = yamlTemplate.Content
	resp.Source = yamlTemplate.Source
	resp.CodehostID = yamlTemplate.CodeHostID
	resp.RepoOwner = yamlTemplate.RepoOwner
	resp.Namespace = yamlTemplate.Namespace
	resp.RepoName = yamlTemplate.RepoName
	resp.Path = yamlTemplate.Path
	resp.BranchName = yamlTemplate.BranchName
	resp.RemoteName = yamlTemplate.RemoteName
	resp.LoadFromDir = yamlTemplate.LoadFromDir
	resp.Commit = yamlTemplate.Commit
	resp.VariableYaml = yamlTemplate.VariableYaml
	resp.ServiceVariableKVs = yamlTemplate.ServiceVariableKVs
	return resp, err
}

func DeleteYamlTemplate(id string, logger *zap.SugaredLogger) error {
	ref, err := commonrepo.NewServiceColl().GetYamlTemplateLatestReference(id)
	if err != nil {
		logger.Errorf("Failed to get service reference for template id: %s, the error is: %s", id, err)
		return err
	}
	if len(ref) > 0 {
		return errors.New("this template is in use")
	}
	productionRef, err := commonrepo.NewProductionServiceColl().GetYamlTemplateLatestReference(id)
	if err != nil {
		logger.Errorf("Failed to get production reference for template id: %s, the error is: %s", id, err)
		return err
	}
	if len(productionRef) > 0 {
		return errors.New("this template is in use")
	}

	origin, err := commonrepo.NewYamlTemplateColl().GetById(id)
	if err != nil {
		logger.Errorf("Failed to get yaml template of id: %s, the error is: %s", id, err)
		return err
	}

	err = commonrepo.NewYamlTemplateColl().DeleteByID(id)
	if err != nil {
		logger.Errorf("Failed to delete yaml template of id: %s, the error is: %s", id, err)
		return err
	}

	if err := commmonservice.ProcessYamlTemplateWebhook(nil, origin, logger); err != nil {
		logger.Errorf("failed to process yaml template webhook for delete %s, err: %s", origin.Name, err)
	}
	return nil
}

func SyncYamlTemplateReference(userName, id string, logger *zap.SugaredLogger) error {
	return service.SyncServiceFromTemplate(userName, setting.ServiceSourceTemplate, id, "", logger)
}

func GetYamlTemplateReference(id string, logger *zap.SugaredLogger) ([]*template.ServiceReference, error) {
	ret := make([]*template.ServiceReference, 0)
	referenceList, err := commonrepo.NewServiceColl().GetYamlTemplateLatestReference(id)
	if err != nil {
		logger.Errorf("Failed to get build reference for yaml template id: %s, the error is: %s", id, err)
		return ret, err
	}
	for _, reference := range referenceList {
		ret = append(ret, &template.ServiceReference{
			ServiceName: reference.ServiceName,
			ProjectName: reference.ProductName,
			Production:  false,
		})
	}

	productionService, err := commonrepo.NewProductionServiceColl().GetYamlTemplateLatestReference(id)
	if err != nil {
		logger.Errorf("Failed to get build reference for yaml template id: %s from production service, the error is: %s", id, err)
		return ret, err
	}
	for _, reference := range productionService {
		ret = append(ret, &template.ServiceReference{
			ServiceName: reference.ServiceName,
			ProjectName: reference.ProductName,
			Production:  true,
		})
	}
	return ret, nil
}

func GetSystemDefaultVariables() []*models.ChartVariable {
	resp := make([]*models.ChartVariable, 0)
	for key, description := range DefaultSystemVariable {
		resp = append(resp, &models.ChartVariable{
			Key:         key,
			Description: description,
		})
	}
	return resp
}

func ValidateVariable(content, variable string) error {
	if len(content) == 0 || len(variable) == 0 {
		return nil
	}

	defaultSystemVariableYaml, err := yaml.Marshal(DefaultSystemVariable)
	if err != nil {
		return fmt.Errorf("failed to marshal default system variable, err: %s", err)
	}

	_, err = commonutil.RenderK8sSvcYamlStrict(content, "FakeProjectName", "ValidateVariable", variable, string(defaultSystemVariableYaml))
	if err != nil {
		return fmt.Errorf("failed to validate variable, err: %s", err)
	}

	return nil
}

func ExtractVariable(yamlContent string) (string, error) {
	if len(yamlContent) == 0 {
		return "", nil
	}
	return yamlutil.ExtractVariableYaml(yamlContent)
}

func FlattenKvs(yamlContent string) ([]*models.VariableKV, error) {
	if len(yamlContent) == 0 {
		return nil, nil
	}

	valuesMap, err := converter.YamlToFlatMap([]byte(yamlContent))
	if err != nil {
		return nil, err
	}

	ret := make([]*models.VariableKV, 0)
	for k, v := range valuesMap {
		ret = append(ret, &models.VariableKV{
			Key:   k,
			Value: v,
		})
	}
	return ret, nil
}
