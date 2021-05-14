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

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types"
)

func CreateBuild(username string, build *commonmodels.Build, log *xlog.Logger) error {
	if len(build.Name) == 0 || len(build.Version) == 0 {
		return e.ErrCreateBuildModule.AddDesc("empty Name or Version")
	}

	build.UpdateBy = username
	correctFields(build)

	if err := commonrepo.NewBuildColl().Create(build); err != nil {
		log.Errorf("[Build.Upsert] %s:%s error: %v", build.Name, build.Version, err)
		return e.ErrCreateBuildModule.AddErr(err)
	}

	return nil
}

func UpdateBuild(username string, build *commonmodels.Build, log *xlog.Logger) error {
	if len(build.Name) == 0 || len(build.Version) == 0 {
		return e.ErrUpdateBuildModule.AddDesc("empty Name or Version")
	}

	existed, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: build.Name, Version: build.Version, ProductName: build.ProductName})
	if err == nil && existed.PreBuild != nil && build.PreBuild != nil {
		EnsureSecretEnvs(existed.PreBuild.Envs, build.PreBuild.Envs)
	}

	correctFields(build)
	build.UpdateBy = username
	build.UpdateTime = time.Now().Unix()

	if err := commonrepo.NewBuildColl().Update(build); err != nil {
		log.Errorf("[Build.Upsert] %s:%s error: %v", build.Name, build.Version, err)
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

	if len(build.Repos) == 0 {
		build.Repos = make([]*types.Repository, 0)
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
}
