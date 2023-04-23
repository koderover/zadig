/*
Copyright 2022 The KodeRover Authors.

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

package kube

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	gotemplate "text/template"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/helm/pkg/releaseutil"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"
	commomtemplate "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
	"github.com/koderover/zadig/pkg/util/converter"
	zadigyamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type GeneSvcYamlOption struct {
	ProductName           string
	EnvName               string
	ServiceName           string
	UpdateServiceRevision bool
	VariableYaml          string
	UnInstall             bool
	Containers            []*models.Container
}

type WorkloadResource struct {
	Type string
	Name string
}

func GeneKVFromYaml(yamlContent string) ([]*commonmodels.VariableKV, error) {
	if len(yamlContent) == 0 {
		return nil, nil
	}
	flatMap, err := converter.YamlToFlatMap([]byte(yamlContent))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert yaml to flat map")
	} else {
		kvs := make([]*commonmodels.VariableKV, 0)
		for k, v := range flatMap {
			if len(k) == 0 {
				continue
			}
			kvs = append(kvs, &models.VariableKV{
				Key:   k,
				Value: v,
			})
		}
		return kvs, nil
	}
}

func GenerateYamlFromKV(kvs []*commonmodels.VariableKV) (string, error) {
	if len(kvs) == 0 {
		return "", nil
	}
	flatMap := make(map[string]interface{})
	for _, kv := range kvs {
		flatMap[kv.Key] = kv.Value
	}

	validKvMap, err := converter.Expand(flatMap)
	if err != nil {
		return "", errors.Wrapf(err, "failed to expand flat map")
	}

	bs, err := yaml.Marshal(validKvMap)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal map to yaml")
	}
	return string(bs), nil
}

// extract valid svc variable from service variable
// keys defined in service vars are valid
// keys not defined in service vars or default values are valid as well
func extractValidSvcVariable(serviceName string, rs *commonmodels.RenderSet, serviceVars []string, serviceDefaultValues string) (string, error) {
	serviceVariable := ""
	for _, v := range rs.ServiceVariables {
		if v.ServiceName != serviceName {
			continue
		}
		if v.OverrideYaml != nil {
			serviceVariable = v.OverrideYaml.YamlContent
		}
		break
	}

	valuesMap, err := converter.YamlToFlatMap([]byte(serviceVariable))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for service variable, err: %s", err)
	}

	serviceDefaultValuesMap, err := converter.YamlToFlatMap([]byte(serviceDefaultValues))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for service default variable, err: %s", err)
	}

	wildcard := commonutil.IsServiceVarsWildcard(serviceVars)

	// keys defined in service vars
	keysSet := sets.NewString(serviceVars...)
	validKvMap, svcValidDefaultValueMap := make(map[string]interface{}), make(map[string]interface{})

	for k, v := range valuesMap {
		if wildcard || keysSet.Has(k) {
			validKvMap[k] = v
		}
	}

	for k, v := range serviceDefaultValuesMap {
		if _, ok := validKvMap[k]; !ok {
			svcValidDefaultValueMap[k] = v
		}
	}

	defaultValuesMap, err := converter.YamlToFlatMap([]byte(rs.DefaultValues))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for default variable, err: %s", err)
	}
	for k := range defaultValuesMap {
		delete(svcValidDefaultValueMap, k)
	}

	for k, v := range svcValidDefaultValueMap {
		validKvMap[k] = v
	}

	validKvMap, err = converter.Expand(validKvMap)
	if err != nil {
		return "", err
	}

	bs, err := yaml.Marshal(validKvMap)
	return string(bs), err
}

func resourceToYaml(obj runtime.Object) (string, error) {
	y := printers.YAMLPrinter{}
	writer := bytes.NewBuffer(nil)
	err := y.PrintObj(obj, writer)
	if err != nil {
		return "", errors.Wrapf(err, "failed to marshal object to yaml")
	}
	return writer.String(), nil
}

// ReplaceWorkloadImages  replace images in yaml with new images
func ReplaceWorkloadImages(rawYaml string, images []*commonmodels.Container) (string, []*WorkloadResource, error) {
	imageMap := make(map[string]*commonmodels.Container)
	for _, image := range images {
		imageMap[image.Name] = image
	}

	splitYams := util.SplitYaml(rawYaml)
	yamlStrs := make([]string, 0)
	var err error
	workloadRes := make([]*WorkloadResource, 0)
	for _, yamlStr := range splitYams {

		resKind := new(types.KubeResourceKind)
		if err := yaml.Unmarshal([]byte(yamlStr), &resKind); err != nil {
			return "", nil, fmt.Errorf("unmarshal ResourceKind error: %v", err)
		}
		decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(yamlStr)), 5*1024*1024)

		switch resKind.Kind {
		case setting.Deployment:
			deployment := &appsv1.Deployment{}
			if err := decoder.Decode(deployment); err != nil {
				return "", nil, fmt.Errorf("unmarshal Deployment error: %v", err)
			}
			workloadRes = append(workloadRes, &WorkloadResource{
				Name: resKind.Metadata.Name,
				Type: resKind.Kind,
			})
			for i, container := range deployment.Spec.Template.Spec.Containers {
				containerName := container.Name
				if image, ok := imageMap[containerName]; ok {
					deployment.Spec.Template.Spec.Containers[i].Image = image.Image
				}
			}

			yamlStr, err = resourceToYaml(deployment)
			if err != nil {
				return "", nil, err
			}
		case setting.StatefulSet:
			statefulSet := &appsv1.StatefulSet{}
			if err := decoder.Decode(statefulSet); err != nil {
				return "", nil, fmt.Errorf("unmarshal StatefulSet error: %v", err)
			}
			workloadRes = append(workloadRes, &WorkloadResource{
				Name: resKind.Metadata.Name,
				Type: resKind.Kind,
			})
			for i, container := range statefulSet.Spec.Template.Spec.Containers {
				containerName := container.Name
				if image, ok := imageMap[containerName]; ok {
					statefulSet.Spec.Template.Spec.Containers[i].Image = image.Image
				}
			}
			yamlStr, err = resourceToYaml(statefulSet)
			if err != nil {
				return "", nil, err
			}
		case setting.Job:
			job := &batchv1.Job{}
			if err := decoder.Decode(job); err != nil {
				return "", nil, fmt.Errorf("unmarshal Job error: %v", err)
			}
			for i, container := range job.Spec.Template.Spec.Containers {
				containerName := container.Name
				if image, ok := imageMap[containerName]; ok {
					job.Spec.Template.Spec.Containers[i].Image = image.Image
				}
			}
			yamlStr, err = resourceToYaml(job)
			if err != nil {
				return "", nil, err
			}
		case setting.CronJob:
			// TODO support new cronjob type
			cronJob := &batchv1beta1.CronJob{}
			if err := decoder.Decode(cronJob); err != nil {
				return "", nil, fmt.Errorf("unmarshal CronJob error: %v", err)
			}
			for i, val := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
				containerName := val.Name
				if image, ok := imageMap[containerName]; ok {
					cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[i].Image = image.Image
				}
			}
			yamlStr, err = resourceToYaml(cronJob)
			if err != nil {
				return "", nil, err
			}
		}
		yamlStrs = append(yamlStrs, yamlStr)
	}

	return util.JoinYamls(yamlStrs), workloadRes, nil
}

func mergeContainers(curContainers []*commonmodels.Container, newContainers ...[]*commonmodels.Container) []*commonmodels.Container {
	curContainerMap := make(map[string]*commonmodels.Container)
	for _, container := range curContainers {
		curContainerMap[container.Name] = container
	}
	for _, containers := range newContainers {
		for _, container := range containers {
			curContainerMap[container.Name] = container
		}
	}
	var containers []*commonmodels.Container
	for _, container := range curContainerMap {
		containers = append(containers, container)
	}
	return containers
}

func FetchCurrentServiceVariable(option *GeneSvcYamlOption) ([]*commonmodels.VariableKV, error) {
	_, err := templaterepo.NewProductColl().Find(option.ProductName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find template product %s", option.ProductName)
	}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		EnvName: option.EnvName,
		Name:    option.ProductName,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find product %s", option.ProductName)
	}

	curProductSvc := productInfo.GetServiceMap()[option.ServiceName]

	productSvcRevision := int64(0)
	if curProductSvc != nil {
		productSvcRevision = curProductSvc.Revision
	}

	prodSvcTemplate, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: option.ProductName,
		ServiceName: option.ServiceName,
		Revision:    productSvcRevision,
	}, productInfo.Production)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find service %s with revision %d", option.ServiceName, productSvcRevision)
	}
	serviceVars := prodSvcTemplate.ServiceVars
	if productInfo.Production {
		serviceVars = setting.ServiceVarWildCard
	}

	var usedRenderset *commonmodels.RenderSet
	usedRenderset, err = commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: productInfo.ProductName,
		EnvName:     productInfo.EnvName,
		IsDefault:   false,
		Revision:    productInfo.Render.Revision,
		Name:        productInfo.Render.Name,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find renderset for %s/%s", productInfo.ProductName, productInfo.EnvName)
	}

	serviceVariableYaml := ""
	serviceRender := usedRenderset.GetServiceRenderMap()[option.ServiceName]
	if serviceRender != nil && serviceRender.OverrideYaml != nil {
		serviceVariableYaml = serviceRender.OverrideYaml.YamlContent
	}

	variableYaml, _, err := commomtemplate.SafeMergeVariableYaml(prodSvcTemplate.VariableYaml, serviceVariableYaml)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to merge variable yaml for %s/%s", option.ProductName, option.ServiceName)
	}
	variableYaml, err = commonutil.ClipVariableYaml(variableYaml, serviceVars)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clip variable yaml for %s/%s", option.ProductName, option.ServiceName)
	}

	return GeneKVFromYaml(variableYaml)
}

// FetchCurrentAppliedYaml generates full yaml of some service currently applied in Zadig
// and returns the service yaml, currently used service revision
func FetchCurrentAppliedYaml(option *GeneSvcYamlOption) (string, int, error) {
	_, err := templaterepo.NewProductColl().Find(option.ProductName)
	if err != nil {
		return "", 0, errors.Wrapf(err, "failed to find template product %s", option.ProductName)
	}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		EnvName: option.EnvName,
		Name:    option.ProductName,
	})
	if err != nil {
		return "", 0, errors.Wrapf(err, "failed to find product %s", option.ProductName)
	}

	curProductSvc := productInfo.GetServiceMap()[option.ServiceName]

	// service not installed, nothing to return
	if curProductSvc == nil {
		return "", 0, nil
	}

	prodSvcTemplate, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: option.ProductName,
		ServiceName: option.ServiceName,
		Revision:    curProductSvc.Revision,
	}, productInfo.Production)
	if err != nil {
		return "", 0, errors.Wrapf(err, "failed to find service %s with revision %d", option.ServiceName, curProductSvc.Revision)
	}
	if productInfo.Production {
		prodSvcTemplate.ServiceVars = setting.ServiceVarWildCard
	}

	// for situations only updating workload images, only return involved manifests of deployments and statefulsets
	if !option.UnInstall && !option.UpdateServiceRevision && len(option.VariableYaml) == 0 {
		manifest, _, err := fetchImportedManifests(option, productInfo, prodSvcTemplate)
		return manifest, int(curProductSvc.Revision), err
	}

	// service not deployed by zadig, should only be updated with images
	if !option.UnInstall && !commonutil.ServiceDeployed(option.ServiceName, productInfo.ServiceDeployStrategy) {
		manifest, _, err := fetchImportedManifests(option, productInfo, prodSvcTemplate)
		return manifest, int(curProductSvc.Revision), err
	}

	var usedRenderset *commonmodels.RenderSet
	usedRenderset, err = commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: productInfo.ProductName,
		EnvName:     productInfo.EnvName,
		IsDefault:   false,
		Revision:    productInfo.Render.Revision,
		Name:        productInfo.Render.Name,
	})
	if err != nil {
		return "", 0, errors.Wrapf(err, "failed to find renderset for %s/%s", productInfo.ProductName, productInfo.EnvName)
	}

	fullRenderedYaml, err := RenderServiceYaml(prodSvcTemplate.Yaml, option.ProductName, option.ServiceName, usedRenderset, prodSvcTemplate.ServiceVars, prodSvcTemplate.VariableYaml)
	if err != nil {
		return "", 0, err
	}
	fullRenderedYaml = ParseSysKeys(productInfo.Namespace, productInfo.EnvName, option.ProductName, option.ServiceName, fullRenderedYaml)
	mergedContainers := mergeContainers(prodSvcTemplate.Containers, curProductSvc.Containers)
	fullRenderedYaml, _, err = ReplaceWorkloadImages(fullRenderedYaml, mergedContainers)
	return fullRenderedYaml, 0, nil
}

func fetchImportedManifests(option *GeneSvcYamlOption, productInfo *models.Product, serviceTmp *models.Service) (string, []*WorkloadResource, error) {
	fakeRenderSet := &models.RenderSet{}
	fullRenderedYaml, err := RenderServiceYaml(serviceTmp.Yaml, option.ProductName, option.ServiceName, fakeRenderSet, serviceTmp.ServiceVars, serviceTmp.VariableYaml)
	if err != nil {
		return "", nil, err
	}
	fullRenderedYaml = ParseSysKeys(productInfo.Namespace, productInfo.EnvName, option.ProductName, option.ServiceName, fullRenderedYaml)

	manifests := releaseutil.SplitManifests(fullRenderedYaml)

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), productInfo.ClusterID)
	if err != nil {
		log.Errorf("cluster is not connected [%s]", productInfo.ClusterID)
		return "", nil, errors.Wrapf(err, "cluster is not connected [%s]", productInfo.ClusterID)
	}

	manifestArr := make([]string, 0)

	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			return "", nil, errors.Wrapf(err, "failed to decode yaml %s", item)
		}
		switch u.GetKind() {
		case setting.Deployment:
			workloadBs, exist, err := getter.GetDeploymentYamlFormat(productInfo.Namespace, u.GetName(), kubeClient)
			if err != nil {
				return "", nil, errors.Wrapf(err, "failed to get deployment %s", u.GetName())
			}
			if !exist {
				return "", nil, errors.Errorf("deployment %s not found", u.GetName())
			}
			manifestArr = append(manifestArr, string(workloadBs))
		case setting.StatefulSet:
			workloadBs, exist, err := getter.GetStatefulSetYamlFormat(productInfo.Namespace, u.GetName(), kubeClient)
			if err != nil {
				return "", nil, errors.Wrapf(err, "failed to get statefulset %s", u.GetName())
			}
			if !exist {
				return "", nil, errors.Errorf("statefulset %s not found", u.GetName())
			}
			manifestArr = append(manifestArr, string(workloadBs))
		}
	}
	return ReplaceWorkloadImages(util.JoinYamls(manifestArr), option.Containers)
}

func variableYamlNil(variableYaml string) bool {
	if len(variableYaml) == 0 {
		return true
	}
	kvMap, _ := converter.YamlToFlatMap([]byte(variableYaml))
	return len(kvMap) == 0
}

// GenerateRenderedYaml generates full yaml of some service defined in Zadig (images not included)
// and returns the service yaml, used service revision
func GenerateRenderedYaml(option *GeneSvcYamlOption) (string, int, []*WorkloadResource, error) {
	_, err := templaterepo.NewProductColl().Find(option.ProductName)
	if err != nil {
		return "", 0, nil, errors.Wrapf(err, "failed to find template product %s", option.ProductName)
	}

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		EnvName: option.EnvName,
		Name:    option.ProductName,
	})
	if err != nil {
		return "", 0, nil, errors.Wrapf(err, "failed to find product %s", option.ProductName)
	}

	curProductSvc := productInfo.GetServiceMap()[option.ServiceName]

	// nothing to render when trying to uninstall a service which is not deployed
	if option.UnInstall && curProductSvc == nil {
		return "", 0, nil, nil
	}

	var prodSvcTemplate, latestSvcTemplate *commonmodels.Service

	latestSvcTemplate, err = repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName:         option.ProductName,
		ServiceName:         option.ServiceName,
		ExcludeStatus:       setting.ProductStatusDeleting,
		Revision:            0,
		IgnoreNoDocumentErr: true,
	}, productInfo.Production)
	if err != nil {
		return "", 0, nil, errors.Wrapf(err, "failed to find latest service template %s", option.ServiceName)
	}

	var svcContainersInProduct []*models.Container
	if curProductSvc != nil {
		svcContainersInProduct = curProductSvc.Containers
		prodSvcTemplate, err = repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ProductName: option.ProductName,
			ServiceName: option.ServiceName,
			Revision:    curProductSvc.Revision,
		}, productInfo.Production)
		if err != nil {
			return "", 0, nil, errors.Wrapf(err, "failed to find service %s with revision %d", option.ServiceName, curProductSvc.Revision)
		}
	} else {
		prodSvcTemplate = latestSvcTemplate
	}

	// use latest service revision
	if latestSvcTemplate == nil {
		latestSvcTemplate = prodSvcTemplate
	}
	if !option.UpdateServiceRevision {
		latestSvcTemplate = prodSvcTemplate
	}

	if productInfo.Production {
		latestSvcTemplate.ServiceVars = setting.ServiceVarWildCard
	}

	// service not deployed by zadig, should only be updated with images
	if !option.UnInstall && !option.UpdateServiceRevision && variableYamlNil(option.VariableYaml) && curProductSvc != nil && !commonutil.ServiceDeployed(option.ServiceName, productInfo.ServiceDeployStrategy) {
		manifest, workloads, err := fetchImportedManifests(option, productInfo, prodSvcTemplate)
		return manifest, int(curProductSvc.Revision), workloads, err
	}

	if latestSvcTemplate == nil {
		return "", 0, nil, fmt.Errorf("failed to find service template %s used in product %s", option.ServiceName, option.EnvName)
	}

	curContainers := latestSvcTemplate.Containers
	if curProductSvc != nil {
		curContainers = curProductSvc.Containers
	}

	usedRenderset, err := commonrepo.NewRenderSetColl().Find(&commonrepo.RenderSetFindOption{
		ProductTmpl: productInfo.ProductName,
		EnvName:     productInfo.EnvName,
		IsDefault:   false,
		Revision:    productInfo.Render.Revision,
		Name:        productInfo.Render.Name,
	})
	if err != nil {
		return "", 0, nil, errors.Wrapf(err, "failed to find renderset for %s/%s", productInfo.ProductName, productInfo.EnvName)
	}
	serviceVariableYaml := ""
	serviceRender := usedRenderset.GetServiceRenderMap()[option.ServiceName]
	if serviceRender != nil && serviceRender.OverrideYaml != nil {
		serviceVariableYaml = serviceRender.OverrideYaml.YamlContent
	}

	serviceVariableYaml = commonutil.ClipVariableYamlNoErr(serviceVariableYaml, latestSvcTemplate.ServiceVars)
	mergedBs, err := zadigyamlutil.Merge([][]byte{[]byte(serviceVariableYaml), []byte(option.VariableYaml)})
	if err != nil {
		return "", 0, nil, errors.Wrapf(err, "failed to merge service variable yaml")
	}

	usedRenderset.ServiceVariables = []*template.ServiceRender{{
		ServiceName: option.ServiceName,
		OverrideYaml: &template.CustomYaml{
			YamlContent: string(mergedBs),
		},
	}}

	fullRenderedYaml, err := RenderServiceYaml(latestSvcTemplate.Yaml, option.ProductName, option.ServiceName, usedRenderset, latestSvcTemplate.ServiceVars, latestSvcTemplate.VariableYaml)
	if err != nil {
		return "", 0, nil, err
	}
	fullRenderedYaml = ParseSysKeys(productInfo.Namespace, productInfo.EnvName, option.ProductName, option.ServiceName, fullRenderedYaml)
	mergedContainers := mergeContainers(curContainers, latestSvcTemplate.Containers, svcContainersInProduct, option.Containers)
	fullRenderedYaml, workloadResource, err := ReplaceWorkloadImages(fullRenderedYaml, mergedContainers)
	return fullRenderedYaml, int(latestSvcTemplate.Revision), workloadResource, err
}

// RenderServiceYaml render service yaml with default values and service variable
// serviceVars = []{*} means all variable are service vars
func RenderServiceYaml(originYaml, productName, serviceName string, rs *commonmodels.RenderSet, serviceVars []string, serviceDefaultValues string) (string, error) {
	if rs == nil {
		originYaml = strings.ReplaceAll(originYaml, setting.TemplateVariableProduct, productName)
		originYaml = strings.ReplaceAll(originYaml, setting.TemplateVariableService, serviceName)
		return originYaml, nil
	}
	tmpl, err := gotemplate.New(fmt.Sprintf("%s:%s", productName, serviceName)).Parse(originYaml)
	if err != nil {
		return originYaml, fmt.Errorf("failed to build template, err: %s", err)
	}
	// tmpl.Option("missingkey=error")

	serviceVariable, err := extractValidSvcVariable(serviceName, rs, serviceVars, serviceDefaultValues)
	if err != nil {
		return "", fmt.Errorf("failed to extract variable for service: %s, err: %s", serviceName, err)
	}
	variableYaml, replacedKv, err := commomtemplate.SafeMergeVariableYaml(rs.DefaultValues, serviceVariable)
	if err != nil {
		return originYaml, err
	}

	variableYaml = strings.ReplaceAll(variableYaml, setting.TemplateVariableProduct, productName)
	variableYaml = strings.ReplaceAll(variableYaml, setting.TemplateVariableService, serviceName)

	variableMap := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(variableYaml), &variableMap)
	if err != nil {
		return originYaml, fmt.Errorf("failed to unmarshal variable yaml, err: %s", err)
	}

	buf := bytes.NewBufferString("")
	err = tmpl.Execute(buf, variableMap)
	if err != nil {
		return originYaml, fmt.Errorf("template validate err: %s", err)
	}

	originYaml = buf.String()

	// replace system variables
	originYaml = strings.ReplaceAll(originYaml, setting.TemplateVariableProduct, productName)
	originYaml = strings.ReplaceAll(originYaml, setting.TemplateVariableService, serviceName)

	for rk, rv := range replacedKv {
		originYaml = strings.ReplaceAll(originYaml, rk, rv)
	}

	return originYaml, nil
}

// RenderEnvService renders service with particular revision and service vars in environment
func RenderEnvService(prod *commonmodels.Product, render *commonmodels.RenderSet, service *commonmodels.ProductService) (yaml string, err error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName: service.ServiceName,
		ProductName: service.ProductName,
		Type:        service.Type,
		Revision:    service.Revision,
	}
	svcTmpl, err := repository.QueryTemplateService(opt, prod.Production)
	if err != nil {
		return "", err
	}

	if prod.Production {
		svcTmpl.ServiceVars = setting.ServiceVarWildCard
	}

	// Note only the keys in TemplateService.ServiceVar can work
	parsedYaml, err := RenderServiceYaml(svcTmpl.Yaml, prod.ProductName, svcTmpl.ServiceName, render, svcTmpl.ServiceVars, svcTmpl.VariableYaml)
	if err != nil {
		log.Error("failed to render service yaml, err: %s", err)
		return "", err
	}
	//parsedYaml := RenderValueForString(svcTmpl.Yaml, prod.ProductName, svcTmpl.ServiceName, render)
	parsedYaml = ParseSysKeys(prod.Namespace, prod.EnvName, prod.ProductName, service.ServiceName, parsedYaml)
	parsedYaml = replaceContainerImages(parsedYaml, svcTmpl.Containers, service.Containers)

	return parsedYaml, nil
}

func replaceContainerImages(tmpl string, ori []*commonmodels.Container, replace []*commonmodels.Container) string {

	replaceMap := make(map[string]string)
	for _, container := range replace {
		replaceMap[container.Name] = container.Image
	}

	for _, container := range ori {
		imageRex := regexp.MustCompile("image:\\s*" + container.Image)
		if _, ok := replaceMap[container.Name]; !ok {
			continue
		}
		tmpl = imageRex.ReplaceAllLiteralString(tmpl, fmt.Sprintf("image: %s", replaceMap[container.Name]))
	}

	return tmpl
}
