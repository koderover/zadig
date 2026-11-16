/*
Copyright 2026 The KodeRover Authors.

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

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// ResolveEnvServiceIsHelmChart reports whether serviceName refers to a chart-only release in the env.
// Zadig services are keyed by service name and chart-only releases by release name, so the same name
// may match both; the caller must then specify isHelmChart explicitly.
func ResolveEnvServiceIsHelmChart(projectKey, envName, serviceName string, production bool) (bool, error) {
	env, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       projectKey,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		return false, e.ErrGetEnv.AddErr(fmt.Errorf("failed to find env %s/%s, production %v: %w", projectKey, envName, production, err))
	}
	_, isService := env.GetServiceMap()[serviceName]
	_, isChart := env.GetChartServiceMap()[serviceName]
	if isService && isChart {
		return false, e.ErrInvalidParam.AddDesc(fmt.Sprintf("%s matches both a service and a chart release in env %s, specify isHelmChart", serviceName, envName))
	}
	return isChart, nil
}

// OpenAPIEnvServiceContainer is a container image recorded in an environment service version.
type OpenAPIEnvServiceContainer struct {
	Name      string                          `json:"name"`
	Type      string                          `json:"type"`
	Image     string                          `json:"image"`
	ImageName string                          `json:"image_name,omitempty"`
	ImagePath *OpenAPIEnvServiceImagePathSpec `json:"image_path,omitempty"`
}

// OpenAPIEnvServiceImagePathSpec locates the image fields in Helm values.
type OpenAPIEnvServiceImagePathSpec struct {
	Repo      string `json:"repo,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Image     string `json:"image,omitempty"`
	Tag       string `json:"tag,omitempty"`
}

type OpenAPIEnvServiceVersion struct {
	ServiceName    string                        `json:"service_name"`
	Revision       int64                         `json:"revision"`
	Containers     []*OpenAPIEnvServiceContainer `json:"containers"`
	Operation      string                        `json:"operation"`
	DeployType     string                        `json:"deploy_type"`
	DeployStrategy string                        `json:"deploy_strategy"`
	Detail         string                        `json:"detail"`
	CreateTime     int64                         `json:"create_time"`
	CreateBy       string                        `json:"create_by"`
}

type OpenAPIEnvServiceVersionDetail struct {
	Type           string                        `json:"type"`
	Yaml           string                        `json:"yaml"`
	VariableYaml   string                        `json:"variable_yaml"`
	OverrideKVs    string                        `json:"override_kvs"`
	Containers     []*OpenAPIEnvServiceContainer `json:"containers"`
	Operation      string                        `json:"operation"`
	DeployType     string                        `json:"deploy_type"`
	DeployStrategy string                        `json:"deploy_strategy"`
	ChartRepo      string                        `json:"chart_repo"`
	ChartName      string                        `json:"chart_name"`
	ChartVersion   string                        `json:"chart_version"`
	Detail         string                        `json:"detail"`
	CreateTime     int64                         `json:"create_time"`
	CreateBy       string                        `json:"create_by"`
}

type OpenAPIEnvServiceVersionDiff struct {
	Type          string                        `json:"type"`
	CreateTimeA   int64                         `json:"create_time_a"`
	CreateTimeB   int64                         `json:"create_time_b"`
	CreateByA     string                        `json:"create_by_a"`
	CreateByB     string                        `json:"create_by_b"`
	YamlA         string                        `json:"yaml_a"`
	YamlB         string                        `json:"yaml_b"`
	VariableYamlA string                        `json:"variable_yaml_a"`
	VariableYamlB string                        `json:"variable_yaml_b"`
	ContainersA   []*OpenAPIEnvServiceContainer `json:"containers_a"`
	ContainersB   []*OpenAPIEnvServiceContainer `json:"containers_b"`
}

type OpenAPIEnvServiceRollbackCheck struct {
	Available         bool     `json:"available"`
	UnavailableImages []string `json:"unavailable_images"`
}

func OpenAPIListEnvServiceVersions(ctx *internalhandler.Context, projectKey, envName, serviceName string, isHelmChart, production bool, log *zap.SugaredLogger) ([]*OpenAPIEnvServiceVersion, error) {
	versions, err := commonservice.ListEnvServiceVersions(ctx, projectKey, envName, serviceName, isHelmChart, production, log)
	if err != nil {
		return nil, err
	}
	resp := make([]*OpenAPIEnvServiceVersion, 0, len(versions))
	for _, version := range versions {
		resp = append(resp, &OpenAPIEnvServiceVersion{
			ServiceName:    version.ServiceName,
			Revision:       version.Revision,
			Containers:     toOpenAPIEnvServiceContainers(version.Containers),
			Operation:      string(version.Operation),
			DeployType:     version.DeployType,
			DeployStrategy: string(version.DeployStrategy),
			Detail:         version.Detail,
			CreateTime:     version.CreateTime,
			CreateBy:       version.CreateBy,
		})
	}
	return resp, nil
}

func OpenAPIGetEnvServiceVersion(ctx *internalhandler.Context, projectKey, envName, serviceName string, revision int64, isHelmChart, production bool, log *zap.SugaredLogger) (*OpenAPIEnvServiceVersionDetail, error) {
	version, err := commonservice.GetEnvServiceVersionYaml(ctx, projectKey, envName, serviceName, revision, isHelmChart, production, log)
	if err != nil {
		return nil, err
	}
	return &OpenAPIEnvServiceVersionDetail{
		Type:           version.Type,
		Yaml:           version.Yaml,
		VariableYaml:   version.VariableYaml,
		OverrideKVs:    version.OverrideKVs,
		Containers:     toOpenAPIEnvServiceContainers(version.Containers),
		Operation:      string(version.Operation),
		DeployType:     version.DeployType,
		DeployStrategy: string(version.DeployStrategy),
		ChartRepo:      version.ChartRepo,
		ChartName:      version.ChartName,
		ChartVersion:   version.ChartVersion,
		Detail:         version.Detail,
		CreateTime:     version.CreateTime,
		CreateBy:       version.CreateBy,
	}, nil
}

func OpenAPIDiffEnvServiceVersions(ctx *internalhandler.Context, projectKey, envName, serviceName string, revisionA, revisionB int64, isHelmChart, production bool, log *zap.SugaredLogger) (*OpenAPIEnvServiceVersionDiff, error) {
	diff, err := commonservice.DiffEnvServiceVersions(ctx, projectKey, envName, serviceName, revisionA, revisionB, isHelmChart, production, log)
	if err != nil {
		return nil, err
	}
	return &OpenAPIEnvServiceVersionDiff{
		Type:          diff.Type,
		CreateTimeA:   diff.CreateTimeA,
		CreateTimeB:   diff.CreateTimeB,
		CreateByA:     diff.CreateByA,
		CreateByB:     diff.CreateByB,
		YamlA:         diff.YamlA,
		YamlB:         diff.YamlB,
		VariableYamlA: diff.VariableYamlA,
		VariableYamlB: diff.VariableYamlB,
		ContainersA:   toOpenAPIEnvServiceContainers(diff.ContainersA),
		ContainersB:   toOpenAPIEnvServiceContainers(diff.ContainersB),
	}, nil
}

func OpenAPICheckRollbackEnvServiceVersion(projectKey, envName, serviceName string, revision int64, isHelmChart, production bool, log *zap.SugaredLogger) (*OpenAPIEnvServiceRollbackCheck, error) {
	check, err := commonservice.CheckRollbackEnvServiceVersion(projectKey, envName, serviceName, revision, isHelmChart, production, log)
	if err != nil {
		return nil, err
	}
	return &OpenAPIEnvServiceRollbackCheck{
		Available:         check.Available,
		UnavailableImages: check.UnavailableImages,
	}, nil
}

func toOpenAPIEnvServiceContainers(containers []*commonmodels.Container) []*OpenAPIEnvServiceContainer {
	if containers == nil {
		return nil
	}
	resp := make([]*OpenAPIEnvServiceContainer, 0, len(containers))
	for _, container := range containers {
		if container == nil {
			continue
		}
		item := &OpenAPIEnvServiceContainer{
			Name:      container.Name,
			Type:      string(container.Type),
			Image:     container.Image,
			ImageName: container.ImageName,
		}
		if container.ImagePath != nil {
			item.ImagePath = &OpenAPIEnvServiceImagePathSpec{
				Repo:      container.ImagePath.Repo,
				Namespace: container.ImagePath.Namespace,
				Image:     container.ImagePath.Image,
				Tag:       container.ImagePath.Tag,
			}
		}
		resp = append(resp, item)
	}
	return resp
}
