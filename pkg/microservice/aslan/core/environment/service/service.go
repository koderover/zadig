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
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalresource "github.com/koderover/zadig/v2/pkg/shared/kube/resource"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

func RestartScale(args *RestartScaleArgs, production bool, _ *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{
		Name:       args.ProductName,
		EnvName:    args.EnvName,
		Production: &production,
	}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return err
	}
	if prod.IsSleeping() {
		return e.ErrScaleService.AddErr(fmt.Errorf("environment is sleeping"))
	}

	// aws secrets needs to be refreshed
	regs, err := commonservice.ListRegistryNamespaces("", true, log.SugaredLogger())
	if err != nil {
		log.Errorf("Failed to get registries to restart container, the error is: %s", err)
		return err
	}
	for _, reg := range regs {
		if reg.RegProvider == config.RegistryTypeAWS {
			if err := kube.CreateOrUpdateRegistrySecret(prod.Namespace, prod.ClusterID, reg, false); err != nil {
				retErr := fmt.Errorf("failed to update pull secret for registry: %s, the error is: %s", reg.ID.Hex(), err)
				log.Errorf("%s\n", retErr.Error())
				return retErr
			}
		}
	}

	switch args.Type {
	case setting.Deployment:
		err = updater.RestartDeploymentV2(context.Background(), prod.ClusterID, prod.Namespace, args.Name)
	case setting.StatefulSet:
		err = updater.RestartStatefulSetV2(context.Background(), prod.ClusterID, prod.Namespace, args.Name)
	}

	if err != nil {
		log.Errorf("failed to restart %s/%s/%s %v", prod.Namespace, args.Type, args.Name)
		return e.ErrRestartService.AddErr(err)
	}

	return nil
}

func GetService(envName, productName, serviceName string, production bool, workLoadType string, log *zap.SugaredLogger) (ret *commonservice.SvcResp, err error) {
	opt := &commonrepo.ProductFindOptions{Name: productName, EnvName: envName, Production: util.GetBoolPointer(production)}
	env, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}

	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(env.ClusterID)
	if err != nil {
		log.Errorf("Failed to create kubernetes clientset for cluster id: %s, the error is: %s", env.ClusterID, err)
		return nil, e.ErrGetService.AddErr(err)
	}
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(env.ClusterID)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}

	inf, err := clientmanager.NewKubeClientManager().GetInformer(env.ClusterID, env.Namespace)
	if err != nil {
		log.Errorf("Failed to create informer for namespace [%s] in cluster [%s], the error is: %s", env.Namespace, env.ClusterID, err)
		return nil, e.ErrGetService.AddErr(err)
	}

	projectInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		return nil, e.ErrGetService.AddErr(errors.Wrapf(err, "failed to find project %s", productName))
	}
	var serviceTmpl *commonmodels.Service
	if projectInfo.IsK8sYamlProduct() || projectInfo.IsHostProduct() {
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
			ret, err = commonservice.GetServiceImpl(serviceName, serviceTmpl, workLoadType, env, clientset, inf, log)
			if err != nil {
				return nil, e.ErrGetService.AddErr(err)
			}
		}
		// when not found service in product, we should find it as a fake service of ZadigxReleaseResource
		// if raw service resources not found will be nil , we should create it
		if ret == nil {
			ret = &commonservice.SvcResp{
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
			for _, releaseType := range types.ZadigReleaseTypeList {
				releaseService, err := GetZadigReleaseServiceImpl(releaseType, serviceName, workLoadType, env, kubeClient, clientset, inf, log)
				if err != nil {
					return nil, e.ErrGetService.AddErr(errors.Wrapf(err, "failed to get %s release service", releaseType))
				}
				ret.Scales = append(ret.Scales, releaseService.Scales...)
				ret.Services = append(ret.Services, releaseService.Services...)
			}
		}
		ret.Workloads = nil
		ret.Namespace = env.Namespace
	} else {
		ret, err = commonservice.GetServiceImpl(serviceName, serviceTmpl, workLoadType, env, clientset, inf, log)
		if err != nil {
			return nil, e.ErrGetService.AddErr(err)
		}
		ret.Workloads = nil
		ret.Namespace = env.Namespace
	}

	return ret, nil
}

func GetServiceWorkloads(svcTmpl *commonmodels.Service, env *commonmodels.Product, inf informers.SharedInformerFactory, log *zap.SugaredLogger) ([]*commonservice.Workload, error) {
	ret := make([]*commonservice.Workload, 0)
	envName, productName, namespace := env.EnvName, env.ProductName, env.Namespace

	svcRender := env.GetSvcRender(svcTmpl.ServiceName)
	parsedYaml, err := kube.RenderServiceYaml(svcTmpl.Yaml, productName, svcTmpl.ServiceName, svcRender)
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
				Selector:   wd.Spec.Selector,
				Type:       setting.Deployment,
				Images:     wd.ImageInfos(),
				Containers: wd.GetContainers(),
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
				Selector:   ws.Spec.Selector,
				Type:       setting.StatefulSet,
				Images:     ws.ImageInfos(),
				Containers: ws.GetContainers(),
				Ready:      ws.Ready(),
				Annotation: ws.Annotations,
			})
		}
	}

	return ret, nil
}

func GetZadigReleaseServiceImpl(releaseType, serviceName string, _ string, env *commonmodels.Product, kubeClient client.Client, _ *kubernetes.Clientset, inf informers.SharedInformerFactory, log *zap.SugaredLogger) (ret *commonservice.SvcResp, err error) {
	envName, productName := env.EnvName, env.ProductName
	ret = &commonservice.SvcResp{
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
			types.ZadigReleaseTypeLabelKey:        releaseType,
		})
		deployments, err := getter.ListDeployments(namespace, selector, kubeClient)
		if err != nil {
			return nil, e.ErrGetService.AddDesc(fmt.Sprintf("failed to list deployments, service %s in %s: %v", serviceName, namespace, err))
		}
		for _, deployment := range deployments {
			ret.Scales = append(ret.Scales, commonservice.GetDeploymentWorkloadResource(deployment, inf, log))
			ret.Scales[len(ret.Scales)-1].ZadigXReleaseType = releaseType
			ret.Scales[len(ret.Scales)-1].ZadigXReleaseTag = deployment.Labels[types.ZadigReleaseVersionLabelKey]
			ret.Workloads = append(ret.Workloads, commonservice.ToDeploymentWorkload(deployment))
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

func FetchServiceYaml(productName, envName, serviceName string, _ *zap.SugaredLogger) (string, error) {
	curYaml, _, err := kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
		ProductName: productName,
		EnvName:     envName,
		ServiceName: serviceName,
	})
	if err != nil {
		curYaml = ""
		err = fmt.Errorf("failed to fetch current applied yaml, productName: %s envName: %s serviceName: %s, err: %s",
			productName, envName, serviceName, err)
		log.Error(err)
	}

	return curYaml, nil
}

func ListK8sServiceResources(productName, envName, serviceName string, production bool, log *zap.SugaredLogger) ([]*K8sServiceResource, error) {
	if getProjectType(productName) != setting.K8SDeployType {
		return nil, e.ErrInvalidParam.AddDesc("only k8s yaml service resources are supported")
	}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       productName,
		EnvName:    envName,
		Production: util.GetBoolPointer(production),
	})
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}
	if productInfo.GetServiceMap()[serviceName] == nil {
		return nil, e.ErrGetService.AddDesc(fmt.Sprintf("service %s not found in environment", serviceName))
	}

	serviceYaml, err := FetchServiceYaml(productName, envName, serviceName, log)
	if err != nil {
		return nil, err
	}

	return parseK8sServiceResources(serviceYaml)
}

func parseK8sServiceResources(serviceYaml string) ([]*K8sServiceResource, error) {
	resources, _, err := kube.ManifestToUnstructured(serviceYaml)
	if err != nil {
		return nil, err
	}

	ret := make([]*K8sServiceResource, 0, len(resources))
	resourceMap := make(map[string]*K8sServiceResource, len(resources))
	serviceSelectorTargets := make([]*serviceSelectorTarget, 0, len(resources))
	for _, resource := range resources {
		serviceResource := &K8sServiceResource{
			APIVersion: resource.GetAPIVersion(),
			Kind:       resource.GetKind(),
			Name:       resource.GetName(),
		}
		ret = append(ret, serviceResource)
		resourceMap[k8sServiceResourceKindNameKey(resource.GetKind(), resource.GetName())] = serviceResource
		if labels := getK8sServiceSelectorTargetLabels(resource); len(labels) > 0 {
			serviceSelectorTargets = append(serviceSelectorTargets, &serviceSelectorTarget{
				Kind:   resource.GetKind(),
				Name:   resource.GetName(),
				Labels: labels,
			})
		}
	}

	for _, resource := range resources {
		key := k8sServiceResourceKindNameKey(resource.GetKind(), resource.GetName())
		serviceResource := resourceMap[key]
		if serviceResource == nil {
			continue
		}
		serviceResource.Dependencies = collectK8sServiceResourceDependencies(resource, resourceMap, serviceSelectorTargets)
	}
	return ret, nil
}

func filterSelectedServiceResourceYaml(serviceYaml string, selectedResources []*K8sServiceResource) (string, error) {
	if len(selectedResources) == 0 {
		return "", nil
	}

	selectedResourceMap := make(map[string]struct{}, len(selectedResources))
	for _, resource := range selectedResources {
		if resource == nil || resource.APIVersion == "" || resource.Kind == "" || resource.Name == "" {
			return "", fmt.Errorf("selected resource api_version, kind and name cannot be empty")
		}
		selectedResourceMap[kube.FormatK8sResourceKey(resource.APIVersion, resource.Kind, resource.Name)] = struct{}{}
	}

	resources, resourceMap, err := kube.ManifestToUnstructured(serviceYaml)
	if err != nil {
		return "", err
	}

	filteredManifests := make([]string, 0, len(selectedResourceMap))
	foundResourceMap := make(map[string]struct{}, len(selectedResourceMap))
	for _, resource := range resources {
		key := kube.FormatK8sResourceKey(resource.GetAPIVersion(), resource.GetKind(), resource.GetName())
		if _, ok := selectedResourceMap[key]; !ok {
			continue
		}

		gvkn := fmt.Sprintf("%s-%s", resource.GetObjectKind().GroupVersionKind(), resource.GetName())
		if resourceInfo, ok := resourceMap[gvkn]; ok {
			filteredManifests = append(filteredManifests, resourceInfo.Manifest)
			foundResourceMap[key] = struct{}{}
		}
	}

	if len(foundResourceMap) != len(selectedResourceMap) {
		missingResources := make([]string, 0, len(selectedResourceMap)-len(foundResourceMap))
		for key := range selectedResourceMap {
			if _, ok := foundResourceMap[key]; !ok {
				missingResources = append(missingResources, key)
			}
		}
		sort.Strings(missingResources)
		return "", fmt.Errorf("selected resources not found in service yaml: %s", strings.Join(missingResources, ", "))
	}

	return util.JoinYamls(filteredManifests), nil
}

func k8sServiceResourceKindNameKey(kind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

type serviceSelectorTarget struct {
	Kind   string
	Name   string
	Labels map[string]string
}

func collectK8sServiceResourceDependencies(resource *unstructured.Unstructured, resourceMap map[string]*K8sServiceResource, serviceSelectorTargets []*serviceSelectorTarget) []*K8sServiceResourceDependency {
	dependencyMap := make(map[string]*K8sServiceResourceDependency)
	addDependency := func(kind, name, referencePath string) {
		if kind == "" || name == "" {
			return
		}

		targetResource, ok := resourceMap[k8sServiceResourceKindNameKey(kind, name)]
		if !ok {
			return
		}

		dependencyKey := k8sServiceResourceKindNameKey(kind, name)
		if _, exists := dependencyMap[dependencyKey]; exists {
			return
		}

		dependencyMap[dependencyKey] = &K8sServiceResourceDependency{
			APIVersion:    targetResource.APIVersion,
			Kind:          targetResource.Kind,
			Name:          targetResource.Name,
			ReferencePath: referencePath,
		}
	}

	switch resource.GetKind() {
	case setting.Ingress:
		collectIngressDependencies(resource, addDependency)
	case setting.StatefulSet:
		serviceName, _, _ := unstructured.NestedString(resource.Object, "spec", "serviceName")
		addDependency(setting.Service, serviceName, "spec.serviceName")
	}

	if podSpec, basePath, found := getK8sServiceResourcePodSpec(resource); found {
		collectPodSpecDependencies(podSpec, basePath, addDependency)
	}

	if resource.GetKind() == setting.Service {
		collectServiceSelectorDependencies(resource, serviceSelectorTargets, addDependency)
	}
	if resource.GetKind() == setting.RoleBinding || resource.GetKind() == setting.ClusterRoleBinding {
		collectRBACDependencies(resource, addDependency)
	}

	ret := make([]*K8sServiceResourceDependency, 0, len(dependencyMap))
	for _, dependency := range dependencyMap {
		ret = append(ret, dependency)
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].Kind != ret[j].Kind {
			return ret[i].Kind < ret[j].Kind
		}
		if ret[i].Name != ret[j].Name {
			return ret[i].Name < ret[j].Name
		}
		if ret[i].APIVersion != ret[j].APIVersion {
			return ret[i].APIVersion < ret[j].APIVersion
		}
		return ret[i].ReferencePath < ret[j].ReferencePath
	})
	return ret
}

func collectIngressDependencies(resource *unstructured.Unstructured, addDependency func(kind, name, referencePath string)) {
	serviceName, _, _ := unstructured.NestedString(resource.Object, "spec", "defaultBackend", "service", "name")
	addDependency(setting.Service, serviceName, "spec.defaultBackend.service.name")

	serviceName, _, _ = unstructured.NestedString(resource.Object, "spec", "backend", "serviceName")
	addDependency(setting.Service, serviceName, "spec.backend.serviceName")

	paths, _, _ := unstructured.NestedSlice(resource.Object, "spec", "rules")
	for _, rule := range paths {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}

		httpPaths, _, _ := unstructured.NestedSlice(ruleMap, "http", "paths")
		for _, path := range httpPaths {
			pathMap, ok := path.(map[string]interface{})
			if !ok {
				continue
			}

			serviceName, _, _ = unstructured.NestedString(pathMap, "backend", "service", "name")
			addDependency(setting.Service, serviceName, "spec.rules[].http.paths[].backend.service.name")

			serviceName, _, _ = unstructured.NestedString(pathMap, "backend", "serviceName")
			addDependency(setting.Service, serviceName, "spec.rules[].http.paths[].backend.serviceName")
		}
	}

	tlsList, _, _ := unstructured.NestedSlice(resource.Object, "spec", "tls")
	for _, item := range tlsList {
		tlsMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		secretName, _, _ := unstructured.NestedString(tlsMap, "secretName")
		addDependency(setting.Secret, secretName, "spec.tls[].secretName")
	}
}

func getK8sServiceResourcePodSpec(resource *unstructured.Unstructured) (map[string]interface{}, string, bool) {
	switch resource.GetKind() {
	case setting.Deployment, setting.StatefulSet, setting.ReplicaSet, setting.Job:
		podSpec, found, _ := unstructured.NestedMap(resource.Object, "spec", "template", "spec")
		return podSpec, "spec.template.spec", found
	case setting.CronJob:
		podSpec, found, _ := unstructured.NestedMap(resource.Object, "spec", "jobTemplate", "spec", "template", "spec")
		return podSpec, "spec.jobTemplate.spec.template.spec", found
	case setting.Pod:
		podSpec, found, _ := unstructured.NestedMap(resource.Object, "spec")
		return podSpec, "spec", found
	default:
		return nil, "", false
	}
}

func collectPodSpecDependencies(podSpec map[string]interface{}, basePath string, addDependency func(kind, name, referencePath string)) {
	serviceAccountName, _, _ := unstructured.NestedString(podSpec, "serviceAccountName")
	addDependency(setting.ServiceAccount, serviceAccountName, basePath+".serviceAccountName")

	imagePullSecrets, _, _ := unstructured.NestedSlice(podSpec, "imagePullSecrets")
	for _, item := range imagePullSecrets {
		secretMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		secretName, _, _ := unstructured.NestedString(secretMap, "name")
		addDependency(setting.Secret, secretName, basePath+".imagePullSecrets[].name")
	}

	volumes, _, _ := unstructured.NestedSlice(podSpec, "volumes")
	for _, item := range volumes {
		volumeMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		configMapName, _, _ := unstructured.NestedString(volumeMap, "configMap", "name")
		addDependency(setting.ConfigMap, configMapName, basePath+".volumes[].configMap.name")

		secretName, _, _ := unstructured.NestedString(volumeMap, "secret", "secretName")
		addDependency(setting.Secret, secretName, basePath+".volumes[].secret.secretName")

		claimName, _, _ := unstructured.NestedString(volumeMap, "persistentVolumeClaim", "claimName")
		addDependency(setting.PersistentVolumeClaim, claimName, basePath+".volumes[].persistentVolumeClaim.claimName")

		projectedSources, _, _ := unstructured.NestedSlice(volumeMap, "projected", "sources")
		for _, source := range projectedSources {
			sourceMap, ok := source.(map[string]interface{})
			if !ok {
				continue
			}

			configMapName, _, _ = unstructured.NestedString(sourceMap, "configMap", "name")
			addDependency(setting.ConfigMap, configMapName, basePath+".volumes[].projected.sources[].configMap.name")

			secretName, _, _ = unstructured.NestedString(sourceMap, "secret", "name")
			addDependency(setting.Secret, secretName, basePath+".volumes[].projected.sources[].secret.name")
		}
	}

	collectContainerDependencies(podSpec, "containers", basePath+".containers[]", addDependency)
	collectContainerDependencies(podSpec, "initContainers", basePath+".initContainers[]", addDependency)
	collectContainerDependencies(podSpec, "ephemeralContainers", basePath+".ephemeralContainers[]", addDependency)
}

func collectContainerDependencies(podSpec map[string]interface{}, fieldName, basePath string, addDependency func(kind, name, referencePath string)) {
	containers, _, _ := unstructured.NestedSlice(podSpec, fieldName)
	for _, item := range containers {
		containerMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		envs, _, _ := unstructured.NestedSlice(containerMap, "env")
		for _, env := range envs {
			envMap, ok := env.(map[string]interface{})
			if !ok {
				continue
			}

			configMapName, _, _ := unstructured.NestedString(envMap, "valueFrom", "configMapKeyRef", "name")
			addDependency(setting.ConfigMap, configMapName, basePath+".env[].valueFrom.configMapKeyRef.name")

			secretName, _, _ := unstructured.NestedString(envMap, "valueFrom", "secretKeyRef", "name")
			addDependency(setting.Secret, secretName, basePath+".env[].valueFrom.secretKeyRef.name")
		}

		envFromList, _, _ := unstructured.NestedSlice(containerMap, "envFrom")
		for _, envFrom := range envFromList {
			envFromMap, ok := envFrom.(map[string]interface{})
			if !ok {
				continue
			}

			configMapName, _, _ := unstructured.NestedString(envFromMap, "configMapRef", "name")
			addDependency(setting.ConfigMap, configMapName, basePath+".envFrom[].configMapRef.name")

			secretName, _, _ := unstructured.NestedString(envFromMap, "secretRef", "name")
			addDependency(setting.Secret, secretName, basePath+".envFrom[].secretRef.name")
		}
	}
}

func collectServiceSelectorDependencies(resource *unstructured.Unstructured, serviceSelectorTargets []*serviceSelectorTarget, addDependency func(kind, name, referencePath string)) {
	selector, _, _ := unstructured.NestedStringMap(resource.Object, "spec", "selector")
	if len(selector) == 0 {
		return
	}

	for _, target := range serviceSelectorTargets {
		if len(target.Labels) == 0 || !isSelectorSubset(selector, target.Labels) {
			continue
		}

		addDependency(target.Kind, target.Name, "spec.selector")
	}
}

func getK8sServiceSelectorTargetLabels(resource *unstructured.Unstructured) map[string]string {
	switch resource.GetKind() {
	case setting.Deployment, setting.StatefulSet, setting.ReplicaSet:
		labels, _, _ := unstructured.NestedStringMap(resource.Object, "spec", "template", "metadata", "labels")
		return labels
	case setting.Pod:
		labels, _, _ := unstructured.NestedStringMap(resource.Object, "metadata", "labels")
		return labels
	default:
		return nil
	}
}

func isSelectorSubset(selector, labels map[string]string) bool {
	if len(selector) == 0 || len(labels) == 0 {
		return false
	}

	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func collectRBACDependencies(resource *unstructured.Unstructured, addDependency func(kind, name, referencePath string)) {
	roleKind, _, _ := unstructured.NestedString(resource.Object, "roleRef", "kind")
	roleName, _, _ := unstructured.NestedString(resource.Object, "roleRef", "name")
	addDependency(roleKind, roleName, "roleRef.name")

	subjects, _, _ := unstructured.NestedSlice(resource.Object, "subjects")
	for _, subject := range subjects {
		subjectMap, ok := subject.(map[string]interface{})
		if !ok {
			continue
		}

		subjectKind, _, _ := unstructured.NestedString(subjectMap, "kind")
		subjectName, _, _ := unstructured.NestedString(subjectMap, "name")
		addDependency(subjectKind, subjectName, "subjects[].name")
	}
}

func PreviewService(args *PreviewServiceArgs, _ *zap.SugaredLogger) (*SvcDiffResult, error) {
	envInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    args.ProductName,
		EnvName: args.EnvName,
	})
	if err != nil {
		return nil, e.ErrPreviewYaml.AddErr(err)
	}

	newVariableYaml, err := commontypes.RenderVariableKVToYaml(args.VariableKVs, true)
	if err != nil {
		return nil, e.ErrPreviewYaml.AddErr(err)
	}

	ret := &SvcDiffResult{
		ServiceName: args.ServiceName,
		Current:     TmplYaml{},
		Latest:      TmplYaml{},
	}

	curYaml := ""
	isImportToDeploy := false
	if envInfo.ServiceDeployStrategy[args.ServiceName] == setting.ServiceDeployStrategyImport {
		// is imported service
		if len(args.DeployContents) > 0 {
			// is workflow
			if len(args.DeployContents) == 1 && slices.Contains(args.DeployContents, config.DeployImage) {
				// only update images
				envSvc := envInfo.GetServiceMap()[args.ServiceName]
				if envSvc == nil {
					return nil, e.ErrPreviewYaml.AddErr(fmt.Errorf("service %s not found in environment", args.ServiceName))
				}
				for _, container := range envSvc.Containers {
					ret.Current.Yaml += container.Image + "\n"
				}
				for _, container := range args.ServiceModules {
					ret.Latest.Yaml += container.Image + "\n"
				}
				return ret, nil
			} else if slices.Contains(args.DeployContents, config.DeployVars) || slices.Contains(args.DeployContents, config.DeployConfig) {
				// set update variables or configuration
				isImportToDeploy = true
			}
		} else {
			// is environment
			isImportToDeploy = true
		}
	}

	curYaml, _, err = kube.FetchCurrentAppliedYaml(&kube.GeneSvcYamlOption{
		ProductName:           args.ProductName,
		EnvName:               args.EnvName,
		ServiceName:           args.ServiceName,
		UpdateServiceRevision: false,
		IsImportToDeploy:      isImportToDeploy,
	})
	if err != nil {
		curYaml = ""
		ret.Error = fmt.Sprintf("failed to fetch current applied yaml, productName: %s envName: %s serviceName: %s, updateSvcRevision: %v, variableYaml: %s err: %s",
			args.ProductName, args.EnvName, args.ServiceName, args.UpdateServiceRevision, newVariableYaml, err)
		log.Errorf(ret.Error)
	}

	candidateOverrides, err := buildPreviewCandidateOverrides(envInfo, args.ServiceName, args.UpdateServiceRevision, args.VariableKVs)
	if err != nil {
		return nil, e.ErrPreviewYaml.AddErr(err)
	}

	latestYaml, _, _, err := kube.GenerateRenderedYaml(&kube.GeneSvcYamlOption{
		ProductName:                   args.ProductName,
		EnvName:                       args.EnvName,
		ServiceName:                   args.ServiceName,
		UpdateServiceRevision:         args.UpdateServiceRevision,
		VariableYaml:                  newVariableYaml,
		VariableKVs:                   args.VariableKVs,
		Containers:                    args.ServiceModules,
		ReplicaOverrides:              candidateOverrides,
		IgnoreCurrentReplicaOverrides: args.UpdateServiceRevision,
	})
	if err != nil {
		return nil, e.ErrPreviewYaml.AddErr(err)
	}

	ret.Current.Yaml = curYaml
	ret.Latest.Yaml = latestYaml

	return ret, nil
}

// RestartService 在kube中, 如果资源存在就更新不存在就创建
func RestartService(envName string, args *SvcOptArgs, production bool, log *zap.SugaredLogger) (err error) {
	productObj, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       args.ProductName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		return err
	}
	if productObj.IsSleeping() {
		return e.ErrScaleService.AddErr(fmt.Errorf("environment is sleeping"))
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(productObj.ClusterID)
	if err != nil {
		return err
	}

	// aws secrets needs to be refreshed
	regs, err := commonservice.ListRegistryNamespaces("", true, log)
	if err != nil {
		log.Errorf("Failed to get registries to restart container, the error is: %s", err)
		return err
	}
	for _, reg := range regs {
		if reg.RegProvider == config.RegistryTypeAWS {
			if err := kube.CreateOrUpdateRegistrySecret(productObj.Namespace, productObj.ClusterID, reg, false); err != nil {
				retErr := fmt.Errorf("failed to update pull secret for registry: %s, the error is: %s", reg.ID.Hex(), err)
				log.Errorf("%s\n", retErr.Error())
				return retErr
			}
		}
	}

	switch productObj.Source {
	case setting.SourceFromHelm:
		deploy, found, err := getter.GetDeployment(productObj.Namespace, args.ServiceName, kubeClient)
		if err != nil {
			return fmt.Errorf("failed to find resource %s, type %s, err %s", args.ServiceName, setting.Deployment, err.Error())
		}
		if found {
			return updater.RestartDeploymentV2(context.Background(), productObj.ClusterID, productObj.Namespace, deploy.Name)
		}

		sts, found, err := getter.GetStatefulSet(productObj.Namespace, args.ServiceName, kubeClient)
		if err != nil {
			return fmt.Errorf("failed to find resource %s, type %s, err %s", args.ServiceName, setting.StatefulSet, err.Error())
		}
		if found {
			return updater.RestartStatefulSetV2(context.Background(), productObj.ClusterID, productObj.Namespace, sts.Name)
		}
	default:
		var productService *commonmodels.ProductService
		serviceObj, ok := productObj.GetServiceMap()[args.ServiceName]
		if !ok {
			return nil
		}
		productService = serviceObj

		err = restartRelatedWorkloads(productObj, productService, productObj.ClusterID, log)
		log.Infof("restart resource from namespace:%s/serviceName:%s ", productObj.Namespace, args.ServiceName)

		if err != nil {
			log.Errorf("failed to restart service, err:%v", err)
			err = e.ErrRestartService.AddErr(err)
			return
		}
	}

	return nil
}
