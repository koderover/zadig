package service

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

func LoadServiceFromYamlTemplate(username, projectName, serviceName, templateID string, variables []*Variable, logger *zap.SugaredLogger) error {
	template, err := commonrepo.NewYamlTemplateColl().GetById(templateID)
	if err != nil {
		logger.Errorf("Failed to find template of ID: %s, the error is: %s", templateID, err)
		return err
	}
	renderedYaml := renderYamlFromTemplate(template.Content, projectName, serviceName, variables)
	service := &commonmodels.Service{
		ServiceName: serviceName,
		Type:        setting.K8SDeployType,
		ProductName: projectName,
		Source:      setting.ServiceSourceTemplate,
		Yaml:        renderedYaml,
		Visibility:  setting.PrivateVisibility,
		TemplateID:  templateID,
	}
	_, err = CreateServiceTemplate(username, service, logger)
	if err != nil {
		logger.Errorf("Failed to create service template from template ID: %s, the error is: %s", templateID, err)
	}
	return err
}

func ReloadServiceFromYamlTemplate(username, projectName, serviceName string, variables []*Variable, logger *zap.SugaredLogger) error {
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
		return errors.New("failed to find template id for service:" + serviceName)
	}
	template, err := commonrepo.NewYamlTemplateColl().GetById(service.TemplateID)
	if err != nil {
		logger.Errorf("Failed to find template of ID: %s, the error is: %s", service.TemplateID, err)
		return err
	}
	renderedYaml := renderYamlFromTemplate(template.Content, projectName, serviceName, variables)
	svc := &commonmodels.Service{
		ServiceName: serviceName,
		Type:        setting.K8SDeployType,
		ProductName: projectName,
		Source:      setting.ServiceSourceTemplate,
		Yaml:        renderedYaml,
		Visibility:  setting.PrivateVisibility,
		TemplateID:  service.TemplateID,
	}
	_, err = CreateServiceTemplate(username, svc, logger)
	if err != nil {
		logger.Errorf("Failed to create service template from template ID: %s, the error is: %s", service.TemplateID, err)
	}
	return err
}

func renderYamlFromTemplate(yaml, productName, serviceName string, variables []*Variable) string {
	for _, variable := range variables {
		yaml = strings.Replace(yaml, buildVariable(variable.Key), variable.Value, -1)
	}
	yaml = strings.Replace(yaml, setting.TemplateVariableProduct, productName, -1)
	yaml = strings.Replace(yaml, setting.TemplateVariableService, serviceName, -1)
	return yaml
}

func buildVariable(key string) string {
	return fmt.Sprintf("{{.%s}}", key)
}
