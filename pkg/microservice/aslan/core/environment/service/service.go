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
	"strings"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	internalresource "github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
)

func GetServiceContainer(envName, productName, serviceName, container string, log *zap.SugaredLogger) error {
	_, err := validateServiceContainer(envName, productName, serviceName, container)
	if err != nil {
		errMsg := fmt.Sprintf("[%s] container not found : %v", envName, container)
		log.Error(errMsg)
		return e.ErrGetServiceContainer.AddDesc(errMsg)
	}
	return nil
}

// ScaleService 增加或者缩减服务的deployment或者stafefulset的副本数量
func ScaleService(envName, productName, serviceName string, number int, log *zap.SugaredLogger) (err error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), prod.ClusterID)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
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

func Scale(args *ScaleArgs, logger *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), prod.ClusterID)
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

func RestartScale(args *RestartScaleArgs, _ *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{Name: args.ProductName, EnvName: args.EnvName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return err
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), prod.ClusterID)
	if err != nil {
		return err
	}

	// aws secrets needs to be refreshed
	regs, err := commonservice.ListRegistryNamespaces("", true, log.SugaredLogger())
	if err != nil {
		log.Errorf("Failed to get registries to restart container, the error is: %s", err)
		return err
	}
	for _, reg := range regs {
		if reg.RegProvider == config.RegistryTypeAWS {
			if err := kube.CreateOrUpdateRegistrySecret(prod.Namespace, reg, false, kubeClient); err != nil {
				retErr := fmt.Errorf("failed to update pull secret for registry: %s, the error is: %s", reg.ID.Hex(), err)
				log.Errorf("%s\n", retErr.Error())
				return retErr
			}
		}
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

func GetService(envName, productName, serviceName string, workLoadType string, log *zap.SugaredLogger) (ret *SvcResp, err error) {
	ret = &SvcResp{
		ServiceName: serviceName,
		EnvName:     envName,
		ProductName: productName,
		Services:    make([]*internalresource.Service, 0),
		Ingress:     make([]*internalresource.Ingress, 0),
		Scales:      make([]*internalresource.Workload, 0),
	}

	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
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

	namespace := env.Namespace
	switch env.Source {
	case setting.SourceFromExternal, setting.SourceFromHelm:
		if workLoadType == "undefined" && env.Source == setting.SourceFromExternal {
			svcOpt := &commonrepo.ServiceFindOption{ProductName: productName, ServiceName: serviceName, Type: setting.K8SDeployType}
			modelSvc, err := commonrepo.NewServiceColl().Find(svcOpt)
			if err != nil {
				return nil, e.ErrGetService.AddErr(err)
			}
			workLoadType = modelSvc.WorkloadType
		}
		k8sServices, _ := getter.ListServices(namespace, nil, kubeClient)
		switch workLoadType {
		case setting.StatefulSet:
			statefulSet, found, err := getter.GetStatefulSet(namespace, serviceName, kubeClient)
			if !found || err != nil {
				return nil, e.ErrGetService.AddDesc(fmt.Sprintf("service %s not found", serviceName))
			}
			scale := getStatefulSetWorkloadResource(statefulSet, kubeClient, log)
			ret.Scales = append(ret.Scales, scale)
			podLabels := labels.Set(statefulSet.Spec.Template.GetLabels())
			for _, svc := range k8sServices {
				if labels.SelectorFromValidatedSet(svc.Spec.Selector).Matches(podLabels) {
					ret.Services = append(ret.Services, wrapper.Service(svc).Resource())
					break
				}
			}
		case setting.Deployment:
			deploy, found, err := getter.GetDeployment(namespace, serviceName, kubeClient)
			if !found || err != nil {
				return nil, e.ErrGetService.AddDesc(fmt.Sprintf("service %s not found", serviceName))
			}
			scale := getDeploymentWorkloadResource(deploy, kubeClient, log)
			ret.Scales = append(ret.Scales, scale)
			podLabels := labels.Set(deploy.Spec.Template.GetLabels())
			for _, svc := range k8sServices {
				if labels.SelectorFromValidatedSet(svc.Spec.Selector).Matches(podLabels) {
					ret.Services = append(ret.Services, wrapper.Service(svc).Resource())
					break
				}
			}
		default:
			return nil, e.ErrGetService.AddDesc(fmt.Sprintf("service %s not found", serviceName))
		}
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
			ServiceName: service.ServiceName,
			ProductName: service.ProductName,
			Type:        service.Type,
			Revision:    service.Revision,
			//ExcludeStatus: setting.ProductStatusDeleting,
		}

		svcTmpl, err := commonrepo.NewServiceColl().Find(opt)
		if err != nil {
			return nil, e.ErrGetService.AddDesc(
				fmt.Sprintf("服务模板未找到 %s:%d", service.ServiceName, service.Revision),
			)
		}

		renderSetFindOpt := &commonrepo.RenderSetFindOption{
			Name:        env.Render.Name,
			Revision:    service.Render.Revision,
			ProductTmpl: env.ProductName,
			EnvName:     envName,
		}
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

				version, err := clientset.Discovery().ServerVersion()
				if err != nil {
					log.Warnf("Failed to determine server version, error is: %s", err)
					continue
				}
				if kubeclient.VersionLessThan122(version) {
					ing, found, err := getter.GetExtensionsV1Beta1Ingress(namespace, u.GetName(), inf)
					if err != nil || !found {
						log.Warnf("no ingress %s found in %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
						continue
					}

					ret.Ingress = append(ret.Ingress, wrapper.Ingress(ing).Resource())
				} else {
					ing, err := getter.GetNetworkingV1Ingress(namespace, u.GetName(), inf)
					if err != nil {
						log.Warnf("no ingress %s found in %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
						continue
					}

					ingress := &internalresource.Ingress{
						Name:     ing.Name,
						Labels:   ing.Labels,
						HostInfo: wrapper.GetIngressHostInfo(ing),
						IPs:      []string{},
						Age:      util.Age(ing.CreationTimestamp.Unix()),
					}

					ret.Ingress = append(ret.Ingress, ingress)
				}

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

	return
}

// RestartService 在kube中, 如果资源存在就更新不存在就创建
func RestartService(envName string, args *SvcOptArgs, log *zap.SugaredLogger) (err error) {
	productObj, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: envName,
	})
	if err != nil {
		return err
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productObj.ClusterID)
	if err != nil {
		return err
	}

	restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), productObj.ClusterID)
	if err != nil {
		return err
	}

	istioClient, err := versionedclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), productObj.ClusterID)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}
	inf, err := informer.NewInformer(productObj.ClusterID, productObj.Namespace, cls)
	if err != nil {
		return e.ErrCreateEnv.AddErr(err)
	}

	// aws secrets needs to be refreshed
	regs, err := commonservice.ListRegistryNamespaces("", true, log)
	if err != nil {
		log.Errorf("Failed to get registries to restart container, the error is: %s", err)
		return err
	}
	for _, reg := range regs {
		if reg.RegProvider == config.RegistryTypeAWS {
			if err := kube.CreateOrUpdateRegistrySecret(productObj.Namespace, reg, false, kubeClient); err != nil {
				retErr := fmt.Errorf("failed to update pull secret for registry: %s, the error is: %s", reg.ID.Hex(), err)
				log.Errorf("%s\n", retErr.Error())
				return retErr
			}
		}
	}

	switch productObj.Source {
	case setting.SourceFromExternal:
		errList := new(multierror.Error)
		serviceObj, found, err := getter.GetService(productObj.Namespace, args.ServiceName, kubeClient)
		if err != nil || !found {
			return err
		}

		selector := labels.SelectorFromValidatedSet(serviceObj.Spec.Selector)
		if deployments, err := getter.ListDeployments(productObj.Namespace, selector, kubeClient); err == nil {
			log.Infof("namespace:%s , selector:%s , len(deployments):%d", productObj.Namespace, selector, len(deployments))
			for _, deployment := range deployments {
				if err = updater.RestartDeployment(productObj.Namespace, deployment.Name, kubeClient); err != nil {
					errList = multierror.Append(errList, err)
				}
			}
		}

		//statefulSets
		if statefulSets, err := getter.ListStatefulSets(productObj.Namespace, selector, kubeClient); err == nil {
			log.Infof("namespace:%s , selector:%s , len(statefulSets):%d", productObj.Namespace, selector, len(statefulSets))
			for _, statefulSet := range statefulSets {
				if err = updater.RestartStatefulSet(productObj.Namespace, statefulSet.Name, kubeClient); err != nil {
					errList = multierror.Append(errList, err)
				}
			}
		}
	case setting.SourceFromHelm:
		deploy, found, err := getter.GetDeployment(productObj.Namespace, args.ServiceName, kubeClient)
		if err != nil {
			return fmt.Errorf("failed to find resource %s, type %s, err %s", args.ServiceName, setting.Deployment, err.Error())
		}
		if found {
			return updater.RestartDeployment(productObj.Namespace, deploy.Name, kubeClient)
		}

		sts, found, err := getter.GetStatefulSet(productObj.Namespace, args.ServiceName, kubeClient)
		if err != nil {
			return fmt.Errorf("failed to find resource %s, type %s, err %s", args.ServiceName, setting.StatefulSet, err.Error())
		}
		if found {
			return updater.RestartStatefulSet(productObj.Namespace, sts.Name, kubeClient)
		}
	default:
		var serviceTmpl *commonmodels.Service
		var newRender *commonmodels.RenderSet
		var productService *commonmodels.ProductService
		for _, group := range productObj.Services {
			for _, serviceObj := range group {
				if serviceObj.ServiceName == args.ServiceName {
					productService = serviceObj
					serviceTmpl, err = commonservice.GetServiceTemplate(
						serviceObj.ServiceName, setting.K8SDeployType, serviceObj.ProductName, setting.ProductStatusDeleting, serviceObj.Revision, log,
					)
					if err != nil {
						err = e.ErrDeleteProduct.AddDesc(e.DeleteServiceContainerErrMsg + ": " + err.Error())
						return
					}
					opt := &commonrepo.RenderSetFindOption{
						Name:        serviceObj.Render.Name,
						Revision:    serviceObj.Render.Revision,
						ProductTmpl: productObj.ProductName,
						EnvName:     productObj.EnvName,
					}
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
					newRender, inf, kubeClient, istioClient, log)

				// 如果创建依赖服务组有返回错误, 停止等待
				if err != nil {
					log.Errorf(e.DeleteServiceContainerErrMsg+": err:%v", err)
				}
			}()
		}
	}

	return nil
}

func queryPodsStatus(namespace, productName, serviceName string, informer informers.SharedInformerFactory, log *zap.SugaredLogger) (string, string, []string) {
	ls := labels.Set{}
	if productName != "" {
		ls[setting.ProductLabel] = productName
	}
	if serviceName != "" {
		ls[setting.ServiceLabel] = serviceName
	}
	return kube.GetSelectedPodsInfo(ls.AsSelector(), informer, log)
}

// validateServiceContainer validate container with envName like dev
func validateServiceContainer(envName, productName, serviceName, container string) (string, error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return "", err
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), prod.ClusterID)
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

	//helm类型的服务查询所有标签的pod
	if source != setting.SourceFromHelm {
		selector = labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceName}.AsSelector()
	}

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

func getDeploymentWorkloadResource(d *appsv1.Deployment, kubeClient client.Client, log *zap.SugaredLogger) *internalresource.Workload {
	pods, err := getter.ListPods(d.Namespace, labels.SelectorFromValidatedSet(d.Spec.Selector.MatchLabels), kubeClient)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}

	return wrapper.Deployment(d).WorkloadResource(pods)
}

func getStatefulSetWorkloadResource(sts *appsv1.StatefulSet, kubeClient client.Client, log *zap.SugaredLogger) *internalresource.Workload {
	pods, err := getter.ListPods(sts.Namespace, labels.SelectorFromValidatedSet(sts.Spec.Selector.MatchLabels), kubeClient)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}

	return wrapper.StatefulSet(sts).WorkloadResource(pods)
}
