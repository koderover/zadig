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

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	commonutil "github.com/koderover/zadig/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/helmclient"
	helmtool "github.com/koderover/zadig/pkg/tool/helmclient"
	"github.com/koderover/zadig/pkg/tool/kube/informer"
	"github.com/koderover/zadig/pkg/util/converter"
)

type ListEnvServiceVersionsResponse struct {
	ServiceName string `json:"service_name"`
	Revision    int64  `json:"revision"`
	CreateTime  int64  `json:"create_time"`
	CreateBy    string `json:"create_by"`
}

func ListEnvServiceVersions(ctx *internalhandler.Context, projectName, envName, serviceName string, isHelmChart, isProduction bool, log *zap.SugaredLogger) ([]ListEnvServiceVersionsResponse, error) {
	resp := []ListEnvServiceVersionsResponse{}
	revisions, err := mongodb.NewEnvServiceVersionColl().ListServiceVersions(projectName, envName, serviceName, isHelmChart, isProduction)
	if err != nil {
		return nil, e.ErrListEnvServiceVersions.AddErr(fmt.Errorf("failed to list service revisions, error: %v", err))
	}

	for _, revision := range revisions {
		name := revision.Service.ServiceName
		if isHelmChart {
			name = revision.Service.ReleaseName
		}
		resp = append(resp, ListEnvServiceVersionsResponse{
			ServiceName: name,
			Revision:    revision.Revision,
			CreateTime:  revision.CreateTime,
			CreateBy:    revision.CreateBy,
		})
	}
	return resp, nil
}

type GetEnvServiceVersionYamlResponse struct {
	Type         string `json:"type"`
	Yaml         string `json:"yaml"`
	VariableYaml string `json:"variable_yaml"`
}

func GetEnvServiceVersionYaml(ctx *internalhandler.Context, projectName, envName, serviceName string, revision int64, isHelmChart, isProduction bool, log *zap.SugaredLogger) (GetEnvServiceVersionYamlResponse, error) {
	resp := GetEnvServiceVersionYamlResponse{}

	envSvcRevision, err := mongodb.NewEnvServiceVersionColl().Find(projectName, envName, serviceName, isHelmChart, isProduction, revision)
	if err != nil {
		return resp, e.ErrDiffEnvServiceVersions.AddErr(fmt.Errorf("failed to find %s/%s/%s service for revision %d, isProduction %v, error: %v", projectName, envName, serviceName, revision, isProduction, err))
	}
	resp.Type = envSvcRevision.Service.Type
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
			return resp, e.ErrDiffEnvServiceVersions.AddErr(err)
		}
		resp.Yaml = parsedYaml
		resp.VariableYaml = envSvcRevision.Service.VariableYaml
	} else if envSvcRevision.Service.Type == setting.HelmDeployType || envSvcRevision.Service.Type == setting.HelmChartDeployType {
		resp.VariableYaml, err = commonutil.GeneHelmMergedValues(envSvcRevision.Service, envSvcRevision.DefaultValues, envSvcRevision.Service.GetServiceRender())
		if err != nil {
			return resp, e.ErrDiffEnvServiceVersions.AddErr(fmt.Errorf("failed to get helm merged values for %s/%s/%s service for version %d, isProduction %v, error: %v", projectName, envName, serviceName, revision, isProduction, err))
		}
	}

	return resp, nil
}

type DiffEnvServiceVersionsResponse struct {
	Type          string `json:"type"`
	YamlA         string `json:"yaml_a"`
	YamlB         string `json:"yaml_b"`
	VariableYamlA string `json:"variable_yaml_a"`
	VariableYamlB string `json:"variable_yaml_b"`
}

func DiffEnvServiceVersions(ctx *internalhandler.Context, projectName, envName, serviceName string, revisionA, revisionB int64, isHelmChart, isProduction bool, log *zap.SugaredLogger) (DiffEnvServiceVersionsResponse, error) {
	resp := DiffEnvServiceVersionsResponse{}

	respA, err := GetEnvServiceVersionYaml(ctx, projectName, envName, serviceName, revisionA, isHelmChart, isProduction, log)
	if err != nil {
		return resp, err
	}
	resp.Type = respA.Type
	resp.YamlA = respA.Yaml
	resp.VariableYamlA = respA.VariableYaml

	respB, err := GetEnvServiceVersionYaml(ctx, projectName, envName, serviceName, revisionB, isHelmChart, isProduction, log)
	if err != nil {
		return resp, err
	}
	resp.YamlB = respB.Yaml
	resp.VariableYamlB = respB.VariableYaml

	return resp, nil
}

func RollbackEnvServiceVersion(ctx *internalhandler.Context, projectName, envName, serviceName string, revision int64, isHelmChart, isProduction bool, log *zap.SugaredLogger) error {
	envSvcVersion, err := mongodb.NewEnvServiceVersionColl().Find(projectName, envName, serviceName, isHelmChart, isProduction, revision)
	if err != nil {
		return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find %s/%s/%s service for revision %d, isProduction %v, error: %v", projectName, envName, serviceName, revision, isProduction, err))
	}

	env, err := mongodb.NewProductColl().Find(&mongodb.ProductFindOptions{
		Name:       projectName,
		EnvName:    envName,
		Production: &isProduction,
	})
	if err != nil {
		return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find %s/%s env, isProduction %v, error: %v", projectName, envName, isProduction, err))
	}

	if envSvcVersion.Service.Type == setting.K8SDeployType {
		kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), env.ClusterID)
		if err != nil {
			return e.ErrRollbackEnvServiceVersion.AddErr(err)
		}

		restConfig, err := kubeclient.GetRESTConfig(config.HubServerAddress(), env.ClusterID)
		if err != nil {
			return e.ErrRollbackEnvServiceVersion.AddErr(err)
		}

		istioClient, err := versionedclient.NewForConfig(restConfig)
		if err != nil {
			return e.ErrRollbackEnvServiceVersion.AddErr(err)
		}

		cls, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), env.ClusterID)
		if err != nil {
			log.Errorf("[%s][%s] error: %v", envName, env.Namespace, err)
			return e.ErrRollbackEnvServiceVersion.AddDesc(err.Error())

		}
		informer, err := informer.NewInformer(env.ClusterID, env.Namespace, cls)
		if err != nil {
			log.Errorf("[%s][%s] error: %v", envName, env.Namespace, err)
			return e.ErrRollbackEnvServiceVersion.AddDesc(err.Error())
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
			return e.ErrRollbackEnvServiceVersion.AddErr(err)
		}

		preProdSvc := env.GetServiceMap()[envSvcVersion.Service.ServiceName]
		if preProdSvc == nil {
			return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find service %s in env %s", envSvcVersion.Service.ServiceName, envSvcVersion.EnvName))
		}
		preResourceYaml, err := kube.RenderEnvService(env, preProdSvc.GetServiceRender(), preProdSvc)
		if err != nil {
			err = fmt.Errorf("Failed to render env %s, service %s, revision %d, error: %v", envSvcVersion.EnvName, envSvcVersion.Service.ServiceName, envSvcVersion.Service.Revision, err)
			return e.ErrRollbackEnvServiceVersion.AddErr(err)
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
			SharedEnvHandler:    EnsureUpdateZadigService,
		}

		_, err = kube.CreateOrPatchResource(resourceApplyParam, log)
		if err != nil {
			return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to create or patch resource for env %s, service %s, revision %d, error: %v", envSvcVersion.EnvName, envSvcVersion.Service.ServiceName, envSvcVersion.Service.Revision, err))
		}

		err = commonutil.CreateEnvServiceVersion(env, envSvcVersion.Service, ctx.UserName, log)
		if err != nil {
			log.Errorf("failed to create env service version for service %s/%s, error: %v", envSvcVersion.EnvName, envSvcVersion.Service.ServiceName, err)
		}
	} else if envSvcVersion.Service.Type == setting.HelmDeployType || envSvcVersion.Service.Type == setting.HelmChartDeployType {
		svcTmpl, err := mongodb.NewServiceColl().Find(&mongodb.ServiceFindOption{
			ProductName: envSvcVersion.ProductName,
			ServiceName: envSvcVersion.Service.ServiceName,
			Type:        envSvcVersion.Service.Type,
			Revision:    envSvcVersion.Service.Revision,
		})
		if err != nil {
			return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to find service temlate %s/%s/%d, error: %v", envSvcVersion.EnvName, envSvcVersion.Service.ServiceName, envSvcVersion.Service.Revision, err))
		}

		if env.DefaultValues != "" {
			mergedValues, err := helmtool.MergeOverrideValues("", envSvcVersion.DefaultValues, envSvcVersion.Service.GetServiceRender().GetOverrideYaml(), envSvcVersion.Service.GetServiceRender().OverrideValues, nil)
			if err != nil {
				return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to merge service %s's override yaml %s and values %s, err: %s", envSvcVersion.Service.ServiceName, envSvcVersion.Service.GetServiceRender().GetOverrideYaml(), envSvcVersion.Service.GetServiceRender().OverrideValues, err))
			}

			mergedValuesFlatMap, err := converter.YamlToFlatMap([]byte(mergedValues))
			if err != nil {
				return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to convert mergedSvcValues to flatMap, err: %s", err))
			}
			defaultValuesFlatMap, err := converter.YamlToFlatMap([]byte(env.DefaultValues))
			if err != nil {
				return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to convert defaultValues to flatMap, err: %s", err))
			}

			needToAddValuesFlatMap := make(map[string]interface{})
			for defaultKey, defaultValue := range defaultValuesFlatMap {
				if _, ok := mergedValuesFlatMap[defaultKey]; !ok {
					needToAddValuesFlatMap[defaultKey] = defaultValue
				}
			}

			if envSvcVersion.Service.Type == setting.HelmDeployType {
				svcTmplValuesFlatMap, err := converter.YamlToFlatMap([]byte(svcTmpl.HelmChart.ValuesYaml))
				if err != nil {
					return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to convert template service %s's values yaml to flatMap, err: %v", svcTmpl.ServiceName, err))
				}
				for key, value := range needToAddValuesFlatMap {
					if v, ok := svcTmplValuesFlatMap[key]; !ok {
						mergedValuesFlatMap[key] = v
					} else {
						mergedValuesFlatMap[key] = value
					}
				}
			} else if envSvcVersion.Service.Type == setting.HelmChartDeployType {
				chartRepoName := envSvcVersion.Service.GetServiceRender().ChartRepo
				chartName := envSvcVersion.Service.GetServiceRender().ChartName
				chartVersion := envSvcVersion.Service.GetServiceRender().ChartVersion
				chartRepo, err := commonrepo.NewHelmRepoColl().Find(&commonrepo.HelmRepoFindOption{RepoName: chartRepoName})
				if err != nil {
					return fmt.Errorf("failed to query chart-repo info, productName: %s, repoName: %s", env.ProductName, chartRepoName)
				}

				hClient, err := helmclient.NewClient()
				if err != nil {
					return err
				}

				valuesYaml, err := hClient.GetChartValues(commonutil.GeneHelmRepo(chartRepo), env.ProductName, serviceName, chartRepoName, chartName, chartVersion)
				if err != nil {
					return fmt.Errorf("failed to get chart values, chartRepo: %s, chartName: %s, chartVersion: %s, err %s", chartRepoName, chartName, chartVersion, err)
				}
				valuesYamlFlatMap, err := converter.YamlToFlatMap([]byte(valuesYaml))
				if err != nil {
					return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to convert mergedSvcValues to flatMap, err: %s", err))
				}

				for key, value := range needToAddValuesFlatMap {
					if v, ok := valuesYamlFlatMap[key]; !ok {
						mergedValuesFlatMap[key] = v
					} else {
						mergedValuesFlatMap[key] = value
					}
				}
			}

			mergedValuesByte, err := yaml.Marshal(mergedValuesFlatMap)
			if err != nil {
				return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to mashal mergedValuesFlatMap, err: %s", err))
			}

			envSvcVersion.Service.GetServiceRender().OverrideYaml.YamlContent = string(mergedValuesByte)
			envSvcVersion.Service.GetServiceRender().OverrideValues = ""
		} else {
			mergedValues, err := helmtool.MergeOverrideValues("", envSvcVersion.DefaultValues, envSvcVersion.Service.GetServiceRender().GetOverrideYaml(), envSvcVersion.Service.GetServiceRender().OverrideValues, nil)
			if err != nil {
				return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to merge service %s's override yaml %s and values %s, err: %s", envSvcVersion.Service.ServiceName, envSvcVersion.Service.GetServiceRender().GetOverrideYaml(), envSvcVersion.Service.GetServiceRender().OverrideValues, err))
			}

			envSvcVersion.Service.GetServiceRender().OverrideYaml.YamlContent = mergedValues
			envSvcVersion.Service.GetServiceRender().OverrideValues = ""
		}

		err = kube.UpgradeHelmRelease(env, envSvcVersion.Service, svcTmpl, nil, 0, ctx.UserName)
		if err != nil {
			return e.ErrRollbackEnvServiceVersion.AddErr(fmt.Errorf("failed to upgrade helm release for env %s, service %s, revision %d, error: %v", envSvcVersion.EnvName, envSvcVersion.Service.ServiceName, envSvcVersion.Service.Revision, err))
		}
	}

	return nil
}
