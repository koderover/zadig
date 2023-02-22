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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/repository"

	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/pkg/errors"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/sets"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commomtemplate "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/converter"
)

type GeneSvcYamlOption struct {
	ProductName           string
	EnvName               string
	ServiceName           string
	UpdateServiceRevision bool
	VariableYaml          string
	UnInstall             bool
}

func ClipVariableYaml(variableYaml string, validKeys []string) (string, error) {
	valuesMap, err := converter.YamlToFlatMap([]byte(variableYaml))
	if err != nil {
		return "", fmt.Errorf("failed to get flat map for service variable, err: %s", err)
	}

	keysSet := sets.NewString(validKeys...)
	validKvMap := make(map[string]interface{})
	for k, v := range valuesMap {
		if keysSet.Has(k) {
			validKvMap[k] = v
		}
	}

	validKvMap, err = converter.Expand(validKvMap)
	if err != nil {
		return "", err
	}

	bs, err := yaml.Marshal(validKvMap)
	return string(bs), err
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

	wildcard := false
	if len(serviceVars) == 1 && serviceVars[0] == "*" {
		wildcard = true
	}

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

func updateCronJobImages(yamlStr string, imageMap map[string]*commonmodels.Container) (string, error) {
	res := new(types.CronjobResource)
	if err := yaml.Unmarshal([]byte(yamlStr), &res); err != nil {
		return yamlStr, fmt.Errorf("unmarshal Resource error: %v", err)
	}
	for _, val := range res.Spec.Template.Spec.Template.Spec.Containers {
		containerName := val["name"].(string)
		if image, ok := imageMap[containerName]; ok {
			val["image"] = image.Image
		}
	}

	bs, err := yaml.Marshal(res)
	return string(bs), err
}

func updateContainerImages(yamlStr string, imageMap map[string]*commonmodels.Container) (string, error) {
	res := new(types.KubeResource)
	if err := yaml.Unmarshal([]byte(yamlStr), &res); err != nil {
		return yamlStr, fmt.Errorf("unmarshal Resource error: %v", err)
	}

	for _, container := range res.Spec.Template.Spec.Containers {
		containerName := container["name"].(string)
		if image, ok := imageMap[containerName]; ok {
			container["image"] = image.Image
		}
	}

	bs, err := yaml.Marshal(res)
	return string(bs), err
}

// ReplaceWorkloadImages  replace images in yaml with new images
func ReplaceWorkloadImages(rawYaml string, images []*commonmodels.Container) (string, error) {
	imageMap := make(map[string]*commonmodels.Container)
	for _, image := range images {
		imageMap[image.Name] = image
	}

	splitYams := util.SplitYaml(rawYaml)
	yamlStrs := make([]string, 0)
	var err error
	for _, yamlStr := range splitYams {
		resKind := new(types.KubeResourceKind)
		if err := yaml.Unmarshal([]byte(yamlStr), &resKind); err != nil {
			return "", fmt.Errorf("unmarshal ResourceKind error: %v", err)
		}

		if resKind.Kind == setting.Deployment || resKind.Kind == setting.StatefulSet || resKind.Kind == setting.Job {
			yamlStr, err = updateContainerImages(yamlStr, imageMap)
			if err != nil {
				return "", err
			}
		} else if resKind.Kind == setting.CronJob {
			yamlStr, err = updateCronJobImages(yamlStr, imageMap)
			if err != nil {
				return "", err
			}
		}
		yamlStrs = append(yamlStrs, yamlStr)
	}

	return util.JoinYamls(yamlStrs), nil
}

func mergeContainers(curContainers, newContainers []*commonmodels.Container) []*commonmodels.Container {
	curContainerMap := make(map[string]*commonmodels.Container)
	for _, container := range curContainers {
		curContainerMap[container.Name] = container
	}
	for _, container := range newContainers {
		curContainerMap[container.Name] = container
	}
	var containers []*commonmodels.Container
	for _, container := range curContainerMap {
		containers = append(containers, container)
	}
	return containers
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
		ProductName:   option.ProductName,
		ServiceName:   option.ServiceName,
		ExcludeStatus: setting.ProductStatusDeleting,
		Revision:      curProductSvc.Revision,
	}, productInfo.Production)
	if err != nil {
		return "", 0, errors.Wrapf(err, "failed to find service %s with revision %d", option.ServiceName, curProductSvc.Revision)
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

	if productInfo.Production {
		prodSvcTemplate.ServiceVars = setting.ServiceVarWildCard
	}

	fullRenderedYaml, err := RenderServiceYaml(prodSvcTemplate.Yaml, option.ProductName, option.ServiceName, usedRenderset, prodSvcTemplate.ServiceVars, prodSvcTemplate.VariableYaml)
	if err != nil {
		return "", 0, err
	}
	fullRenderedYaml = ParseSysKeys(productInfo.Namespace, productInfo.EnvName, option.ProductName, option.ServiceName, fullRenderedYaml)

	return fullRenderedYaml, 0, nil
}

// GenerateRenderedYaml generates full yaml of some service defined in Zadig (images not included)
// and returns the service yaml, used service revision
func GenerateRenderedYaml(option *GeneSvcYamlOption) (string, int, error) {
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

	// nothing to render when trying to uninstall a service which is not deployed
	if option.UnInstall && curProductSvc == nil {
		return "", 0, nil
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
		return "", 0, errors.Wrapf(err, "failed to find latest service template %s", option.ServiceName)
	}

	if curProductSvc != nil {
		prodSvcTemplate, err = repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ProductName:   option.ProductName,
			ServiceName:   option.ServiceName,
			ExcludeStatus: setting.ProductStatusDeleting,
			Revision:      curProductSvc.Revision,
		}, productInfo.Production)
		if err != nil {
			return "", 0, errors.Wrapf(err, "failed to find service %s with revision %d", option.ServiceName, curProductSvc.Revision)
		}
	} else {
		prodSvcTemplate = latestSvcTemplate
	}

	// use latest service revision
	if latestSvcTemplate == nil || !option.UpdateServiceRevision {
		latestSvcTemplate = prodSvcTemplate
	}
	if latestSvcTemplate == nil {
		return "", 0, fmt.Errorf("failed to find service template %s used in product %s", option.ServiceName, option.EnvName)
	}

	curContainers := latestSvcTemplate.Containers
	if curProductSvc != nil {
		curContainers = curProductSvc.Containers
	}

	var usedRenderset *commonmodels.RenderSet
	if option.UnInstall {
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
	} else {
		usedRenderset = &commonmodels.RenderSet{
			ProductTmpl: productInfo.ProductName,
			EnvName:     productInfo.EnvName,
			ServiceVariables: []*template.ServiceRender{
				{
					ServiceName: option.ServiceName,
					OverrideYaml: &template.CustomYaml{
						YamlContent: option.VariableYaml,
					},
				},
			},
		}
	}

	if productInfo.Production {
		latestSvcTemplate.ServiceVars = setting.ServiceVarWildCard
	}

	fullRenderedYaml, err := RenderServiceYaml(latestSvcTemplate.Yaml, option.ProductName, option.ServiceName, usedRenderset, latestSvcTemplate.ServiceVars, latestSvcTemplate.VariableYaml)
	if err != nil {
		return "", 0, err
	}
	fullRenderedYaml = ParseSysKeys(productInfo.Namespace, productInfo.EnvName, option.ProductName, option.ServiceName, fullRenderedYaml)

	fullRenderedYaml, err = ReplaceWorkloadImages(fullRenderedYaml, mergeContainers(curContainers, latestSvcTemplate.Containers))
	return fullRenderedYaml, int(latestSvcTemplate.Revision), err
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
