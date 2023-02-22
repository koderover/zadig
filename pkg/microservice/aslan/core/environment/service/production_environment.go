/*
Copyright 2023 The KodeRover Authors.

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

	"github.com/pkg/errors"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"go.uber.org/zap"
)

func ListProductionEnvs(projectName string, envNames []string, log *zap.SugaredLogger) ([]*EnvResp, error) {
	return ListProducts(projectName, envNames, true, log)
}

// ListProductionGroups TODO we need to verify if the access to the production environment is allowed
func ListProductionGroups(serviceName, envName, productName string, perPage, page int, log *zap.SugaredLogger) ([]*commonservice.ServiceResp, int, error) {
	return ListGroups(serviceName, envName, productName, perPage, page, true, log)
}

func DeleteProductionProduct(username, envName, productName, requestID string, log *zap.SugaredLogger) (err error) {
	eventStart := time.Now().Unix()
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName})
	if err != nil {
		log.Errorf("find product error: %v", err)
		return e.ErrDeleteEnv.AddErr(errors.Wrapf(err, "find product %s error", productName))
	}

	// delete informer's cache
	informer.DeleteInformer(productInfo.ClusterID, productInfo.Namespace)
	envCMMap, err := collaboration.GetEnvCMMap([]string{productName}, log)
	if err != nil {
		return e.ErrUpdateEnv.AddErr(errors.Wrapf(err, "get env cm map error"))
	}
	if cmSets, ok := envCMMap[collaboration.BuildEnvCMMapKey(productName, envName)]; ok {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("this is a base environment, collaborations:%v is related", cmSets.List()))
	}

	err = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusDeleting)
	if err != nil {
		log.Errorf("[%s][%s] update product status error: %v", username, productInfo.Namespace, err)
		return e.ErrDeleteEnv.AddDesc("更新环境状态失败: " + err.Error())
	}

	log.Infof("[%s] delete product %s", username, productInfo.Namespace)
	commonservice.LogProductStats(username, setting.DeleteProductEvent, productName, requestID, eventStart, log)

	err = commonrepo.NewProductColl().Delete(envName, productName)
	if err != nil {
		log.Errorf("Production product delete error: %v", err)
		title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 失败!", productName, envName)
		commonservice.SendErrorMessage(username, title, requestID, err, log)
		_ = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
	} else {
		title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
		content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
		commonservice.SendMessage(username, title, content, requestID, log)
	}
	return nil
}
