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

package service

import (
	"fmt"

	"github.com/koderover/zadig/pkg/setting"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	openapitool "github.com/koderover/zadig/pkg/tool/openapi"
	"github.com/koderover/zadig/pkg/types"
)

func OpenAPICreateBuildModule(username string, req *OpenAPIBuildCreationReq, log *zap.SugaredLogger) error {
	buildModule, err := generateBuildModuleFromOpenAPIRequest(req, log)
	if err != nil {
		return err
	}

	return CreateBuild(username, buildModule, log)
}

func OpenAPICreateBuildModuleFromTemplate(username string, req *OpenAPIBuildCreationFromTemplateReq, log *zap.SugaredLogger) error {
	buildModule, err := generateBuildModuleFromOpenAPITemplateRequest(req, log)
	if err != nil {
		return err
	}

	return CreateBuild(username, buildModule, log)
}

func generateBuildModuleFromOpenAPIRequest(req *OpenAPIBuildCreationReq, log *zap.SugaredLogger) (*commonmodels.Build, error) {
	// generate basic build information
	ret := &commonmodels.Build{
		Name:        req.Name,
		Source:      "zadig",
		Description: req.Description,
		Scripts:     req.BuildScript,
		ProductName: req.ProjectName,
		Timeout:     int(req.AdvancedSetting.Timeout),
		// just set this to true always to make the menu open forever
		AdvancedSettingsModified: true,
	}

	// otherwise this is a zadig build, we need to generate a bunch of information from the user input
	// first we generate the pre build info from the user input
	prebuildInfo := &commonmodels.PreBuild{
		CleanWorkspace: false,
		EnableProxy:    false,
		Installs:       req.Addons,
		// for now, file upload is not supported
		UploadPkg: false,
	}

	cluster, err := commonrepo.NewK8SClusterColl().FindByName(req.AdvancedSetting.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster of name: %s, the error is: %s", req.AdvancedSetting.ClusterName, err)
	}
	prebuildInfo.ClusterID = cluster.ID.Hex()

	kvList := make([]*commonmodels.KeyVal, 0)
	for _, kv := range req.Parameters {
		kvList = append(kvList, &commonmodels.KeyVal{
			Key:          kv.Key,
			Value:        kv.DefaultValue,
			Type:         commonmodels.ParameterSettingType(kv.Type),
			ChoiceOption: kv.ChoiceOption,
			IsCredential: kv.IsCredential,
		})
	}
	prebuildInfo.Envs = kvList

	// if there are special image name like bionic or focal, generate special stuff
	imageInfo, err := commonrepo.NewBasicImageColl().FindByImageName(req.ImageName)
	if err != nil {
		return nil, fmt.Errorf("failed to find the basic image of name: %s, error is: %s", req.ImageName, err)
	}
	prebuildInfo.ImageID = imageInfo.ID.Hex()
	if req.ImageName == "bionic" || req.ImageName == "focal" {
		prebuildInfo.ImageFrom = "koderover"
		prebuildInfo.BuildOS = req.ImageName
	} else {
		prebuildInfo.ImageFrom = "custom"
		prebuildInfo.BuildOS = imageInfo.Value
	}
	// generate spec request from user input spec request
	prebuildInfo.ResReqSpec = req.AdvancedSetting.Spec
	prebuildInfo.ResReq = req.AdvancedSetting.Spec.FindResourceRequestType()

	// for now, pre build is done
	ret.PreBuild = prebuildInfo

	//repo info conversion
	repoList := make([]*types.Repository, 0)
	for _, repo := range req.RepoInfo {
		systemRepoInfo, err := openapitool.ToBuildRepository(repo)
		if err != nil {
			log.Errorf("failed to convert user repository input info into system repository, codehostName: [%s], repoName[%s], err: %s", repo.CodeHostName, repo.RepoName, err)
			return nil, fmt.Errorf("failed to convert user repository input info into system repository, codehostName: [%s], repoName[%s], err: %s", repo.CodeHostName, repo.RepoName, err)
		}
		repoList = append(repoList, systemRepoInfo)
	}

	// insert repo info into our build module
	ret.Repos = repoList

	// create target info for the build to work
	targetInfo := make([]*commonmodels.ServiceModuleTarget, 0)
	for _, target := range req.TargetServices {
		targetInfo = append(targetInfo, &commonmodels.ServiceModuleTarget{
			ProductName:   req.ProjectName,
			ServiceName:   target.ServiceName,
			ServiceModule: target.ServiceModule,
		})
	}
	ret.Targets = targetInfo

	// apply the rest of the advanced settings to the build settings
	ret.CacheEnable = req.AdvancedSetting.CacheSetting.Enabled
	if ret.CacheEnable {
		ret.CacheDirType = "user_defined"
		ret.CacheUserDir = req.AdvancedSetting.CacheSetting.CacheDir
	}

	// generate post build information from user generated input
	postBuildInfo := &commonmodels.PostBuild{
		FileArchive: &commonmodels.FileArchive{FileLocation: req.FileArchivePath},
		Scripts:     req.PostBuildScript,
	}

	if req.DockerBuildInfo != nil {
		dockerBuildInfo := &commonmodels.DockerBuild{
			WorkDir:   req.DockerBuildInfo.WorkingDirectory,
			BuildArgs: req.DockerBuildInfo.BuildArgs,
			Source:    req.DockerBuildInfo.DockerfileType,
		}
		switch req.DockerBuildInfo.DockerfileType {
		case "local":
			dockerBuildInfo.DockerFile = req.DockerBuildInfo.DockerfileDirectory
		case "template":
			dockerBuildInfo.TemplateName = req.DockerBuildInfo.TemplateName
			templateInfo, err := commonrepo.NewDockerfileTemplateColl().GetByName(req.DockerBuildInfo.TemplateName)
			if err != nil {
				return nil, fmt.Errorf("failed to find the dockerfile template of name: [%s], err: %s", req.DockerBuildInfo.TemplateName, err)
			}
			dockerBuildInfo.TemplateID = templateInfo.ID.Hex()
		default:
			return nil, fmt.Errorf("unsupported dockerfile type, it can only be local or template")
		}

		postBuildInfo.DockerBuild = dockerBuildInfo
	}

	ret.PostBuild = postBuildInfo

	return ret, nil
}

func generateBuildModuleFromOpenAPITemplateRequest(req *OpenAPIBuildCreationFromTemplateReq, log *zap.SugaredLogger) (*commonmodels.Build, error) {
	// generate basic build information
	ret := &commonmodels.Build{
		Name:        req.Name,
		Source:      "zadig",
		ProductName: req.ProjectName,
		// just set this to true always to make the menu open forever
		AdvancedSettingsModified: true,
	}

	buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
		Name: req.TemplateName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find the build template specified by the user, template name: [%s], err: %s", req.TemplateName, err)
	}
	ret.TemplateID = buildTemplate.ID.Hex()

	targetRepoInfo := make([]*commonmodels.TargetRepo, 0)

	for _, targetService := range req.TargetServices {
		serviceInfo := &commonmodels.ServiceModuleTargetBase{
			ProductName:   req.ProjectName,
			ServiceName:   targetService.ServiceName,
			ServiceModule: targetService.ServiceModule,
		}

		repoInfo := make([]*types.Repository, 0)
		for _, serviceRepo := range targetService.RepoInfo {
			systemRepoInfo, err := openapitool.ToBuildRepository(serviceRepo)
			if err != nil {
				log.Errorf("failed to convert user repository input info into system repository, codehostName: [%s], repoName[%s], err: %s", serviceRepo.CodeHostName, serviceRepo.RepoName, err)
				return nil, fmt.Errorf("failed to convert user repository input info into system repository, codehostName: [%s], repoName[%s], err: %s", serviceRepo.CodeHostName, serviceRepo.RepoName, err)
			}
			repoInfo = append(repoInfo, systemRepoInfo)
		}

		kvList := make([]*commonmodels.KeyVal, 0)
		for _, kv := range targetService.Inputs {
			kvList = append(kvList, &commonmodels.KeyVal{
				Key:          kv.Key,
				Value:        kv.Value,
				Type:         commonmodels.ParameterSettingType(kv.Type),
				IsCredential: kv.IsCredential,
			})
		}

		targetRepo := &commonmodels.TargetRepo{
			Service: serviceInfo,
			Repos:   repoInfo,
			Envs:    kvList,
		}

		targetRepoInfo = append(targetRepoInfo, targetRepo)
	}

	ret.TargetRepos = targetRepoInfo

	// for fucking stupid reason, prebuild and postbuild need to be non-empty
	ret.PreBuild = &commonmodels.PreBuild{
		CleanWorkspace: false,
		ResReq:         "low",
		ResReqSpec: setting.RequestSpec{
			CpuLimit:    1000,
			MemoryLimit: 512,
		},
		BuildOS:             "focal",
		ImageFrom:           "koderover",
		ImageID:             "61af6b575c9beafb9ab130dc",
		Installs:            nil,
		Envs:                nil,
		EnableProxy:         false,
		Parameters:          nil,
		UploadPkg:           false,
		ClusterID:           "0123456789abcdef12345678",
		UseHostDockerDaemon: false,
		Namespace:           "",
	}

	ret.PostBuild = &commonmodels.PostBuild{
		DockerBuild:         nil,
		ObjectStorageUpload: nil,
		FileArchive:         nil,
		Scripts:             "",
	}

	return ret, nil
}
