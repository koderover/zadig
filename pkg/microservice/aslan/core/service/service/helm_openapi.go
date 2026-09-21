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
	"path"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	fsservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/fs"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
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
	TemplateName           string `json:"template_name"`
	ValuesEdited           bool   `json:"values_edited"`
	TemplateAutoSyncActive bool   `json:"template_auto_sync_active"`
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
		ValuesYAML:    svc.HelmChart.ValuesYaml,
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
	err := EditFileContent(serviceName, projectKey, userName, requestID, &HelmChartEditInfo{
		FilePath:         setting.ValuesYaml,
		FileContent:      req.ValuesYAML,
		Production:       production,
		expectedRevision: &req.ExpectedRevision,
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
		return &OpenAPIHelmTemplateSourceDetail{
			TemplateName:           createFrom.TemplateName,
			ValuesEdited:           svc.Source == setting.SourceFromCustomEdit,
			TemplateAutoSyncActive: svc.Source == setting.SourceFromChartTemplate && svc.AutoSync,
		}, nil
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
	codehost, err := codehostrepo.NewCodehostColl().GetCodeHostByID(codehostID, false)
	if err == mongo.ErrNoDocuments {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to find codehost: %w", err)
	}
	return codehost.Alias, nil
}

type OpenAPIHelmValuesScanPath struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type OpenAPIQueryHelmValuesReq struct {
	CodehostName string                       `json:"codehostName"`
	Owner        string                       `json:"owner"`
	Namespace    string                       `json:"namespace"`
	Repo         string                       `json:"repo"`
	Branch       string                       `json:"branch"`
	Paths        []*OpenAPIHelmValuesScanPath `json:"paths"`
}

func (r *OpenAPIQueryHelmValuesReq) Validate() error {
	if r.CodehostName == "" {
		return fmt.Errorf("codehostName cannot be empty")
	}
	if r.Owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if r.Repo == "" {
		return fmt.Errorf("repo cannot be empty")
	}
	if r.Branch == "" {
		return fmt.Errorf("branch cannot be empty")
	}
	if len(r.Paths) == 0 {
		return fmt.Errorf("paths cannot be empty")
	}
	for _, scanPath := range r.Paths {
		if scanPath == nil || scanPath.Path == "" {
			return fmt.Errorf("path cannot be empty")
		}
		if err := validateOpenAPIRepoPath(scanPath.Path); err != nil {
			return err
		}
	}
	return nil
}

type OpenAPIQueryHelmValuesResp struct {
	ValuesPaths []string `json:"valuesPaths"`
}

type OpenAPIBulkCreateHelmServiceReq struct {
	TemplateName string   `json:"templateName"`
	CodehostName string   `json:"codehostName"`
	Owner        string   `json:"owner"`
	Namespace    string   `json:"namespace"`
	Repo         string   `json:"repo"`
	Branch       string   `json:"branch"`
	ValuesPaths  []string `json:"valuesPaths"`
	AutoSync     bool     `json:"autoSync"`
}

func (r *OpenAPIBulkCreateHelmServiceReq) Validate() error {
	if r.TemplateName == "" {
		return fmt.Errorf("templateName cannot be empty")
	}
	if r.CodehostName == "" {
		return fmt.Errorf("codehostName cannot be empty")
	}
	if r.Owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if r.Repo == "" {
		return fmt.Errorf("repo cannot be empty")
	}
	if r.Branch == "" {
		return fmt.Errorf("branch cannot be empty")
	}
	if len(r.ValuesPaths) == 0 {
		return fmt.Errorf("valuesPaths cannot be empty")
	}
	for _, valuesPath := range r.ValuesPaths {
		if valuesPath == "" {
			return fmt.Errorf("values path cannot be empty")
		}
		if err := validateOpenAPIRepoPath(valuesPath); err != nil {
			return err
		}
		if !isOpenAPIHelmValuesFile(path.Base(valuesPath)) {
			return fmt.Errorf("%s is not a values file", valuesPath)
		}
		if helmServiceNameFromValuesPath(valuesPath) == "" {
			return fmt.Errorf("values path %s has an empty service name", valuesPath)
		}
	}
	return nil
}

func validateOpenAPIRepoPath(repoPath string) error {
	if strings.HasPrefix(repoPath, "/") || strings.Contains(repoPath, "..") {
		return fmt.Errorf("invalid path: %s", repoPath)
	}
	return nil
}

// QueryHelmValuesOpenAPI scans the given repo paths and returns the values files that
// can be imported as Helm services.
func QueryHelmValuesOpenAPI(projectKey string, req *OpenAPIQueryHelmValuesReq, logger *zap.SugaredLogger) (*OpenAPIQueryHelmValuesResp, error) {
	codehostID, err := openAPIAvailableCodehostID(projectKey, req.CodehostName)
	if err != nil {
		return nil, e.ErrListWorkspace.AddErr(err)
	}

	getter, err := fsservice.GetTreeGetter(codehostID)
	if err != nil {
		logger.Errorf("Failed to get tree getter of codehost %s, err: %s", req.CodehostName, err)
		return nil, e.ErrListWorkspace.AddErr(err)
	}

	owner := req.Namespace
	if owner == "" {
		owner = req.Owner
	}

	resp := &OpenAPIQueryHelmValuesResp{ValuesPaths: make([]string, 0)}
	visited := sets.NewString()
	for _, scanPath := range req.Paths {
		var valuesPaths []string
		if scanPath.IsDir {
			valuesPaths, err = listOpenAPIHelmValuesFiles(getter, owner, req.Repo, req.Branch, scanPath.Path)
		} else {
			valuesPaths, err = checkOpenAPIHelmValuesFile(getter, owner, req.Repo, req.Branch, scanPath.Path)
		}
		if err != nil {
			return nil, e.ErrListWorkspace.AddErr(err)
		}

		for _, valuesPath := range valuesPaths {
			if visited.Has(valuesPath) {
				continue
			}
			visited.Insert(valuesPath)
			resp.ValuesPaths = append(resp.ValuesPaths, valuesPath)
		}
	}

	return resp, nil
}

func listOpenAPIHelmValuesFiles(getter fsservice.TreeGetter, owner, repo, branch, dir string) ([]string, error) {
	treeNodes, err := getter.GetTree(owner, repo, dir, branch)
	if err != nil {
		return nil, err
	}

	valuesPaths := make([]string, 0, len(treeNodes))
	for _, treeNode := range treeNodes {
		if treeNode == nil {
			continue
		}
		if !treeNode.IsDir {
			if isOpenAPIHelmValuesFile(treeNode.Name) {
				valuesPaths = append(valuesPaths, treeNode.FullPath)
			}
			continue
		}

		subPaths, err := listOpenAPIHelmValuesFiles(getter, owner, repo, branch, treeNode.FullPath)
		if err != nil {
			return nil, err
		}
		valuesPaths = append(valuesPaths, subPaths...)
	}

	return valuesPaths, nil
}

func checkOpenAPIHelmValuesFile(getter fsservice.TreeGetter, owner, repo, branch, filePath string) ([]string, error) {
	if !isOpenAPIHelmValuesFile(path.Base(filePath)) {
		return nil, fmt.Errorf("%s is not a values file", filePath)
	}

	dir := path.Dir(filePath)
	if dir == "." {
		dir = ""
	}
	treeNodes, err := getter.GetTree(owner, repo, dir, branch)
	if err != nil {
		return nil, err
	}

	for _, treeNode := range treeNodes {
		if treeNode != nil && !treeNode.IsDir && treeNode.FullPath == filePath {
			return []string{filePath}, nil
		}
	}

	return nil, fmt.Errorf("values file %s is not found in repo %s branch %s", filePath, repo, branch)
}

func isOpenAPIHelmValuesFile(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

// BulkCreateHelmServicesOpenAPI creates one Helm service per selected values file,
// every service uses the same chart template.
func BulkCreateHelmServicesOpenAPI(ctx *internalhandler.Context, projectKey string, production bool, req *OpenAPIBulkCreateHelmServiceReq) (*OpenAPILoadHelmServiceResp, error) {
	codehostID, err := openAPIAvailableCodehostID(projectKey, req.CodehostName)
	if err != nil {
		return nil, e.ErrLoadServiceTemplate.AddErr(err)
	}
	getter, err := fsservice.GetTreeGetter(codehostID)
	if err != nil {
		return nil, e.ErrLoadServiceTemplate.AddErr(err)
	}
	owner := req.Namespace
	if owner == "" {
		owner = req.Owner
	}
	for _, valuesPath := range req.ValuesPaths {
		if _, err := checkOpenAPIHelmValuesFile(getter, owner, req.Repo, req.Branch, valuesPath); err != nil {
			return nil, e.ErrLoadServiceTemplate.AddErr(err)
		}
	}

	// values files sharing a name would create the same service, and the creations run
	// in parallel, so the duplicates are rejected instead of racing each other
	valuesPaths := make([]string, 0, len(req.ValuesPaths))
	conflicts := make([]*OpenAPIFailedHelmService, 0)
	createdBy := make(map[string]string, len(req.ValuesPaths))
	for _, valuesPath := range req.ValuesPaths {
		serviceName := helmServiceNameFromValuesPath(valuesPath)
		if firstPath, ok := createdBy[serviceName]; ok {
			conflicts = append(conflicts, &OpenAPIFailedHelmService{
				Path:  valuesPath,
				Error: fmt.Sprintf("service:%s is already created from values file %s", serviceName, firstPath),
			})
			continue
		}
		createdBy[serviceName] = valuesPath
		valuesPaths = append(valuesPaths, valuesPath)
	}

	args := &BulkHelmServiceCreationArgs{
		HelmLoadSource: HelmLoadSource{
			Source: LoadFromChartTemplate,
		},
		CreateFrom: &CreateFromChartTemplate{
			TemplateName: req.TemplateName,
		},
		CreatedBy:  ctx.UserName,
		RequestID:  ctx.RequestID,
		AutoSync:   req.AutoSync,
		Production: production,
		ValuesData: &commonservice.ValuesDataArgs{
			YamlSource: setting.SourceFromGitRepo,
			GitRepoConfig: &commonservice.RepoConfig{
				CodehostID:  codehostID,
				Owner:       req.Owner,
				Namespace:   req.Namespace,
				Repo:        req.Repo,
				Branch:      req.Branch,
				ValuesPaths: valuesPaths,
			},
		},
	}

	resp, err := CreateOrUpdateBulkHelmService(projectKey, args, false, ctx.Logger)
	if resp == nil {
		return nil, err
	}
	if err != nil {
		// the services are already created, only the auto deploy to envs failed, so the
		// creation result is still reported instead of being dropped
		ctx.Logger.Errorf("Failed to auto deploy Helm services to envs of project %s, err: %s", projectKey, err)
	}

	openAPIResp := &OpenAPILoadHelmServiceResp{
		SuccessServices: resp.SuccessServices,
		FailedServices:  conflicts,
	}
	for _, failedService := range resp.FailedServices {
		openAPIResp.FailedServices = append(openAPIResp.FailedServices, &OpenAPIFailedHelmService{
			Path:  failedService.Path,
			Error: failedService.Error,
		})
	}

	return openAPIResp, nil
}

// openAPIAvailableCodehostID resolves a codehost alias within a project, covering both
// system level and project level integrations.
func openAPIAvailableCodehostID(projectKey, codehostName string) (int, error) {
	codehosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(projectKey)
	if err != nil {
		return 0, fmt.Errorf("failed to list available codehosts of project %s: %s", projectKey, err)
	}

	codehostID := 0
	for _, codehost := range codehosts {
		if codehost.Alias != codehostName {
			continue
		}
		if codehostID != 0 {
			return 0, fmt.Errorf("multiple codehosts named %s are available in project %s", codehostName, projectKey)
		}
		codehostID = codehost.ID
	}
	if codehostID == 0 {
		return 0, fmt.Errorf("codehost %s is not available in project %s", codehostName, projectKey)
	}

	return codehostID, nil
}
