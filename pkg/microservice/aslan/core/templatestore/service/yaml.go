package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

var DefaultSystemVariable = map[string]string{
	setting.TemplateVariableProduct: setting.TemplateVariableProductDescription,
	setting.TemplateVariableService: setting.TemplateVariableServiceDescription,
}

func CreateYamlTemplate(template *YamlTemplate, logger *zap.SugaredLogger) error {
	variableList, err := getYamlVariables(template.Content, logger)
	if err != nil {
		return err
	}
	variables := make([]*models.Variable, 0)
	for _, v := range variableList {
		variables = append(variables, &models.Variable{
			Key:   v.Key,
			Value: v.Value,
		})
	}
	err = mongodb.NewYamlTemplateColl().Create(&models.YamlTemplate{
		Name:      template.Name,
		Content:   template.Content,
		Variables: variables,
	})
	if err != nil {
		logger.Errorf("create dockerfile template error: %s", err)
	}
	return err
}

func UpdateYamlTemplate(id string, template *YamlTemplate, logger *zap.SugaredLogger) error {
	variableList, err := getYamlVariables(template.Content, logger)
	if err != nil {
		return err
	}
	variables := make([]*models.Variable, 0)
	for _, v := range variableList {
		variables = append(variables, &models.Variable{
			Key:   v.Key,
			Value: v.Value,
		})
	}
	err = mongodb.NewYamlTemplateColl().Update(
		id,
		&models.YamlTemplate{
			Name:      template.Name,
			Content:   template.Content,
			Variables: variables,
		},
	)
	if err != nil {
		logger.Errorf("update dockerfile template error: %s", err)
	}
	return err
}

func ListYamlTemplate(pageNum, pageSize int, logger *zap.SugaredLogger) ([]*YamlListObject, int, error) {
	resp := make([]*YamlListObject, 0)
	templateList, total, err := mongodb.NewYamlTemplateColl().List(pageNum, pageSize)
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
	yamlTemplate, err := mongodb.NewYamlTemplateColl().GetById(id)
	if err != nil {
		logger.Errorf("Failed to get dockerfile template from id: %s, the error is: %s", id, err)
		return nil, err
	}
	variables := make([]*Variable, 0)
	for _, v := range yamlTemplate.Variables {
		variables = append(variables, &Variable{
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
	err = mongodb.NewYamlTemplateColl().DeleteByID(id)
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

func UpdateYamlTemplateVariables(id string, variables []*Variable, logger *zap.SugaredLogger) error {
	templateDetail, err := mongodb.NewYamlTemplateColl().GetById(id)
	if err != nil {
		logger.Errorf("Failed to find yaml template of id: %s, the error is: %s", id, err)
		return err
	}
	keyMap := make(map[string]int)
	// the keys in database will not collide
	for _, v := range templateDetail.Variables {
		keyMap[v.Key] = 1
	}
	newVars := make([]*models.Variable, 0)
	for _, vars := range variables {
		if keyMap[vars.Key] == 0 {
			errorMsg := fmt.Sprintf("Given key [%s] does not exist in yaml template [%s]", vars.Key, id)
			return errors.New(errorMsg)
		}
		newVars = append(newVars, &models.Variable{
			Key:   vars.Key,
			Value: vars.Value,
		})
	}
	err = mongodb.NewYamlTemplateColl().Update(
		id,
		&models.YamlTemplate{
			Name:      templateDetail.Name,
			Content:   templateDetail.Content,
			Variables: newVars,
		},
	)
	if err != nil {
		logger.Errorf("update dockerfile template error: %s", err)
	}
	return err
}

func getYamlVariables(s string, logger *zap.SugaredLogger) ([]*Variable, error) {
	resp := make([]*Variable, 0)
	regex, err := regexp.Compile(setting.RegExpParameter)
	if err != nil {
		logger.Errorf("Cannot get regexp from the expression: %s, the error is: %s", setting.RegExpParameter, err)
		return []*Variable{}, err
	}
	params := regex.FindAllString(s, -1)
	keyMap := make(map[string]int)
	for _, param := range params {
		key := getParameterKey(param)
		if keyMap[key] == 0 {
			resp = append(resp, &Variable{
				Key: key,
			})
			keyMap[key] = 1
		}
	}
	return resp, nil
}

func GetSystemDefaultVariables() []*Variable {
	resp := make([]*Variable, 0)
	for key, description := range DefaultSystemVariable {
		resp = append(resp, &Variable{
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
