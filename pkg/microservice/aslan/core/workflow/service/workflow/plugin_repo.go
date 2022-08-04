/*
Copyright 2022 The KodeRover Authors.

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

package workflow

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

const (
	OfficalRepoOwner = "koderover"
	OfficalRepoName  = "zadig"
	OfficalRepoURL   = "https://github.com/" + OfficalRepoOwner + "/" + OfficalRepoName
	OfficalBranch    = "main"
)

//go:embed plugins
var officalPluginRepoFiles embed.FS

func UpsertUserPluginRepository(args *commonmodels.PluginRepo, log *zap.SugaredLogger) {
	// clean args status
	args.IsOffical = false
	args.PluginTemplates = []*commonmodels.PluginTemplate{}
	args.Error = ""

	defer func() {
		if err := commonrepo.NewPluginRepoColl().Upsert(args); err != nil {
			log.Errorf("upsert plugin repo error: %v", err)
		}
	}()

	codehost, err := systemconfig.New().GetCodeHost(args.CodehostID)
	if err != nil {
		errMsg := fmt.Sprintf("get code host %d error: %v", args.CodehostID, err)
		log.Error(errMsg)
		args.Error = errMsg
	}

	checkoutPath := path.Join(config.S3StoragePath(), args.RepoName)
	if err := command.RunGitCmds(codehost, args.RepoOwner, args.RepoNamespace, args.RepoName, args.Branch, "origin"); err != nil {
		errMsg := fmt.Sprintf("run git cmds error: %v", err)
		log.Error(errMsg)
		args.Error = errMsg
	}

	plugins, err := loadPluginRepoInfos(checkoutPath, args.IsOffical, os.ReadDir, os.ReadFile)
	if err != nil {
		errMsg := fmt.Sprintf("load plugin from user user repo error: %s", err)
		log.Error(errMsg)
		args.Error = errMsg
	}
	args.PluginTemplates = plugins
}

func UpdateOfficalPluginRepository(log *zap.SugaredLogger) {
	officalPluginRepo := &commonmodels.PluginRepo{
		RepoOwner: OfficalRepoOwner,
		RepoName:  OfficalRepoName,
		Branch:    OfficalBranch,
		RepoURL:   OfficalRepoURL,
		IsOffical: true,
	}
	plugins, err := loadPluginRepoInfos("plugins", officalPluginRepo.IsOffical, officalPluginRepoFiles.ReadDir, officalPluginRepoFiles.ReadFile)
	if err != nil {
		log.Errorf("load offical plugin repo error: %v", err)
		return
	}
	officalPluginRepo.PluginTemplates = plugins
	if err := commonrepo.NewPluginRepoColl().Upsert(officalPluginRepo); err != nil {
		log.Errorf("update offical plugin repo error: %v", err)
		return
	}
}

type readDir func(name string) ([]fs.DirEntry, error)
type readFile func(name string) ([]byte, error)

func loadPluginRepoInfos(baseDir string, isOffical bool, readDir readDir, readFile readFile) ([]*commonmodels.PluginTemplate, error) {
	resp := []*commonmodels.PluginTemplate{}
	dirs, err := readDir(baseDir)
	if err != nil {
		return resp, fmt.Errorf("loop plugin repo error: %v", err)
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		subDirs, err := readDir(path.Join(baseDir, dir.Name()))
		if err != nil {
			return resp, fmt.Errorf("loop plugin repo dirs error: %v", err)
		}
		for _, subDir := range subDirs {
			if !subDir.IsDir() {
				continue
			}
			files, err := readDir(path.Join(baseDir, dir.Name(), subDir.Name()))
			if err != nil {
				return resp, fmt.Errorf("loop plugin repo sub dirs error: %v", err)
			}
			for _, file := range files {
				if file.IsDir() {
					continue
				}
				if file.Name() != dir.Name()+".yaml" {
					continue
				}
				yamlFilebyte, err := readFile(path.Join(baseDir, dir.Name(), subDir.Name(), file.Name()))
				if err != nil {
					return resp, fmt.Errorf("read yaml files error: %v", err)
				}
				pluginTemplate := &commonmodels.PluginTemplate{}
				if err := yaml.Unmarshal(yamlFilebyte, pluginTemplate); err != nil {
					return resp, fmt.Errorf("unmarshal yaml files error: %v", err)
				}
				pluginTemplate.IsOffical = isOffical
				resp = append(resp, pluginTemplate)
			}
		}
	}
	return resp, nil
}

func ListPluginRepositories(log *zap.SugaredLogger) ([]*commonmodels.PluginRepo, error) {
	resp := []*commonmodels.PluginRepo{}
	repos, err := commonrepo.NewPluginRepoColl().List()
	if err != nil {
		log.Errorf("list Plugin repos error: %v", err)
		return resp, e.ErrListPluginRepo.AddDesc(err.Error())
	}
	for _, repo := range repos {
		if repo.IsOffical {
			continue
		}
		resp = append(resp, repo)
	}
	return resp, nil
}

func DeletePluginRepo(id string, log *zap.SugaredLogger) error {
	if err := commonrepo.NewPluginRepoColl().Delete(id); err != nil {
		log.Errorf("delete Plugin repos error: %v", err)
		return e.ErrListPluginRepo.AddDesc(err.Error())
	}
	return nil
}

func ListPluginTemplates(log *zap.SugaredLogger) ([]*commonmodels.PluginTemplate, error) {
	resp := []*commonmodels.PluginTemplate{}
	repos, err := commonrepo.NewPluginRepoColl().List()
	if err != nil {
		log.Errorf("list plugin templates error: %v", err)
		return resp, e.ErrListPluginRepo.AddDesc(err.Error())
	}
	for _, repo := range repos {
		for _, template := range repo.PluginTemplates {
			template.RepoURL = fmt.Sprintf("%s/%s", repo.RepoOwner, repo.RepoName)
			resp = append(resp, template)
		}
	}
	return resp, nil
}
