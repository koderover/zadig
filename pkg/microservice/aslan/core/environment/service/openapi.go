/*
Copyright 2023 The KodeRover Authors.

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
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm/utils"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func GetEnvDetail(projectName, envName string, production bool, logger *zap.SugaredLogger) (*OpenAPIEnvDetail, error) {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       projectName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		logger.Errorf("failed to find project:%s env:%s", projectName, envName)
		return nil, err
	}

	resp := &OpenAPIEnvDetail{
		EnvName:     envName,
		ProjectName: env.ProductName,
		ClusterID:   env.ClusterID,
		Namespace:   env.Namespace,
		RegistryID:  env.RegistryID,
		Alias:       env.Alias,
		Status:      strings.ToLower(env.Status),
		UpdateBy:    env.UpdateBy,
		UpdateTime:  env.UpdateTime,
	}

	// get service and service variables
	services := make([]*OpenAPIServiceDetail, 0)
	serviceCount := 0
	for _, servs := range env.Services {
		for _ = range servs {
			serviceCount++
		}
	}
	// get service status
	groups, _, err := ListGroups("", envName, projectName, serviceCount, 1, env.Production, logger)
	if err != nil {
		logger.Errorf("failed to list group for env:%s", envName)
		return nil, err
	}
	for _, servs := range env.Services {
		for _, serv := range servs {
			service := &OpenAPIServiceDetail{
				ServiceName: serv.ServiceName,
				Containers:  serv.Containers,
				Type:        serv.Type,
			}
			if !env.Production {
				servDetail, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
					ProductName: projectName,
					ServiceName: serv.ServiceName,
				})
				if err != nil {
					logger.Errorf("failed to find service:%s", serv.ServiceName)
					return nil, e.ErrGetEnv.AddDesc(err.Error())
				}
				service.VariableKVs = servDetail.ServiceVariableKVs
			} else {
				servDetail, err := commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
					ProductName: projectName,
					ServiceName: serv.ServiceName,
				})
				if err != nil {
					logger.Errorf("failed to find service:%s", serv.ServiceName)
					return nil, e.ErrGetEnv.AddDesc(err.Error())
				}
				service.VariableKVs = servDetail.ServiceVariableKVs
			}

			for _, group := range groups {
				if group.ServiceName == serv.ServiceName {
					service.Status = strings.ToLower(group.Status)
				}
			}
			services = append(services, service)
		}
	}
	resp.Services = services

	// get global variables
	variables, _, err := GetGlobalVariables(projectName, envName, production, logger)
	if err != nil {
		logger.Errorf("failed to get global variables for project:%s env:%s", projectName, envName)
		return nil, err
	}
	resp.GlobalVariables = variables
	return resp, nil
}

func OpenAPIUpdateEnvBasicInfo(args *EnvBasicInfoArgs, userName, projectName, envName string, production bool, logger *zap.SugaredLogger) error {
	if args.RegistryID != "" {
		err := UpdateProductRegistry(envName, projectName, args.RegistryID, production, logger)
		if err != nil {
			logger.Errorf("failed to update registry for project:%s env:%s", projectName, envName)
			return err
		}
	}
	if production {
		err := UpdateProductAlias(envName, projectName, args.Alias, production)
		if err != nil {
			logger.Errorf("failed to update alias for project:%s env:%s", projectName, envName)
			return err
		}
	}
	return nil
}

func OpenAPIRestartService(projectName, envName, serviceName string, production bool, logger *zap.SugaredLogger) error {
	args := &SvcOptArgs{
		EnvName:     envName,
		ProductName: projectName,
		ServiceName: serviceName,
	}
	return RestartService(envName, args, production, logger)
}

func OpenAPIGetGlobalVariables(projectName, envName string, production bool, logger *zap.SugaredLogger) ([]*commontypes.GlobalVariableKV, error) {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       projectName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		logger.Errorf("failed to find env from db, project:%s env:%s", projectName, envName)
		return nil, e.ErrGetEnv.AddErr(fmt.Errorf("failed to find env from db, project:%s env:%s", projectName, envName))
	}
	if env.Production != production {
		logger.Errorf("env:%s is invalid, the env production field is: %v, but the request env production field is: %v", envName, env.Production, production)
		return nil, fmt.Errorf("invalid environment:%s", envName)
	}
	resp, _, err := GetGlobalVariables(projectName, envName, production, logger)
	if err != nil {
		logger.Errorf("failed to get global variables for project:%s", projectName)
		return nil, err
	}
	return resp, nil
}

func OpenAPIUpdateYamlService(req *OpenAPIServiceVariablesReq, userName, requestID, projectName, envName string, production bool, logger *zap.SugaredLogger) error {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: projectName, EnvName: envName})
	if err != nil {
		logger.Errorf("failed to find env:%s from db, project:%s", envName, projectName)
		return e.ErrNotFound.AddDesc(err.Error())
	}
	if env.Production != production {
		logger.Errorf("env:%s is invalid, the env production field is: %v, but the request env production field is: %v", envName, env.Production, production)
		return fmt.Errorf("env:%s is invalid, the env production field is: %v, but the request env production field is: %v", envName, env.Production, production)
	}

	args := make([]*UpdateEnv, 0)
	arg := &UpdateEnv{
		EnvName:  envName,
		Services: make([]*UpdateServiceArg, 0),
	}

	for _, serv := range req.ServiceList {
		// check and set global variable to service variable
		err = setGlobalVariableToServiceVariable(serv.Variables, serv.ServiceName, projectName, envName, production, logger)
		if err != nil {
			return e.ErrUpdateEnv.AddDesc(err.Error())
		}

		// fill in the details of service variables
		serv.Variables, err = fillServiceVariableAttribute(serv.Variables, serv.ServiceName, projectName, env, production, false, logger)
		if err != nil {
			return e.ErrUpdateEnv.AddDesc(err.Error())
		}

		serviceArg := &UpdateServiceArg{
			ServiceName:    serv.ServiceName,
			DeployStrategy: setting.ServiceDeployStrategyDeploy,
			VariableKVs:    serv.Variables,
		}
		arg.Services = append(arg.Services, serviceArg)
	}
	args = append(args, arg)

	if len(args) == 0 {
		return nil
	}
	_, err = UpdateMultipleK8sEnv(args, []string{envName}, projectName, requestID, true, production, userName, logger)
	return err
}

func OpenAPIApplyYamlService(projectKey string, req *OpenAPIApplyYamlServiceReq, production bool, requestID, userName string, logger *zap.SugaredLogger) ([]*EnvStatus, error) {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: projectKey, EnvName: req.EnvName})
	if err != nil {
		logger.Errorf("failed to find env:%s from db, project:%s", req.EnvName, projectKey)
		return nil, e.ErrNotFound.AddDesc(err.Error())
	}
	if env.Production != production {
		err := fmt.Errorf("environment %s is invalid, the env production field is: %v, but the request env production field is: %v", req.EnvName, env.Production, production)
		logger.Errorf(err.Error())
		return nil, err
	}

	if err := checkServiceInEnv(env.Services, req.ServiceList); err != nil {
		logger.Errorf(err.Error())
		return nil, err
	}

	args := make([]*UpdateEnv, 0)
	svcList := make([]*UpdateServiceArg, 0)
	for _, service := range req.ServiceList {
		err := setGlobalVariableToServiceVariable(service.VariableKvs, service.ServiceName, projectKey, req.EnvName, production, logger)
		if err != nil {
			return nil, e.ErrUpdateService.AddErr(err)
		}

		service.VariableKvs, err = fillServiceVariableAttribute(service.VariableKvs, service.ServiceName, projectKey, env, production, true, logger)
		if err != nil {
			return nil, e.ErrUpdateService.AddErr(err)
		}

		svcList = append(svcList, &UpdateServiceArg{
			ServiceName:    service.ServiceName,
			DeployStrategy: setting.ServiceDeployStrategyDeploy,
			VariableKVs:    service.VariableKvs,
		})
	}
	args = append(args, &UpdateEnv{
		EnvName:  req.EnvName,
		Services: svcList,
	})

	return UpdateMultipleK8sEnv(args, []string{req.EnvName}, projectKey, requestID, false, false, userName, logger)
}

func checkServiceInEnv(envServices [][]*commonmodels.ProductService, services []*YamlServiceWithKV) error {
	for _, servs := range envServices {
		for _, serv := range servs {
			for _, service := range services {
				if serv.ServiceName == service.ServiceName {
					return fmt.Errorf("service %s already exist in env, cannot repeatedly add the same service to the environment", service.ServiceName)
				}
			}
		}
	}
	return nil
}

func setGlobalVariableToServiceVariable(variables []*commontypes.RenderVariableKV, serviceName, projectName, envName string, production bool, logger *zap.SugaredLogger) error {
	// check the variable can set to be global
	envGlobalKvs, _, err := GetGlobalVariables(projectName, envName, production, logger)
	if err != nil {
		logger.Errorf("failed to get env global variables, projectName:%s envName:%s", projectName, envName)
		return err
	}

	for _, variable := range variables {
		if variable.UseGlobalVariable {
			if !checkVariableInEnvGlobalVariables(envGlobalKvs, variable.Key) {
				variable.UseGlobalVariable = false
			} else {
				for _, vb := range envGlobalKvs {
					if vb.Key == variable.Key {
						variable.Value = vb.Value
					}
				}
			}
		}
	}
	return nil
}

func fillServiceVariableAttribute(variablesFromUser []*commontypes.RenderVariableKV, serviceName, projectName string, env *commonmodels.Product, production, isApply bool, logger *zap.SugaredLogger) ([]*commontypes.RenderVariableKV, error) {
	var currentVariables []*commontypes.RenderVariableKV
	if env == nil || isApply {
		if variablesFromUser == nil {
			variablesFromUser = make([]*commontypes.RenderVariableKV, 0)
		}
		var service *commonmodels.Service
		var err error
		if !production {
			service, err = commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
				ProductName: projectName,
				ServiceName: serviceName,
			})

		} else {
			service, err = commonrepo.NewProductionServiceColl().Find(&commonrepo.ServiceFindOption{
				ProductName: projectName,
				ServiceName: serviceName,
			})
		}
		if err != nil {
			logger.Errorf("failed to find service:%s, project:%s", serviceName, projectName)
			return nil, e.ErrUpdateEnv.AddDesc(fmt.Errorf("service %s not found from db", serviceName).Error())
		}

		for _, vb := range service.ServiceVariableKVs {
			currentVariables = append(currentVariables, &commontypes.RenderVariableKV{
				ServiceVariableKV: commontypes.ServiceVariableKV{
					Key:     vb.Key,
					Value:   vb.Value,
					Type:    vb.Type,
					Desc:    vb.Desc,
					Options: vb.Options,
				}})
		}
	} else {
		currentVariables = env.GetSvcRender(serviceName).OverrideYaml.RenderVariableKVs
	}

	for _, vbFromUser := range variablesFromUser {
		for _, vbFromDB := range currentVariables {
			if vbFromDB.Key == vbFromUser.Key {
				vbFromUser.Type = vbFromDB.Type
				vbFromUser.Desc = vbFromDB.Desc
				vbFromUser.Options = vbFromDB.Options
				break
			}
		}
	}

	keys := make([]string, 0)
	for _, vb := range variablesFromUser {
		keys = append(keys, vb.Key)
	}

	for _, vb := range currentVariables {
		if !utils.Contains(keys, vb.Key) {
			variablesFromUser = append(variablesFromUser, &commontypes.RenderVariableKV{
				ServiceVariableKV: commontypes.ServiceVariableKV{
					Key:     vb.Key,
					Value:   vb.Value,
					Options: vb.Options,
					Desc:    vb.Desc,
					Type:    vb.Type,
				},
			})
		}
	}
	return variablesFromUser, nil
}

func OpenAPIListEnvs(userID, projectName string, envNames []string, production bool, log *zap.SugaredLogger) ([]*OpenAPIListEnvBrief, error) {
	envs, err := ListProducts(userID, projectName, envNames, production, log)
	if err != nil {
		return nil, err
	}

	resp := make([]*OpenAPIListEnvBrief, 0)
	for _, env := range envs {
		resp = append(resp, &OpenAPIListEnvBrief{
			Production: env.Production,
			EnvName:    env.Name,
			Alias:      env.Alias,
			Status:     strings.ToLower(env.Status),
			ClusterID:  env.ClusterID,
			Namespace:  env.Namespace,
			RegistryID: env.RegistryID,
			UpdateBy:   env.UpdateBy,
			UpdateTime: env.UpdateTime,
		})
	}
	return resp, nil
}

func OpenAPIListProductionEnvs(userId string, projectName string, envNames []string, log *zap.SugaredLogger) ([]*OpenAPIListEnvBrief, error) {
	envs, err := ListProductionEnvs(userId, projectName, envNames, log)
	if err != nil {
		return nil, err
	}

	resp := make([]*OpenAPIListEnvBrief, 0)
	for _, env := range envs {
		resp = append(resp, &OpenAPIListEnvBrief{
			Production: env.Production,
			EnvName:    env.Name,
			Alias:      env.Alias,
			Status:     strings.ToLower(env.Status),
			ClusterID:  env.ClusterID,
			Namespace:  env.Namespace,
			RegistryID: env.RegistryID,
			UpdateBy:   env.UpdateBy,
			UpdateTime: env.UpdateTime,
		})
	}
	return resp, nil
}

func OpenAPICreateCommonEnvCfg(projectName string, args *OpenAPIEnvCfgArgs, userName string, logger *zap.SugaredLogger) error {
	arg := &commonmodels.CreateUpdateCommonEnvCfgArgs{
		ProductName:      projectName,
		EnvName:          args.EnvName,
		Name:             args.Name,
		YamlData:         args.YamlData,
		CommonEnvCfgType: args.CommonEnvCfgType,
	}

	return CreateCommonEnvCfg(arg, userName, logger)
}

func OpenAPIListCommonEnvCfg(projectName, envName string, cfgType string, production bool, logger *zap.SugaredLogger) ([]*OpenAPIEnvCfgBrief, error) {
	resp := make([]*OpenAPIEnvCfgBrief, 0)
	switch cfgType {
	case string(config.CommonEnvCfgTypeIngress):
		ingress, err := ListIngresses(envName, projectName, production, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to list ingress: %s", err)
			return nil, err
		}

		for _, in := range ingress {
			opts := &commonrepo.QueryEnvResourceOption{
				ProductName: projectName,
				EnvName:     envName,
				Name:        in.Name,
			}
			brief := &OpenAPIEnvCfgBrief{
				ProjectName:      projectName,
				EnvName:          envName,
				Name:             in.Name,
				CommonEnvCfgType: in.Type,
				UpdateBy:         in.UpdateUserName,
				UpdateTime:       in.CreateTime.Unix(),
			}
			if brief.UpdateTime == 0 || brief.UpdateBy == "" {
				res, err := commonrepo.NewEnvResourceColl().Find(opts)
				if err != nil {
					msg := fmt.Errorf("OpenAPI: failed to find ingress from db, project:%s, env:%s, name:%s, type:%s, error:%v", projectName, envName, in.Name, in.Type, err).Error()
					logger.Errorf(msg)
				} else {
					brief.UpdateTime = res.CreateTime
					brief.UpdateBy = res.UpdateUserName
				}
			}
			resp = append(resp, brief)
		}
	case string(config.CommonEnvCfgTypeConfigMap):
		args := &ListConfigMapArgs{
			EnvName:     envName,
			ProductName: projectName,
			Production:  production,
		}

		configMapList, err := ListConfigMaps(args, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to list configmap: %s", err)
			return nil, err
		}
		for _, configMap := range configMapList {
			opts := &commonrepo.QueryEnvResourceOption{
				ProductName: projectName,
				EnvName:     envName,
				Name:        configMap.Name,
				Type:        string(configMap.Type),
			}
			brief := &OpenAPIEnvCfgBrief{
				Name:             configMap.Name,
				CommonEnvCfgType: configMap.Type,
				EnvName:          envName,
				ProjectName:      projectName,
				UpdateTime:       configMap.CreateTime.Unix(),
				UpdateBy:         configMap.UpdateUserName,
			}

			if brief.UpdateTime == 0 || brief.UpdateBy == "" {
				res, err := commonrepo.NewEnvResourceColl().Find(opts)
				if err != nil {
					msg := fmt.Errorf("OpenAPI: failed to find ingress from db, project:%s, env:%s, name:%s, type:%s, error:%v", projectName, envName, configMap.Name, configMap.Type, err).Error()
					logger.Errorf(msg)
				} else {
					brief.UpdateBy = res.UpdateUserName
					brief.UpdateTime = res.CreateTime
				}
			}
			resp = append(resp, brief)
		}
		return resp, nil
	case string(config.CommonEnvCfgTypeSecret):
		secretList, err := ListSecrets(envName, projectName, production, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to list secrets: %s", err)
			return nil, err
		}
		for _, secret := range secretList {
			opts := &commonrepo.QueryEnvResourceOption{
				ProductName: projectName,
				EnvName:     envName,
				Name:        secret.Name,
				Type:        string(secret.Type),
			}
			brief := &OpenAPIEnvCfgBrief{
				Name:             secret.Name,
				CommonEnvCfgType: secret.Type,
				EnvName:          envName,
				ProjectName:      projectName,
				UpdateTime:       secret.CreateTime.Unix(),
				UpdateBy:         secret.UpdateUserName,
			}

			if brief.UpdateTime == 0 || brief.UpdateBy == "" {
				res, err := commonrepo.NewEnvResourceColl().Find(opts)
				if err != nil {
					msg := fmt.Errorf("OpenAPI: failed to find ingress from db, project:%s, env:%s, name:%s, type:%s, error:%v", projectName, envName, secret.Name, secret.Type, err).Error()
					logger.Errorf(msg)
				} else {
					brief.UpdateBy = res.UpdateUserName
					brief.UpdateTime = res.CreateTime
				}
			}
			resp = append(resp, brief)
		}
		return resp, nil
	case string(config.CommonEnvCfgTypePvc):
		pvcList, err := ListPvcs(envName, projectName, production, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to list pvcs: %s", err)
			return nil, err
		}
		for _, pvc := range pvcList {
			opts := &commonrepo.QueryEnvResourceOption{
				ProductName: projectName,
				EnvName:     envName,
				Name:        pvc.Name,
				Type:        string(pvc.Type),
			}
			brief := &OpenAPIEnvCfgBrief{
				Name:             pvc.Name,
				CommonEnvCfgType: pvc.Type,
				EnvName:          envName,
				ProjectName:      projectName,
				UpdateTime:       pvc.CreateTime.Unix(),
				UpdateBy:         pvc.UpdateUserName,
			}
			if brief.UpdateTime == 0 || brief.UpdateBy == "" {
				res, err := commonrepo.NewEnvResourceColl().Find(opts)
				if err != nil {
					msg := fmt.Errorf("OpenAPI: failed to find ingress from db, project:%s, env:%s, name:%s, type:%s, error:%v", projectName, envName, pvc.Name, pvc.Type, err).Error()
					logger.Errorf(msg)
				} else {
					brief.UpdateBy = res.UpdateUserName
					brief.UpdateTime = res.CreateTime
				}
			}
			resp = append(resp, brief)
		}
		return resp, nil
	default:
		return nil, fmt.Errorf("invalid common env cfg type: %s", cfgType)
	}
	return resp, nil
}

func OpenAPIGetCommonEnvCfg(projectName, envName, cfgType, name string, production bool, logger *zap.SugaredLogger) (*OpenAPIEnvCfgDetail, error) {
	switch cfgType {
	case string(config.CommonEnvCfgTypeIngress):
		ingress, err := ListIngresses(envName, projectName, production, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to list ingress: %s", err)
			return nil, err
		}
		for _, in := range ingress {
			if in.Name == name {
				var source *CfgRepoInfo
				if in.SourceDetail != nil && in.SourceDetail.GitRepoConfig != nil && in.SourceDetail.GitRepoConfig.CodehostID != 0 {
					codeSource, err := systemconfig.New().GetCodeHost(in.SourceDetail.GitRepoConfig.CodehostID)
					if err != nil {
						return nil, fmt.Errorf("failed to get codehost, err: %v", err)
					}
					source = &CfgRepoInfo{
						LoadPath: in.SourceDetail.LoadPath,
						GitRepoConfig: &GitRepoConfig{
							Owner:       in.SourceDetail.GitRepoConfig.Owner,
							Repo:        in.SourceDetail.GitRepoConfig.Repo,
							Branch:      in.SourceDetail.GitRepoConfig.Branch,
							ValuesPaths: in.SourceDetail.GitRepoConfig.ValuesPaths,
							CodehostKey: codeSource.Alias,
						},
					}
				}

				return &OpenAPIEnvCfgDetail{
					IngressDetail: &OpenAPIEnvCfgIngressDetail{
						Name:             in.Name,
						CommonEnvCfgType: in.Type,
						EnvName:          envName,
						ProjectName:      projectName,
						CreatedTime:      in.CreateTime.Unix(),
						UpdateBy:         in.UpdateUserName,
						SourceDetail:     source,
						YamlData:         in.YamlData,
						HostInfo:         in.HostInfo,
						Address:          in.Address,
						Ports:            in.Ports,
						ErrorReason:      in.ErrorReason,
					},
				}, err
			}
		}
	case string(config.CommonEnvCfgTypePvc):
		pvc, err := ListPvcs(envName, projectName, production, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to list pvc: %s", err)
			return nil, err
		}
		for _, p := range pvc {
			if p.Name == name {
				var source *CfgRepoInfo
				if p.SourceDetail != nil && p.SourceDetail.GitRepoConfig != nil && p.SourceDetail.GitRepoConfig.CodehostID != 0 {
					codeSource, err := systemconfig.New().GetCodeHost(p.SourceDetail.GitRepoConfig.CodehostID)
					if err != nil {
						return nil, fmt.Errorf("failed to get codehost, err: %v", err)
					}
					source = &CfgRepoInfo{
						LoadPath: p.SourceDetail.LoadPath,
						GitRepoConfig: &GitRepoConfig{
							Owner:       p.SourceDetail.GitRepoConfig.Owner,
							Repo:        p.SourceDetail.GitRepoConfig.Repo,
							Branch:      p.SourceDetail.GitRepoConfig.Branch,
							ValuesPaths: p.SourceDetail.GitRepoConfig.ValuesPaths,
							CodehostKey: codeSource.Alias,
						},
					}
				}

				return &OpenAPIEnvCfgDetail{
					PvcDetail: &OpenAPIEnvCfgPvcDetail{
						Name:             p.Name,
						CommonEnvCfgType: p.Type,
						EnvName:          envName,
						ProjectName:      projectName,
						CreatedTime:      p.CreateTime.Unix(),
						UpdateBy:         p.UpdateUserName,
						Services:         p.Services,
						SourceDetail:     source,
						YamlData:         p.YamlData,
						Status:           p.Status,
						Volume:           p.Volume,
						AccessModes:      p.AccessModes,
						StorageClass:     p.StorageClass,
						Capacity:         p.Capacity,
					}}, nil
			}
		}
	case string(config.CommonEnvCfgTypeSecret):
		secret, err := ListSecrets(envName, projectName, production, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to list secret: %s", err)
			return nil, err
		}
		for _, s := range secret {
			if s.Name == name {
				var source *CfgRepoInfo
				if s.SourceDetail != nil && s.SourceDetail.GitRepoConfig != nil && s.SourceDetail.GitRepoConfig.CodehostID != 0 {
					codeSource, err := systemconfig.New().GetCodeHost(s.SourceDetail.GitRepoConfig.CodehostID)
					if err != nil {
						return nil, fmt.Errorf("failed to get codehost, err: %v", err)
					}
					source = &CfgRepoInfo{
						LoadPath: s.SourceDetail.LoadPath,
						GitRepoConfig: &GitRepoConfig{
							Owner:       s.SourceDetail.GitRepoConfig.Owner,
							Repo:        s.SourceDetail.GitRepoConfig.Repo,
							Branch:      s.SourceDetail.GitRepoConfig.Branch,
							ValuesPaths: s.SourceDetail.GitRepoConfig.ValuesPaths,
							CodehostKey: codeSource.Alias,
						},
					}
				}

				return &OpenAPIEnvCfgDetail{
					SecretDetail: &OpenAPIEnvCfgSecretDetail{
						Name:             s.Name,
						CommonEnvCfgType: s.Type,
						EnvName:          envName,
						ProjectName:      projectName,
						CreatedTime:      s.CreateTime.Unix(),
						UpdateBy:         s.UpdateUserName,
						Services:         s.Services,
						SourceDetail:     source,
						YamlData:         s.YamlData,
					}}, nil
			}
		}
	case string(config.CommonEnvCfgTypeConfigMap):
		args := &ListConfigMapArgs{
			EnvName:     envName,
			ProductName: projectName,
			Production:  production,
		}

		configMap, err := ListConfigMaps(args, logger)
		if err != nil {
			logger.Errorf("OpenAPI: failed to list configmap: %s", err)
			return nil, err
		}
		for _, c := range configMap {
			if c.Name == name {
				var source *CfgRepoInfo
				if c.SourceDetail != nil && c.SourceDetail.GitRepoConfig != nil && c.SourceDetail.GitRepoConfig.CodehostID != 0 {
					codeSource, err := systemconfig.New().GetCodeHost(c.SourceDetail.GitRepoConfig.CodehostID)
					if err != nil {
						return nil, fmt.Errorf("failed to get codehost, err: %v", err)
					}
					source = &CfgRepoInfo{
						LoadPath: c.SourceDetail.LoadPath,
						GitRepoConfig: &GitRepoConfig{
							Owner:       c.SourceDetail.GitRepoConfig.Owner,
							Repo:        c.SourceDetail.GitRepoConfig.Repo,
							Branch:      c.SourceDetail.GitRepoConfig.Branch,
							ValuesPaths: c.SourceDetail.GitRepoConfig.ValuesPaths,
							CodehostKey: codeSource.Alias,
						},
					}
				}
				return &OpenAPIEnvCfgDetail{
					ConfigMapDetail: &OpenAPIEnvCfgConfigMapDetail{
						Name:             c.Name,
						CommonEnvCfgType: c.Type,
						EnvName:          envName,
						ProjectName:      projectName,
						CreatedTime:      c.CreateTime.Unix(),
						UpdateBy:         c.UpdateUserName,
						Services:         c.Services,
						SourceDetail:     source,
						YamlData:         c.YamlData,
					}}, nil
			}
		}
	default:
		return nil, fmt.Errorf("invalid common env cfg type: %s", cfgType)
	}
	return nil, fmt.Errorf("not found")
}

func OpenAPIDeleteCommonEnvCfg(projectName, envName, cfgType, name string, logger *zap.SugaredLogger) error {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: projectName, EnvName: envName})
	if err != nil {
		logger.Errorf("failed to find product %s: %v", projectName, err)
		return e.ErrDeleteEnv.AddDesc(fmt.Errorf("failed to find product %s: %v", projectName, err).Error())
	}
	if env.Production {
		logger.Errorf("environment %s is production environment", envName)
		return e.ErrDeleteEnv.AddDesc(fmt.Errorf("environment %s is production environment, cannot delete it", envName).Error())
	}
	return DeleteCommonEnvCfg(envName, projectName, name, config.CommonEnvCfgType(cfgType), false, logger)
}

func OpenAPIDeleteProductionEnvCommonEnvCfg(projectName, envName, cfgType, name string, logger *zap.SugaredLogger) error {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: projectName, EnvName: envName})
	if err != nil {
		logger.Errorf("failed to find product %s: %v", projectName, err)
		return e.ErrDeleteEnv.AddDesc(fmt.Errorf("failed to find product %s: %v", projectName, err).Error())
	}
	if !env.Production {
		logger.Errorf("environment %s is not production environment", envName)
		return e.ErrDeleteEnv.AddDesc(fmt.Errorf("environment %s is not production environment, cannot delete it", envName).Error())
	}
	return DeleteCommonEnvCfg(envName, projectName, name, config.CommonEnvCfgType(cfgType), true, logger)
}

func OpenAPICreateHelmEnv(ctx *internalhandler.Context, args *OpenAPICreateHelmEnvArgs) error {
	cluster, err := commonrepo.NewK8SClusterColl().FindByID(args.ClusterID)
	if err != nil {
		err = fmt.Errorf("failed to find cluster %s: %v", args.ClusterID, err)
		ctx.Logger.Error(err)
		return e.ErrCreateEnv.AddErr(err)
	}

	shareEnv := commonmodels.ProductShareEnv{}
	if args.SubEnv != nil && args.SubEnv.Enable {
		shareEnv.Enable = args.SubEnv.Enable
		shareEnv.BaseEnv = args.SubEnv.BaseEnv
	}

	createArgs := []*CreateSingleProductArg{
		{
			EnvName:     args.EnvName,
			ClusterID:   cluster.ID.Hex(),
			Namespace:   args.Namespace,
			RegistryID:  args.RegistryID,
			Production:  args.Production,
			Alias:       args.Alias,
			ProductName: args.ProjectKey,
			ShareEnv:    shareEnv,
		},
	}
	return CreateHelmProduct(args.ProjectKey, ctx.UserName, ctx.RequestID, createArgs, ctx.Logger)
}

func OpenAPICreateK8sEnv(args *OpenAPICreateEnvArgs, userName, requestID string, logger *zap.SugaredLogger) error {
	product, err := templaterepo.NewProductColl().Find(args.ProjectName)
	if err != nil {
		logger.Errorf("failed to find product %s: %v", args.ProjectName, err)
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	if product.ProductFeature.DeployType != setting.K8SDeployType {
		return e.ErrCreateEnv.AddDesc("only support k8s type")
	}

	projectGlobalVariables := product.GlobalVariables
	globalVariableMap := make(map[string]*commontypes.GlobalVariableKV, 0)
	for _, vb := range args.GlobalVariables {
		globalVariableMap[vb.Key] = vb
	}

	// fill service variable attributes
	services := make([]*ProductK8sServiceCreationInfo, 0)
	for _, s := range args.Services {
		for _, vb := range s.VariableKVs {
			if checkVariableInProjectGlobalVariables(projectGlobalVariables, vb.Key) && vb.UseGlobalVariable {
				if _, ok := globalVariableMap[vb.Key]; ok {
					if globalVariableMap[vb.Key].RelatedServices == nil {
						globalVariableMap[vb.Key].RelatedServices = make([]string, 0)
					}
					globalVariableMap[vb.Key].RelatedServices = append(globalVariableMap[vb.Key].RelatedServices, s.ServiceName)
					vb.Value = globalVariableMap[vb.Key].Value
				}
			}
		}

		service, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
			ServiceName: s.ServiceName,
			ProductName: args.ProjectName,
		})
		if err != nil {
			logger.Errorf("failed to find service from db, serviceName:%s, projectName:%s error: %v", s.ServiceName, args.ProjectName, err)
			return e.ErrCreateEnv.AddDesc(fmt.Errorf("failed to find service from db, serviceName:%s, projectName:%s error: %v", s.ServiceName, args.ProjectName, err).Error())
		}

		s.VariableKVs, err = fillServiceVariableAttribute(s.VariableKVs, s.ServiceName, args.ProjectName, nil, false, true, logger)
		if err != nil {
			return e.ErrCreateEnv.AddDesc(err.Error())
		}

		variableYaml, err := commontypes.RenderVariableKVToYaml(s.VariableKVs, true)
		if err != nil {
			logger.Errorf("failed to render variable: %v", err)
			return e.ErrCreateEnv.AddErr(fmt.Errorf("failed to render variable: %v", err))
		}

		serv := &ProductK8sServiceCreationInfo{
			ProductService: &commonmodels.ProductService{},
			DeployStrategy: setting.ServiceDeployStrategyDeploy,
		}
		serv.ServiceName = s.ServiceName
		serv.ProductName = args.ProjectName
		serv.Type = product.ProductFeature.DeployType
		serv.Revision = service.Revision
		serv.VariableYaml = variableYaml
		serv.VariableKVs = s.VariableKVs
		serv.Containers = s.Containers

		services = append(services, serv)
	}

	// update global variables
	for _, globalVb := range projectGlobalVariables {
		for _, vb := range args.GlobalVariables {
			if globalVb.Key == vb.Key {
				vb.Type = globalVb.Type
				vb.Desc = globalVb.Desc
				vb.Options = globalVb.Options
			}
		}
	}

	// create env args
	createArg := &CreateSingleProductArg{
		ProductName: args.ProjectName,
		EnvName:     args.EnvName,
		Namespace:   args.Namespace,
		ClusterID:   args.ClusterID,
		RegistryID:  args.RegistryID,
		Production:  false,
	}
	createArg.Services = make([][]*ProductK8sServiceCreationInfo, 0)
	createArg.Services = append(createArg.Services, services)
	createArg.GlobalVariables = args.GlobalVariables
	createArg.EnvConfigs = make([]*commonmodels.CreateUpdateCommonEnvCfgArgs, 0)
	for _, cfg := range args.EnvConfigs {
		createArg.EnvConfigs = append(createArg.EnvConfigs, &commonmodels.CreateUpdateCommonEnvCfgArgs{
			Name:          cfg.Name,
			AutoSync:      cfg.AutoSync,
			YamlData:      cfg.YamlData,
			GitRepoConfig: cfg.GitRepoConfig,
		})
	}

	if args.SubEnv != nil && args.SubEnv.Enable {
		createArg.ShareEnv = commonmodels.ProductShareEnv{
			Enable:  args.SubEnv.Enable,
			BaseEnv: args.SubEnv.BaseEnv,
		}
	}

	createArgs := make([]*CreateSingleProductArg, 0)
	createArgs = append(createArgs, createArg)

	return CreateYamlProduct(args.ProjectName, userName, requestID, createArgs, logger)
}

func checkVariableInProjectGlobalVariables(variables []*commontypes.ServiceVariableKV, key string) bool {
	for _, vb := range variables {
		if vb.Key == key {
			return true
		}
	}
	return false
}

func checkVariableInEnvGlobalVariables(variables []*commontypes.GlobalVariableKV, key string) bool {
	for _, vb := range variables {
		if vb.Key == key {
			return true
		}
	}
	return false
}

func OpenAPICreateProductionEnv(args *OpenAPICreateEnvArgs, userName, requestID string, logger *zap.SugaredLogger) error {
	product, err := templaterepo.NewProductColl().Find(args.ProjectName)
	if err != nil {
		return e.ErrCreateEnv.AddDesc(err.Error())
	}
	if product.ProductFeature.DeployType != setting.K8SDeployType {
		return e.ErrCreateEnv.AddDesc("only support k8s type")
	}

	istioGrayscale := commonmodels.IstioGrayscale{}
	if args.SubEnv != nil && args.SubEnv.Enable {
		istioGrayscale.Enable = args.SubEnv.Enable
		istioGrayscale.BaseEnv = args.SubEnv.BaseEnv
	}

	createArgs := make([]*CreateSingleProductArg, 0)
	createArgs = append(createArgs, &CreateSingleProductArg{
		ProductName:    args.ProjectName,
		EnvName:        args.EnvName,
		Production:     true,
		Namespace:      args.Namespace,
		ClusterID:      args.ClusterID,
		RegistryID:     args.RegistryID,
		Alias:          args.Alias,
		IstioGrayscale: istioGrayscale,
		Services:       nil,
		EnvConfigs:     nil,
		ChartValues:    nil,
	})

	err = EnsureProductionNamespace(createArgs)
	if err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}

	return CreateYamlProduct(args.ProjectName, userName, requestID, createArgs, logger)
}

func OpenAPIUpdateCommonEnvCfg(projectName string, args *OpenAPIEnvCfgArgs, userName string, logger *zap.SugaredLogger) error {
	configArgs := &commonmodels.CreateUpdateCommonEnvCfgArgs{
		EnvName:              args.EnvName,
		ProductName:          projectName,
		ServiceName:          args.ServiceName,
		Name:                 args.Name,
		YamlData:             args.YamlData,
		RestartAssociatedSvc: true,
		CommonEnvCfgType:     args.CommonEnvCfgType,
	}
	return UpdateCommonEnvCfg(configArgs, userName, false, logger)
}
