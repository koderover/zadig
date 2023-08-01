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
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	internalresource "github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
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

func OpenAPIScale(req *OpenAPIScaleServiceReq, logger *zap.SugaredLogger) error {
	args := &ScaleArgs{
		Type:        req.WorkloadType,
		ProductName: req.ProjectKey,
		EnvName:     req.EnvName,
		Name:        req.WorkloadName,
		Number:      req.TargetReplicas,
	}

	return Scale(args, logger)
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

func GetService(envName, productName, serviceName string, production bool, workLoadType string, isHelmChartDeploy bool, log *zap.SugaredLogger) (ret *SvcResp, err error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: util.GetBoolPointer(production)}
	env, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}

	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		log.Errorf("Failed to create kubernetes clientset for cluster id: %s, the error is: %s", env.ClusterID, err)
		return nil, e.ErrGetService.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}

	inf, err := informer.NewInformer(env.ClusterID, env.Namespace, clientset)
	if err != nil {
		log.Errorf("Failed to create informer for namespace [%s] in cluster [%s], the error is: %s", env.Namespace, env.ClusterID, err)
		return nil, e.ErrGetService.AddErr(err)
	}

	projectInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		return nil, e.ErrGetService.AddErr(errors.Wrapf(err, "failed to find project %s", productName))
	}
	var serviceTmpl *commonmodels.Service
	if projectInfo.IsK8sYamlProduct() {
		productSvc := env.GetServiceMap()[serviceName]
		if productSvc != nil {
			serviceTmpl, err = repository.QueryTemplateService(&commonrepo.ServiceFindOption{
				ServiceName: serviceName,
				Revision:    productSvc.Revision,
				ProductName: productSvc.ProductName,
			}, env.Production)
			if err != nil {
				return nil, e.ErrGetService.AddErr(fmt.Errorf("failed to find template service %s in product %s", serviceName, productName))
			}
			ret, err = GetServiceImpl(serviceName, serviceTmpl, workLoadType, env, clientset, inf, log)
			if err != nil {
				return nil, e.ErrGetService.AddErr(err)
			}
		}
		// when not found service in product, we should find it as a fake service of ZadigxReleaseResource
		// if raw service resources not found will be nil , we should create it
		if ret == nil {
			ret = &SvcResp{
				ServiceName: serviceName,
				EnvName:     envName,
				ProductName: productName,
				Services:    make([]*internalresource.Service, 0),
				Ingress:     make([]*internalresource.Ingress, 0),
				Scales:      make([]*internalresource.Workload, 0),
				CronJobs:    make([]*internalresource.CronJob, 0),
			}
		}
		if env.Source == setting.SourceFromZadig {
			mseResp, err := GetMseServiceImpl(serviceName, workLoadType, env, kubeClient, clientset, inf, log)
			if err != nil {
				return nil, e.ErrGetService.AddErr(errors.Wrap(err, "failed to get mse service"))
			}
			ret.Scales = append(ret.Scales, mseResp.Scales...)
			ret.Services = append(ret.Services, mseResp.Services...)
		}
		ret.Workloads = nil
		ret.Namespace = env.Namespace
	} else {
		ret, err = GetServiceImpl(serviceName, serviceTmpl, workLoadType, env, clientset, inf, log)
		if err != nil {
			return nil, e.ErrGetService.AddErr(err)
		}
		ret.Workloads = nil
		ret.Namespace = env.Namespace
	}

	return ret, nil
}

func toDeploymentWorkload(v *appsv1.Deployment) *commonservice.Workload {
	workload := &commonservice.Workload{
		Name:       v.Name,
		Spec:       v.Spec.Template,
		Type:       setting.Deployment,
		Images:     wrapper.Deployment(v).ImageInfos(),
		Ready:      wrapper.Deployment(v).Ready(),
		Annotation: v.Annotations,
	}
	return workload
}

func toStsWorkload(v *appsv1.StatefulSet) *commonservice.Workload {
	workload := &commonservice.Workload{
		Name:       v.Name,
		Spec:       v.Spec.Template,
		Type:       setting.StatefulSet,
		Images:     wrapper.StatefulSet(v).ImageInfos(),
		Ready:      wrapper.StatefulSet(v).Ready(),
		Annotation: v.Annotations,
	}
	return workload
}

func GetServiceWorkloads(svcTmpl *commonmodels.Service, env *commonmodels.Product, inf informers.SharedInformerFactory, log *zap.SugaredLogger) ([]*commonservice.Workload, error) {
	ret := make([]*commonservice.Workload, 0)
	envName, productName, namespace := env.EnvName, env.ProductName, env.Namespace

	renderSetFindOpt := &commonrepo.RenderSetFindOption{
		Name:        env.Render.Name,
		Revision:    env.Render.Revision,
		ProductTmpl: env.ProductName,
		EnvName:     envName,
	}
	rs, err := commonrepo.NewRenderSetColl().Find(renderSetFindOpt)
	if err != nil {
		log.Errorf("find renderset[%s] error: %v", env.Render.Name, err)
		return nil, e.ErrGetService.AddDesc(fmt.Sprintf("未找到变量集: %s", env.Render.Name))
	}

	parsedYaml, err := kube.RenderServiceYaml(svcTmpl.Yaml, productName, svcTmpl.ServiceName, rs)
	if err != nil {
		log.Errorf("failed to render service yaml, err: %s", err)
		return nil, err
	}
	parsedYaml = kube.ParseSysKeys(namespace, envName, productName, svcTmpl.ServiceName, parsedYaml)

	manifests := releaseutil.SplitManifests(parsedYaml)
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Warnf("Failed to decode yaml to Unstructured, err: %s", err)
			continue
		}

		switch u.GetKind() {
		case setting.Deployment:
			d, err := getter.GetDeploymentByNameWithCache(u.GetName(), namespace, inf)
			if err != nil {
				log.Errorf("failed to get deployment %s, err: %s", u.GetName(), err)
				continue
			}
			wd := wrapper.Deployment(d)
			ret = append(ret, &commonservice.Workload{
				Name:       wd.Name,
				Spec:       wd.Spec.Template,
				Type:       setting.Deployment,
				Images:     wd.ImageInfos(),
				Ready:      wd.Ready(),
				Annotation: wd.Annotations,
			})

		case setting.StatefulSet:
			sts, err := getter.GetStatefulSetByNameWWithCache(u.GetName(), namespace, inf)
			if err != nil {
				log.Errorf("failed to get statefuset %s, err: %s", u.GetName(), err)
				continue
			}
			ws := wrapper.StatefulSet(sts)
			ret = append(ret, &commonservice.Workload{
				Name:       ws.Name,
				Spec:       ws.Spec.Template,
				Type:       setting.StatefulSet,
				Images:     ws.ImageInfos(),
				Ready:      ws.Ready(),
				Annotation: ws.Annotations,
			})
		}
	}

	return ret, nil
}

func GetServiceImpl(serviceName string, serviceTmpl *commonmodels.Service, workLoadType string, env *commonmodels.Product, clientset *kubernetes.Clientset, inf informers.SharedInformerFactory, log *zap.SugaredLogger) (ret *SvcResp, err error) {
	envName, productName := env.EnvName, env.ProductName
	ret = &SvcResp{
		ServiceName: serviceName,
		EnvName:     envName,
		ProductName: productName,
		Services:    make([]*internalresource.Service, 0),
		Ingress:     make([]*internalresource.Ingress, 0),
		Scales:      make([]*internalresource.Workload, 0),
		CronJobs:    make([]*internalresource.CronJob, 0),
	}
	err = nil

	namespace := env.Namespace
	switch env.Source {
	case setting.SourceFromExternal, setting.SourceFromHelm:
		if env.Source == setting.SourceFromExternal {
			svcOpt := &commonrepo.ServiceFindOption{ProductName: productName, ServiceName: serviceName, Type: setting.K8SDeployType}
			modelSvc, err := commonrepo.NewServiceColl().Find(svcOpt)
			if err != nil {
				return nil, e.ErrGetService.AddErr(err)
			}
			workLoadType = modelSvc.WorkloadType
		}
		k8sServices, _ := getter.ListServicesWithCache(nil, inf)
		switch workLoadType {
		case setting.StatefulSet:
			statefulSet, err := getter.GetStatefulSetByNameWWithCache(serviceName, namespace, inf)
			if err != nil {
				return nil, e.ErrGetService.AddDesc(fmt.Sprintf("service %s not found", serviceName))
			}
			scale := getStatefulSetWorkloadResource(statefulSet, inf, log)
			ret.Scales = append(ret.Scales, scale)
			ret.Workloads = append(ret.Workloads, toStsWorkload(statefulSet))
			podLabels := labels.Set(statefulSet.Spec.Template.GetLabels())
			for _, svc := range k8sServices {
				if labels.SelectorFromValidatedSet(svc.Spec.Selector).Matches(podLabels) {
					ret.Services = append(ret.Services, wrapper.Service(svc).Resource())
					break
				}
			}
		case setting.Deployment:
			deploy, err := getter.GetDeploymentByNameWithCache(serviceName, namespace, inf)
			if err != nil {
				return nil, e.ErrGetService.AddDesc(fmt.Sprintf("service %s not found, err: %s", serviceName, err.Error()))
			}
			scale := getDeploymentWorkloadResource(deploy, inf, log)
			ret.Workloads = append(ret.Workloads, toDeploymentWorkload(deploy))
			ret.Scales = append(ret.Scales, scale)
			podLabels := labels.Set(deploy.Spec.Template.GetLabels())
			for _, svc := range k8sServices {
				if labels.SelectorFromValidatedSet(svc.Spec.Selector).Matches(podLabels) {
					ret.Services = append(ret.Services, wrapper.Service(svc).Resource())
					break
				}
			}
		case setting.CronJob:
			version, err := clientset.Discovery().ServerVersion()
			if err != nil {
				log.Warnf("Failed to determine server version, error is: %s", err)
				return ret, nil
			}
			cj, cjBeta, err := getter.GetCronJobByNameWithCache(serviceName, namespace, inf, kubeclient.VersionLessThan121(version))
			if err != nil {
				return ret, nil
			}
			ret.CronJobs = append(ret.CronJobs, getCronJobWorkLoadResource(cj, cjBeta, log))
		default:
			return nil, e.ErrGetService.AddDesc(fmt.Sprintf("service %s not found", serviceName))
		}
	default:
		service := env.GetServiceMap()[serviceName]
		if service == nil {
			return nil, e.ErrGetService.AddDesc(fmt.Sprintf("failed to find service in environment: %s", envName))
		}

		env.EnsureRenderInfo()
		renderSetFindOpt := &commonrepo.RenderSetFindOption{
			Name:        env.Render.Name,
			Revision:    env.Render.Revision,
			ProductTmpl: env.ProductName,
			EnvName:     envName,
		}
		rs, err := commonrepo.NewRenderSetColl().Find(renderSetFindOpt)
		if err != nil {
			log.Errorf("find renderset[%s] error: %v", env.Render.Name, err)
			return nil, e.ErrGetService.AddDesc(fmt.Sprintf("未找到变量集: %s", env.Render.Name))
		}

		parsedYaml, err := kube.RenderServiceYaml(serviceTmpl.Yaml, productName, serviceTmpl.ServiceName, rs)
		if err != nil {
			log.Errorf("failed to render service yaml, err: %s", err)
			return nil, err
		}
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
				d, err := getter.GetDeploymentByNameWithCache(u.GetName(), namespace, inf)
				if err != nil {
					continue
				}

				ret.Scales = append(ret.Scales, getDeploymentWorkloadResource(d, inf, log))
				ret.Workloads = append(ret.Workloads, toDeploymentWorkload(d))
			case setting.StatefulSet:
				sts, err := getter.GetStatefulSetByNameWWithCache(u.GetName(), namespace, inf)
				if err != nil {
					continue
				}

				ret.Scales = append(ret.Scales, getStatefulSetWorkloadResource(sts, inf, log))
				ret.Workloads = append(ret.Workloads, toStsWorkload(sts))
			case setting.CronJob:
				version, err := clientset.Discovery().ServerVersion()
				if err != nil {
					log.Warnf("Failed to determine server version, error is: %s", err)
					continue
				}
				cj, cjBeta, err := getter.GetCronJobByNameWithCache(u.GetName(), namespace, inf, kubeclient.VersionLessThan121(version))
				if err != nil {
					continue
				}
				ret.CronJobs = append(ret.CronJobs, getCronJobWorkLoadResource(cj, cjBeta, log))
			case setting.Ingress:
				version, err := clientset.Discovery().ServerVersion()
				if err != nil {
					log.Warnf("Failed to determine server version, error is: %s", err)
					continue
				}
				if kubeclient.VersionLessThan122(version) {
					ing, found, err := getter.GetExtensionsV1Beta1Ingress(namespace, u.GetName(), inf)
					if err != nil || !found {
						//log.Warnf("no ingress %s found in %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
						continue
					}

					ret.Ingress = append(ret.Ingress, wrapper.Ingress(ing).Resource())
				} else {
					ing, err := getter.GetNetworkingV1Ingress(namespace, u.GetName(), inf)
					if err != nil {
						//log.Warnf("no ingress %s found in %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
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
				svc, err := getter.GetServiceByNameFromCache(u.GetName(), namespace, inf)
				if err != nil {
					//log.Warnf("no svc %s found in %s:%s %v", u.GetName(), service.ServiceName, namespace, err)
					continue
				}

				ret.Services = append(ret.Services, wrapper.Service(svc).Resource())
			}
		}
	}
	return
}

func GetMseServiceImpl(serviceName string, workLoadType string, env *commonmodels.Product, kubeClient client.Client, clientset *kubernetes.Clientset, inf informers.SharedInformerFactory, log *zap.SugaredLogger) (ret *SvcResp, err error) {
	envName, productName := env.EnvName, env.ProductName
	ret = &SvcResp{
		ServiceName: serviceName,
		EnvName:     envName,
		ProductName: productName,
		Services:    make([]*internalresource.Service, 0),
		Ingress:     make([]*internalresource.Ingress, 0),
		Scales:      make([]*internalresource.Workload, 0),
		CronJobs:    make([]*internalresource.CronJob, 0),
	}
	err = nil

	namespace := env.Namespace
	switch env.Source {
	case setting.SourceFromExternal, setting.SourceFromHelm:
		return nil, e.ErrGetService.AddDesc("not support external service")
	default:
		selector := labels.SelectorFromSet(map[string]string{
			types.ZadigReleaseServiceNameLabelKey: serviceName,
			types.ZadigReleaseTypeLabelKey:        types.ZadigReleaseTypeMseGray,
		})
		deployments, err := getter.ListDeployments(namespace, selector, kubeClient)
		if err != nil {
			return nil, e.ErrGetService.AddDesc(fmt.Sprintf("failed to list deployments, service %s in %s: %v", serviceName, namespace, err))
		}
		for _, deployment := range deployments {
			ret.Scales = append(ret.Scales, getDeploymentWorkloadResource(deployment, inf, log))
			ret.Scales[len(ret.Scales)-1].ZadigXReleaseType = config.ZadigXMseGrayRelease
			ret.Scales[len(ret.Scales)-1].ZadigXReleaseTag = deployment.Labels[types.ZadigReleaseVersionLabelKey]
			ret.Workloads = append(ret.Workloads, toDeploymentWorkload(deployment))
		}
		services, err := getter.ListServices(namespace, selector, kubeClient)
		if err != nil {
			return nil, e.ErrGetService.AddDesc(fmt.Sprintf("failed to get service, service %s in %s: %v", serviceName, namespace, err))
		}
		for _, service := range services {
			ret.Services = append(ret.Services, wrapper.Service(service).Resource())
		}
	}

	return
}

func BatchPreviewService(args []*PreviewServiceArgs, logger *zap.SugaredLogger) ([]*SvcDiffResult, error) {
	ret := make([]*SvcDiffResult, 0)
	for _, arg := range args {
		previewRet, err := PreviewService(arg, logger)
		if err != nil {
			previewRet = &SvcDiffResult{
				ServiceName: arg.ServiceName,
				Error:       err.Error(),
			}
		}
		ret = append(ret, previewRet)
	}
	return ret, nil
}

func PreviewService(args *PreviewServiceArgs, _ *zap.SugaredLogger) (*SvcDiffResult, error) {
	newVariableYaml, err := commontypes.RenderVariableKVToYaml(args.VariableKVs)
	if err != nil {
		return nil, e.ErrPreviewYaml.AddErr(err)
	}

	ret := &SvcDiffResult{
		ServiceName: args.ServiceName,
		Current:     TmplYaml{},
		Latest:      TmplYaml{},
	}

	curYaml, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
		ProductName:           args.ProductName,
		EnvName:               args.EnvName,
		ServiceName:           args.ServiceName,
		UpdateServiceRevision: args.UpdateServiceRevision,
	})
	if err != nil {
		curYaml = ""
		ret.Error = fmt.Sprintf("failed to fetch current applied yaml, productName: %s envName: %s serviceName: %s, updateSvcRevision: %v, variableYaml: %s err: %s",
			args.ProductName, args.EnvName, args.ServiceName, args.UpdateServiceRevision, newVariableYaml, err)
		log.Errorf(ret.Error)
	}

	// for situations only update images, replace images directly
	if !args.UpdateServiceRevision && len(args.VariableKVs) == 0 {
		latestYaml, _, err := kube.ReplaceWorkloadImages(curYaml, args.ServiceModules)
		if err != nil {
			return nil, e.ErrPreviewYaml.AddErr(err)
		}
		ret.Current.Yaml = curYaml
		ret.Latest.Yaml = latestYaml
		return ret, nil
	}

	latestYaml, _, _, err := kube.GenerateRenderedYaml(&kube.GeneSvcYamlOption{
		ProductName:           args.ProductName,
		EnvName:               args.EnvName,
		ServiceName:           args.ServiceName,
		UpdateServiceRevision: args.UpdateServiceRevision,
		VariableYaml:          newVariableYaml,
		VariableKVs:           args.VariableKVs,
		Containers:            args.ServiceModules,
	})
	if err != nil {
		return nil, e.ErrPreviewYaml.AddErr(err)
	}

	ret.Current.Yaml = curYaml
	ret.Latest.Yaml = latestYaml

	return ret, nil
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
		productObj.EnsureRenderInfo()
		oldRenderInfo := productObj.Render
		var productService *commonmodels.ProductService

		serviceObj, ok := productObj.GetServiceMap()[args.ServiceName]
		if !ok {
			return nil
		}

		productService = serviceObj

		serviceTmpl, err = repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ProductName: serviceObj.ProductName,
			ServiceName: serviceObj.ServiceName,
			Revision:    serviceObj.Revision,
			Type:        setting.K8SDeployType,
		}, productObj.Production)

		opt := &commonrepo.RenderSetFindOption{
			Name:        oldRenderInfo.Name,
			Revision:    oldRenderInfo.Revision,
			ProductTmpl: productObj.ProductName,
			EnvName:     productObj.EnvName,
		}
		newRender, err = commonrepo.NewRenderSetColl().Find(opt)
		if err != nil {
			log.Errorf("[%s][P:%s]renderset Find error: %v", productObj.EnvName, productObj.ProductName, err)
			err = e.ErrRestartService.AddErr(err)
			return
		}

		// for services deployed by zadig, service will be applied when restarting
		if commonutil.ServiceDeployed(serviceTmpl.ServiceName, productObj.ServiceDeployStrategy) {
			_, err = upsertService(
				productObj,
				productService,
				productService,
				newRender, oldRenderInfo, !productObj.Production, inf, kubeClient, istioClient, log)
		} else {
			err = restartRelatedWorkloads(productObj, productService, newRender, kubeClient, log)
		}
		log.Infof("restart resource from namespace:%s/serviceName:%s ", productObj.Namespace, args.ServiceName)

		if err != nil {
			log.Errorf("failed to restart service, err:%v", err)
			err = e.ErrRestartService.AddErr(err)
			return
		}
	}

	return nil
}

func queryPodsStatus(productInfo *commonmodels.Product, serviceTmpl *commonmodels.Service, serviceName string, clientset *kubernetes.Clientset, informer informers.SharedInformerFactory, log *zap.SugaredLogger) *ZadigServiceStatusResp {
	resp := &ZadigServiceStatusResp{
		ServiceName: serviceName,
		PodStatus:   setting.PodError,
		Ready:       setting.PodNotReady,
		Ingress:     nil,
		Images:      []string{},
	}

	if len(serviceTmpl.Containers) == 0 {
		resp.PodStatus = setting.PodSucceeded
		resp.Ready = setting.PodReady
		return resp
	}

	svcResp, err := GetServiceImpl(serviceTmpl.ServiceName, serviceTmpl, "", productInfo, clientset, informer, log)
	if err != nil {
		return resp
	}
	resp.Ingress = svcResp.Ingress
	resp.Workloads = svcResp.Workloads

	suspendCronJobCount := 0
	for _, cronJob := range svcResp.CronJobs {
		if cronJob.Suspend {
			suspendCronJobCount++
		}
	}

	pods := make([]*resource.Pod, 0)
	for _, svc := range svcResp.Scales {
		pods = append(pods, svc.Pods...)
	}

	if len(pods) == 0 && len(svcResp.CronJobs) == 0 {
		resp.PodStatus, resp.Ready = setting.PodNonStarted, setting.PodNotReady
		return resp
	}

	imageSet := sets.String{}
	for _, pod := range pods {
		for _, container := range pod.ContainerStatuses {
			imageSet.Insert(container.Image)
		}
	}

	for _, cronJob := range svcResp.CronJobs {
		for _, image := range cronJob.Images {
			imageSet.Insert(image.Image)
		}
	}

	resp.Images = imageSet.List()

	ready := setting.PodReady

	if len(svcResp.Workloads) == 0 && len(svcResp.CronJobs) > 0 {
		if len(svcResp.CronJobs) == suspendCronJobCount {
			resp.PodStatus = setting.ServiceStatusAllSuspended
		} else if suspendCronJobCount == 0 {
			resp.PodStatus = setting.ServiceStatusNoSuspended
		} else {
			resp.PodStatus = setting.ServiceStatusPartSuspended
		}
		return resp
	}

	succeededPods := 0
	for _, pod := range pods {
		if pod.Succeed {
			succeededPods++
			continue
		}
		if !pod.Ready {
			resp.PodStatus, resp.Ready = setting.PodUnstable, setting.PodNotReady
			return resp
		}
	}

	if len(pods) == succeededPods {
		resp.PodStatus, resp.Ready = string(corev1.PodSucceeded), setting.JobReady
		return resp
	}

	resp.PodStatus, resp.Ready = setting.PodRunning, ready
	return resp
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

func getDeploymentWorkloadResource(d *appsv1.Deployment, informer informers.SharedInformerFactory, log *zap.SugaredLogger) *internalresource.Workload {
	pods, err := getter.ListPodsWithCache(labels.SelectorFromValidatedSet(d.Spec.Selector.MatchLabels), informer)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}

	return wrapper.Deployment(d).WorkloadResource(pods)
}

func getStatefulSetWorkloadResource(sts *appsv1.StatefulSet, informer informers.SharedInformerFactory, log *zap.SugaredLogger) *internalresource.Workload {
	pods, err := getter.ListPodsWithCache(labels.SelectorFromValidatedSet(sts.Spec.Selector.MatchLabels), informer)
	if err != nil {
		log.Warnf("Failed to get pods, err: %s", err)
	}

	return wrapper.StatefulSet(sts).WorkloadResource(pods)
}

func getCronJobWorkLoadResource(cornJob *batchv1.CronJob, cronJobBeta *v1beta1.CronJob, log *zap.SugaredLogger) *internalresource.CronJob {
	return wrapper.CronJob(cornJob, cronJobBeta).CronJobResource()
}
