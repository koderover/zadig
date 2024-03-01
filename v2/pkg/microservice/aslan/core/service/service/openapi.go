package service

import (
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"go.uber.org/zap"
)

func OpenAPILoadServiceFromYamlTemplate(username string, req *OpenAPILoadServiceFromYamlTemplateReq, force bool, logger *zap.SugaredLogger) error {
	template, err := commonrepo.NewYamlTemplateColl().GetByName(req.TemplateName)
	if err != nil {
		logger.Errorf("Failed to find template of name: %s, err: %w", req.TemplateName, err)
		return fmt.Errorf("failed to find template of name: %s, err: %w", req.TemplateName, err)
	}

	mergedYaml, mergedKVs, err := commonutil.MergeServiceVariableKVsAndKVInput(template.ServiceVariableKVs, req.VariableYaml)
	if err != nil {
		return fmt.Errorf("failed to merge variable yaml, err: %w", err)
	}

	loadArgs := &LoadServiceFromYamlTemplateReq{
		ProjectName:        req.ProjectKey,
		ServiceName:        req.ServiceName,
		TemplateID:         template.ID.Hex(),
		AutoSync:           req.AutoSync,
		VariableYaml:       mergedYaml,
		ServiceVariableKVs: mergedKVs,
	}

	if req.Production {
		return LoadProductionServiceFromYamlTemplate(username, loadArgs, force, logger)
	}
	return LoadServiceFromYamlTemplate(username, loadArgs, force, logger)
}

func CreateRawYamlServicesOpenAPI(userName, projectKey string, req *OpenAPICreateYamlServiceReq, logger *zap.SugaredLogger) error {
	createArgs := &commonmodels.Service{
		ServiceName:        req.ServiceName,
		Type:               "k8s",
		ProductName:        projectKey,
		Source:             "spock",
		Yaml:               req.Yaml,
		CreateBy:           userName,
		Production:         req.Production,
		ServiceVariableKVs: req.VariableYaml,
	}

	var err error
	if req.Production {
		_, err = CreateProductionServiceTemplate(userName, createArgs, false, logger)
	} else {
		_, err = CreateServiceTemplate(userName, createArgs, false, logger)
	}
	return err
}

func OpenAPIUpdateServiceConfig(userName string, args *OpenAPIUpdateServiceConfigArgs, log *zap.SugaredLogger) error {
	svc := &commonmodels.Service{
		ServiceName: args.ServiceName,
		Type:        args.Type,
		ProductName: args.ProjectName,
		CreateBy:    userName,
		Source:      setting.SourceFromZadig,
		Yaml:        args.Yaml,
	}
	currentService, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: args.ProjectName,
		ServiceName: args.ServiceName,
	})
	if err != nil {
		return e.ErrUpdateTemplate.AddDesc(err.Error())
	}

	if currentService.Source == setting.ServiceSourceTemplate && currentService.AutoSync {
		return e.ErrUpdateService.AddDesc("service is created by template and auto_sync is true, can't update")
	}
	svc.ServiceVariableKVs = currentService.ServiceVariableKVs
	svc.VariableYaml = currentService.VariableYaml

	_, err = CreateServiceTemplate(userName, svc, true, log)
	return err
}

func OpenAPIProductionUpdateServiceConfig(userName string, args *OpenAPIUpdateServiceConfigArgs, log *zap.SugaredLogger) error {
	svc := &commonmodels.Service{
		ServiceName: args.ServiceName,
		Type:        args.Type,
		ProductName: args.ProjectName,
		CreateBy:    userName,
		Source:      setting.SourceFromZadig,
		Yaml:        args.Yaml,
	}

	// check the service can update
	service, err := commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: args.ProjectName,
		ServiceName: args.ServiceName,
	})
	if err != nil {
		return e.ErrUpdateTemplate.AddDesc(fmt.Errorf("failed to find service %s in db, project:%s, err: %v", args.ServiceName, args.ProjectName, err).Error())
	}
	if service.Source == setting.ServiceSourceTemplate && service.AutoSync {
		return e.ErrUpdateService.AddDesc("service is created by template and auto_sync is true, can't update")
	}
	currentService, err := commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: args.ProjectName,
		ServiceName: args.ServiceName,
	})
	if err != nil {
		return e.ErrUpdateService.AddDesc(err.Error())
	}
	svc.ServiceVariableKVs = currentService.ServiceVariableKVs
	svc.VariableYaml = currentService.VariableYaml

	_, err = CreateProductionServiceTemplate(userName, svc, true, log)
	return err
}

func OpenAPIUpdateServiceVariable(userName, projectName, serviceName string, args *OpenAPIUpdateServiceVariableRequest, logger *zap.SugaredLogger) error {
	service, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: projectName,
		ServiceName: serviceName,
	})
	if err != nil {
		logger.Errorf("failed to find service %s in db, project:%s, err: %v", serviceName, projectName, err)
		return e.ErrUpdateService.AddDesc(err.Error())
	}

	serviceKvs := make([]*commontypes.ServiceVariableKV, 0)
	for _, kv := range service.ServiceVariableKVs {
		serviceKvs = append(serviceKvs, kv)
	}
	for _, kv := range serviceKvs {
		for _, newKv := range args.ServiceVariableKVs {
			if kv.Key == newKv.Key {
				kv.Value = newKv.Value
				kv.Desc = newKv.Desc
				kv.Type = newKv.Type
				kv.Options = newKv.Options
			}
		}
	}
	yaml, err := commontypes.ServiceVariableKVToYaml(serviceKvs)
	if err != nil {
		logger.Errorf("failed to convert service variable kv to yaml, err: %v", err)
		return fmt.Errorf("failed to convert service variable kv to yaml, err: %w", err)
	}
	servceTmplObjectargs := &commonservice.ServiceTmplObject{
		ProductName:        projectName,
		ServiceName:        serviceName,
		Username:           userName,
		ServiceVariableKVs: serviceKvs,
		VariableYaml:       yaml,
	}

	return UpdateServiceVariables(servceTmplObjectargs)
}

func OpenAPIUpdateProductionServiceVariable(userName, projectName, serviceName string, args *OpenAPIUpdateServiceVariableRequest, logger *zap.SugaredLogger) error {
	service, err := commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: projectName,
		ServiceName: serviceName,
	})
	if err != nil {
		logger.Errorf("failed to find service %s in db, project:%s, err: %v", serviceName, projectName, err)
		return e.ErrUpdateService.AddDesc(err.Error())
	}
	serviceKvs := make([]*commontypes.ServiceVariableKV, 0)
	for _, kv := range service.ServiceVariableKVs {
		serviceKvs = append(serviceKvs, kv)
	}
	for _, newKv := range args.ServiceVariableKVs {
		for _, kv := range serviceKvs {
			if kv.Key == newKv.Key {
				kv.Value = newKv.Value
				kv.Desc = newKv.Desc
				kv.Type = newKv.Type
				kv.Options = newKv.Options
			}
		}
	}

	yaml, err := commontypes.ServiceVariableKVToYaml(serviceKvs)
	if err != nil {
		logger.Errorf("failed to convert service variable kv to yaml, err: %v", err)
		return fmt.Errorf("failed to convert service variable kv to yaml, err: %w", err)
	}

	servceTmplObjectargs := &commonservice.ServiceTmplObject{
		ProductName:        projectName,
		ServiceName:        serviceName,
		Username:           userName,
		ServiceVariableKVs: serviceKvs,
		VariableYaml:       yaml,
	}

	return UpdateProductionServiceVariables(servceTmplObjectargs)
}

func OpenAPIGetYamlService(projectKey, serviceName string, logger *zap.SugaredLogger) (*OpenAPIGetYamlServiceResp, error) {
	var resp *OpenAPIGetYamlServiceResp
	service, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ProductName: projectKey,
		ServiceName: serviceName,
	})
	if err != nil {
		msg := fmt.Errorf("failed to get service from db, projectKey: %s, serviceName: %s, error: %v", projectKey, serviceName, err)
		logger.Errorf(msg.Error())
		return nil, msg
	}

	template := &commonmodels.YamlTemplate{}
	if service.Source == setting.ServiceSourceTemplate && service.TemplateID != "" {
		template, err = commonrepo.NewYamlTemplateColl().GetById(service.TemplateID)
		if err != nil {
			logger.Errorf("failed to get service template from db, id: %s, err: %v", service.TemplateID, err)
			return nil, e.ErrGetTemplate.AddErr(fmt.Errorf("failed to get service template from db, err: %v", err))
		}
	}

	resp = &OpenAPIGetYamlServiceResp{
		ServiceName:        service.ServiceName,
		Source:             service.Source,
		Type:               service.Type,
		TemplateName:       template.Name,
		CreatedBy:          service.CreateBy,
		CreatedTime:        service.CreateTime,
		Yaml:               service.Yaml,
		ServiceVariableKvs: service.ServiceVariableKVs,
		Containers:         service.Containers,
	}

	return resp, nil
}

func GetProductionYamlServiceOpenAPI(projectKey, serviceName string, logger *zap.SugaredLogger) (*OpenAPIGetYamlServiceResp, error) {
	var resp *OpenAPIGetYamlServiceResp
	service, err := GetProductionK8sService(serviceName, projectKey, logger)
	if err != nil {
		msg := fmt.Errorf("failed to get production service from db, projectKey: %s, serviceName: %s, error: %v", projectKey, serviceName, err)
		logger.Errorf(msg.Error())
		return nil, msg
	}

	resp = &OpenAPIGetYamlServiceResp{
		ServiceName:        service.ServiceName,
		Source:             service.Source,
		Type:               service.Type,
		CreatedBy:          service.CreateBy,
		CreatedTime:        service.CreateTime,
		Yaml:               service.Yaml,
		ServiceVariableKvs: service.ServiceVariableKVs,
		Containers:         service.Containers,
	}

	if service.Source == setting.ServiceSourceTemplate && service.TemplateID != "" {
		template, err := commonrepo.NewYamlTemplateColl().GetById(service.TemplateID)
		if err != nil {
			logger.Errorf("failed to get template from db, id: %s, err: %v", service.TemplateID, err)
			return nil, e.ErrGetTemplate.AddErr(fmt.Errorf("failed to get service template from db, err: %v", err))
		}
		resp.TemplateName = template.Name
	}

	return resp, nil
}
