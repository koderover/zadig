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
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	githubservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	gitlabservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

type LoadServicePath struct {
	ServiceName string `json:"service_name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"is_dir"`
}

func preloadService(ch *systemconfig.CodeHost, owner, repo, branch string, paths []PreLoadServicePath, logger *zap.SugaredLogger) ([]LoadServicePath, error) {
	logger.Infof("Preloading service from %s with owner %s, repo %s, branch %s and path %s", ch.Type, owner, repo, branch, paths)

	loader, err := getLoader(ch)
	if err != nil {
		logger.Errorf("Failed to create loader client, err: %s", err)
		return nil, e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}

	resp := make([]LoadServicePath, 0)
	for _, path := range paths {
		if !path.IsDir {
			if !isYaml(path.Path) {
				return nil, e.ErrPreloadServiceTemplate.AddDesc("File is not of type yaml or yml, select again")
			}

			resp = append(resp, LoadServicePath{
				ServiceName: getFileName(path.Path),
				Path:        path.Path,
				IsDir:       path.IsDir,
			})
		} else {
			treeNodes, err := loader.GetTree(owner, repo, path.Path, branch)
			if err != nil {
				logger.Errorf("Failed to get tree under path %s, err: %s", path.Path, err)
				return nil, e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}

			folders, files := getFoldersAndYAMLFiles(treeNodes)
			// if load path is a directory, we will load services in following rules:
			// 1. if there is any yaml files under this directory, collect them as a service and ignore other files and directories
			// 2. if not, but there is some directories under this directory, load each of them as a service
			if len(files) > 0 {
				resp = append(resp, LoadServicePath{
					ServiceName: getFileName(path.Path),
					Path:        path.Path,
					IsDir:       path.IsDir,
				})
			} else if len(folders) > 0 {
				for _, f := range folders {
					tns, err := loader.GetTree(owner, repo, f.FullPath, branch)
					if err != nil {
						logger.Errorf("Failed to get tree under path %s, err: %s", f.FullPath, err)
						return nil, e.ErrLoadServiceTemplate.AddDesc(err.Error())
					}

					if hasYAMLFiles(tns) {
						resp = append(resp, LoadServicePath{
							ServiceName: getFileName(f.FullPath),
							Path:        f.FullPath,
							IsDir:       f.IsDir,
						})
					}

				}
			}
		}
	}

	if len(resp) == 0 {
		log.Errorf("no valid service is found under paths %s", paths)
		return nil, e.ErrPreloadServiceTemplate.AddDesc("所选路径下没有yaml文件，请重新选择")
	}

	return resp, nil
}

type serviceInfo struct {
	path  string
	name  string
	isDir bool
	yamls []string
}

func loadService(username string, ch *systemconfig.CodeHost, owner, namespace, repo, branch string, args *LoadServiceReq, force, production bool, logger *zap.SugaredLogger) error {
	logger.Infof("Loading service from %s with owner %s, namespace %s, repo %s, branch %s and path %v", ch.Type, owner, namespace, repo, branch, args.ServicePaths)

	loader, err := getLoader(ch)
	if err != nil {
		logger.Errorf("Failed to create loader client, err: %s", err)
		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
	}

	var services []serviceInfo
	for _, loadPath := range args.ServicePaths {
		if !loadPath.IsDir {
			yamls, err := loader.GetYAMLContents(namespace, repo, loadPath.Path, branch, false, true)
			if err != nil {
				logger.Errorf("Failed to get yamls under path %s, err: %s", loadPath.Path, err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}

			services = append(services, serviceInfo{path: loadPath.Path, name: loadPath.ServiceName, isDir: false, yamls: yamls})
		} else {
			treeNodes, err := loader.GetTree(namespace, repo, loadPath.Path, branch)
			if err != nil {
				logger.Errorf("Failed to get tree under path %s, err: %s", loadPath.Path, err)
				return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			}

			_, files := getFoldersAndYAMLFiles(treeNodes)
			// if load path is a directory, we will load services in following rules:
			// 1. if there is any yaml files under this directory, collect them as a service and ignore other files and directories
			// 2. if not, but there is some directories under this directory, load each of them as a service
			if len(files) > 0 {
				var yamls []string
				for _, f := range files {
					res, err := loader.GetYAMLContents(namespace, repo, f.FullPath, branch, false, true)
					if err != nil {
						logger.Errorf("Failed to get yamls under path %s, err: %s", f.FullPath, err)
						return e.ErrLoadServiceTemplate.AddDesc(err.Error())
					}
					yamls = append(yamls, res...)
				}

				services = append(services, serviceInfo{path: loadPath.Path, name: loadPath.ServiceName, isDir: true, yamls: yamls})
			} else {
				return e.ErrLoadServiceTemplate.AddDesc(fmt.Sprintf("%s 路径下没有yaml文件，请重新选择", loadPath.Path))
			}

			// } else if len(folders) > 0 {
			// for _, f := range folders {
			// 	res, err := loader.GetYAMLContents(namespace, repo, f.FullPath, branch, true, true)
			// 	if err != nil {
			// 		logger.Errorf("Failed to get yamls under path %s, err: %s", f.FullPath, err)
			// 		return e.ErrLoadServiceTemplate.AddDesc(err.Error())
			// 	}
			// 	services = append(services, serviceInfo{path: f.FullPath, name: loadPath.ServiceName, isDir: true, yamls: res})
			// }
		}
	}

	for _, info := range services {
		if len(info.yamls) == 0 {
			continue
		}

		commit, err := loader.GetLatestRepositoryCommit(namespace, repo, info.path, branch)
		if err != nil {
			logger.Errorf("Failed to get latest commit under path %s, error: %s", info.path, err)
			return e.ErrLoadServiceTemplate.AddDesc(err.Error())
		}

		pathType := "tree"
		if !info.isDir {
			pathType = "blob"
		}
		createSvcArgs := &models.Service{
			CodehostID:    ch.ID,
			RepoName:      repo,
			RepoOwner:     owner,
			RepoNamespace: namespace,
			BranchName:    branch,
			LoadPath:      info.path,
			LoadFromDir:   info.isDir,
			KubeYamls:     info.yamls,
			SrcPath:       fmt.Sprintf("%s/%s/%s/%s/%s/%s", ch.Address, namespace, repo, pathType, branch, info.path),
			CreateBy:      username,
			ServiceName:   info.name,
			Type:          args.Type,
			ProductName:   args.ProductName,
			Source:        ch.Type,
			Yaml:          util.CombineManifests(info.yamls),
			Commit:        &models.Commit{SHA: commit.SHA, Message: commit.Message},
		}
		_, err = CreateServiceTemplate(username, createSvcArgs, force, production, logger)
		if err != nil {
			logger.Errorf("Failed to create service template, err: %s", err)
			_, messageMap := e.ErrorMessage(err)
			if description, ok := messageMap["description"]; ok {
				return e.ErrLoadServiceTemplate.AddDesc(description.(string))
			}
			return e.ErrLoadServiceTemplate.AddDesc("Load Service Error for unknown reason")
		}
	}

	return nil
}

func getFoldersAndYAMLFiles(treeNodes []*git.TreeNode) ([]*git.TreeNode, []*git.TreeNode) {
	var folders, files []*git.TreeNode
	for _, tn := range treeNodes {
		if tn.IsDir {
			folders = append(folders, tn)
		} else if isYaml(tn.Name) {
			files = append(files, tn)
		}
	}

	return folders, files
}

func hasYAMLFiles(treeNodes []*git.TreeNode) bool {
	for _, tn := range treeNodes {
		if !tn.IsDir && isYaml(tn.Name) {
			return true
		}
	}

	return false
}

type yamlLoader interface {
	GetYAMLContents(owner, repo, path, branch string, isDir, split bool) ([]string, error)
	GetLatestRepositoryCommit(owner, repo, path, branch string) (*git.RepositoryCommit, error)
	GetTree(owner, repo, path, branch string) ([]*git.TreeNode, error)
}

func getLoader(ch *systemconfig.CodeHost) (yamlLoader, error) {
	switch ch.Type {
	case setting.SourceFromGithub:
		return githubservice.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy), nil
	case setting.SourceFromGitlab:
		return gitlabservice.NewClient(ch.ID, ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
	default:
		// should not have happened here
		log.DPanicf("invalid source: %s", ch.Type)
		return nil, fmt.Errorf("invalid source: %s", ch.Type)
	}
}

func isYaml(filename string) bool {
	filename = strings.ToLower(filename)
	return strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml")
}

func getFileName(fullName string) string {
	name := filepath.Base(fullName)
	ext := filepath.Ext(name)
	return name[0:(len(name) - len(ext))]
}
