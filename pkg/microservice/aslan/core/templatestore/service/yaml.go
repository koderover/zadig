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
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/converter"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
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
		//Variables:    nil,
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
			Name:    template.Name,
			Content: template.Content,
			//Variables:    nil,
			VariableYaml: template.VariableYaml,
			ServiceVars:  template.ServiceVars,
		},
	)
	if err != nil {
		logger.Errorf("update yaml template error: %s", err)
	}
	return err
}

func UpdateYamlTemplateVariable(id string, template *template.YamlTemplate, logger *zap.SugaredLogger) error {
	// NOTE. technically the content of variable yaml should be validated
	// but the service is not rendered before applied to k8s, we ignore the validation on template
	err := commonrepo.NewYamlTemplateColl().UpdateVariable(id, template.VariableYaml, template.ServiceVars)
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
	resp.ID = yamlTemplate.ID.Hex()
	resp.Name = yamlTemplate.Name
	resp.Content = yamlTemplate.Content
	resp.ServiceVars = yamlTemplate.ServiceVars
	resp.VariableYaml = yamlTemplate.VariableYaml
	if len(resp.VariableYaml) > 0 {
		flatMap, err := converter.YamlToFlatMap([]byte(resp.VariableYaml))
		if err != nil {
			log.Errorf("failed to get flat map of variable, err: %s", err)
		} else {
			allKeys := sets.NewString()
			for k, v := range flatMap {
				resp.VariableKVs = append(resp.VariableKVs, &models.VariableKV{
					Key:   k,
					Value: v,
				})
				allKeys.Insert(k)
			}
			validServiceVars := make([]string, 0)
			for _, k := range resp.ServiceVars {
				if allKeys.Has(k) {
					validServiceVars = append(validServiceVars, k)
				}
			}
			resp.ServiceVars = validServiceVars
		}
	}
	//resp.VariableYaml, err = template.GetTemplateVariableYaml(yamlTemplate.Variables, yamlTemplate.VariableYaml)
	return resp, err
}

func DeleteYamlTemplate(id string, logger *zap.SugaredLogger) error {
	ref, err := commonrepo.NewServiceColl().GetYamlTemplateReference(id)
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
	referenceList, err := commonrepo.NewServiceColl().GetYamlTemplateReference(id)
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

func ExtractVariable(yamlContent string) (string, error) {
	if len(yamlContent) == 0 {
		return "", nil
	}
	return yamlutil.ExtractVariableYaml(yamlContent)
}

func FlattenKvs(yamlContent string) ([]*models.VariableKV, error) {
	if len(yamlContent) == 0 {
		return nil, nil
	}

	valuesMap, err := converter.YamlToFlatMap([]byte(yamlContent))
	if err != nil {
		return nil, err
	}

	ret := make([]*models.VariableKV, 0)
	for k, v := range valuesMap {
		ret = append(ret, &models.VariableKV{
			Key:   k,
			Value: v,
		})
	}
	return ret, nil
}
