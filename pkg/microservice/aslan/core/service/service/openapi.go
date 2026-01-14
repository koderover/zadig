package service

import (
	"fmt"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
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

	return LoadServiceFromYamlTemplate(username, loadArgs, force, req.Production, logger)
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

	_, err := CreateServiceTemplate(userName, createArgs, false, req.Production, logger)
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

	_, err = CreateServiceTemplate(userName, svc, true, false, log)
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

	_, err = CreateServiceTemplate(userName, svc, true, true, log)
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
	yaml, err := commontypes.ServiceVariableKVToYaml(serviceKvs, true)
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

	return UpdateServiceVariables(servceTmplObjectargs, false)
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

	yaml, err := commontypes.ServiceVariableKVToYaml(serviceKvs, true)
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

	return UpdateServiceVariables(servceTmplObjectargs, true)
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

func OpenAPIListServiceLabels(projectKey, serviceName string, production *bool, log *zap.SugaredLogger) ([]*ServiceLabelResp, error) {
	resp := make([]*ServiceLabelResp, 0)

	labelbindings, err := commonrepo.NewLabelBindingColl().List(&commonrepo.LabelBindingListOption{
		ServiceName: serviceName,
		ProjectKey:  projectKey,
		Production:  production,
	})

	if err != nil {
		log.Errorf("failed to list label bindings for project: %s, service: %s, error: %s", projectKey, serviceName, err)
		return nil, fmt.Errorf("failed to list label bindings for project: %s, service: %s, error: %s", projectKey, serviceName, err)
	}

	for _, binding := range labelbindings {
		// TODO: query the data all at once
		labelSetting, err := commonrepo.NewLabelColl().GetByID(binding.LabelID)
		if err != nil {
			log.Errorf("failed to find the label setting for id: %s", binding.LabelID)
			return nil, fmt.Errorf("failed to find the label setting for id: %s", binding.LabelID)
		}

		resp = append(resp, &ServiceLabelResp{
			Key:   labelSetting.Key,
			Value: binding.Value,
		})
	}

	return resp, nil
}

func GetProductionYamlServiceOpenAPI(projectKey, serviceName string, logger *zap.SugaredLogger) (*OpenAPIGetYamlServiceResp, error) {
	var resp *OpenAPIGetYamlServiceResp
	service, err := commonservice.GetServiceTemplate(serviceName, setting.K8SDeployType, projectKey, setting.ProductStatusDeleting, 0, true, logger)
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

func OpenAPILoadHelmServiceFromTemplate(ctx *internalhandler.Context, projectKey string, req *OpenAPILoadHelmServiceFromTemplateReq) error {
	// Convert util.KVInput to []*Variable
	variables := make([]*Variable, 0, len(req.Variables))
	for _, kv := range req.Variables {
		valueStr := ""
		if kv.Value != nil {
			valueStr = fmt.Sprintf("%v", kv.Value)
		}
		variables = append(variables, &Variable{
			Key:   kv.Key,
			Value: valueStr,
		})
	}

	loadArgs := &HelmServiceCreationArgs{
		HelmLoadSource: HelmLoadSource{
			Source: LoadFromChartTemplate,
		},
		Name:       req.ServiceName,
		CreatedBy:  ctx.UserName,
		AutoSync:   req.AutoSync,
		Production: req.Production,
		CreateFrom: &CreateFromChartTemplate{
			TemplateName: req.TemplateName,
			ValuesYAML:   req.ValuesYaml,
			Variables:    variables,
		},
	}

	_, err := CreateOrUpdateHelmService(projectKey, loadArgs, false, ctx.Logger)
	return err
}
