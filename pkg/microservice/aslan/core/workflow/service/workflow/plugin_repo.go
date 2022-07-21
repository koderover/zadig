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
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

const (
	OfficalRepoOwner = "dianqihanwangzi"
	OfficalRepoName  = "zadig-plugins"
	OfficalRepoURL   = "https://github.com/" + OfficalRepoOwner + "/" + OfficalRepoName
	OfficalBranch    = "main"
)

func UpsertPluginRepository(args *commonmodels.PluginRepo, log *zap.SugaredLogger) error {
	args.PluginTemplates = []*commonmodels.PluginTemplate{}
	// TODO: git clone and checkout the plugin
	checkoutPath := fmt.Sprintf("/tmp/%s", uuid.New().String())
	defer os.RemoveAll(checkoutPath)
	if args.IsOffical {
		args.RepoOwner = OfficalRepoOwner
		args.RepoName = OfficalRepoName
		args.Branch = OfficalBranch
		args.RepoURL = OfficalRepoURL
	}
	if _, err := git.PlainClone(checkoutPath, false, &git.CloneOptions{
		URL:           args.RepoURL,
		Depth:         1,
		ReferenceName: plumbing.NewBranchReferenceName(args.Branch),
	}); err != nil {
		log.Errorf("clone plugin repo error: %v", err)
		return e.ErrUpsertPluginRepo.AddDesc(err.Error())
	}

	dirs, err := ioutil.ReadDir(checkoutPath)
	if err != nil {
		log.Errorf("loop plugin repo error: %v", err)
		return e.ErrUpsertPluginRepo.AddDesc(err.Error())
	}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		subDirs, err := ioutil.ReadDir(path.Join(checkoutPath, dir.Name()))
		if err != nil {
			log.Errorf("loop plugin repo dirs error: %v", err)
			return e.ErrUpsertPluginRepo.AddDesc(err.Error())
		}
		for _, subDir := range subDirs {
			if !subDir.IsDir() {
				continue
			}
			files, err := ioutil.ReadDir(path.Join(checkoutPath, dir.Name(), subDir.Name()))
			if err != nil {
				log.Errorf("loop plugin repo sub dirs error: %v", err)
				return e.ErrUpsertPluginRepo.AddDesc(err.Error())
			}
			for _, file := range files {
				if file.IsDir() {
					continue
				}
				if file.Name() != dir.Name()+".yaml" {
					continue
				}
				yamlFile, err := os.Open(path.Join(checkoutPath, dir.Name(), subDir.Name(), file.Name()))
				if err != nil {
					log.Errorf("load yaml files error: %v", err)
					return e.ErrUpsertPluginRepo.AddDesc(err.Error())
				}
				defer yamlFile.Close()
				content, err := ioutil.ReadAll(yamlFile)
				if err != nil {
					log.Errorf("read yaml files error: %v", err)
					return e.ErrUpsertPluginRepo.AddDesc(err.Error())
				}
				pluginTemplate := &commonmodels.PluginTemplate{}
				if err := yaml.Unmarshal(content, pluginTemplate); err != nil {
					log.Errorf("unmarshal yaml files error: %v", err)
					return e.ErrUpsertPluginRepo.AddDesc(err.Error())
				}
				args.PluginTemplates = append(args.PluginTemplates, pluginTemplate)
			}
		}
	}
	return commonrepo.NewPluginRepoColl().Upsert(args)
}

func UpdateOfficalPluginRepository(log *zap.SugaredLogger) error {
	return UpsertPluginRepository(&commonmodels.PluginRepo{IsOffical: true}, log)
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
