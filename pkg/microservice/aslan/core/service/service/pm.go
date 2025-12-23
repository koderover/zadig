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

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/pm"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type ServiceTmplBuildObject struct {
	ServiceTmplObject *commonservice.ServiceTmplObject `json:"pm_service_tmpl"`
}

func CreatePMService(username string, args *ServiceTmplBuildObject, log *zap.SugaredLogger) error {
	if len(args.ServiceTmplObject.ServiceName) == 0 {
		return e.ErrInvalidParam.AddDesc("服务名称为空，请检查")
	}
	if !config.ServiceNameRegex.MatchString(args.ServiceTmplObject.ServiceName) {
		return e.ErrInvalidParam.AddDesc("服务名称格式错误，请检查")
	}

	// set the env configs to nil since the user should not be able to set that during service creation step.
	// env config is now set by creating an env.
	args.ServiceTmplObject.EnvConfigs = nil

	opt := &commonrepo.ServiceFindOption{
		ServiceName:   args.ServiceTmplObject.ServiceName,
		ProductName:   args.ServiceTmplObject.ProductName,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	serviceNotFound := false
	if serviceTmpl, err := commonrepo.NewServiceColl().Find(opt); err != nil {
		log.Debugf("Failed to find service with option %+v, err: %s", opt, err)
		serviceNotFound = true
	} else {
		if serviceTmpl.ProductName != args.ServiceTmplObject.ProductName {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("项目 [%s] %s", serviceTmpl.ProductName, "有相同的服务名称存在,请检查!"))
		}
	}

	rev, err := commonutil.GenerateServiceNextRevision(false, args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.ProductName)
	if err != nil {
		return fmt.Errorf("get next pm service revision error: %v", err)
	}
	args.ServiceTmplObject.Revision = rev

	if err := commonrepo.NewServiceColl().Delete(args.ServiceTmplObject.ServiceName, args.ServiceTmplObject.Type, args.ServiceTmplObject.ProductName, setting.ProductStatusDeleting, args.ServiceTmplObject.Revision); err != nil {
		log.Errorf("pmService.delete %s error: %v", args.ServiceTmplObject.ServiceName, err)
	}
	envStatus, err := pm.GenerateEnvStatus(args.ServiceTmplObject.EnvConfigs, log)
	if err != nil {
		log.Errorf("GenerateEnvStatus %s", err)
		return err
	}
	serviceObj := &commonmodels.Service{
		ServiceName:  args.ServiceTmplObject.ServiceName,
		Type:         args.ServiceTmplObject.Type,
		ProductName:  args.ServiceTmplObject.ProductName,
		Revision:     args.ServiceTmplObject.Revision,
		Visibility:   args.ServiceTmplObject.Visibility,
		HealthChecks: args.ServiceTmplObject.HealthChecks,
		EnvConfigs:   args.ServiceTmplObject.EnvConfigs,
		EnvStatuses:  envStatus,
		CreateTime:   time.Now().Unix(),
		CreateBy:     username,
	}

	if err := commonrepo.NewServiceColl().Create(serviceObj); err != nil {
		log.Errorf("pmService.Create %s error: %v", args.ServiceTmplObject.ServiceName, err)
		return e.ErrCreateTemplate.AddDesc(err.Error())
	}

	if serviceNotFound {
		productTempl, err := templaterepo.NewProductColl().Find(args.ServiceTmplObject.ProductName)
		if err != nil {
			log.Errorf("Failed to find project %s, err: %s", args.ServiceTmplObject.ProductName, err)
			return e.ErrCreateTemplate.AddDesc(err.Error())
		}

		//获取项目里面的所有服务
		if len(productTempl.Services) > 0 && !sets.NewString(productTempl.Services[0]...).Has(args.ServiceTmplObject.ServiceName) {
			productTempl.Services[0] = append(productTempl.Services[0], args.ServiceTmplObject.ServiceName)
		} else {
			productTempl.Services = [][]string{{args.ServiceTmplObject.ServiceName}}
		}
		//更新项目模板
		err = templaterepo.NewProductColl().Update(args.ServiceTmplObject.ProductName, productTempl)
		if err != nil {
			log.Errorf("CreatePMService Update %s error: %v", args.ServiceTmplObject.ServiceName, err)
			return e.ErrCreateTemplate.AddDesc(err.Error())
		}

	}
	return nil
}
