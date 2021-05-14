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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type BuildResp struct {
	ID          string                              `json:"id"`
	Version     string                              `json:"version"`
	Name        string                              `json:"name"`
	Targets     []*commonmodels.ServiceModuleTarget `json:"targets"`
	UpdateTime  int64                               `json:"update_time"`
	UpdateBy    string                              `json:"update_by"`
	Pipelines   []string                            `json:"pipelines"`
	ProductName string                              `json:"productName"`
}

func FindBuild(name, version, productName string, log *xlog.Logger) (*commonmodels.Build, error) {
	opt := &commonrepo.BuildFindOption{
		Name:        name,
		Version:     version,
		ProductName: productName,
	}

	resp, err := commonrepo.NewBuildColl().Find(opt)
	if err != nil {
		log.Errorf("[Build.Find] %s:%s error: %v", name, version, err)
		return nil, e.ErrGetBuildModule.AddErr(err)
	}

	commonservice.EnsureResp(resp)

	return resp, nil
}

func ListBuild(name, version, targets, productName string, log *xlog.Logger) ([]*BuildResp, error) {
	opt := &commonrepo.BuildListOption{
		Name:        name,
		Version:     version,
		ProductName: productName,
	}

	if len(strings.TrimSpace(targets)) != 0 {
		opt.Targets = strings.Split(targets, ",")
	}

	currentProductBuilds, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		log.Errorf("[Pipeline.List] %s:%s error: %v", name, version, err)
		return nil, e.ErrListBuildModule.AddErr(err)
	}
	// 获取全部 pipeline
	pipes, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{IsPreview: true})
	if err != nil {
		log.Errorf("[Pipeline.List] %s:%s error: %v", name, version, err)
		return nil, e.ErrListBuildModule.AddErr(err)
	}

	resp := make([]*BuildResp, 0)
	for _, build := range currentProductBuilds {
		b := &BuildResp{
			ID:          build.ID.Hex(),
			Version:     build.Version,
			Name:        build.Name,
			Targets:     build.Targets,
			UpdateTime:  build.UpdateTime,
			UpdateBy:    build.UpdateBy,
			ProductName: build.ProductName,
			Pipelines:   []string{},
		}

		for _, pipe := range pipes {
			if pipe.BuildModuleVer == b.Version {
				//	current build module used by this pipeline
				for _, serviceModuleTarget := range b.Targets {
					if serviceModuleTarget.ServiceModule == pipe.Target {
						b.Pipelines = append(b.Pipelines, pipe.Name)
					}
				}
			}
		}
		resp = append(resp, b)
	}

	return resp, nil
}

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
		commonservice.EnsureSecretEnvs(existed.PreBuild.Envs, build.PreBuild.Envs)
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

func DeleteBuild(name, version, productName string, log *xlog.Logger) error {

	if len(name) == 0 || len(version) == 0 {
		return e.ErrDeleteBuildModule.AddDesc("empty Name or Version")
	}

	existed, err := FindBuild(name, version, productName, log)
	if err != nil {
		log.Errorf("[Build.Delete] %s:%s error: %v", name, version, err)
		return e.ErrDeleteBuildModule.AddErr(err)
	}

	// 如果使用过编译模块
	if len(existed.Targets) != 0 {
		targets := sets.String{}
		for _, target := range existed.Targets {
			if !targets.Has(target.ServiceModule) {
				targets.Insert(target.ServiceModule)
			}
		}
		opt := &commonrepo.PipelineListOption{
			BuildModuleVer: version,
			Targets:        targets.List(),
		}

		// 获取全部 pipeline
		pipes, err := commonrepo.NewPipelineColl().List(opt)
		if err != nil {
			log.Errorf("[Pipeline.List] %s:%s error: %v", name, version, err)
			return e.ErrDeleteBuildModule.AddErr(err)
		}

		if len(pipes) > 0 {
			pipeNames := []string{}
			for _, pipe := range pipes {
				pipeNames = append(pipeNames, pipe.Name)
			}
			msg := fmt.Sprintf("build module used by pipelines %v", pipeNames)
			return e.ErrDeleteBuildModule.AddDesc(msg)
		}
	}
	serviceTempRevisions, _ := commonrepo.NewServiceColl().DistinctServices(&commonrepo.ServiceListOption{BuildName: name, ProductName: productName, ExcludeStatus: setting.ProductStatusDeleting})
	serviceNames := make([]string, 0)
	for _, serviceTempRevision := range serviceTempRevisions {
		serviceNames = append(serviceNames, serviceTempRevision.ServiceName)
	}
	if len(serviceNames) > 0 {
		return e.ErrDeleteBuildModule.AddDesc(fmt.Sprintf("该构建被服务 [%s] 引用，请解除引用之后再做删除!", strings.Join(serviceNames, ",")))
	}
	// 删除服务配置，检查工作流是否有引用该编译模板，需要二次确认
	if err := commonrepo.NewBuildColl().Delete(name, version, productName); err != nil {
		log.Errorf("[Build.Delete] %s:%s error: %v", name, version, err)
		return e.ErrDeleteBuildModule.AddErr(err)
	}
	return nil
}

func UpdateBuildParam(name, version, productName string, params []*commonmodels.Parameter, log *xlog.Logger) error {
	err := commonrepo.NewBuildColl().UpdateBuildParam(name, version, productName, params)
	if err != nil {
		log.Errorf("[Build.UpdateBuildParam] %s:%s error: %v", name, version, err)
		return e.ErrUpdateBuildParam.AddErr(err)
	}
	return nil
}

func UpdateBuildTargets(name, productName string, targets []*commonmodels.ServiceModuleTarget, log *xlog.Logger) error {
	if err := verifyBuildTargets(name, productName, targets, log); err != nil {
		return e.ErrUpdateBuildParam.AddErr(err)
	}

	err := commonrepo.NewBuildColl().UpdateTargets(name, productName, targets)
	if err != nil {
		log.Errorf("[Build.UpdateServices] %s error: %v", name, err)
		return e.ErrUpdateBuildServiceTmpls.AddErr(err)
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

func verifyBuildTargets(name, productName string, targets []*commonmodels.ServiceModuleTarget, log *xlog.Logger) error {
	if hasDuplicateTargets(targets) {
		return errors.New("duplicate target found")
	}

	existed, err := commonrepo.NewBuildColl().DistinctTargets([]string{name}, productName)
	if err != nil {
		log.Errorf("[Build.DistinctTargets] error: %v", err)
		return err
	}

	for _, serviceModuleTarget := range targets {
		target := fmt.Sprintf("%s-%s-%s", serviceModuleTarget.ProductName, serviceModuleTarget.ServiceName, serviceModuleTarget.ServiceModule)
		if _, ok := existed[target]; ok {
			return fmt.Errorf("target already existed: %s", target)
		}
	}
	return nil
}

func hasDuplicateTargets(serviceModuleTargets []*commonmodels.ServiceModuleTarget) bool {
	tMap := make(map[string]bool)
	for _, serviceModuleTarget := range serviceModuleTargets {
		target := fmt.Sprintf("%s-%s-%s", serviceModuleTarget.ProductName, serviceModuleTarget.ServiceName, serviceModuleTarget.ServiceModule)
		if _, ok := tMap[target]; ok {
			return true
		}
		tMap[target] = true
	}
	return false
}
