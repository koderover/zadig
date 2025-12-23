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
	"time"

	"github.com/koderover/zadig/v2/pkg/util"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func FindDeploy(project, deployName string) (*commonmodels.Deploy, error) {
	opt := &commonrepo.DeployFindOption{
		Name:        deployName,
		ProjectName: project,
	}

	resp, err := commonrepo.NewDeployColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("find deploy %s/%s failed, err: %w", project, deployName, err)
		log.Error(err)
		return nil, e.ErrGetDeployModule.AddErr(err)
	}

	commonservice.EnsureDeployResp(resp)

	return resp, nil
}

func ListDeploy(project string) ([]*commonmodels.Deploy, error) {
	opt := &commonrepo.DeployListOption{
		ProjectName: project,
	}

	deploys, err := commonrepo.NewDeployColl().List(opt)
	if err != nil {
		err = fmt.Errorf("list deploys %s failed, err: %w", project, err)
		return nil, e.ErrListDeployModule.AddErr(err)
	}

	return deploys, nil
}

func CreateDeploy(ctx *internalhandler.Context, deploy *commonmodels.Deploy) error {
	if len(deploy.Name) == 0 {
		return e.ErrCreateDeployModule.AddDesc("empty name")
	}

	if err := commonutil.CheckDefineResourceParam(deploy.PreDeploy.ResReq, deploy.PreDeploy.ResReqSpec); err != nil {
		return e.ErrUpdateBuildModule.AddDesc(err.Error())
	}

	deploy.UpdateBy = ctx.UserName
	err := correctDeployFields(deploy)
	if err != nil {
		return err
	}

	templateProdct, err := template.NewProductColl().Find(deploy.ProjectName)
	if err != nil {
		return e.ErrCreateDeployModule.AddErr(fmt.Errorf("failed to find product %s, err: %s", deploy.ProjectName, err))
	}

	if templateProdct.IsCVMProduct() {
		existed, err := commonrepo.NewDeployColl().Exist(&commonrepo.DeployFindOption{
			ProjectName: deploy.ProjectName,
			Name:        deploy.Name,
			ServiceName: deploy.ServiceName,
		})
		if err != nil {
			err = fmt.Errorf("check deploy %s/%s/%s, exist failed, err: %w", deploy.ProjectName, deploy.Name, deploy.ServiceName, err)
			log.Error(err)
			return err
		}

		if existed {
			return fmt.Errorf("deploy %s/%s/%s already exist", deploy.ProjectName, deploy.Name, deploy.ServiceName)
		}
	}

	if err := commonrepo.NewDeployColl().Create(deploy); err != nil {
		err = fmt.Errorf("failed to create deploy %s/%s/%s, err: %w", deploy.ProjectName, deploy.Name, deploy.ServiceName, err)
		log.Error(err)
		return e.ErrCreateDeployModule.AddErr(err)
	}

	return nil
}

func UpdateDeploy(ctx *internalhandler.Context, deploy *commonmodels.Deploy) error {
	if len(deploy.Name) == 0 {
		return e.ErrUpdateDeployModule.AddDesc("empty name")
	}

	if err := commonutil.CheckDefineResourceParam(deploy.PreDeploy.ResReq, deploy.PreDeploy.ResReqSpec); err != nil {
		return e.ErrUpdateBuildModule.AddDesc(err.Error())
	}

	existed, err := commonrepo.NewDeployColl().Find(&commonrepo.DeployFindOption{
		Name:        deploy.Name,
		ProjectName: deploy.ProjectName,
		ServiceName: deploy.ServiceName,
	})
	if err == nil && existed.PreDeploy != nil && deploy.PreDeploy != nil {
		commonservice.EnsureSecretEnvs(existed.PreDeploy.Envs, deploy.PreDeploy.Envs)
	}

	err = correctDeployFields(deploy)
	if err != nil {
		return err
	}

	deploy.UpdateBy = ctx.UserName
	deploy.UpdateTime = time.Now().Unix()
	if err := commonrepo.NewDeployColl().Update(deploy); err != nil {
		err = fmt.Errorf("failed to update deploy %s/%s/%s, err: %w", deploy.ProjectName, deploy.Name, deploy.ServiceName, err)
		log.Error(err)
		return e.ErrUpdateDeployModule.AddErr(err)
	}

	return nil
}

func DeleteDeploy(projectName, name string, log *zap.SugaredLogger) error {
	if len(name) == 0 {
		return e.ErrDeleteDeployModule.AddDesc("empty name")
	}

	if err := commonrepo.NewDeployColl().Delete(projectName, name); err != nil {
		log.Errorf("[Deploy.Delete] %s error: %v", name, err)
		return e.ErrDeleteBuildModule.AddErr(err)
	}
	return nil
}

func correctDeployFields(deploy *commonmodels.Deploy) error {
	for _, repo := range deploy.Repos {
		if repo.Source != setting.SourceFromOther {
			continue
		}
		modifyAuthType(repo)
	}

	// calculate all the referenced keys for frontend
	for _, kv := range deploy.PreDeploy.Envs {
		if kv.Type == commonmodels.Script {
			kv.FunctionReference = util.FindVariableKeyRef(kv.CallFunction)
		}
	}

	return nil
}
