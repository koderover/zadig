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
	"time"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"go.uber.org/zap"
)

type OpenAPIHelmServiceDetail struct {
	ServiceName   string                         `json:"service_name"`
	Type          string                         `json:"type"`
	Source        string                         `json:"source"`
	SourceDetail  interface{}                    `json:"source_detail"`
	Revision      int64                          `json:"revision"`
	ChartName     string                         `json:"chart_name"`
	ChartVersion  string                         `json:"chart_version"`
	ValuesYAML    string                         `json:"values_yaml"`
	ReleaseNaming string                         `json:"release_naming"`
	Containers    []*OpenAPIHelmServiceContainer `json:"containers"`
	CreatedBy     string                         `json:"created_by"`
	CreatedTime   int64                          `json:"created_time"`
}

type OpenAPIHelmServiceContainer struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	ImageName string `json:"image_name"`
}

type OpenAPIHelmTemplateSourceDetail struct {
	TemplateName string `json:"template_name"`
	Customized   bool   `json:"customized"`
	AutoSync     bool   `json:"auto_sync"`
}

type OpenAPIHelmRepoSourceDetail struct {
	CodehostName string `json:"codehost_name"`
	Owner        string `json:"owner"`
	Namespace    string `json:"namespace"`
	Repo         string `json:"repo"`
	Branch       string `json:"branch"`
	Path         string `json:"path"`
}

type OpenAPIHelmPublicRepoSourceDetail struct {
	RepoURL string `json:"repo_url"`
	Path    string `json:"path"`
}

type OpenAPIHelmChartRepoSourceDetail struct {
	ChartRepoName string `json:"chart_repo_name"`
}

type OpenAPIUpdateHelmServiceReq struct {
	ExpectedRevision int64  `json:"expected_revision"`
	ValuesYAML       string `json:"values_yaml"`
}

func (r *OpenAPIUpdateHelmServiceReq) Validate() error {
	if r.ExpectedRevision <= 0 {
		return fmt.Errorf("expected_revision must be greater than 0")
	}
	if r.ValuesYAML == "" {
		return fmt.Errorf("values_yaml cannot be empty")
	}
	return nil
}

type OpenAPIUpdateHelmServiceResp struct {
	Revision int64 `json:"revision"`
}

func GetHelmServiceOpenAPI(projectKey, serviceName string, production bool, logger *zap.SugaredLogger) (*OpenAPIHelmServiceDetail, error) {
	svc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName:   projectKey,
		ServiceName:   serviceName,
		Type:          setting.HelmDeployType,
		ExcludeStatus: setting.ProductStatusDeleting,
	}, production)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}
	if svc.HelmChart == nil {
		return nil, e.ErrGetService.AddDesc("Helm chart data is empty")
	}

	valuesYAML, err := commonutil.MaskSensitiveValuesYAML(svc.HelmChart.ValuesYaml)
	if err != nil {
		return nil, e.ErrGetService.AddDesc(fmt.Sprintf("failed to mask Values YAML: %s", err))
	}
	containers, err := commonservice.ResolveServiceTemplateContainers(svc, production)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}
	sourceDetail, err := openAPIHelmServiceSourceDetail(svc)
	if err != nil {
		logger.Errorf("failed to build Helm service source detail, project: %s, service: %s, err: %s", projectKey, serviceName, err)
		return nil, e.ErrGetService.AddErr(err)
	}

	return &OpenAPIHelmServiceDetail{
		ServiceName:   svc.ServiceName,
		Type:          svc.Type,
		Source:        normalizeOpenAPIHelmServiceSource(svc.Source),
		SourceDetail:  sourceDetail,
		Revision:      svc.Revision,
		ChartName:     svc.HelmChart.Name,
		ChartVersion:  svc.HelmChart.Version,
		ValuesYAML:    valuesYAML,
		ReleaseNaming: svc.GetReleaseNaming(),
		Containers:    openAPIHelmServiceContainers(containers),
		CreatedBy:     svc.CreateBy,
		CreatedTime:   svc.CreateTime,
	}, nil
}

func openAPIHelmServiceContainers(containers []*commonmodels.Container) []*OpenAPIHelmServiceContainer {
	result := make([]*OpenAPIHelmServiceContainer, 0, len(containers))
	for _, container := range containers {
		if container == nil {
			continue
		}
		result = append(result, &OpenAPIHelmServiceContainer{Name: container.Name, Image: container.Image, ImageName: container.ImageName})
	}
	return result
}

func UpdateHelmServiceOpenAPI(projectKey, serviceName, userName, requestID string, production bool, req *OpenAPIUpdateHelmServiceReq, logger *zap.SugaredLogger) (*OpenAPIUpdateHelmServiceResp, error) {
	lock := cache.NewRedisLockWithExpiry(fmt.Sprintf("openapi_helm_service_update:%s:%s:%t", projectKey, serviceName, production), 30*time.Minute)
	if err := lock.Lock(); err != nil {
		return nil, e.ErrUpdateService.AddErr(fmt.Errorf("failed to acquire service update lock: %w", err))
	}
	defer lock.Unlock()

	current, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{ProductName: projectKey, ServiceName: serviceName, Type: setting.HelmDeployType, ExcludeStatus: setting.ProductStatusDeleting}, production)
	if err != nil {
		return nil, e.ErrUpdateService.AddErr(err)
	}
	if current.Source != setting.SourceFromChartTemplate && current.Source != setting.SourceFromCustomEdit {
		return nil, e.ErrInvalidParam.AddDesc("only Helm services created from a chart template can be updated")
	}
	if current.Revision != req.ExpectedRevision {
		return nil, e.NewHTTPError(409, "Conflict", fmt.Sprintf("expected_revision %d does not match current revision %d", req.ExpectedRevision, current.Revision))
	}
	if current.HelmChart == nil {
		return nil, e.ErrUpdateService.AddDesc("Helm chart data is empty")
	}
	valuesYAML, err := commonutil.RestoreMaskedSensitiveValuesYAML(current.HelmChart.ValuesYaml, req.ValuesYAML)
	if err != nil {
		return nil, e.ErrInvalidParam.AddErr(err)
	}

	err = EditFileContent(serviceName, projectKey, userName, requestID, &HelmChartEditInfo{
		FilePath:    setting.ValuesYaml,
		FileContent: valuesYAML,
		Production:  production,
	}, logger)
	if err != nil {
		return nil, err
	}

	// A no-op edit keeps its revision, and failed writes may leave gaps in the counter.
	updated, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{ProductName: projectKey, ServiceName: serviceName, Type: setting.HelmDeployType, ExcludeStatus: setting.ProductStatusDeleting}, production)
	if err != nil {
		return nil, e.ErrGetService.AddErr(err)
	}
	return &OpenAPIUpdateHelmServiceResp{Revision: updated.Revision}, nil
}

func normalizeOpenAPIHelmServiceSource(source string) string {
	switch source {
	case setting.SourceFromGithub, setting.SourceFromGitlab, setting.SourceFromOther, setting.SourceFromGitRepo:
		return string(LoadFromRepo)
	case setting.SourceFromCustomEdit, setting.SourceFromChartTemplate:
		return string(LoadFromChartTemplate)
	default:
		return source
	}
}

func openAPIHelmServiceSourceDetail(svc *commonmodels.Service) (interface{}, error) {
	switch svc.Source {
	case setting.SourceFromChartTemplate, setting.SourceFromCustomEdit:
		createFrom, err := svc.GetHelmCreateFrom()
		if err != nil {
			return nil, err
		}
		return &OpenAPIHelmTemplateSourceDetail{TemplateName: createFrom.TemplateName, Customized: svc.Source == setting.SourceFromCustomEdit, AutoSync: svc.AutoSync}, nil
	case setting.SourceFromChartRepo:
		createFrom := new(commonmodels.CreateFromChartRepo)
		if err := commonmodels.IToi(svc.CreateFrom, createFrom); err != nil {
			return nil, err
		}
		return &OpenAPIHelmChartRepoSourceDetail{ChartRepoName: createFrom.ChartRepoName}, nil
	case setting.SourceFromPublicRepo:
		createFrom := new(commonmodels.CreateFromPublicRepo)
		if err := commonmodels.IToi(svc.CreateFrom, createFrom); err != nil {
			return nil, err
		}
		return &OpenAPIHelmPublicRepoSourceDetail{RepoURL: createFrom.RepoLink, Path: createFrom.LoadPath}, nil
	case setting.SourceFromGerrit:
		codehostName, err := openAPICodehostName(svc.GerritCodeHostID)
		if err != nil {
			return nil, err
		}
		return &OpenAPIHelmRepoSourceDetail{CodehostName: codehostName, Owner: svc.RepoOwner, Namespace: svc.GetRepoNamespace(), Repo: svc.GerritRepoName, Branch: svc.GerritBranchName, Path: svc.GerritPath}, nil
	case setting.SourceFromGithub, setting.SourceFromGitlab, setting.SourceFromOther, setting.SourceFromGitRepo, setting.SourceFromGitee, setting.SourceFromGiteeEE:
		codehostName, err := openAPICodehostName(svc.CodehostID)
		if err != nil {
			return nil, err
		}
		return &OpenAPIHelmRepoSourceDetail{CodehostName: codehostName, Owner: svc.RepoOwner, Namespace: svc.GetRepoNamespace(), Repo: svc.RepoName, Branch: svc.BranchName, Path: svc.LoadPath}, nil
	default:
		return nil, fmt.Errorf("unsupported Helm service source %q", svc.Source)
	}
}

func openAPICodehostName(codehostID int) (string, error) {
	codehost, err := codehostrepo.NewCodehostColl().GetCodeHostByID(codehostID, true)
	if err != nil {
		return "", fmt.Errorf("failed to find codehost: %w", err)
	}
	return codehost.Alias, nil
}
