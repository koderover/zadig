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

package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	gotemplate "text/template"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	commomtemplate "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

type LoadServiceFromYamlTemplateReq struct {
	ServiceName string `json:"service_name"`
	ProjectName string `json:"project_name"`
	TemplateID  string `json:"template_id"`
	AutoSync    bool   `json:"auto_sync"`
	//Variables    []*Variable `json:"variables"`
	VariableYaml string `json:"variable_yaml"`
}

func geneCreateFromDetail(templateId string, variableYaml string) *commonmodels.CreateFromYamlTemplate {
	//vs := make([]*commonmodels.Variable, 0, len(variables))
	//for _, kv := range variables {
	//	vs = append(vs, &commonmodels.Variable{
	//		Key:   kv.Key,
	//		Value: kv.Value,
	//	})
	//}
	return &commonmodels.CreateFromYamlTemplate{
		TemplateID: templateId,
		//Variables:    vs,
		VariableYaml: variableYaml,
	}
}

func LoadServiceFromYamlTemplate(username string, req *LoadServiceFromYamlTemplateReq, force bool, logger *zap.SugaredLogger) error {
	projectName, serviceName, templateID, autoSync := req.ProjectName, req.ServiceName, req.TemplateID, req.AutoSync
	template, err := commonrepo.NewYamlTemplateColl().GetById(templateID)
	if err != nil {
		logger.Errorf("Failed to find template of ID: %s, the error is: %s", templateID, err)
		return err
	}
	renderedYaml := renderSystemVars(template.Content, projectName, serviceName)
	fullRenderedYaml, err := renderK8sSvcYaml(template.Content, projectName, serviceName, template.VariableYaml, req.VariableYaml)
	if err != nil {
		return err
	}
	service := &commonmodels.Service{
		ServiceName:  serviceName,
		Type:         setting.K8SDeployType,
		ProductName:  projectName,
		Source:       setting.ServiceSourceTemplate,
		Yaml:         renderedYaml,
		RenderedYaml: fullRenderedYaml,
		Visibility:   setting.PrivateVisibility,
		TemplateID:   templateID,
		AutoSync:     autoSync,
		VariableYaml: req.VariableYaml,
		ServiceVars:  template.ServiceVars,
		CreateFrom:   geneCreateFromDetail(templateID, req.VariableYaml),
	}
	_, err = CreateServiceTemplate(username, service, force, logger)
	if err != nil {
		logger.Errorf("Failed to create service template from template ID: %s, the error is: %s", templateID, err)
	}
	return err
}

func ReloadServiceFromYamlTemplate(username string, req *LoadServiceFromYamlTemplateReq, logger *zap.SugaredLogger) error {
	projectName, serviceName, templateID, autoSync := req.ProjectName, req.ServiceName, req.TemplateID, req.AutoSync
	service, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		ProductName: projectName,
	})
	if err != nil {
		logger.Errorf("Cannot find service of name [%s] from project [%s], the error is: %s", serviceName, projectName, err)
		return err
	}
	if service.Source != setting.ServiceSourceTemplate {
		return errors.New("service is not created from template")
	}
	if service.TemplateID == "" {
		return fmt.Errorf("failed to find template id for service: %s", serviceName)
	}
	template, err := commonrepo.NewYamlTemplateColl().GetById(templateID)
	if err != nil {
		logger.Errorf("Failed to find template of ID: %s, the error is: %s", templateID, err)
		return err
	}

	service.AutoSync = autoSync
	return reloadServiceFromYamlTemplateImpl(username, projectName, template, service, req.VariableYaml)
}

func PreviewServiceFromYamlTemplate(req *LoadServiceFromYamlTemplateReq, logger *zap.SugaredLogger) (string, error) {
	yamlTemplate, err := commonrepo.NewYamlTemplateColl().GetById(req.TemplateID)
	if err != nil {
		return "", fmt.Errorf("failed to preview service, err: %s", err)
	}

	//templateVariableYaml, err := commomtemplate.GetTemplateVariableYaml(yamlTemplate.Variables, yamlTemplate.VariableYaml)
	//if err != nil {
	//	return "", fmt.Errorf("failed to get variable yaml from yaml template")
	//}
	//templateVariableYaml := yamlTemplate.VariableYaml
	return renderK8sSvcYaml(yamlTemplate.Content, req.ProjectName, req.ServiceName, yamlTemplate.VariableYaml, req.VariableYaml)
}

func renderSystemVars(originYaml, productName, serviceName string) string {
	originYaml = strings.ReplaceAll(originYaml, setting.TemplateVariableProduct, productName)
	originYaml = strings.ReplaceAll(originYaml, setting.TemplateVariableService, serviceName)
	return originYaml
}

func renderK8sSvcYaml(originYaml, productName, serviceName string, variableYamls ...string) (string, error) {
	tmpl, err := gotemplate.New(serviceName).Parse(originYaml)
	if err != nil {
		return originYaml, fmt.Errorf("failed to build template, err: %s", err)
	}

	variableYaml, replacedKv, err := commomtemplate.SafeMergeVariableYaml(variableYamls...)
	if err != nil {
		return originYaml, err
	}

	//for _, variable := range variables {
	//	variableYaml = strings.ReplaceAll(variableYaml, buildVariable(variable.Key), variable.Value)
	//}
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

//func buildVariable(key string) string {
//	return fmt.Sprintf("{{.%s}}", key)
//}

// SyncServiceFromTemplate syncs services from (yaml|chart)template
func SyncServiceFromTemplate(userName, source, templateId, templateName string, logger *zap.SugaredLogger) error {
	if source == setting.ServiceSourceTemplate {
		return syncServicesFromYamlTemplate(userName, templateId, logger)
	} else {
		return syncServicesFromChartTemplate(userName, templateName, logger)
	}
}

func syncServicesFromYamlTemplate(userName, templateId string, logger *zap.SugaredLogger) error {
	serviceList, err := commonrepo.NewServiceColl().GetYamlTemplateReference(templateId)
	if err != nil {
		return err
	}
	servicesByProject := make(map[string][]*commonmodels.Service)
	for _, service := range serviceList {
		if !service.AutoSync {
			continue
		}
		servicesByProject[service.ProductName] = append(servicesByProject[service.ProductName], service)
	}
	yamlTemplate, err := commonrepo.NewYamlTemplateColl().GetById(templateId)
	if err != nil {
		return fmt.Errorf("failed to find yaml template: %s, err: %s", templateId, err)
	}
	for _, services := range servicesByProject {
		go func(pServices []*commonmodels.Service) {
			for _, service := range pServices {
				err := reloadServiceFromYamlTemplate(userName, service.ProductName, yamlTemplate, service)
				if err != nil {
					logger.Error(err)
					title := fmt.Sprintf("从模板更新 [%s] 的 [%s] 服务失败", service.ProductName, service.ServiceName)
					commonservice.SendErrorMessage(userName, title, "", err, logger)
				}
			}
		}(services)
	}
	return nil
}

func syncServicesFromChartTemplate(userName, templateName string, logger *zap.SugaredLogger) error {
	chartTemplate, err := prepareChartTemplateData(templateName, log.SugaredLogger())
	if err != nil {
		return err
	}

	serviceList, err := commonrepo.NewServiceColl().ListMaxRevisionServicesByChartTemplate(templateName)
	if err != nil {
		return err
	}
	servicesByProject := make(map[string][]*commonmodels.Service)
	for _, service := range serviceList {
		if !service.AutoSync {
			continue
		}
		servicesByProject[service.ProductName] = append(servicesByProject[service.ProductName], service)
	}

	for _, services := range servicesByProject {
		go func(pService []*commonmodels.Service) {
			for _, service := range pService {
				err := reloadServiceFromChartTemplate(service, chartTemplate)
				if err != nil {
					logger.Errorf("failed to reload service %s/%s from chart template, err: %s", service.ProductName, service.ServiceName, err)
					title := fmt.Sprintf("从模板更新 [%s] 的 [%s] 服务失败", service.ProductName, service.ServiceName)
					commonservice.SendErrorMessage(userName, title, "", err, logger)
				}
			}
		}(services)
	}
	return nil
}

func reloadServiceFromChartTemplate(service *commonmodels.Service, chartTemplate *ChartTemplateData) error {
	variable, customYaml, err := buildChartTemplateVariables(service, chartTemplate.TemplateData)
	if err != nil {
		return err
	}

	templateArgs := &CreateFromChartTemplate{
		TemplateName: chartTemplate.TemplateName,
		ValuesYAML:   customYaml,
		Variables:    variable,
	}
	args := &HelmServiceCreationArgs{
		HelmLoadSource: HelmLoadSource{},
		Name:           service.ServiceName,
		CreatedBy:      "system",
		ValuesData:     nil,
		CreationDetail: service.CreateFrom,
		AutoSync:       service.AutoSync,
	}
	ret, err := createOrUpdateHelmServiceFromChartTemplate(templateArgs, chartTemplate, service.ProductName, args, true, log.SugaredLogger())
	if err != nil {
		return err
	}
	if len(ret.FailedServices) == 1 {
		return errors.New(ret.FailedServices[0].Error)
	}
	return nil
}

func buildYamlTemplateVariables(service *commonmodels.Service, template *commonmodels.YamlTemplate) (string, error) {
	//variables := make([]*Variable, 0)
	//variableMap := make(map[string]*Variable)
	//for _, v := range template.Variables {
	//	kv := &Variable{
	//		Key:   v.Key,
	//		Value: v.Value,
	//	}
	//	variableMap[v.Key] = kv
	//	variables = append(variables, kv)
	//}

	//templateVariable, err := commomtemplate.GetTemplateVariableYaml(template.Variables, template.VariableYaml)
	//if err != nil {
	//	return nil, "", err
	//}
	//templateVariable := template.VariableYaml

	//creation := &commonmodels.CreateFromYamlTemplate{}
	//vbs := make([]*commonmodels.Variable, 0)
	//if service.CreateFrom != nil {
	//bs, err := json.Marshal(service.CreateFrom)
	//if err != nil {
	//	log.Errorf("failed to marshal creation data: %s", err)
	//	return variables, "", err
	//}
	//
	//err = json.Unmarshal(bs, creation)
	//if err != nil {
	//	log.Errorf("failed to unmarshal creation data: %s", err)
	//	return variables, "", err
	//}
	//for _, kv := range creation.Variables {
	//	if tkv, ok := variableMap[kv.Key]; ok {
	//		tkv.Value = kv.Value
	//	}
	//}

	//serviceVariable, err := commomtemplate.GetTemplateVariableYaml(creation.Variables, creation.VariableYaml)
	//if err != nil {
	//	return nil, "", err
	//}

	//serviceVariable := creation.VariableYaml
	//
	//kvs := make(map[string]string)
	//templateVariable, kvs, err = commomtemplate.SafeMergeVariableYaml(templateVariable, serviceVariable)
	//for k, v := range kvs {
	//	templateVariable = strings.ReplaceAll(templateVariable, k, v)
	//}
	//creation.VariableYaml = templateVariable
	//} else {
	//	creation.TemplateID = template.ID.Hex()
	//}

	kvs := make(map[string]string)
	templateVariable, kvs, err := commomtemplate.SafeMergeVariableYaml(template.VariableYaml, service.VariableYaml)
	for k, v := range kvs {
		templateVariable = strings.ReplaceAll(templateVariable, k, v)
	}
	if err != nil {
		log.Errorf("failed to sage merge variables, err: %s", err)
	}

	//for _, kv := range variables {
	//	vbs = append(vbs, &commonmodels.Variable{
	//		Key:   kv.Key,
	//		Value: kv.Value,
	//	})
	//}
	//creation.Variables = vbs
	//service.CreateFrom = creation

	return templateVariable, nil
}

func buildChartTemplateVariables(service *commonmodels.Service, template *commonmodels.Chart) ([]*Variable, string, error) {
	variables := make([]*Variable, 0)
	variableMap := make(map[string]*Variable)

	for _, v := range template.ChartVariables {
		kv := &Variable{
			Key:   v.Key,
			Value: v.Value,
		}
		variableMap[v.Key] = kv
		variables = append(variables, kv)
	}

	customYaml := ""
	if service.CreateFrom != nil {
		bs, err := json.Marshal(service.CreateFrom)
		if err != nil {
			log.Errorf("failed to marshal creation data: %s", err)
			return variables, "", err
		}
		creation := &commonmodels.CreateFromChartTemplate{}
		err = json.Unmarshal(bs, creation)
		if err != nil {
			log.Errorf("failed to unmarshal creation data: %s", err)
			return variables, "", err
		}
		for _, kv := range creation.Variables {
			if tkv, ok := variableMap[kv.Key]; ok {
				tkv.Value = kv.Value
			}
		}
		if creation.YamlData != nil {
			customYaml = creation.YamlData.YamlContent
		}
		vbs := make([]*commonmodels.Variable, 0)
		for _, kv := range variables {
			vbs = append(vbs, &commonmodels.Variable{
				Key:   kv.Key,
				Value: kv.Value,
			})
		}
		creation.Variables = vbs
		service.CreateFrom = creation
	}
	return variables, customYaml, nil
}

func reloadServiceFromYamlTemplateImpl(userName, projectName string, template *commonmodels.YamlTemplate, service *commonmodels.Service, variableYaml string) error {
	renderedYaml := renderSystemVars(template.Content, projectName, service.ServiceName)
	fullRenderedYaml, err := renderK8sSvcYaml(template.Content, projectName, service.ServiceName, template.VariableYaml, variableYaml)
	if err != nil {
		return err
	}

	serviceVars := service.ServiceVars
	// for services auto sync from yaml template, use service vars define in yaml template
	if service.AutoSync {
		serviceVars = template.ServiceVars
	}

	svc := &commonmodels.Service{
		ServiceName:  service.ServiceName,
		Type:         setting.K8SDeployType,
		ProductName:  projectName,
		Source:       setting.ServiceSourceTemplate,
		Yaml:         renderedYaml,
		RenderedYaml: fullRenderedYaml,
		Visibility:   setting.PrivateVisibility,
		ServiceVars:  serviceVars,
		VariableYaml: variableYaml,
		TemplateID:   service.TemplateID,
		CreateFrom:   geneCreateFromDetail(service.TemplateID, variableYaml),
		AutoSync:     service.AutoSync,
	}
	_, err = CreateServiceTemplate(userName, svc, true, log.SugaredLogger())
	if err != nil {
		return fmt.Errorf("failed to reload service template from template ID: %s, error : %s", service.TemplateID, err)
	}
	return nil
}

func reloadServiceFromYamlTemplate(userName, projectName string, template *commonmodels.YamlTemplate, service *commonmodels.Service) error {
	//extract variables from current service
	// merge service variable and yaml variable
	variableYaml, err := buildYamlTemplateVariables(service, template)
	if err != nil {
		return err
	}

	return reloadServiceFromYamlTemplateImpl(userName, projectName, template, service, variableYaml)
}
