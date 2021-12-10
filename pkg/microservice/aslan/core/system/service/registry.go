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
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/registry"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/util"
)

type RepoImgResp struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
	Tag   string `json:"tag"`
}

type RepoInfo struct {
	RegType      string `json:"regType"`
	RegProvider  string `json:"regProvider"`
	RegNamespace string `json:"regNamespace"`
	RegAddr      string `json:"regAddr"`
	ID           string `json:"id"`
}

const (
	PERPAGE = 100
	PAGE    = 1
)

// ListRegistries 为了抹掉ak和sk的数据
func ListRegistries(log *zap.SugaredLogger) ([]*commonmodels.RegistryNamespace, error) {
	registryNamespaces, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %v", err)
		return registryNamespaces, fmt.Errorf("RegistryNamespace.List error: %v", err)
	}
	for _, registryNamespace := range registryNamespaces {
		registryNamespace.AccessKey = ""
		registryNamespace.SecretKey = ""
	}
	return registryNamespaces, nil
}

func CreateRegistryNamespace(username string, args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	regOps := new(commonrepo.FindRegOps)
	regOps.IsDefault = true
	registryInfoList, _ := GetRegistryNamespaces(regOps, log)
	if args.IsDefault {
		for _, registryInfo := range registryInfoList {
			registryInfo.IsDefault = false
			err := UpdateRegistryNamespaceDefault(registryInfo, log)
			if err != nil {
				log.Errorf("updateRegistry error: %v", err)
				return fmt.Errorf("RegistryNamespace.Create error: %v", err)
			}
		}

		envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{ExcludeSource: setting.SourceFromExternal})
		if err != nil {
			log.Errorf("Failed to list namespaces to update")
		}
		for _, env := range envs {
			go func(prod *commonmodels.Product) {
				kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), prod.ClusterID)
				if err != nil {
					log.Errorf("[updateRegistry] Failed to get kubecli for namespace: %s", prod.Namespace)
					return
				}
				err = commonservice.EnsureDefaultRegistrySecret(prod.Namespace, args.ID.Hex(), kubeClient, log)
				if err != nil {
					log.Errorf("[updateRegistry] Failed to update registry secret for namespace: %s, the error is: %+v", prod.Namespace, err)
				}
			}(env)
		}
	} else {
		hasDefault := false
		for _, registryInfo := range registryInfoList {
			if registryInfo.IsDefault {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			log.Errorf("create registry error: There must be at least 1 default registry")
			return fmt.Errorf("RegistryNamespace.Create error: %s", "There must be at least 1 default registry")
		}
	}

	args.UpdateBy = username
	args.Namespace = strings.TrimSpace(args.Namespace)

	if err := commonrepo.NewRegistryNamespaceColl().Create(args); err != nil {
		log.Errorf("RegistryNamespace.Create error: %v", err)
		return fmt.Errorf("RegistryNamespace.Create error: %v", err)
	}
	return nil
}

func UpdateRegistryNamespace(username, id string, args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	regOps := new(commonrepo.FindRegOps)
	regOps.IsDefault = true
	registryInfoList, _ := GetRegistryNamespaces(regOps, log)
	if args.IsDefault {
		for _, registryInfo := range registryInfoList {
			registryInfo.IsDefault = false
			err := UpdateRegistryNamespaceDefault(registryInfo, log)
			if err != nil {
				log.Errorf("updateRegistry error: %v", err)
				return fmt.Errorf("RegistryNamespace.Update error: %v", err)
			}
		}
	} else {
		hasDefault := false
		for _, registryInfo := range registryInfoList {
			if registryInfo.ID != args.ID {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			log.Errorf("update registry error: There must be at least 1 default registry")
			return fmt.Errorf("RegistryNamespace.Create error: %s", "There must be at least 1 default registry")
		}
	}
	args.UpdateBy = username
	args.Namespace = strings.TrimSpace(args.Namespace)

	if err := commonrepo.NewRegistryNamespaceColl().Update(id, args); err != nil {
		log.Errorf("RegistryNamespace.Update error: %v", err)
		return fmt.Errorf("RegistryNamespace.Update error: %v", err)
	}

	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{ExcludeSource: setting.SourceFromExternal})
	if err != nil {
		log.Errorf("Failed to list namespaces to update")
	}
	for _, env := range envs {
		go func(prod *commonmodels.Product) {
			kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), prod.ClusterID)
			if err != nil {
				log.Errorf("[updateRegistry] Failed to get kubecli for namespace: %s", prod.Namespace)
				return
			}
			err = commonservice.EnsureDefaultRegistrySecret(prod.Namespace, id, kubeClient, log)
			if err != nil {
				log.Errorf("[updateRegistry] Failed to update registry secret for namespace: %s, the error is: %+v", prod.Namespace, err)
			}
		}(env)
	}
	return nil
}

func DeleteRegistryNamespace(id string, log *zap.SugaredLogger) error {
	if err := commonrepo.NewRegistryNamespaceColl().Delete(id); err != nil {
		log.Errorf("RegistryNamespace.Delete error: %v", err)
		return err
	}
	return nil
}

func ListAllRepos(log *zap.SugaredLogger) ([]*RepoInfo, error) {
	repoInfos := make([]*RepoInfo, 0)
	resp, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %v", err)
		return nil, fmt.Errorf("RegistryNamespace.List error: %v", err)
	}
	for _, rn := range resp {
		repoInfo := new(RepoInfo)
		repoInfo.RegProvider = rn.RegProvider
		repoInfo.RegType = rn.RegType
		repoInfo.RegNamespace = rn.Namespace
		repoInfo.RegAddr = rn.RegAddr
		repoInfo.ID = rn.ID.Hex()

		repoInfos = append(repoInfos, repoInfo)
	}
	return repoInfos, nil
}

func GetRegistryNamespace(regOps *commonrepo.FindRegOps, log *zap.SugaredLogger) (*commonmodels.RegistryNamespace, error) {
	resp, err := commonrepo.NewRegistryNamespaceColl().Find(regOps)
	if err != nil {
		log.Errorf("RegistryNamespace.get error: %v", err)
		return nil, fmt.Errorf("RegistryNamespace.get error: %v", err)
	}
	return resp, nil
}

func ListReposTags(registryInfo *commonmodels.RegistryNamespace, names []string, logger *zap.SugaredLogger) ([]*RepoImgResp, error) {
	repos, err := registry.NewV2Service(registryInfo.RegProvider).ListRepoImages(registry.ListRepoImagesOption{
		Endpoint: registry.Endpoint{
			Addr:      registryInfo.RegAddr,
			Ak:        registryInfo.AccessKey,
			Sk:        registryInfo.SecretKey,
			Namespace: registryInfo.Namespace,
			Region:    registryInfo.Region,
		},
		Repos: names,
	}, logger)

	images := make([]*RepoImgResp, 0)
	if err == nil {
		for _, repo := range repos.Repos {
			for _, tag := range repo.Tags {
				img := &RepoImgResp{
					Host:  util.GetURLHostName(registryInfo.RegAddr),
					Owner: repo.Namespace,
					Name:  repo.Name,
					Tag:   tag,
				}
				images = append(images, img)
			}
		}
	} else {
		err = e.ErrListImages.AddErr(err)
	}

	return images, err
}

func GetRepoTags(registryInfo *commonmodels.RegistryNamespace, name string, log *zap.SugaredLogger) (*registry.ImagesResp, error) {
	var resp *registry.ImagesResp
	repos, err := registry.NewV2Service(registryInfo.RegProvider).ListRepoImages(registry.ListRepoImagesOption{
		Endpoint: registry.Endpoint{
			Addr:      registryInfo.RegAddr,
			Ak:        registryInfo.AccessKey,
			Sk:        registryInfo.SecretKey,
			Namespace: registryInfo.Namespace,
			Region:    registryInfo.Region,
		},
		Repos: []string{name},
	}, log)

	if err != nil {
		err = e.ErrListImages.AddErr(err)
	} else {
		if len(repos.Repos) == 0 {
			err = e.ErrListImages.AddDesc(fmt.Sprintf("找不到Repo %s", name))
		} else {
			repo := repos.Repos[0]
			var images []registry.Image
			for _, tag := range repo.Tags {
				images = append(images, registry.Image{Tag: tag})
			}

			resp = &registry.ImagesResp{
				NameSpace: registryInfo.Namespace,
				Name:      name,
				Total:     len(repo.Tags),
				Images:    images,
			}
		}
	}

	return resp, err
}

func GetRegistryNamespaces(regOps *commonrepo.FindRegOps, log *zap.SugaredLogger) ([]*commonmodels.RegistryNamespace, error) {
	resp, err := commonrepo.NewRegistryNamespaceColl().FindAll(regOps)
	if err != nil {
		log.Errorf("RegistryNamespace.findAll error: %+v", err)
		return nil, fmt.Errorf("RegistryNamespace.findAll error: %v", err)
	}
	return resp, nil
}

func UpdateRegistryNamespaceDefault(args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	if err := commonrepo.NewRegistryNamespaceColl().Update(args.ID.Hex(), args); err != nil {
		log.Errorf("UpdateRegistryNamespaceDefault.Update error: %v", err)
		return fmt.Errorf("UpdateRegistryNamespaceDefault.Update error: %v", err)
	}
	return nil
}
