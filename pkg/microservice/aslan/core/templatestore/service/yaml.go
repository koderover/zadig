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
	"bytes"
	"errors"
	"fmt"
	gotemplate "text/template"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/pkg/setting"
)

var DefaultSystemVariable = map[string]string{
	setting.TemplateVariableProduct: setting.TemplateVariableProductDescription,
	setting.TemplateVariableService: setting.TemplateVariableServiceDescription,
}

func CreateYamlTemplate(template *template.YamlTemplate, logger *zap.SugaredLogger) error {
	err := commonrepo.NewYamlTemplateColl().Create(&models.YamlTemplate{
		Name:         template.Name,
		Content:      template.Content,
		VariableYaml: template.VariableYaml,
		Variables:    nil,
	})
	if err != nil {
		logger.Errorf("create dockerfile template error: %s", err)
	}
	return err
}

func UpdateYamlTemplate(id string, template *template.YamlTemplate, logger *zap.SugaredLogger) error {
	err := commonrepo.NewYamlTemplateColl().Update(
		id,
		&models.YamlTemplate{
			Name:         template.Name,
			Content:      template.Content,
			Variables:    nil,
			VariableYaml: template.VariableYaml,
		},
	)
	if err != nil {
		logger.Errorf("update yaml template error: %s", err)
	}
	return err
}

func UpdateYamlTemplateVariable(id string, template *template.YamlTemplate, logger *zap.SugaredLogger) error {
	err := commonrepo.NewYamlTemplateColl().UpdateVariable(id, template.VariableYaml)
	if err != nil {
		logger.Errorf("update yaml template variable error: %s", err)
	}
	return err
}

func ListYamlTemplate(pageNum, pageSize int, logger *zap.SugaredLogger) ([]*template.YamlListObject, int, error) {
	resp := make([]*template.YamlListObject, 0)
	templateList, total, err := commonrepo.NewYamlTemplateColl().List(pageNum, pageSize)
	if err != nil {
		logger.Errorf("list dockerfile template error: %s", err)
		return resp, 0, err
	}
	for _, obj := range templateList {
		resp = append(resp, &template.YamlListObject{
			ID:   obj.ID.Hex(),
			Name: obj.Name,
		})
	}
	return resp, total, err
}

func GetYamlTemplateDetail(id string, logger *zap.SugaredLogger) (*template.YamlDetail, error) {
	resp := new(template.YamlDetail)
	yamlTemplate, err := commonrepo.NewYamlTemplateColl().GetById(id)
	if err != nil {
		logger.Errorf("Failed to get dockerfile template from id: %s, the error is: %s", id, err)
		return nil, err
	}
	variables := make([]*models.ChartVariable, 0)
	for _, v := range yamlTemplate.Variables {
		variables = append(variables, &models.ChartVariable{
			Key:   v.Key,
			Value: v.Value,
		})
	}
	resp.ID = yamlTemplate.ID.Hex()
	resp.Name = yamlTemplate.Name
	resp.Content = yamlTemplate.Content
	resp.Variables = variables
	resp.VariableYaml, err = template.GetTemplateVariableYaml(yamlTemplate.Variables, yamlTemplate.VariableYaml)
	return resp, err
}

func DeleteYamlTemplate(id string, logger *zap.SugaredLogger) error {
	ref, err := commonrepo.NewServiceColl().GetTemplateReference(id)
	if err != nil {
		logger.Errorf("Failed to get service reference for template id: %s, the error is: %s", id, err)
		return err
	}
	if len(ref) > 0 {
		return errors.New("this template is in use")
	}
	err = commonrepo.NewYamlTemplateColl().DeleteByID(id)
	if err != nil {
		logger.Errorf("Failed to delete dockerfile template of id: %s, the error is: %s", id, err)
	}
	return err
}

func SyncYamlTemplateReference(userName, id string, logger *zap.SugaredLogger) error {
	return service.SyncServiceFromTemplate(userName, setting.ServiceSourceTemplate, id, "", logger)
}

func GetYamlTemplateReference(id string, logger *zap.SugaredLogger) ([]*template.ServiceReference, error) {
	ret := make([]*template.ServiceReference, 0)
	referenceList, err := commonrepo.NewServiceColl().GetTemplateReference(id)
	if err != nil {
		logger.Errorf("Failed to get build reference for dockerfile template id: %s, the error is: %s", id, err)
		return ret, err
	}
	for _, reference := range referenceList {
		ret = append(ret, &template.ServiceReference{
			ServiceName: reference.ServiceName,
			ProjectName: reference.ProductName,
		})
	}
	return ret, nil
}

func GetSystemDefaultVariables() []*models.ChartVariable {
	resp := make([]*models.ChartVariable, 0)
	for key, description := range DefaultSystemVariable {
		resp = append(resp, &models.ChartVariable{
			Key:         key,
			Description: description,
		})
	}
	return resp
}

func ValidateVariable(content, variable string) error {
	if len(content) == 0 || len(variable) == 0 {
		return nil
	}
	variable, _, err := template.SafeMergeVariableYaml(variable)
	if err != nil {
		return err
	}
	valuesMap := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(variable), &valuesMap); err != nil {
		return fmt.Errorf("failed to unmarshal yaml: %s", err)
	}

	tmpl, err := gotemplate.New("").Parse(content)
	if err != nil {
		return fmt.Errorf("failed to build template, err: %s", err)
	}

	for k := range DefaultSystemVariable {
		valuesMap[k] = k
	}
	buf := bytes.NewBufferString("")
	err = tmpl.Execute(buf, valuesMap)
	if err != nil {
		return fmt.Errorf("template validate err: %s", err)
	}
	return nil
}
