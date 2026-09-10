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
	"strings"
	"time"

	"go.uber.org/zap"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	helmservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/helm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	codehostmodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	codehostrepo "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/util"
)

type OpenAPIHelmServiceValues struct {
	ReleaseName         string                  `json:"release_name"`
	Revision            int64                   `json:"revision"`
	ValuesYAML          string                  `json:"values_yaml"`
	EffectiveValuesYAML string                  `json:"effective_values_yaml"`
	OverrideKVs         []*commonservice.KVPair `json:"override_kvs"`
}

type OpenAPIHelmValuesSource struct {
	Configured   bool   `json:"configured"`
	CodehostName string `json:"codehost_name,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	Repo         string `json:"repo,omitempty"`
	Branch       string `json:"branch,omitempty"`
	ValuePath    string `json:"value_path,omitempty"`
	AutoSync     *bool  `json:"auto_sync,omitempty"`
}

type OpenAPIUpdateHelmValuesSourceReq struct {
	CodehostName string `json:"codehost_name"`
	Namespace    string `json:"namespace"`
	Repo         string `json:"repo"`
	Branch       string `json:"branch"`
	ValuePath    string `json:"value_path"`
	AutoSync     bool   `json:"auto_sync"`
}

func (r *OpenAPIUpdateHelmValuesSourceReq) Validate() error {
	if strings.TrimSpace(r.CodehostName) == "" {
		return fmt.Errorf("codehost_name cannot be empty")
	}
	if strings.TrimSpace(r.Namespace) == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	if strings.TrimSpace(r.Repo) == "" {
		return fmt.Errorf("repo cannot be empty")
	}
	if strings.TrimSpace(r.Branch) == "" {
		return fmt.Errorf("branch cannot be empty")
	}
	if strings.TrimSpace(r.ValuePath) == "" {
		return fmt.Errorf("value_path cannot be empty")
	}
	return nil
}

type OpenAPIUpdateHelmValuesReq struct {
	ExpectedRevision      *int64                   `json:"expected_revision"`
	ValuesYAML            *string                  `json:"values_yaml,omitempty"`
	OverrideKVs           *[]*commonservice.KVPair `json:"override_kvs,omitempty"`
	SyncValuesFromSource  bool                     `json:"sync_values_from_source,omitempty"`
	UpdateServiceRevision bool                     `json:"update_service_revision,omitempty"`
	ValueMergeStrategy    string                   `json:"value_merge_strategy,omitempty"`
}

func (r *OpenAPIUpdateHelmValuesReq) Validate() error {
	if r.ExpectedRevision == nil || *r.ExpectedRevision < 0 {
		return fmt.Errorf("expected_revision must be greater than or equal to 0")
	}
	if r.ValuesYAML == nil && r.OverrideKVs == nil && !r.SyncValuesFromSource && !r.UpdateServiceRevision {
		return fmt.Errorf("no Values update was specified")
	}
	if r.ValuesYAML != nil && r.SyncValuesFromSource {
		return fmt.Errorf("values_yaml and sync_values_from_source cannot be used together")
	}
	if r.ValueMergeStrategy == "" {
		r.ValueMergeStrategy = string(config.ValueMergeStrategyOverride)
	}
	if r.ValueMergeStrategy != string(config.ValueMergeStrategyOverride) && r.ValueMergeStrategy != string(config.ValueMergeStrategyReuseValue) {
		return fmt.Errorf("value_merge_strategy must be override or reuse-values")
	}
	if r.ValueMergeStrategy == string(config.ValueMergeStrategyReuseValue) && r.ValuesYAML == nil {
		return fmt.Errorf("reuse-values requires values_yaml")
	}
	if r.OverrideKVs != nil {
		return validateOpenAPIOverrideKVs(*r.OverrideKVs)
	}
	return nil
}

type OpenAPIHelmValuesPreview struct {
	CurrentReleaseName string `json:"current_release_name"`
	LatestReleaseName  string `json:"latest_release_name"`
	CurrentValuesYAML  string `json:"current_values_yaml"`
	LatestValuesYAML   string `json:"latest_values_yaml"`
}

func GetHelmServiceValuesOpenAPI(projectKey, envName, serviceName string, production bool) (*OpenAPIHelmServiceValues, error) {
	product, productService, err := getOpenAPIHelmEnvService(projectKey, envName, serviceName, production)
	if err != nil {
		return nil, err
	}
	renderArg := new(commonservice.HelmSvcRenderArg)
	renderArg.LoadFromRenderChartModel(productService.GetServiceRender())

	templateService, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: productService.ProductName,
		ServiceName: serviceName,
		Type:        setting.HelmDeployType,
		Revision:    productService.Revision,
	}, production)
	if err != nil {
		return nil, e.ErrGetEnv.AddErr(err)
	}
	if templateService.HelmChart == nil {
		return nil, e.ErrGetEnv.AddDesc("Helm chart data is empty")
	}
	// Use the same stored configuration and image overrides as the preview.
	helmDeployService := helmservice.NewHelmDeployService()
	mergedValues, err := helmDeployService.GenMergedValues(productService, product.DefaultValues, nil)
	if err != nil {
		return nil, e.ErrGetEnv.AddErr(err)
	}
	effectiveValues, err := helmDeployService.GeneFullValues(templateService.HelmChart.ValuesYaml, mergedValues)
	if err != nil {
		return nil, e.ErrGetEnv.AddErr(err)
	}
	valuesYAML, err := commonutil.MaskSensitiveValuesYAML(renderArg.OverrideYaml)
	if err != nil {
		return nil, e.ErrGetEnv.AddDesc(fmt.Sprintf("failed to mask Values YAML: %s", err))
	}
	effectiveValuesYAML, err := commonutil.MaskSensitiveValuesYAML(effectiveValues)
	if err != nil {
		return nil, e.ErrGetEnv.AddDesc(fmt.Sprintf("failed to mask effective Values YAML: %s", err))
	}
	revision, err := getOpenAPIHelmValuesRevision(projectKey, envName, serviceName, production)
	if err != nil {
		return nil, err
	}

	return &OpenAPIHelmServiceValues{
		ReleaseName:         util.GeneReleaseName(templateService.GetReleaseNaming(), productService.ProductName, product.Namespace, envName, serviceName),
		Revision:            revision,
		ValuesYAML:          valuesYAML,
		EffectiveValuesYAML: effectiveValuesYAML,
		OverrideKVs:         MaskOpenAPIOverrideKVs(renderArg.OverrideValues),
	}, nil
}

func GetHelmValuesSourceOpenAPI(projectKey, envName, serviceName string, production bool) (*OpenAPIHelmValuesSource, error) {
	_, productService, err := getOpenAPIHelmEnvService(projectKey, envName, serviceName, production)
	if err != nil {
		return nil, err
	}
	yamlData := productService.GetServiceRender().OverrideYaml
	if yamlData == nil || yamlData.Source != setting.SourceFromGitRepo {
		return &OpenAPIHelmValuesSource{Configured: false}, nil
	}
	if yamlData.SourceDetail == nil {
		return nil, e.ErrGetEnv.AddDesc("invalid Helm Values source configuration")
	}
	sourceDetail, err := commonservice.UnMarshalSourceDetail(yamlData.SourceDetail)
	if err != nil || sourceDetail == nil || sourceDetail.GitRepoConfig == nil {
		return nil, e.ErrGetEnv.AddDesc("invalid Helm Values source configuration")
	}
	codehost, err := codehostrepo.NewCodehostColl().GetCodeHostByID(sourceDetail.GitRepoConfig.CodehostID, true)
	if err != nil {
		return nil, e.ErrGetEnv.AddErr(fmt.Errorf("failed to find codehost: %w", err))
	}
	autoSync := yamlData.AutoSync
	namespace := sourceDetail.GitRepoConfig.Namespace
	if namespace == "" {
		namespace = sourceDetail.GitRepoConfig.Owner
	}
	return &OpenAPIHelmValuesSource{
		Configured:   true,
		CodehostName: codehost.Alias,
		Namespace:    namespace,
		Repo:         sourceDetail.GitRepoConfig.Repo,
		Branch:       sourceDetail.GitRepoConfig.Branch,
		ValuePath:    sourceDetail.LoadPath,
		AutoSync:     &autoSync,
	}, nil
}

func UpdateHelmValuesSourceOpenAPI(projectKey, envName, serviceName, userName string, production bool, req *OpenAPIUpdateHelmValuesSourceReq) error {
	codehosts, err := codehostrepo.NewCodehostColl().AvailableCodeHost(projectKey)
	if err != nil {
		return e.ErrInvalidParam.AddDesc(fmt.Sprintf("failed to get codehost by name %s: %s", req.CodehostName, err))
	}
	var codehost *codehostmodels.CodeHost
	for _, candidate := range codehosts {
		if candidate.Alias != req.CodehostName {
			continue
		}
		if codehost != nil {
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("codehost_name %q matches multiple codehosts available to project %s", req.CodehostName, projectKey))
		}
		codehost = candidate
	}
	if codehost == nil {
		return e.ErrInvalidParam.AddDesc(fmt.Sprintf("codehost %q is not available to project %s", req.CodehostName, projectKey))
	}
	return UpdateHelmValuesSource(projectKey, envName, serviceName, userName, production, false, &UpdateHelmValuesSourceArgs{ValuesData: &commonservice.ValuesDataArgs{
		YamlSource: setting.SourceFromGitRepo,
		AutoSync:   req.AutoSync && codehost.Type != setting.SourceFromOther,
		GitRepoConfig: &commonservice.RepoConfig{
			CodehostID:  codehost.ID,
			Owner:       req.Namespace,
			Namespace:   req.Namespace,
			Repo:        req.Repo,
			Branch:      req.Branch,
			ValuesPaths: []string{req.ValuePath},
		},
	}})
}

func PreviewHelmServiceValuesOpenAPI(projectKey, envName, serviceName string, production bool, req *OpenAPIUpdateHelmValuesReq, logger *zap.SugaredLogger) (*OpenAPIHelmValuesPreview, error) {
	product, productService, err := getOpenAPIHelmEnvService(projectKey, envName, serviceName, production)
	if err != nil {
		return nil, err
	}
	if err := checkOpenAPIHelmValuesRevision(projectKey, envName, serviceName, production, *req.ExpectedRevision); err != nil {
		return nil, err
	}
	renderArg, err := buildOpenAPIHelmRenderArg(productService, req)
	if err != nil {
		return nil, err
	}
	// buildOpenAPIHelmRenderArg already applied the requested merge strategy;
	// pass the complete target override to avoid merging a second time.
	estimated, err := GenEstimatedValues(projectKey, envName, product.Namespace, serviceName, EstimateValuesSceneUpdateService, EstimateContentTypeValues, EstimateValuesResponseFormatYaml, &EstimateValuesArg{
		OverrideYaml:   renderArg.OverrideYaml,
		OverrideValues: renderArg.OverrideValues,
		Production:     production,
	}, req.UpdateServiceRevision, production, false, config.ValueMergeStrategyOverride, logger)
	if err != nil {
		return nil, e.ErrUpdateEnv.AddErr(err)
	}
	current, err := commonutil.MaskSensitiveValuesYAML(estimated.Current)
	if err != nil {
		return nil, e.ErrUpdateEnv.AddErr(err)
	}
	latest, err := commonutil.MaskSensitiveValuesYAML(estimated.Latest)
	if err != nil {
		return nil, e.ErrUpdateEnv.AddErr(err)
	}
	return &OpenAPIHelmValuesPreview{
		CurrentReleaseName: estimated.CurrentReleaseName,
		LatestReleaseName:  estimated.LatestReleaseName,
		CurrentValuesYAML:  current,
		LatestValuesYAML:   latest,
	}, nil
}

func UpdateHelmServiceValuesOpenAPI(projectKey, envName, serviceName, userName, requestID string, production bool, req *OpenAPIUpdateHelmValuesReq, logger *zap.SugaredLogger) error {
	// Serialize environment updates through source loading and setting updating.
	// The existing updating status guards the subsequent asynchronous deployment.
	lock := cache.NewRedisLockWithExpiry(fmt.Sprintf("openapi_helm_values:%s:%s:%t", projectKey, envName, production), 30*time.Minute)
	if err := lock.Lock(); err != nil {
		return e.ErrUpdateEnv.AddErr(fmt.Errorf("failed to acquire Helm Values lock: %w", err))
	}
	defer lock.Unlock()

	product, productService, err := getOpenAPIHelmEnvService(projectKey, envName, serviceName, production)
	if err != nil {
		return err
	}
	if product.Status == setting.ProductStatusUpdating {
		return e.NewHTTPError(409, "Conflict", fmt.Sprintf("environment %s is updating", envName))
	}
	if err := checkOpenAPIHelmValuesRevision(projectKey, envName, serviceName, production, *req.ExpectedRevision); err != nil {
		return err
	}
	renderArg, err := buildOpenAPIHelmRenderArg(productService, req)
	if err != nil {
		return err
	}
	renderArg.DeployStrategy = product.ServiceDeployStrategy[serviceName]

	return UpdateHelmProductCharts(projectKey, envName, userName, requestID, production, &EnvRendersetArg{
		DeployType:        setting.HelmDeployType,
		ChartValues:       []*commonservice.HelmSvcRenderArg{renderArg},
		UpdateServiceTmpl: req.UpdateServiceRevision,
	}, logger)
}

func getOpenAPIHelmEnvService(projectKey, envName, serviceName string, production bool) (*commonmodels.Product, *commonmodels.ProductService, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{Name: projectKey, EnvName: envName, Production: &production})
	if err != nil {
		return nil, nil, e.ErrGetEnv.AddErr(err)
	}
	productService := product.GetServiceMap()[serviceName]
	if productService == nil || productService.Type != setting.HelmDeployType {
		return nil, nil, e.ErrGetEnv.AddDesc(fmt.Sprintf("Helm service %s not found in environment %s", serviceName, envName))
	}
	return product, productService, nil
}

func buildOpenAPIHelmRenderArg(productService *commonmodels.ProductService, req *OpenAPIUpdateHelmValuesReq) (*commonservice.HelmSvcRenderArg, error) {
	render := productService.GetServiceRender()
	arg := new(commonservice.HelmSvcRenderArg)
	arg.LoadFromRenderChartModel(render)
	valuesData, err := openAPIValuesDataFromCustomYAML(render.OverrideYaml)
	if err != nil {
		return nil, e.ErrUpdateEnv.AddDesc(fmt.Sprintf("invalid Helm Values source configuration: %s", err))
	}
	arg.ValuesData = valuesData

	if req.ValuesYAML != nil {
		if render.OverrideYaml.AutoSync {
			return nil, e.ErrInvalidParam.AddDesc("values_yaml cannot be used while automatic Values synchronization is enabled")
		}
		valuesYAML, err := commonutil.RestoreMaskedSensitiveValuesYAML(arg.OverrideYaml, *req.ValuesYAML)
		if err != nil {
			return nil, e.ErrInvalidParam.AddErr(err)
		}
		if req.ValueMergeStrategy == string(config.ValueMergeStrategyReuseValue) {
			currentValues, err := helmservice.GetValuesMapFromString(arg.OverrideYaml)
			if err != nil {
				return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid current Values YAML: %s", err))
			}
			updatedValues, err := helmservice.GetValuesMapFromString(valuesYAML)
			if err != nil {
				return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid values_yaml: %s", err))
			}
			merged, err := yaml.Marshal(helmservice.MergeHelmValues(currentValues, updatedValues))
			if err != nil {
				return nil, e.ErrInvalidParam.AddErr(err)
			}
			valuesYAML = string(merged)
		}
		arg.OverrideYaml = valuesYAML
	}
	if req.SyncValuesFromSource {
		if render.OverrideYaml.Source != setting.SourceFromGitRepo {
			return nil, e.ErrInvalidParam.AddDesc("Git Values source is not configured")
		}
		repoConfig := arg.ValuesData.GitRepoConfig
		valuesYAML, err := GetMergedYamlContent(&YamlContentRequestArg{
			CodehostID: repoConfig.CodehostID,
			Owner:      repoConfig.Owner,
			Namespace:  repoConfig.Namespace,
			Repo:       repoConfig.Repo,
			Branch:     repoConfig.Branch,
			ValuesPath: repoConfig.ValuesPaths[0],
		})
		if err != nil {
			return nil, e.ErrUpdateEnv.AddErr(err)
		}
		arg.OverrideYaml = valuesYAML
		arg.ValuesData.AutoSyncYaml = valuesYAML
	}
	if req.OverrideKVs != nil {
		overrideKVs, err := restoreMaskedOpenAPIOverrideKVs(arg.OverrideValues, *req.OverrideKVs)
		if err != nil {
			return nil, e.ErrInvalidParam.AddErr(err)
		}
		arg.OverrideValues = overrideKVs
	}
	if _, err := helmtool.MergeOverrideValues("", "", arg.OverrideYaml, arg.ToOverrideValueString(), nil); err != nil {
		return nil, e.ErrInvalidParam.AddDesc(fmt.Sprintf("invalid Helm Values: %s", err))
	}
	return arg, nil
}

func openAPIValuesDataFromCustomYAML(yamlData *templatemodels.CustomYaml) (*commonservice.ValuesDataArgs, error) {
	if yamlData == nil {
		return nil, nil
	}
	valuesData := &commonservice.ValuesDataArgs{YamlSource: yamlData.Source, SourceID: yamlData.SourceID, AutoSync: yamlData.AutoSync, AutoSyncYaml: yamlData.AutoSyncYaml}
	if yamlData.Source != setting.SourceFromGitRepo {
		return valuesData, nil
	}
	if yamlData.SourceDetail == nil {
		return nil, fmt.Errorf("Git repository source detail is empty")
	}
	sourceDetail, err := commonservice.UnMarshalSourceDetail(yamlData.SourceDetail)
	if err != nil || sourceDetail == nil || sourceDetail.GitRepoConfig == nil {
		return nil, fmt.Errorf("invalid Git repository source detail")
	}
	valuesData.GitRepoConfig = &commonservice.RepoConfig{
		CodehostID:  sourceDetail.GitRepoConfig.CodehostID,
		Owner:       sourceDetail.GitRepoConfig.Owner,
		Namespace:   sourceDetail.GitRepoConfig.Namespace,
		Repo:        sourceDetail.GitRepoConfig.Repo,
		Branch:      sourceDetail.GitRepoConfig.Branch,
		ValuesPaths: []string{sourceDetail.LoadPath},
	}
	valuesData.Commit = sourceDetail.Commit
	return valuesData, nil
}

func validateOpenAPIOverrideKVs(kvs []*commonservice.KVPair) error {
	keys := make(map[string]struct{}, len(kvs))
	for _, kv := range kvs {
		if kv == nil || strings.TrimSpace(kv.Key) == "" {
			return fmt.Errorf("override_kvs key cannot be empty")
		}
		if _, ok := keys[kv.Key]; ok {
			return fmt.Errorf("override_kvs key %q is duplicated", kv.Key)
		}
		keys[kv.Key] = struct{}{}
		switch kv.Value.(type) {
		case string, bool, float64:
		default:
			return fmt.Errorf("override_kvs value for %q must be a string, number, or bool", kv.Key)
		}
	}
	return nil
}

func restoreMaskedOpenAPIOverrideKVs(current, updated []*commonservice.KVPair) ([]*commonservice.KVPair, error) {
	currentByKey := make(map[string]interface{}, len(current))
	for _, kv := range current {
		if kv != nil {
			currentByKey[kv.Key] = kv.Value
		}
	}
	result := make([]*commonservice.KVPair, 0, len(updated))
	for _, kv := range updated {
		copied := &commonservice.KVPair{Key: kv.Key, Value: kv.Value}
		if commonutil.IsSensitiveValuesKey(kv.Key) && kv.Value == setting.MaskValue {
			currentValue, ok := currentByKey[kv.Key]
			if !ok {
				return nil, fmt.Errorf("masked sensitive value %q does not exist in current override_kvs", kv.Key)
			}
			copied.Value = currentValue
		}
		result = append(result, copied)
	}
	return result, nil
}

func MaskOpenAPIOverrideKVs(kvs []*commonservice.KVPair) []*commonservice.KVPair {
	result := make([]*commonservice.KVPair, 0, len(kvs))
	for _, kv := range kvs {
		if kv == nil {
			continue
		}
		value := kv.Value
		if commonutil.IsSensitiveValuesKey(kv.Key) && value != nil {
			value = setting.MaskValue
		}
		result = append(result, &commonservice.KVPair{Key: kv.Key, Value: value})
	}
	return result
}

func checkOpenAPIHelmValuesRevision(projectKey, envName, serviceName string, production bool, expected int64) error {
	current, err := getOpenAPIHelmValuesRevision(projectKey, envName, serviceName, production)
	if err != nil {
		return err
	}
	if current != expected {
		return e.NewHTTPError(409, "Conflict", fmt.Sprintf("expected_revision %d does not match current revision %d", expected, current))
	}
	return nil
}

func getOpenAPIHelmValuesRevision(projectKey, envName, serviceName string, production bool) (int64, error) {
	revision, err := commonrepo.NewEnvServiceVersionColl().GetLatestRevision(projectKey, envName, serviceName, false, production)
	if err != nil {
		return 0, e.ErrGetEnv.AddErr(err)
	}
	return revision, nil
}
