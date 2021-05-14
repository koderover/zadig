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

	"helm.sh/helm/v3/pkg/releaseutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	internalresource "github.com/koderover/zadig/lib/internal/kube/resource"
	"github.com/koderover/zadig/lib/internal/kube/wrapper"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/lib/microservice/aslan/internal/log"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/kube/serializer"
	"github.com/koderover/zadig/lib/tool/kube/updater"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type helmResource struct {
	Deployments  []*appsv1.Deployment
	StatefulSets []*appsv1.StatefulSet
	Services     []*corev1.Service
	Ingress      []*extensionsv1beta1.Ingress
}

func GetServiceContainer(envName, productName, serviceName, container string, log *xlog.Logger) error {
	_, err := validateServiceContainer(envName, productName, serviceName, container)
	if err != nil {
		errMsg := fmt.Sprintf("[%s] container not found : %v", envName, container)
		log.Error(errMsg)
		return e.ErrGetServiceContainer.AddDesc(errMsg)
	}
	return nil
}

// ScaleService 增加或者缩减服务的deployment或者stafefulset的副本数量
func ScaleService(envName, productName, serviceName string, number int, log *xlog.Logger) (err error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	kubeClient, err := kube.GetKubeClient(prod.ClusterId)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	// TODO: 目前强制定死扩容数量为0-20, 以后可以改成配置
	if number > 20 || number < 0 {
		return e.ErrInvalidParam.AddDesc("伸缩数量范围[0..20]")
	}

	selector := labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceName}.AsSelector()
	ds, err := getter.ListDeployments(prod.Namespace, selector, kubeClient)
	if err != nil {
		log.Error(err)
		return e.ErrScaleService.AddDesc("伸缩Deployment失败")
	}
	for _, d := range ds {
		err := updater.ScaleDeployment(d.Namespace, d.Name, number, kubeClient)
		if err != nil {
			log.Error(err)
			return e.ErrScaleService.AddDesc("伸缩Deployment失败")
		}
	}

	ss, err := getter.ListStatefulSets(prod.Namespace, selector, kubeClient)
	if err != nil {
		log.Error(err)
		return e.ErrScaleService.AddDesc("伸缩StatefulSet失败")
	}
	for _, s := range ss {
		err := updater.ScaleStatefulSet(s.Namespace, s.Name, number, kubeClient)
		if err != nil {
			log.Error(err)
			return e.ErrScaleService.AddDesc("伸缩StatefulSet失败")
		}
	}

	return nil
}

func Scale(args *ScaleArgs, logger *xlog.Logger) error {
	if args.Number > 20 || args.Number < 0 {
		return e.ErrInvalidParam.AddDesc("伸缩数量范围[0..20]")
	}

	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	kubeClient, err := kube.GetKubeClient(prod.ClusterId)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	namespace := prod.Namespace

	switch args.Type {
	case setting.Deployment:
		err := updater.ScaleDeployment(namespace, args.Name, args.Number, kubeClient)
		if err != nil {
			logger.Errorf("failed to scale %s/deploy/%s to %d", namespace, args.Name, args.Number)
		}
	case setting.StatefulSet:
		err := updater.ScaleStatefulSet(namespace, args.Name, args.Number, kubeClient)
		if err != nil {
			logger.Errorf("failed to scale %s/sts/%s to %d", namespace, args.Name, args.Number)
		}
	}

	return nil
}

func RestartScale(args *RestartScaleArgs, _ *xlog.Logger) error {
	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return err
	}

	kubeClient, err := kube.GetKubeClient(prod.ClusterId)
	if err != nil {
		return err
	}

	switch args.Type {
	case setting.Deployment:
		err = updater.RestartDeployment(prod.Namespace, args.Name, kubeClient)
	case setting.StatefulSet:
		err = updater.RestartStatefulSet(prod.Namespace, args.Name, kubeClient)
	}

	if err != nil {
		log.Errorf("failed to restart %s/%s/%s %v", prod.Namespace, args.Type, args.Name)
		return e.ErrRestartService.AddErr(err)
	}

	return nil
}

func UpdateService(envName string, args *SvcOptArgs, log *xlog.Logger) error {

	svc := &commonmodels.ProductService{
		ServiceName: args.ServiceName,
		Type:        args.ServiceType,
		Revision:    args.ServiceRev.NextRevision,
		Containers:  args.ServiceRev.Containers,
		Configs:     make([]*commonmodels.ServiceConfig, 0),
	}

	if svc.Type == setting.K8SDeployType {
		opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: envName}
		exitedProd, err := commonrepo.NewProductColl().Find(opt)
		if err != nil {
			log.Error(err)
			return errors.New(e.UpsertServiceErrMsg)
		}

		kubeClient, err := kube.GetKubeClient(exitedProd.ClusterId)
		if err != nil {
			return e.ErrUpdateEnv.AddErr(err)
		}

		switch exitedProd.Status {
		case setting.ProductStatusCreating, setting.ProductStatusUpdating, setting.ProductStatusDeleting:
			log.Errorf("[%s][P:%s] Product is not in valid status", envName, args.ProductName)
			return e.ErrUpdateEnv.AddDesc(e.EnvCantUpdatedMsg)
		default:
			// do nothing
		}

		// 适配老的产品没有renderset为空的情况
		if exitedProd.Render == nil {
			exitedProd.Render = &commonmodels.RenderInfo{ProductTmpl: exitedProd.ProductName}
		}
		// 检查renderset是否覆盖服务所有key
		newRender, err := commonservice.ValidateRenderSet(args.ProductName, exitedProd.Render.Name, args.ServiceName, log)
		if err != nil {
			log.Errorf("[%s][P:%s] validate product renderset error: %v", envName, args.ProductName, err)
			return e.ErrUpdateProduct.AddDesc(err.Error())
		}
		svc.Render = &commonmodels.RenderInfo{Name: newRender.Name, Revision: newRender.Revision, ProductTmpl: newRender.ProductTmpl}

		for _, cr := range args.ServiceRev.ConfigRevisions {
			cfg := &commonmodels.ServiceConfig{
				ConfigName: cr.ConfigName,
				Revision:   cr.NextRevision,
			}
			svc.Configs = append(svc.Configs, cfg)
		}

		_, err = upsertService(
			true,
			exitedProd,
			svc,
			exitedProd.GetServiceMap()[svc.ServiceName],
			newRender, kubeClient, log)

		// 如果创建依赖服务组有返回错误, 停止等待
		if err != nil {
			log.Error(err)
			return e.ErrUpdateProduct.AddDesc(err.Error())
		}

		// 只有配置模板改动才会重启pod
		err = restartService(envName, args.ProductName, args.ServiceRev, log)
		if err != nil {
			log.Error(err)
			return err
		}

		// 更新产品服务
		for _, group := range exitedProd.Services {
			for i, service := range group {
				if service.ServiceName == args.ServiceName && service.Type == args.ServiceType {
					group[i] = svc
				}
			}
		}

		if err := commonrepo.NewProductColl().Update(exitedProd); err != nil {
			log.Errorf("[%s][%s] Product.Update error: %v", envName, args.ProductName, err)
			return e.ErrUpdateProduct
		}
	}

	return nil
}

func GetService(envName, productName, serviceName string, log *xlog.Logger) (ret *SvcResp, err error) {
	ret = &SvcResp{
		ServiceName: serviceName,
		EnvName:     envName,
		ProductName: productName,
	}

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	env, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}

	kubeClient, err := kube.GetKubeClient(env.ClusterId)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}

	namespace := env.Namespace
	switch env.Source {
	default:
		var service *commonmodels.ProductService
		for _, svcArray := range env.Services {
			for _, svc := range svcArray {
				if svc.ServiceName != serviceName {
					continue
				} else {
					service = svc
					break
				}
			}
		}

		if service == nil {
			return nil, e.ErrGetService.AddDesc("没有找到服务: " + serviceName)
		}

		// 获取服务模板
		opt := &commonrepo.ServiceFindOption{
			ServiceName:   service.ServiceName,
			Type:          service.Type,
			Revision:      service.Revision,
			ExcludeStatus: setting.ProductStatusDeleting,
		}

		svcTmpl, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			return nil, e.ErrGetService.AddDesc(
				fmt.Sprintf("服务模板未找到 %s:%d", service.ServiceName, service.Revision),
			)
		}

		renderSetFindOpt := &commonrepo.RenderSetFindOption{Name: env.Render.Name, Revision: env.Render.Revision}
		rs, err := commonrepo.NewRenderSetColl().Find(renderSetFindOpt)
		if err != nil {
			log.Errorf("find renderset[%s] error: %v", service.Render.Name, err)
			return nil, e.ErrGetService.AddDesc(fmt.Sprintf("未找到变量集: %s", service.Render.Name))
		}

		// 渲染配置集
		parsedYaml := commonservice.RenderValueForString(svcTmpl.Yaml, rs)
		// 渲染系统变量键值
		parsedYaml = kube.ParseSysKeys(namespace, envName, productName, service.ServiceName, parsedYaml)

		manifests := releaseutil.SplitManifests(parsedYaml)
		for _, item := range manifests {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
			if err != nil {
				log.Warnf("Failed to decode yaml to Unstructured, err: %s", err)
				continue
			}

			switch u.GetKind() {
			case setting.Deployment:
				d, found, err := getter.GetDeployment(namespace, u.GetName(), kubeClient)
				if err != nil || !found {
					log.Warnf("failed to get deployment %s %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
					continue
				}

				ret.Scales = append(ret.Scales, getDeploymentWorkloadResource(d, kubeClient, log))

			case setting.StatefulSet:
				sts, found, err := getter.GetStatefulSet(namespace, u.GetName(), kubeClient)
				if err != nil || !found {
					log.Warnf("failed to get statefulSet %s %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
					continue
				}

				ret.Scales = append(ret.Scales, getStatefulSetWorkloadResource(sts, kubeClient, log))

			case setting.Ingress:
				ing, found, err := getter.GetIngress(namespace, u.GetName(), kubeClient)
				if err != nil || !found {
					log.Warnf("no ingress %s found in %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
					continue
				}

				ret.Ingress = append(ret.Ingress, wrapper.Ingress(ing).Resource())

			case setting.Service:
				svc, found, err := getter.GetService(namespace, u.GetName(), kubeClient)
				if err != nil || !found {
					log.Warnf("no svc %s found in %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
					continue
				}

				ret.Services = append(ret.Services, wrapper.Service(svc).Resource())
			}
		}
	}

	if len(ret.Ingress) == 0 {
		ret.Ingress = make([]*internalresource.Ingress, 0)
	}

	if len(ret.Scales) == 0 {
		ret.Scales = make([]*internalresource.Workload, 0)
	}

	if len(ret.Services) == 0 {
		ret.Services = make([]*internalresource.Service, 0)
	}

	return
}

// RestartService 在kube中, 如果资源存在就更新不存在就创建
func RestartService(envName string, args *SvcOptArgs, log *xlog.Logger) (err error) {
	productObj, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: envName,
	})
	if err != nil {
		return err
	}
	kubeClient, err := kube.GetKubeClient(productObj.ClusterId)
	if err != nil {
		return err
	}

	var serviceTmpl *commonmodels.Service
	var newRender *commonmodels.RenderSet
	var productService *commonmodels.ProductService
	for _, group := range productObj.Services {
		for _, serviceObj := range group {
			if serviceObj.ServiceName == args.ServiceName {
				productService = serviceObj
				serviceTmpl, err = commonservice.GetServiceTemplate(
					serviceObj.ServiceName, setting.K8SDeployType, "", setting.ProductStatusDeleting, serviceObj.Revision, log,
				)
				if err != nil {
					err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
					return
				}
				opt := &commonrepo.RenderSetFindOption{Name: serviceObj.Render.Name, Revision: serviceObj.Render.Revision}
				newRender, err = commonrepo.NewRenderSetColl().Find(opt)
				if err != nil {
					log.Errorf("[%s][P:%s]renderset Find error: %v", productObj.EnvName, productObj.ProductName, err)
					err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
					return
				}
				break
			}
		}
	}
	if serviceTmpl != nil && newRender != nil && productService != nil {
		go func() {
			log.Infof("upsert resource from namespace:%s/serviceName:%s ", productObj.Namespace, args.ServiceName)
			_, err := upsertService(
				true,
				productObj,
				productService,
				productService,
				newRender, kubeClient, log)

			// 如果创建依赖服务组有返回错误, 停止等待
			if err != nil {
				log.Errorf(e.DeleteServiceContainerErrMsg+": err:%v", err)
			}
		}()
	}

	return nil
}

// queryServiceStatus query service status
// If service has pods, service status = pod status (if pod is not running or succeed or failed, service status = unstable)
// If service doesnt have pods, service status = success (all objects created) or failed (fail to create some objects).
// 正常：StatusRunning or StatusSucceed
// 错误：StatusError or StatusFailed
func queryServiceStatus(namespace, envName, productName string, serviceTmpl *commonmodels.Service, kubeClient client.Client, log *xlog.Logger) (string, string, []string) {
	log.Infof("queryServiceStatus of service: %s of product: %s in namespace %s", serviceTmpl.ServiceName, productName, namespace)

	if len(serviceTmpl.Containers) > 0 {
		// 有容器时，根据pods status判断服务状态
		return queryPodsStatus(namespace, productName, serviceTmpl.ServiceName, kubeClient, log)
	}

	return setting.PodSucceeded, setting.PodReady, []string{}

}

func queryPodsStatus(namespace, productName, serviceName string, kubeClient client.Client, log *xlog.Logger) (string, string, []string) {
	ls := labels.Set{}
	if productName != "" {
		ls[setting.ProductLabel] = productName
	}
	if serviceName != "" {
		ls[setting.ServiceLabel] = serviceName
	}
	return kube.GetSelectedPodsInfo(namespace, ls.AsSelector(), kubeClient, log)
}

func listHelmResources(namespace string, release string, kubeClient client.Client) (*helmResource, error) {
	var err error
	res := &helmResource{}
	selector := labels.Set{"release": release}.AsSelector()
	// old style
	ss, err := getter.ListServices(namespace, selector, kubeClient)
	if err != nil {
		return nil, err
	}
	res.Services = append(res.Services, ss...)

	ing, err := getter.ListIngresses(namespace, selector, kubeClient)
	if err != nil {
		return nil, err
	}
	res.Ingress = append(res.Ingress, ing...)

	ds, err := getter.ListDeployments(namespace, selector, kubeClient)
	if err != nil {
		return nil, err
	}
	res.Deployments = append(res.Deployments, ds...)

	sts, err := getter.ListStatefulSets(namespace, selector, kubeClient)
	if err != nil {
		return nil, err
	}
	res.StatefulSets = append(res.StatefulSets, sts...)

	return res, nil
}

// validateServiceContainer validate container with envName like dev
func validateServiceContainer(envName, productName, serviceName, container string) (string, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return "", err
	}

	kubeClient, err := kube.GetKubeClient(prod.ClusterId)
	if err != nil {
		return "", err
	}

	return validateServiceContainer2(
		prod.Namespace, envName, productName, serviceName, container, prod.Source, kubeClient,
	)
}

// validateServiceContainer2 validate container with raw namespace like dev-product
func validateServiceContainer2(namespace, envName, productName, serviceName, container, source string, kubeClient client.Client) (string, error) {
	var selector labels.Selector

	pods, err := getter.ListPods(namespace, selector, kubeClient)
	if err != nil {
		return "", fmt.Errorf("[%s] ListPods %s/%s error: %v", namespace, productName, serviceName, err)
	}

	for _, pod := range pods {
		for _, c := range pod.Spec.Containers {
			if c.Name == container || strings.Contains(c.Image, container) {
				return c.Image, nil
			}
		}
	}
	log.Errorf("[%s]container %s not found", namespace, container)

	return "", &ContainerNotFound{
		ServiceName: serviceName,
		Container:   container,
		EnvName:     envName,
		ProductName: productName,
	}
}

func getDeploymentWorkloadResource(d *appsv1.Deployment, kubeClient client.Client, log *xlog.Logger) *internalresource.Workload {
	pods, err := getter.ListPods(d.Namespace, labels.SelectorFromValidatedSet(d.Spec.Selector.MatchLabels), kubeClient)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}

	return wrapper.Deployment(d).WorkloadResource(pods)
}

func getStatefulSetWorkloadResource(sts *appsv1.StatefulSet, kubeClient client.Client, log *xlog.Logger) *internalresource.Workload {
	pods, err := getter.ListPods(sts.Namespace, labels.SelectorFromValidatedSet(sts.Spec.Selector.MatchLabels), kubeClient)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}

	return wrapper.StatefulSet(sts).WorkloadResource(pods)
}
