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
	"strings"
	"time"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
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

func EnsureResp(build *commonmodels.Build) {
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
				envs = MergeBuildEnvs(buildTemplate.PreBuild.Envs, envs)
			}
			targetRepo := &commonmodels.TargetRepo{
				Service: &commonmodels.ServiceModuleTargetBase{
					ProductName:   target.ProductName,
					ServiceName:   target.ServiceName,
					ServiceModule: target.ServiceModule,
				},
				Repos: target.Repos,
				Envs:  envs,
			}
			for _, v := range targetRepo.Envs {
				if v.IsCredential {
					v.Value = setting.MaskValue
				}
			}
			build.TargetRepos = append(build.TargetRepos, targetRepo)
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

func MergeBuildEnvs(templateEnvs []*commonmodels.KeyVal, customEnvs []*commonmodels.KeyVal) []*commonmodels.KeyVal {
	customEnvMap := make(map[string]*commonmodels.KeyVal)
	for _, v := range customEnvs {
		customEnvMap[v.Key] = v
	}
	retEnvs := make([]*commonmodels.KeyVal, 0)
	for _, v := range templateEnvs {
		if cv, ok := customEnvMap[v.Key]; ok {
			retEnvs = append(retEnvs, cv)
		} else {
			retEnvs = append(retEnvs, v)
		}
	}
	return retEnvs
}
