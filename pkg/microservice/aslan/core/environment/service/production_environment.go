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

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/v2/pkg/setting"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/informer"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/util"
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

func ExportProductionYaml(envName, productName, serviceName string, log *zap.SugaredLogger) ([]string, error) {
	var yamls [][]byte
	res := make([]string, 0)

	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{EnvName: envName, Name: productName, Production: util.GetBoolPointer(true)})
	if err != nil {
		log.Errorf("failed to find env [%s][%s] %v", envName, productName, err)
		return res, fmt.Errorf("failed to find env %s/%s %v", envName, productName, err)
	}

	// for services just import not deployed, workloads can't be queried by labels
	productService, ok := env.GetServiceMap()[serviceName]
	if !ok {
		log.Errorf("failed to find product service: %s", serviceName)
		return res, fmt.Errorf("failed to find product service: %s", serviceName)
	}

	namespace := env.Namespace
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		log.Errorf("cluster is not connected [%s][%s][%s]", env.EnvName, env.ProductName, env.ClusterID)
		return res, errors.Wrapf(err, "failed to init kube client for cluster %s", env.ClusterID)
	}

	rederedYaml, err := kube.RenderEnvService(env, productService.GetServiceRender(), productService)
	if err != nil {
		log.Errorf("failed to render service yaml, err: %s", err)
		return res, errors.Wrapf(err, "failed to render service yaml")
	}

	manifests := releaseutil.SplitManifests(rederedYaml)
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Errorf("failed to convert yaml to Unstructured when check resources, manifest is\n%s\n, error: %v", item, err)
			continue
		}
		switch u.GetKind() {
		case setting.Deployment, setting.StatefulSet, setting.ConfigMap, setting.Service, setting.Ingress, setting.CronJob:
			resource, exists, err := getter.GetResourceYamlInCache(namespace, u.GetName(), u.GroupVersionKind(), kubeClient)
			if err != nil {
				log.Errorf("failed to get resource yaml, err: %s", err)
				continue
			}
			if !exists {
				continue
			}
			yamls = append(yamls, resource)
		}
	}

	for _, y := range yamls {
		res = append(res, string(y))
	}
	return res, nil
}

func DeleteProductionProduct(username, envName, productName, requestID string, log *zap.SugaredLogger) (err error) {
	eventStart := time.Now().Unix()
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: util.GetBoolPointer(true)})
	if err != nil {
		log.Errorf("find product error: %v", err)
		return e.ErrDeleteEnv.AddErr(errors.Wrapf(err, "failed to find product from db, project:%s envName:%s error:%v", productName, envName, err))
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

	if productInfo.IstioGrayscale.Enable && !productInfo.IstioGrayscale.IsBase {
		ctx := context.TODO()
		clusterID := productInfo.ClusterID

		kclient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
		if err != nil {
			return fmt.Errorf("failed to get kube client: %s", err)
		}

		restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
		if err != nil {
			return fmt.Errorf("failed to get rest config: %s", err)
		}

		istioClient, err := versionedclient.NewForConfig(restConfig)
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
