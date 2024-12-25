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
	"context"
	"fmt"
	"time"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func ListProductionEnvs(userId string, projectName string, envNames []string, log *zap.SugaredLogger) ([]*EnvResp, error) {
	envs, err := ListProducts(userId, projectName, envNames, true, log)
	if err != nil {
		return nil, err
	}
	for _, env := range envs {
		relatedEnvs, err := commonrepo.NewProductColl().ListEnvByNamespace(env.ClusterID, env.Namespace)
		if err != nil {
			continue
		}
		if len(relatedEnvs) > 1 {
			env.SharedNS = true
		}
	}
	return envs, nil
}

func ListProductionGroups(serviceName, envName, productName string, perPage, page int, log *zap.SugaredLogger) ([]*commonservice.ServiceResp, int, error) {
	return ListGroups(serviceName, envName, productName, perPage, page, true, log)
}

func DeleteProductionProduct(username, envName, productName, requestID string, log *zap.SugaredLogger) (err error) {
	eventStart := time.Now().Unix()
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: util.GetBoolPointer(true)})
	if err != nil {
		log.Errorf("find product error: %v", err)
		return e.ErrDeleteEnv.AddErr(errors.Wrapf(err, "failed to find product from db, project:%s envName:%s error:%v", productName, envName, err))
	}

	// delete informer's cache
	clientmanager.NewKubeClientManager().DeleteInformer(productInfo.ClusterID, productInfo.Namespace)
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
		notify.SendErrorMessage(username, title, requestID, err, log)
		_ = commonrepo.NewProductColl().UpdateStatus(envName, productName, setting.ProductStatusUnknown)
	} else {
		title := fmt.Sprintf("删除项目:[%s] 环境:[%s] 成功!", productName, envName)
		content := fmt.Sprintf("namespace:%s", productInfo.Namespace)
		notify.SendMessage(username, title, content, requestID, log)
	}

	// remove custom labels
	err = service.DeleteZadigLabelFromNamespace(productInfo.Namespace, productInfo.ClusterID, log)
	if err != nil {
		log.Errorf("failed to delete zadig label from namespace %s, error: %v", productInfo.Namespace, err)
	}

	err = deleteEnvSleepCron(productInfo.ProductName, productInfo.EnvName)
	if err != nil {
		log.Errorf("deleteEnvSleepCron error: %v", err)
	}

	if productInfo.IstioGrayscale.Enable && !productInfo.IstioGrayscale.IsBase {
		ctx := context.TODO()
		clusterID := productInfo.ClusterID

		kclient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
		if err != nil {
			return fmt.Errorf("failed to get kube client: %s", err)
		}

		istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(clusterID)
		if err != nil {
			return fmt.Errorf("failed to new istio client: %s", err)
		}

		// Delete all VirtualServices delivered by the Zadig.
		err = deleteVirtualServices(ctx, kclient, istioClient, productInfo.Namespace)
		if err != nil {
			return fmt.Errorf("failed to delete VirtualServices that Zadig created in ns `%s`: %s", productInfo.Namespace, err)
		}

		// Remove the `istio-injection=enabled` label of the namespace.
		err = removeIstioLabel(ctx, kclient, productInfo.Namespace)
		if err != nil {
			return fmt.Errorf("failed to remove istio label on ns `%s`: %s", productInfo.Namespace, err)
		}

		// Restart the istio-Proxy injected Pods.
		err = removePodsIstioProxy(ctx, kclient, productInfo.Namespace)
		if err != nil {
			return fmt.Errorf("failed to remove istio-proxy from pods in ns `%s`: %s", productInfo.Namespace, err)
		}
	}

	return nil
}
