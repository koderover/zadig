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

package template

import (
	"errors"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

var DefaultSystemVariable = map[string]string{
	setting.TemplateVariableProduct: setting.TemplateVariableProductDescription,
	setting.TemplateVariableService: setting.TemplateVariableServiceDescription,
}

func CreateYamlTemplate(template *YamlTemplate, logger *zap.SugaredLogger) error {
	vars := make([]*models.Variable, 0)
	for _, variable := range template.Variable {
		vars = append(vars, &models.Variable{
			Key:   variable.Key,
			Value: variable.Value,
		})
	}
	err := commonrepo.NewYamlTemplateColl().Create(&models.YamlTemplate{
		Name:      template.Name,
		Content:   template.Content,
		Variables: vars,
	})
	if err != nil {
		logger.Errorf("create dockerfile template error: %s", err)
	}
	return err
}

func UpdateYamlTemplate(id string, template *YamlTemplate, logger *zap.SugaredLogger) error {
	vars := make([]*models.Variable, 0)
	for _, variable := range template.Variable {
		vars = append(vars, &models.Variable{
			Key:   variable.Key,
			Value: variable.Value,
		})
	}
	err := commonrepo.NewYamlTemplateColl().Update(
		id,
		&models.YamlTemplate{
			Name:      template.Name,
			Content:   template.Content,
			Variables: vars,
		},
	)
	if err != nil {
		logger.Errorf("update dockerfile template error: %s", err)
	}
	return err
}

func ListYamlTemplate(pageNum, pageSize int, logger *zap.SugaredLogger) ([]*YamlListObject, int, error) {
	resp := make([]*YamlListObject, 0)
	templateList, total, err := commonrepo.NewYamlTemplateColl().List(pageNum, pageSize)
	if err != nil {
		logger.Errorf("list dockerfile template error: %s", err)
		return resp, 0, err
	}
	for _, obj := range templateList {
		resp = append(resp, &YamlListObject{
			ID:   obj.ID.Hex(),
			Name: obj.Name,
		})
	}
	return resp, total, err
}

func GetYamlTemplateDetail(id string, logger *zap.SugaredLogger) (*YamlDetail, error) {
	resp := new(YamlDetail)
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
	return resp, nil
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

func GetYamlTemplateReference(id string, logger *zap.SugaredLogger) ([]*ServiceReference, error) {
	ret := make([]*ServiceReference, 0)
	referenceList, err := commonrepo.NewServiceColl().GetTemplateReference(id)
	if err != nil {
		logger.Errorf("Failed to get build reference for dockerfile template id: %s, the error is: %s", id, err)
		return ret, err
	}
	for _, reference := range referenceList {
		ret = append(ret, &ServiceReference{
			ServiceName: reference.ServiceName,
			ProjectName: reference.ProductName,
		})
	}
	return ret, nil
}

func GetYamlVariables(s string, logger *zap.SugaredLogger) ([]*models.ChartVariable, error) {
	resp := make([]*models.ChartVariable, 0)
	regex, err := regexp.Compile(setting.RegExpParameter)
	if err != nil {
		logger.Errorf("Cannot get regexp from the expression: %s, the error is: %s", setting.RegExpParameter, err)
		return []*models.ChartVariable{}, err
	}
	params := regex.FindAllString(s, -1)
	keyMap := make(map[string]int)
	for _, param := range params {
		key := getParameterKey(param)
		if keyMap[key] == 0 {
			resp = append(resp, &models.ChartVariable{
				Key: key,
			})
			keyMap[key] = 1
		}
	}
	return resp, nil
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

// getParameter
func getParameterKey(parameter string) string {
	a := strings.TrimPrefix(parameter, "{{.")
	return strings.TrimSuffix(a, "}}")
}
