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

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
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

func UpsertUserPluginRepository(args *commonmodels.PluginRepo, log *zap.SugaredLogger) error {
	args.IsOffical = false
	args.PluginTemplates = []*commonmodels.PluginTemplate{}
	checkoutPath := fmt.Sprintf("/tmp/%s", uuid.New().String())
	defer os.RemoveAll(checkoutPath)
	// TODO: git clone and checkout the plugin
	plugins, err := loadPluginRepoInfos(checkoutPath, args.IsOffical, os.ReadDir, os.ReadFile)
	if err != nil {
		errMsg := fmt.Sprintf("load plugin from user user repo error: %s", err)
		log.Error(errMsg)
		return e.ErrUpsertPluginRepo.AddDesc(errMsg)
	}
	args.PluginTemplates = plugins
	return commonrepo.NewPluginRepoColl().Upsert(args)
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
	resp, err := commonrepo.NewPluginRepoColl().List()
	if err != nil {
		log.Errorf("list Plugin repos error: %v", err)
		return resp, e.ErrListPluginRepo.AddDesc(err.Error())
	}
	return resp, nil
}

func DeletePluginRepo(id string, log *zap.SugaredLogger) error {
	if err := commonrepo.NewPluginRepoColl().Delete(id); err != nil {
		log.Errorf("list Plugin repos error: %v", err)
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
