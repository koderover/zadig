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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"helm.sh/helm/v3/pkg/releaseutil"

	"github.com/koderover/zadig/pkg/util"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/collaboration"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func ListProductionEnvs(projectName string, envNames []string, log *zap.SugaredLogger) ([]*EnvResp, error) {
	return ListProducts(projectName, envNames, true, log)
}

// ListProductionGroups TODO we need to verify if the access to the production environment is allowed
func ListProductionGroups(serviceName, envName, productName string, perPage, page int, log *zap.SugaredLogger) ([]*commonservice.ServiceResp, int, error) {
	return ListGroups(serviceName, envName, productName, perPage, page, true, log)
}

func GetServiceInProductionEnv(envName, productName, serviceName string, workLoadType string, log *zap.SugaredLogger) (ret *SvcResp, err error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: util.GetBoolPointer(false)}
	env, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}

	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		log.Errorf("Failed to create kubernetes clientset for cluster id: %s, the error is: %s", env.ClusterID, err)
		return nil, e.ErrGetService.AddErr(err)
	}

	inf, err := informer.NewInformer(env.ClusterID, env.Namespace, clientset)
	if err != nil {
		log.Errorf("Failed to create informer for namespace [%s] in cluster [%s], the error is: %s", env.Namespace, env.ClusterID, err)
		return nil, e.ErrGetService.AddErr(err)
	}

	return GetServiceImpl(serviceName, workLoadType, env, kubeClient, clientset, inf, log)
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

	opt := &commonrepo.RenderSetFindOption{
		Name:        env.Render.Name,
		Revision:    env.Render.Revision,
		EnvName:     env.EnvName,
		ProductTmpl: env.ProductName,
	}
	renderset, exists, err := commonrepo.NewRenderSetColl().FindRenderSet(opt)
	if err != nil || !exists {
		log.Errorf("failed to find renderset for env: %s, err: %v", envName, err)
		return res, errors.Wrapf(err, "failed to find renderset for env: %s", envName)
	}
	rederedYaml, err := kube.RenderEnvService(env, renderset, productService)
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
		case setting.Deployment, setting.StatefulSet, setting.ConfigMap, setting.Service, setting.Ingress:
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
