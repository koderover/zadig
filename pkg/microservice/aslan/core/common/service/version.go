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
	"context"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	helmservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/helm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/notify"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/jobcontroller"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	yamlutil "github.com/koderover/zadig/v2/pkg/util/yaml"
)

type ListEnvServiceVersionsResponse struct {
	ServiceName    string                    `json:"service_name"`
	Revision       int64                     `json:"revision"`
	Containers     []*commonmodels.Container `json:"containers"`
	Operation      config.EnvOperation       `json:"operation"`
	DeployType     string                    `json:"deploy_type"`
	DeployStrategy string                    `json:"deploy_strategy"`
	Detail         string                    `json:"detail"`
	CreateTime     int64                     `json:"create_time"`
	CreateBy       string                    `json:"create_by"`
}

func ListEnvServiceVersions(ctx *internalhandler.Context, projectName, envName, serviceName string, isHelmChart, isProduction bool, log *zap.SugaredLogger) ([]ListEnvServiceVersionsResponse, error) {
	resp := []ListEnvServiceVersionsResponse{}
	revisions, err := mongodb.NewEnvServiceVersionColl().ListServiceVersions(projectName, envName, serviceName, isHelmChart, isProduction)
	if err != nil {
		return nil, e.ErrListEnvServiceVersions.AddErr(fmt.Errorf("failed to list service revisions, error: %v", err))
	}

	project, err := template.NewProductColl().Find(projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get project %s", projectName)
	}

	for _, revision := range revisions {
		name := revision.Service.ServiceName
		if isHelmChart {
			name = revision.Service.ReleaseName
		}

		if project.IsHostProduct() && revision.ProductFeature == nil {
			continue
		}

		deployType := ""
		if revision.ProductFeature != nil {
			deployType = revision.ProductFeature.GetDeployType()
		}

		resp = append(resp, ListEnvServiceVersionsResponse{
			ServiceName:    name,
			Revision:       revision.Revision,
			Containers:     revision.Service.Containers,
			Operation:      revision.Operation,
			DeployType:     deployType,
			DeployStrategy: revision.DeployStrategy,
			Detail:         revision.Detail,
			CreateTime:     revision.CreateTime,
			CreateBy:       revision.CreateBy,
		})
	}
	return resp, nil
}

type GetEnvServiceVersionYamlResponse struct {
	Type           string                    `json:"type"`
	Yaml           string                    `json:"yaml"`
	VariableYaml   string                    `json:"variable_yaml"`
	OverrideKVs    string                    `json:"override_kvs"`
	Containers     []*commonmodels.Container `json:"containers"`
	Operation      config.EnvOperation       `json:"operation"`
	DeployType     string                    `json:"deploy_type"`
	DeployStrategy string                    `json:"deploy_strategy"`
	Detail         string                    `json:"detail"`
	CreateTime     int64                     `json:"create_time"`
	CreateBy       string                    `json:"create_by"`
}

func GetEnvServiceVersionYaml(ctx *internalhandler.Context, projectName, envName, serviceName string, revision int64, isHelmChart, isProduction bool, log *zap.SugaredLogger) (GetEnvServiceVersionYamlResponse, error) {
	resp := GetEnvServiceVersionYamlResponse{}

	envSvcRevision, err := mongodb.NewEnvServiceVersionColl().Find(projectName, envName, serviceName, isHelmChart, isProduction, revision)
	if err != nil {
		if mongodb.IsErrNoDocuments(err) {
			return resp, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("历史版本 %d 不存在", revision))
		}

		return resp, fmt.Errorf("failed to find %s/%s/%s service for revision %d, isHelmChart %v isProduction %v, error: %v", projectName, envName, serviceName, revision, isHelmChart, isProduction, err)
	}
	resp.Type = envSvcRevision.Service.Type
	resp.Operation = envSvcRevision.Operation
	resp.DeployType = envSvcRevision.ProductFeature.GetDeployType()
	resp.Detail = envSvcRevision.Detail
	resp.CreateTime = envSvcRevision.CreateTime
	resp.CreateBy = envSvcRevision.CreateBy
	resp.DeployStrategy = envSvcRevision.DeployStrategy
	resp.Containers = envSvcRevision.Service.Containers

	if envSvcRevision.ProductFeature.IsHostProduct() {
		return resp, nil
	}

	if envSvcRevision.Service.Type == setting.K8SDeployType {
		fakeEnv := &commonmodels.Product{
			ProductName: envSvcRevision.ProductName,
			EnvName:     envSvcRevision.EnvName,
			Namespace:   envSvcRevision.Namespace,
			Production:  envSvcRevision.Production,
		}
		parsedYaml, err := kube.RenderEnvService(fakeEnv, envSvcRevision.Service.GetServiceRender(), envSvcRevision.Service)
		if err != nil {
			err = fmt.Errorf("Failed to render env Service %s, error: %v", envSvcRevision.Service.ServiceName, err)
			return resp, err
		}
		resp.Yaml = parsedYaml
		resp.VariableYaml = envSvcRevision.Service.Render.GetOverrideYaml()
	} else if envSvcRevision.Service.Type == setting.HelmDeployType {
		tmplSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ServiceName: serviceName,
			ProductName: projectName,
			Type:        setting.HelmDeployType,
			Revision:    envSvcRevision.Service.Revision,
		}, isProduction)
		if err != nil {
			return resp, fmt.Errorf("failed to find %s/%s/%s service for version %d, isProduction %v, error: %v", projectName, envName, serviceName, revision, isProduction, err)
		}

		helmDeploySvc := helmservice.NewHelmDeployService()
		resp.VariableYaml, err = helmDeploySvc.GenMergedValues(envSvcRevision.Service, envSvcRevision.DefaultValues, nil)
		if err != nil {
			return resp, fmt.Errorf("failed to merged values for %s/%s/%s service for version %d, isProduction %v, error: %v", projectName, envName, serviceName, revision, isProduction, err)
		}
		resp.OverrideKVs = envSvcRevision.Service.GetServiceRender().OverrideValues

		resp.Yaml, err = helmDeploySvc.GeneFullValues(tmplSvc.HelmChart.ValuesYaml, resp.VariableYaml)
		if err != nil {
			return resp, fmt.Errorf("failed to generate full values for %s/%s/%s service for version %d, isProduction %v, error: %v", projectName, envName, serviceName, revision, isProduction, err)
		}
	} else if envSvcRevision.Service.Type == setting.HelmChartDeployType {
		chartRepoName := envSvcRevision.Service.GetServiceRender().ChartRepo
		chartName := envSvcRevision.Service.GetServiceRender().ChartName
		chartVersion := envSvcRevision.Service.GetServiceRender().ChartVersion
		chartRepo, err := commonrepo.NewHelmRepoColl().Find(&commonrepo.HelmRepoFindOption{RepoName: chartRepoName})
		if err != nil {
			return resp, fmt.Errorf("failed to query chart-repo info, repoName: %s", chartRepoName)
		}

		client, err := commonutil.NewHelmClient(chartRepo)
		if err != nil {
			return resp, fmt.Errorf("failed to new helm client, err %s", err)
		}

		valuesYaml, err := client.GetChartValues(commonutil.GeneHelmRepo(chartRepo), projectName, serviceName, chartRepoName, chartName, chartVersion, isProduction)
		if err != nil {
			return resp, fmt.Errorf("failed to get chart values, chartRepo: %s, chartName: %s, chartVersion: %s, err %s", chartRepoName, chartName, chartVersion, err)
		}

		helmDeploySvc := helmservice.NewHelmDeployService()
		mergedValues, err := helmDeploySvc.GenMergedValues(envSvcRevision.Service, envSvcRevision.DefaultValues, nil)
		if err != nil {
			return resp, fmt.Errorf("failed to merge values, err %s", err)
		}
		mergedValues, err = helmDeploySvc.GeneFullValues(valuesYaml, mergedValues)
		if err != nil {
			return resp, fmt.Errorf("failed to generate full values, err %s", err)
		}
		resp.VariableYaml = mergedValues
	}

	return resp, nil
}

type DiffEnvServiceVersionsResponse struct {
	Type          string                    `json:"type"`
	CreateTimeA   int64                     `json:"create_time_a"`
	CreateTimeB   int64                     `json:"create_time_b"`
	CreateByA     string                    `json:"create_by_a"`
	CreateByB     string                    `json:"create_by_b"`
	YamlA         string                    `json:"yaml_a"`
	YamlB         string                    `json:"yaml_b"`
	VariableYamlA string                    `json:"variable_yaml_a"`
	VariableYamlB string                    `json:"variable_yaml_b"`
	ContainersA   []*commonmodels.Container `json:"containers_a"`
	ContainersB   []*commonmodels.Container `json:"containers_b"`
}

func DiffEnvServiceVersions(ctx *internalhandler.Context, projectName, envName, serviceName string, revisionA, revisionB int64, isHelmChart, isProduction bool, log *zap.SugaredLogger) (DiffEnvServiceVersionsResponse, error) {
	resp := DiffEnvServiceVersionsResponse{}

	if revisionA == -1 || revisionB == -1 {
		latestRevision, err := commonrepo.NewEnvServiceVersionColl().GetLatestRevision(projectName, envName, serviceName, isHelmChart, isProduction)
		if err != nil {
			return resp, e.ErrDiffEnvServiceVersions.AddErr(fmt.Errorf("failed to get latest revision for %s/%s/%s, isHelmChart %v isProduction %v, error: %v", projectName, envName, serviceName, isHelmChart, isProduction, err))
		}
		if revisionA == -1 {
			revisionA = latestRevision
		}
		if revisionB == -1 {
			revisionB = latestRevision
		}
	}

	respA, err := GetEnvServiceVersionYaml(ctx, projectName, envName, serviceName, revisionA, isHelmChart, isProduction, log)
	if err != nil {
		return resp, e.ErrDiffEnvServiceVersions.AddErr(err)
	}
	resp.Type = respA.Type
	resp.YamlA = respA.Yaml
	resp.VariableYamlA = respA.VariableYaml
	resp.ContainersA = respA.Containers
	resp.CreateByA = respA.CreateBy
	resp.CreateTimeA = respA.CreateTime

	respB, err := GetEnvServiceVersionYaml(ctx, projectName, envName, serviceName, revisionB, isHelmChart, isProduction, log)
	if err != nil {
		return resp, e.ErrDiffEnvServiceVersions.AddErr(err)
	}
	resp.YamlB = respB.Yaml
	resp.VariableYamlB = respB.VariableYaml
	resp.ContainersB = respB.Containers
	resp.CreateByB = respB.CreateBy
	resp.CreateTimeB = respB.CreateTime

	return resp, nil
}

type RollbackEnvServiceVersionData struct {
	ReplaceResources     []commonmodels.Resource
	RelatedPodLabels     []map[string]string
	HelmDeployStatusChan chan bool
}

func RollbackEnvServiceVersion(ctx *internalhandler.Context, projectName, envName, serviceName string, revision int64, isHelmChart, isProduction bool, detail string, log *zap.SugaredLogger) (*RollbackEnvServiceVersionData, error) {
	envSvcVersion, err := mongodb.NewEnvServiceVersionColl().Find(projectName, envName, serviceName, isHelmChart, isProduction, revision)
	if err != nil {
		if mongodb.IsErrNoDocuments(err) {
			return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("历史版本 %d 不存在", revision))
		}

		return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find %s/%s/%s service for revision %d, isProduction %v, error: %v", projectName, envName, serviceName, revision, isProduction, err))
	}

	env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
		Name:       projectName,
		EnvName:    envName,
		Production: &isProduction,
	})
	if err != nil {
		return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find %s/%s env, isProduction %v, error: %v", projectName, envName, isProduction, err))
	}

	preProdSvc := env.GetServiceMap()[serviceName]
	if preProdSvc == nil {
		if envSvcVersion.Service.Type == setting.HelmChartDeployType {
			preProdSvc = env.GetChartServiceMap()[serviceName]
		} else {
			return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find service %s in env %s", envSvcVersion.Service.ServiceName, envSvcVersion.EnvName))
		}
	}

	session := mongotool.Session()
	defer session.EndSession(context.Background())

	rollbackStatus := &RollbackEnvServiceVersionData{
		HelmDeployStatusChan: make(chan bool),
	}

	if envSvcVersion.ProductFeature.IsHostProduct() {
		project, err := template.NewProductColl().Find(projectName)
		if err != nil {
			return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find %s product, error: %v", projectName, err))
		}

		if !project.ProductFeature.IsHostProduct() {
			return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("项目类型已发生变更，系统不支持回滚至旧类型生成的版本"))
		}

		kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(env.ClusterID)
		if err != nil {
			return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
		}

		clientSet, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(env.ClusterID)
		if err != nil {
			return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
		}

		option := &kube.GeneSvcYamlOption{
			ProductName:           env.ProductName,
			EnvName:               envName,
			ServiceName:           serviceName,
			UpdateServiceRevision: false,
			VariableYaml:          envSvcVersion.Service.GetServiceRender().GetOverrideYaml(),
			VariableKVs:           envSvcVersion.Service.GetServiceRender().OverrideYaml.RenderVariableKVs,
			Containers:            envSvcVersion.Service.Containers,
		}
		_, _, resources, err := kube.GenerateRenderedYaml(option)
		if err != nil {
			return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to generate service yaml, error: %v", err))
		}

		for _, rollbackContainer := range envSvcVersion.Service.Containers {
			serviceModule := &commonmodels.DeployServiceModule{
				ServiceModule: envSvcVersion.Service.ServiceName,
				Image:         rollbackContainer.Image,
				ImageName:     rollbackContainer.ImageName,
			}

			replaceResources, relatedPodLabels, err := jobcontroller.UpdateExternalServiceModule(ctx, kubeClient, clientSet, resources, env, serviceName, serviceModule, detail, ctx.UserName, log)
			if err != nil {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
			}
			rollbackStatus.ReplaceResources = append(rollbackStatus.ReplaceResources, replaceResources...)
			rollbackStatus.RelatedPodLabels = append(rollbackStatus.RelatedPodLabels, relatedPodLabels...)
		}
	} else {
		if envSvcVersion.Service.Type == setting.K8SDeployType {
			kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(env.ClusterID)
			if err != nil {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
			}

			istioClient, err := clientmanager.NewKubeClientManager().GetIstioClientSet(env.ClusterID)
			if err != nil {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
			}

			informer, err := clientmanager.NewKubeClientManager().GetInformer(env.ClusterID, env.Namespace)
			if err != nil {
				log.Errorf("[%s][%s] error: %v", envName, env.Namespace, err)
				return nil, e.ErrRollbackEnvServiceVersion.AddDesc(err.Error())
			}

			clientSet, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(env.ClusterID)
			if err != nil {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
			}

			if envSvcVersion.DeployStrategy == setting.ServiceDeployStrategyImport {
				currentDeployStrategy := env.ServiceDeployStrategy[envSvcVersion.Service.ServiceName]
				if currentDeployStrategy != setting.ServiceDeployStrategyImport {
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("服务 %s 的部署策略发生变化，不能回滚", envSvcVersion.Service.ServiceName))
				}

				option := &kube.GeneSvcYamlOption{
					ProductName:           env.ProductName,
					EnvName:               envName,
					ServiceName:           serviceName,
					UpdateServiceRevision: false,
					VariableYaml:          envSvcVersion.Service.GetServiceRender().GetOverrideYaml(),
					VariableKVs:           envSvcVersion.Service.GetServiceRender().OverrideYaml.RenderVariableKVs,
					Containers:            envSvcVersion.Service.Containers,
				}
				_, _, resources, err := kube.GenerateRenderedYaml(option)
				if err != nil {
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to generate service yaml, error: %v", err))
				}

				for _, rollbackContainer := range envSvcVersion.Service.Containers {
					serviceModule := &commonmodels.DeployServiceModule{
						ServiceModule: envSvcVersion.Service.ServiceName,
						Image:         rollbackContainer.Image,
						ImageName:     rollbackContainer.ImageName,
					}

					replaceResources, relatedPodLabels, err := jobcontroller.UpdateExternalServiceModule(ctx, kubeClient, clientSet, resources, env, serviceName, serviceModule, detail, ctx.UserName, log)
					if err != nil {
						return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
					}
					rollbackStatus.ReplaceResources = append(rollbackStatus.ReplaceResources, replaceResources...)
					rollbackStatus.RelatedPodLabels = append(rollbackStatus.RelatedPodLabels, relatedPodLabels...)
				}
			} else {
				currentDeployStrategy := env.ServiceDeployStrategy[envSvcVersion.Service.ServiceName]
				if currentDeployStrategy != "" && currentDeployStrategy != setting.ServiceDeployStrategyDeploy {
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("服务 %s 的部署策略发生变化，不能回滚", envSvcVersion.Service.ServiceName))
				}

				fakeEnv := &commonmodels.Product{
					ProductName: envSvcVersion.ProductName,
					EnvName:     envSvcVersion.EnvName,
					Namespace:   envSvcVersion.Namespace,
					Production:  envSvcVersion.Production,
				}
				parsedYaml, err := kube.RenderEnvService(fakeEnv, envSvcVersion.Service.GetServiceRender(), envSvcVersion.Service)
				if err != nil {
					err = fmt.Errorf("Failed to render env %s, service %s, revision %d, error: %v", envSvcVersion.EnvName, envSvcVersion.Service.ServiceName, envSvcVersion.Service.Revision, err)
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
				}
				envSvcVersion.Service.RenderedYaml = parsedYaml

				preResourceYaml, err := kube.RenderEnvService(env, preProdSvc.GetServiceRender(), preProdSvc)
				if err != nil {
					err = fmt.Errorf("Failed to render env %s, service %s, revision %d, error: %v", envSvcVersion.EnvName, envSvcVersion.Service.ServiceName, envSvcVersion.Service.Revision, err)
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
				}
				preProdSvc.RenderedYaml = preResourceYaml

				err = kube.CheckResourceAppliedByOtherEnv(parsedYaml, env, envSvcVersion.Service.ServiceName)
				if err != nil {
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
				}

				resourceApplyParam := &kube.ResourceApplyParam{
					ProductInfo:         env,
					ServiceName:         envSvcVersion.Service.ServiceName,
					CurrentResourceYaml: preResourceYaml,
					UpdateResourceYaml:  parsedYaml,
					Informer:            informer,
					KubeClient:          kubeClient,
					IstioClient:         istioClient,
					InjectSecrets:       true,
					Uninstall:           false,
					AddZadigLabel:       !isProduction,
					SharedEnvHandler:    kube.EnsureUpdateZadigService,
				}

				unstructuredList, err := kube.CreateOrPatchResource(resourceApplyParam, log)
				if err != nil {
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to create or patch resource for env %s, service %s, revision %d, error: %v", envSvcVersion.EnvName, envSvcVersion.Service.ServiceName, envSvcVersion.Service.Revision, err))
				}

				for _, us := range unstructuredList {
					switch us.GetKind() {
					case setting.Deployment, setting.StatefulSet:
						podLabels, _, err := unstructured.NestedStringMap(us.Object, "spec", "template", "metadata", "labels")
						if err == nil {
							rollbackStatus.RelatedPodLabels = append(rollbackStatus.RelatedPodLabels, podLabels)
						}
						rollbackStatus.ReplaceResources = append(rollbackStatus.ReplaceResources, commonmodels.Resource{Name: us.GetName(), Kind: us.GetKind()})
					case setting.CronJob, setting.Job:
						rollbackStatus.ReplaceResources = append(rollbackStatus.ReplaceResources, commonmodels.Resource{Name: us.GetName(), Kind: us.GetKind()})
					}
				}

				err = mongotool.StartTransaction(session)
				if err != nil {
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
				}

				groupIndex := -1
				svcIndex := -1
				env.LintServices()
				for i, group := range env.Services {
					for j, svc := range group {
						if svc.ServiceName == envSvcVersion.Service.ServiceName {
							svcIndex = j
							groupIndex = i
							svc.Resources = kube.UnstructuredToResources(unstructuredList)
							for _, kv := range envSvcVersion.Service.GetServiceRender().OverrideYaml.RenderVariableKVs {
								kv.UseGlobalVariable = false
							}
							break
						}
					}
				}
				if groupIndex < 0 || svcIndex < 0 {
					mongotool.AbortTransaction(session)
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find service %s in env %s/%s, isProudction %v", envSvcVersion.Service.ServiceName, envSvcVersion.ProductName, envSvcVersion.EnvName, envSvcVersion.Production))
				}

				env.Services[groupIndex][svcIndex] = envSvcVersion.Service
				err = helmservice.UpdateServiceInEnv(env, envSvcVersion.Service, ctx.UserName, config.EnvOperationRollback, detail)
				if err != nil {
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to update service %s in env %s/%s, isProudction %v", envSvcVersion.Service.ServiceName, envSvcVersion.ProductName, envSvcVersion.EnvName, envSvcVersion.Production))
				}
			}

			// create rollback record
			rollbackRecord := &commonmodels.EnvInfo{
				ProjectName:   projectName,
				EnvName:       envName,
				EnvType:       config.EnvTypeZadig,
				Production:    env.Production,
				Operation:     config.EnvOperationRollback,
				OperationType: config.EnvOperationTypeZadig,
				ServiceName:   serviceName,
				ServiceType:   envSvcVersion.Service.GetServiceType(),
				Detail:        detail,
				OriginService: preProdSvc,
				UpdateService: envSvcVersion.Service,
			}
			err = mongodb.NewEnvInfoCollWithSession(session).Create(ctx, rollbackRecord)
			if err != nil {
				mongotool.AbortTransaction(session)
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to create rollback record for env %s/%s, isProudction %v, service: %s, err: %v", envSvcVersion.ProductName, envSvcVersion.EnvName, envSvcVersion.Production, serviceName, err))
			}

			for _, globalKV := range env.GlobalVariables {
				relatedServiceSet := sets.NewString(globalKV.RelatedServices...)
				if relatedServiceSet.Has(serviceName) {
					relatedServiceSet.Delete(serviceName)
				}
				globalKV.RelatedServices = relatedServiceSet.List()
			}
			err = commonrepo.NewProductCollWithSession(session).UpdateGlobalVariable(env)
			if err != nil {
				mongotool.AbortTransaction(session)
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to update global variables in env %s/%s, isProudction %v", envSvcVersion.ProductName, envSvcVersion.EnvName, envSvcVersion.Production))
			}
			err = mongotool.CommitTransaction(session)
			if err != nil {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
			}
		} else if envSvcVersion.Service.Type == setting.HelmDeployType || envSvcVersion.Service.Type == setting.HelmChartDeployType {
			if envSvcVersion.DeployStrategy == setting.ServiceDeployStrategyImport {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("Helm 服务不能回滚至仅导入状态"))
			}

			templateProduct, err := templaterepo.NewProductColl().Find(envSvcVersion.ProductName)
			if err != nil {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find project %s, error: %v", envSvcVersion.ProductName, err))
			}

			var svcTmpl *commonmodels.Service
			if envSvcVersion.Service.Type == setting.HelmDeployType {
				svcTmpl, err = repository.QueryTemplateService(&commonrepo.ServiceFindOption{
					ProductName: envSvcVersion.ProductName,
					ServiceName: envSvcVersion.Service.ServiceName,
					Type:        envSvcVersion.Service.Type,
					Revision:    envSvcVersion.Service.Revision,
				}, env.Production)
				if err != nil {
					return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find service temlate %s/%s/%d, production %v, error: %v", envSvcVersion.ProductName, envSvcVersion.Service.ServiceName, envSvcVersion.Service.Revision, env.Production, err))
				}
			}

			mergedValuesYaml, err := yamlutil.Merge([][]byte{[]byte(envSvcVersion.DefaultValues), []byte(envSvcVersion.Service.GetServiceRender().GetOverrideYaml())})
			if err != nil {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to merge values yaml, err: %s", err))
			}

			env.DefaultValues = ""
			envSvcVersion.Service.DeployStrategy = setting.ServiceDeployStrategyDeploy
			envSvcVersion.Service.GetServiceRender().SetOverrideYaml(string(mergedValuesYaml))

			go func(done chan bool) {
				err = kube.DeploySingleHelmRelease(env, envSvcVersion.Service, svcTmpl, nil, templateProduct.ReleaseMaxHistory, 0, ctx.UserName)
				if err != nil {
					title := fmt.Sprintf("回滚 %s/%s 环境 %s 服务失败", projectName, envName, serviceName)
					notify.SendErrorMessage(ctx.UserName, title, ctx.RequestID, err, log)
					done <- false
				}
				done <- true
			}(rollbackStatus.HelmDeployStatusChan)

			rollbackRecord := &commonmodels.EnvInfo{
				ProjectName:   projectName,
				EnvName:       envName,
				EnvType:       config.EnvTypeZadig,
				Production:    env.Production,
				Operation:     config.EnvOperationRollback,
				OperationType: config.EnvOperationTypeZadig,
				ServiceName:   serviceName,
				ServiceType:   envSvcVersion.Service.GetServiceType(),
				OriginService: preProdSvc,
				UpdateService: envSvcVersion.Service,
			}
			err = mongodb.NewEnvInfoCollWithSession(session).Create(ctx, rollbackRecord)
			if err != nil {
				mongotool.AbortTransaction(session)
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to create rollback record for env %s/%s, isProudction %v, service: %s", envSvcVersion.ProductName, envSvcVersion.EnvName, envSvcVersion.Production, serviceName))
			}

			err = mongotool.CommitTransaction(session)
			if err != nil {
				return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
			}
		}
	}

	return rollbackStatus, nil
}
