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
	"sync"
	"time"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
)

func CreateBuild(username string, build *commonmodels.Build, log *zap.SugaredLogger) error {
	if len(build.Name) == 0 {
		return e.ErrCreateBuildModule.AddDesc("empty name")
	}
	if err := commonutil.CheckDefineResourceParam(build.PreBuild.ResReq, build.PreBuild.ResReqSpec); err != nil {
		return e.ErrCreateBuildModule.AddDesc(err.Error())
	}

	build.UpdateBy = username
	correctFields(build)

	if err := commonrepo.NewBuildColl().Create(build); err != nil {
		log.Errorf("[Build.Upsert] %s error: %v", build.Name, err)
		return e.ErrCreateBuildModule.AddErr(err)
	}

	return nil
}

func UpdateBuild(username string, build *commonmodels.Build, log *zap.SugaredLogger) error {
	if len(build.Name) == 0 {
		return e.ErrUpdateBuildModule.AddDesc("empty name")
	}
	if err := commonutil.CheckDefineResourceParam(build.PreBuild.ResReq, build.PreBuild.ResReqSpec); err != nil {
		return e.ErrUpdateBuildModule.AddDesc(err.Error())
	}

	existed, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.Name, ProductName: build.ProductName})
	if err == nil && existed.PreBuild != nil && build.PreBuild != nil {
		EnsureSecretEnvs(existed.PreBuild.Envs, build.PreBuild.Envs)
	}

	correctFields(build)
	build.UpdateBy = username
	build.UpdateTime = time.Now().Unix()

	if err := commonrepo.NewBuildColl().Update(build); err != nil {
		log.Errorf("[Build.Upsert] %s error: %v", build.Name, err)
		return e.ErrUpdateBuildModule.AddErr(err)
	}

	return nil
}

func correctFields(build *commonmodels.Build) {
	// make sure cache has no empty field
	caches := make([]string, 0)
	for _, cache := range build.Caches {
		cache = strings.Trim(cache, " /")
		if cache != "" {
			caches = append(caches, cache)
		}
	}
	build.Caches = caches

	// trim the docker file and context
	if build.PostBuild != nil && build.PostBuild.DockerBuild != nil {
		build.PostBuild.DockerBuild.DockerFile = strings.Trim(build.PostBuild.DockerBuild.DockerFile, " ")
		build.PostBuild.DockerBuild.WorkDir = strings.Trim(build.PostBuild.DockerBuild.WorkDir, " ")
	}
}

// EnsureSecretEnvs 转换敏感信息前端传入的Mask内容为真实内容
func EnsureSecretEnvs(existedKVs []*commonmodels.KeyVal, newKVs []*commonmodels.KeyVal) {

	if len(existedKVs) == 0 || len(newKVs) == 0 {
		return
	}

	existedKVsMap := make(map[string]string)
	for _, v := range existedKVs {
		existedKVsMap[v.Key] = v.Value
	}

	for _, kv := range newKVs {
		// 如果用户的value已经给mask了，认为不需要修改
		if kv.Value == setting.MaskValue {
			kv.Value = existedKVsMap[kv.Key]
		}
	}
}

func EnsureBuildResp(build *commonmodels.Build) {
	if len(build.Targets) == 0 {
		build.Targets = make([]*commonmodels.ServiceModuleTarget, 0)
	}
	build.Repos = build.SafeRepos()

	for _, repo := range build.Repos {
		repo.RepoNamespace = repo.GetRepoNamespace()
	}

	if build.PreBuild != nil {
		if len(build.PreBuild.Installs) == 0 {
			build.PreBuild.Installs = make([]*commonmodels.Item, 0)
		}

		if len(build.PreBuild.Envs) == 0 {
			build.PreBuild.Envs = make([]*commonmodels.KeyVal, 0)
		}

		// 隐藏用户设置的敏感信息
		for k := range build.PreBuild.Envs {
			if build.PreBuild.Envs[k].IsCredential {
				build.PreBuild.Envs[k].Value = setting.MaskValue
			}
		}

		if len(build.PreBuild.Parameters) == 0 {
			build.PreBuild.Parameters = make([]*commonmodels.Parameter, 0)
		}
	}

	if build.TemplateID != "" {
		buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
			ID: build.TemplateID,
		})
		//NOTE deleted template should not block the normal logic of build modules
		if err != nil {
			log.Warnf("failed to find build template with id: %s, err: %s", build.TemplateID, err)
		}
		build.TargetRepos = make([]*commonmodels.TargetRepo, 0, len(build.Targets))
		for _, target := range build.Targets {
			for _, repo := range target.Repos {
				repo.RepoNamespace = repo.GetRepoNamespace()
			}
			envs := target.Envs
			if buildTemplate != nil {
				envs = MergeBuildEnvs(buildTemplate.PreBuild.Envs.ToRuntimeList(), envs.ToRuntimeList()).ToKVList()
			}
			targetRepo := &commonmodels.TargetRepo{
				Service: &commonmodels.ServiceModuleTargetBase{
					ProductName: target.ProductName,
					ServiceWithModule: commonmodels.ServiceWithModule{
						ServiceName:   target.ServiceName,
						ServiceModule: target.ServiceModule,
					},
				},
				Repos: target.Repos,
				Envs:  envs,
			}
			build.TargetRepos = append(build.TargetRepos, targetRepo)
		}
	}
}

func EnsureDeployResp(deploy *commonmodels.Deploy) {
	deploy.Repos = deploy.SafeRepos()

	for _, repo := range deploy.Repos {
		repo.RepoNamespace = repo.GetRepoNamespace()
	}

	if deploy.PreDeploy != nil {
		if len(deploy.PreDeploy.Installs) == 0 {
			deploy.PreDeploy.Installs = make([]*commonmodels.Item, 0)
		}

		if len(deploy.PreDeploy.Envs) == 0 {
			deploy.PreDeploy.Envs = make([]*commonmodels.KeyVal, 0)
		}

		// 隐藏用户设置的敏感信息
		for k := range deploy.PreDeploy.Envs {
			if deploy.PreDeploy.Envs[k].IsCredential {
				deploy.PreDeploy.Envs[k].Value = setting.MaskValue
			}
		}
	}
}

func FindReposByTarget(projectName, serviceName, serviceModule string, build *commonmodels.Build) []*types.Repository {
	if build.TemplateID == "" {
		return build.SafeRepos()
	}
	for _, target := range build.Targets {
		if target.ServiceName == serviceName && target.ProductName == projectName && target.ServiceModule == serviceModule {
			return target.Repos
		}
	}
	return build.SafeRepos()
}

func MergeBuildEnvs(templateEnvs, customEnvs commonmodels.RuntimeKeyValList) commonmodels.RuntimeKeyValList {
	customEnvMap := make(map[string]*commonmodels.RuntimeKeyVal)
	for _, v := range customEnvs {
		customEnvMap[v.Key] = v
	}
	retEnvs := make([]*commonmodels.RuntimeKeyVal, 0)
	for _, v := range templateEnvs {
		if cv, ok := customEnvMap[v.Key]; ok {
			cv.ChoiceOption = v.ChoiceOption
			cv.Description = v.Description
			cv.Script = v.Script
			cv.FunctionReference = v.FunctionReference
			retEnvs = append(retEnvs, cv)
		} else {
			retEnvs = append(retEnvs, v)
		}
	}
	return retEnvs
}

func MergeParams(templateEnvs []*commonmodels.Param, customEnvs []*commonmodels.Param) []*commonmodels.Param {
	customEnvMap := make(map[string]*commonmodels.Param)
	for _, v := range customEnvs {
		customEnvMap[v.Name] = v
	}
	retEnvs := make([]*commonmodels.Param, 0)
	for _, v := range templateEnvs {
		if cv, ok := customEnvMap[v.Name]; ok {
			retEnvs = append(retEnvs, cv)
		} else {
			retEnvs = append(retEnvs, v)
		}
	}
	return retEnvs
}

type BuildService struct {
	BuildMap         sync.Map
	BuildTemplateMap sync.Map
}

func NewBuildService() *BuildService {
	return &BuildService{
		BuildMap:         sync.Map{},
		BuildTemplateMap: sync.Map{},
	}
}

func (c *BuildService) GetBuild(buildName, serviceName, serviceModule string) (*commonmodels.Build, error) {
	var err error
	buildInfo := &commonmodels.Build{}
	buildMapValue, ok := c.BuildMap.Load(buildName)
	if !ok {
		// TODO: add project name
		buildInfo, err = commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: buildName})
		if err != nil {
			c.BuildMap.Store(buildName, nil)
			return nil, fmt.Errorf("find build: %s error: %v", buildName, err)
		}
		c.BuildMap.Store(buildName, buildInfo)
	} else {
		if buildMapValue == nil {
			return nil, fmt.Errorf("failed to find build: %s", buildName)
		}
		buildInfo = buildMapValue.(*commonmodels.Build)
	}

	if err := FillBuildDetail(buildInfo, serviceName, serviceModule, &c.BuildTemplateMap); err != nil {
		return nil, err
	}
	return buildInfo, nil
}

func FillBuildDetail(moduleBuild *commonmodels.Build, serviceName, serviceModule string, cacheMap *sync.Map) error {
	if moduleBuild.TemplateID == "" {
		return nil
	}

	var err error
	var buildTemplate *commonmodels.BuildTemplate

	if cacheMap == nil {
		buildTemplate, err = commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
			ID: moduleBuild.TemplateID,
		})
		if err != nil {
			return fmt.Errorf("failed to find build template with id: %s, err: %s", moduleBuild.TemplateID, err)
		}
	} else {
		buildTemplateMapValue, ok := cacheMap.Load(moduleBuild.TemplateID)
		if !ok {
			buildTemplate, err = commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{
				ID: moduleBuild.TemplateID,
			})
			if err != nil {
				return fmt.Errorf("failed to find build template with id: %s, err: %s", moduleBuild.TemplateID, err)
			}
			cacheMap.Store(moduleBuild.TemplateID, buildTemplate)
		} else {
			buildTemplate = buildTemplateMapValue.(*commonmodels.BuildTemplate)
		}
	}

	moduleBuild.Timeout = buildTemplate.Timeout
	moduleBuild.PreBuild = buildTemplate.PreBuild
	moduleBuild.JenkinsBuild = buildTemplate.JenkinsBuild
	moduleBuild.ScriptType = buildTemplate.ScriptType
	moduleBuild.Scripts = buildTemplate.Scripts
	moduleBuild.PostBuild = buildTemplate.PostBuild
	moduleBuild.SSHs = buildTemplate.SSHs
	moduleBuild.PMDeployScripts = buildTemplate.PMDeployScripts
	moduleBuild.CacheEnable = buildTemplate.CacheEnable
	moduleBuild.CacheDirType = buildTemplate.CacheDirType
	moduleBuild.CacheUserDir = buildTemplate.CacheUserDir
	moduleBuild.AdvancedSettingsModified = buildTemplate.AdvancedSettingsModified
	moduleBuild.EnablePrivilegedMode = buildTemplate.EnablePrivilegedMode
	moduleBuild.Outputs = buildTemplate.Outputs
	moduleBuild.Infrastructure = buildTemplate.Infrastructure
	moduleBuild.VMLabels = buildTemplate.VmLabels

	// repos are configured by service modules
	for _, serviceConfig := range moduleBuild.Targets {
		if serviceConfig.ServiceName == serviceName && serviceConfig.ServiceModule == serviceModule {
			moduleBuild.Repos = serviceConfig.Repos
			if moduleBuild.PreBuild == nil {
				moduleBuild.PreBuild = &commonmodels.PreBuild{}
			}
			moduleBuild.PreBuild.Envs = MergeBuildEnvs(moduleBuild.PreBuild.Envs.ToRuntimeList(), serviceConfig.Envs.ToRuntimeList()).ToKVList()
			break
		}
	}
	return nil
}
