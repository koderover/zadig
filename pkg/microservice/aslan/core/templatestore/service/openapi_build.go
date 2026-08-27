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
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	openapitool "github.com/koderover/zadig/v2/pkg/tool/openapi"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

type OpenAPIBuildTemplateInput struct {
	Name            string                           `json:"name"`
	Infrastructure  string                           `json:"infrastructure"`
	VMLabels        []string                         `json:"vm_labels"`
	BuildOS         string                           `json:"build_os"`
	Installs        []*types.OpenAPIToolItem         `json:"installs"`
	Parameters      []*OpenAPIBuildTemplateParameter `json:"parameters"`
	ScriptType      types.ScriptType                 `json:"script_type"`
	BuildScript     string                           `json:"build_script"`
	DockerBuildStep *types.OpenAPIDockerBuildStep    `json:"docker_build_step"`
	AdvancedSetting *types.OpenAPIAdvancedSetting    `json:"advanced_settings"`
}

type OpenAPIBuildTemplateParameter struct {
	Key          string                     `json:"key"`
	Type         types.ParameterSettingType `json:"type"`
	DefaultValue string                     `json:"default_value"`
	ChoiceOption []string                   `json:"choice_option"`
	ChoiceValue  []string                   `json:"choice_value"`
	IsCredential bool                       `json:"is_credential"`
	Description  string                     `json:"description"`
	Required     bool                       `json:"required"`
}

type OpenAPIBuildTemplateDetail struct {
	ID string `json:"id"`
	OpenAPIBuildTemplateInput
	UpdateTime int64  `json:"update_time"`
	UpdateBy   string `json:"update_by"`
}

type OpenAPIBuildTemplateBrief struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Infrastructure string `json:"infrastructure"`
	UpdateTime     int64  `json:"update_time"`
	UpdateBy       string `json:"update_by"`
}

type OpenAPIBuildTemplateListResp struct {
	Total          int                          `json:"total"`
	BuildTemplates []*OpenAPIBuildTemplateBrief `json:"build_templates"`
}

func (req OpenAPIBuildTemplateInput) Validate() error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if req.Infrastructure != setting.JobK8sInfrastructure && req.Infrastructure != setting.JobVMInfrastructure {
		return fmt.Errorf("infrastructure must be kubernetes or vm")
	}
	if req.Infrastructure == setting.JobVMInfrastructure && len(req.VMLabels) == 0 {
		return fmt.Errorf("vm_labels cannot be empty for vm")
	}
	for i, label := range req.VMLabels {
		if strings.TrimSpace(label) == "" {
			return fmt.Errorf("vm_labels[%d] cannot be empty", i)
		}
	}
	if strings.TrimSpace(req.BuildOS) == "" {
		return fmt.Errorf("build_os cannot be empty")
	}
	if req.ScriptType != types.ScriptTypeShell && req.ScriptType != types.ScriptTypeBatchFile && req.ScriptType != types.ScriptTypePowerShell {
		return fmt.Errorf("script_type is unsupported")
	}
	if req.BuildScript == "" {
		return fmt.Errorf("build_script cannot be empty")
	}
	if err := validateAdvancedSetting(req.Infrastructure, req.AdvancedSetting); err != nil {
		return err
	}
	if err := validateDockerBuildStep(req.DockerBuildStep); err != nil {
		return err
	}
	for i, parameter := range req.Parameters {
		if parameter == nil {
			return fmt.Errorf("parameters[%d] cannot be null", i)
		}
		if strings.TrimSpace(parameter.Key) == "" {
			return fmt.Errorf("parameters[%d].key cannot be empty", i)
		}
		if parameter.DefaultValue == "" {
			return fmt.Errorf("parameters[%d].default_value cannot be empty", i)
		}
		if parameter.Type != types.StringType && parameter.Type != types.ChoiceType && parameter.Type != types.MultiSelectType {
			return fmt.Errorf("parameters[%d].type is unsupported", i)
		}
	}
	for i, install := range req.Installs {
		if install == nil || strings.TrimSpace(install.Name) == "" || strings.TrimSpace(install.Version) == "" {
			return fmt.Errorf("installs[%d].name and version cannot be empty", i)
		}
	}
	return nil
}

func validateAdvancedSetting(infrastructure string, advanced *types.OpenAPIAdvancedSetting) error {
	if advanced == nil {
		return nil
	}
	if advanced.Timeout <= 0 {
		return fmt.Errorf("advanced_settings.timeout must be greater than 0")
	}
	if infrastructure == setting.JobK8sInfrastructure && strings.TrimSpace(advanced.ClusterName) == "" {
		return fmt.Errorf("advanced_settings.cluster_name cannot be empty for kubernetes")
	}
	spec := advanced.Spec
	if spec.CpuReq < 0 || spec.CpuLimit < 0 || spec.MemoryReq < 0 || spec.MemoryLimit < 0 {
		return fmt.Errorf("advanced_settings.resource_spec cannot contain negative values")
	}
	if spec.CpuReq > spec.CpuLimit || spec.MemoryReq > spec.MemoryLimit {
		return fmt.Errorf("advanced_settings resource request cannot be greater than resource limit")
	}
	if err := commonutil.CheckDefineResourceParam(spec.FindResourceRequestType(), spec); err != nil {
		return fmt.Errorf("invalid advanced_settings.resource_spec: %w", err)
	}
	if advanced.CacheSetting != nil && advanced.CacheSetting.Enabled && advanced.CacheSetting.CacheDir != "" && strings.TrimSpace(advanced.CacheSetting.CacheDir) == "" {
		return fmt.Errorf("advanced_settings.cache_setting.cache_dir cannot be empty when cache is enabled")
	}
	if err := validateStorages(advanced.Storages); err != nil {
		return err
	}
	return nil
}

func validateStorages(storages *types.OpenAPIStorages) error {
	if storages == nil || !storages.Enabled {
		return nil
	}
	for i, storage := range storages.StoragesProperties {
		if storage == nil {
			return fmt.Errorf("advanced_settings.storages.storages_properties[%d] cannot be null", i)
		}
		if strings.TrimSpace(storage.MountPath) == "" {
			return fmt.Errorf("advanced_settings.storages.storages_properties[%d].mount_path cannot be empty", i)
		}
		switch storage.ProvisionType {
		case types.DynamicProvision:
			if strings.TrimSpace(storage.StorageClass) == "" || storage.StorageSizeInGiB <= 0 {
				return fmt.Errorf("advanced_settings.storages.storages_properties[%d] dynamic storage requires storage_class and positive storage_size_in_gib", i)
			}
		case types.StaticProvision:
			if strings.TrimSpace(storage.PVC) == "" {
				return fmt.Errorf("advanced_settings.storages.storages_properties[%d] static storage requires pvc", i)
			}
		default:
			return fmt.Errorf("advanced_settings.storages.storages_properties[%d].provision_type must be dynamic or static", i)
		}
	}
	return nil
}

func validateDockerBuildStep(step *types.OpenAPIDockerBuildStep) error {
	if step == nil {
		return nil
	}
	if step.DockerfileSource != setting.DockerfileSourceLocal && step.DockerfileSource != setting.DockerfileSourceTemplate {
		return fmt.Errorf("dockerfile_source must be local or template")
	}
	if strings.TrimSpace(step.BuildContextDir) == "" {
		return fmt.Errorf("build_context_dir cannot be empty")
	}
	if step.DockerfileSource == setting.DockerfileSourceLocal {
		if strings.TrimSpace(step.DockerfileDirectory) == "" {
			return fmt.Errorf("dockerfile_directory cannot be empty when dockerfile_source is local")
		}
	} else if strings.TrimSpace(step.TemplateName) == "" {
		return fmt.Errorf("template_name cannot be empty when dockerfile_source is template")
	}
	if step.EnableBuildkit && strings.TrimSpace(step.Platforms) == "" {
		return fmt.Errorf("platforms cannot be empty when buildkit is enabled")
	}
	return nil
}

func OpenAPIListBuildTemplates(pageNum, pageSize int) (*OpenAPIBuildTemplateListResp, error) {
	buildTemplates, count, err := commonrepo.NewBuildTemplateColl().List(pageNum, pageSize)
	if err != nil {
		return nil, e.ErrListTemplate.AddErr(err)
	}
	resp := &OpenAPIBuildTemplateListResp{
		Total:          count,
		BuildTemplates: make([]*OpenAPIBuildTemplateBrief, 0, len(buildTemplates)),
	}
	for _, template := range buildTemplates {
		resp.BuildTemplates = append(resp.BuildTemplates, &OpenAPIBuildTemplateBrief{
			ID:             template.ID.Hex(),
			Name:           template.Name,
			Infrastructure: template.Infrastructure,
			UpdateTime:     template.UpdateTime,
			UpdateBy:       template.UpdateBy,
		})
	}
	return resp, nil
}

func OpenAPIGetBuildTemplate(id string, logger *zap.SugaredLogger) (*OpenAPIBuildTemplateDetail, error) {
	template, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{ID: id})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, e.ErrNotFound.AddErr(err)
		}
		logger.Errorf("OpenAPI: failed to get build template %s, error: %s", id, err)
		return nil, e.ErrGetTemplate.AddErr(err)
	}
	resp, err := convertBuildTemplateToOpenAPI(template)
	if err != nil {
		return nil, e.ErrGetTemplate.AddErr(err)
	}
	return resp, nil
}

func OpenAPICreateBuildTemplate(req *OpenAPIBuildTemplateInput, userName string, logger *zap.SugaredLogger) error {
	if err := req.Validate(); err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}
	resolved, err := resolveOpenAPIBuildTemplate(req)
	if err != nil {
		return e.ErrCreateTemplate.AddErr(err)
	}
	buildTemplate := new(commonmodels.BuildTemplate)
	applyOpenAPIBuildTemplate(buildTemplate, req, resolved, userName)
	if err := AddBuildTemplate(userName, buildTemplate, logger); err != nil {
		logger.Errorf("OpenAPI: failed to create build template %s, error: %s", req.Name, err)
		return e.ErrCreateTemplate.AddErr(err)
	}
	return nil
}

func OpenAPIUpdateBuildTemplate(id string, req *OpenAPIBuildTemplateInput, userName string, logger *zap.SugaredLogger) error {
	if err := req.Validate(); err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}
	existing, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{ID: id})
	if err != nil {
		return e.ErrUpdateTemplate.AddErr(err)
	}
	resolved, err := resolveOpenAPIBuildTemplate(req)
	if err != nil {
		return e.ErrUpdateTemplate.AddErr(err)
	}
	applyOpenAPIBuildTemplate(existing, req, resolved, userName)
	if err := UpdateBuildTemplate(id, existing, logger); err != nil {
		logger.Errorf("OpenAPI: failed to update build template %s, error: %s", id, err)
		return e.ErrUpdateTemplate.AddErr(err)
	}
	return nil
}

type resolvedOpenAPIBuildTemplate struct {
	image                *commonmodels.BasicImage
	advanced             *types.OpenAPIAdvancedSetting
	defaultAdvanced      bool
	clusterID            string
	strategyID           string
	dockerfileTemplateID string
}

func resolveOpenAPIBuildTemplate(req *OpenAPIBuildTemplateInput) (*resolvedOpenAPIBuildTemplate, error) {
	image, err := commonrepo.NewBasicImageColl().FindByImageName(strings.TrimSpace(req.BuildOS))
	if err != nil {
		return nil, fmt.Errorf("failed to find build image %s, error: %w", req.BuildOS, err)
	}
	resolved := &resolvedOpenAPIBuildTemplate{image: image, advanced: req.AdvancedSetting}
	if resolved.advanced == nil {
		// Keep omitted advanced settings aligned with the default build configuration.
		resolved.advanced = &types.OpenAPIAdvancedSetting{
			Timeout: 60,
			Spec: setting.RequestSpec{
				CpuLimit:    1000,
				MemoryLimit: 512,
			},
			Storages: &types.OpenAPIStorages{
				StoragesProperties: []*types.NFSProperties{},
			},
		}
		resolved.defaultAdvanced = true
	}
	if req.DockerBuildStep != nil && req.DockerBuildStep.DockerfileSource == setting.DockerfileSourceTemplate {
		template, err := commonrepo.NewDockerfileTemplateColl().GetByName(req.DockerBuildStep.TemplateName)
		if err != nil {
			return nil, fmt.Errorf("failed to find dockerfile template %s, error: %w", req.DockerBuildStep.TemplateName, err)
		}
		resolved.dockerfileTemplateID = template.ID.Hex()
	}
	if req.Infrastructure == setting.JobK8sInfrastructure {
		var cluster *commonmodels.K8SCluster
		clusterName := resolved.advanced.ClusterName
		if resolved.defaultAdvanced {
			clusterName = setting.LocalClusterID
			cluster, err = commonrepo.NewK8SClusterColl().FindByID(setting.LocalClusterID)
		} else {
			cluster, err = commonrepo.NewK8SClusterColl().FindByName(clusterName)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to find cluster %s, error: %w", clusterName, err)
		}
		resolved.clusterID = cluster.ID.Hex()
		if cluster.AdvancedConfig != nil {
			for _, strategy := range cluster.AdvancedConfig.ScheduleStrategy {
				if resolved.defaultAdvanced && strategy.Default {
					resolved.strategyID = strategy.StrategyID
					break
				}
				if !resolved.defaultAdvanced && strategy.StrategyName == resolved.advanced.StrategyName {
					resolved.strategyID = strategy.StrategyID
					break
				}
			}
		}
		if resolved.advanced.StrategyName != "" && resolved.strategyID == "" {
			return nil, fmt.Errorf("failed to find strategy %s in cluster %s", resolved.advanced.StrategyName, resolved.advanced.ClusterName)
		}
	}
	return resolved, nil
}

func applyOpenAPIBuildTemplate(template *commonmodels.BuildTemplate, req *OpenAPIBuildTemplateInput, resolved *resolvedOpenAPIBuildTemplate, userName string) {
	advanced := resolved.advanced
	if template.PreBuild == nil {
		template.PreBuild = new(commonmodels.PreBuild)
	}
	if template.PostBuild == nil {
		template.PostBuild = new(commonmodels.PostBuild)
	}
	preBuild := template.PreBuild
	if resolved.image.ImageFrom == commonmodels.ImageFromCustom {
		preBuild.BuildOS = resolved.image.Value
	} else {
		preBuild.BuildOS = resolved.image.Label
	}
	preBuild.ImageFrom = resolved.image.ImageFrom
	preBuild.ImageID = resolved.image.ID.Hex()
	preBuild.Installs = openapitool.ToBuildInstalls(req.Installs)
	preBuild.Envs = convertOpenAPIBuildTemplateParameters(req.Parameters)
	preBuild.ResReq = advanced.Spec.FindResourceRequestType()
	if resolved.defaultAdvanced {
		preBuild.ResReq = setting.LowRequest
	}
	preBuild.ResReqSpec = advanced.Spec
	preBuild.ClusterID = resolved.clusterID
	preBuild.StrategyID = resolved.strategyID
	preBuild.UseHostDockerDaemon = advanced.UseHostDockerDaemon
	preBuild.CustomAnnotations = openapitool.ToKeyVals(advanced.CustomAnnotations)
	preBuild.CustomLabels = openapitool.ToKeyVals(advanced.CustomLabels)
	preBuild.Storages = nil
	if advanced.Storages != nil {
		preBuild.Storages = &commonmodels.Storages{Enabled: advanced.Storages.Enabled, StoragesProperties: advanced.Storages.StoragesProperties}
	}

	template.Name = strings.TrimSpace(req.Name)
	template.Timeout = int(advanced.Timeout)
	template.UpdateTime = time.Now().Unix()
	template.UpdateBy = userName
	template.ScriptType = req.ScriptType
	template.Scripts = req.BuildScript
	template.CacheEnable = advanced.CacheSetting != nil && advanced.CacheSetting.Enabled
	template.CacheDirType = ""
	template.CacheUserDir = ""
	if resolved.defaultAdvanced || template.CacheEnable && advanced.CacheSetting.CacheDir == "" {
		template.CacheDirType = types.WorkspaceCacheDir
	} else if template.CacheEnable {
		template.CacheDirType = types.UserDefinedCacheDir
		template.CacheUserDir = advanced.CacheSetting.CacheDir
	}
	template.EnablePrivilegedMode = advanced.PrivilegedMode
	template.AdvancedSettingsModified = !resolved.defaultAdvanced
	template.Outputs = openapitool.ToOutputs(advanced.Outputs)
	template.Infrastructure = req.Infrastructure
	template.VmLabels = make([]string, 0, len(req.VMLabels))
	for _, label := range req.VMLabels {
		template.VmLabels = append(template.VmLabels, strings.TrimSpace(label))
	}
	template.PostBuild.DockerBuild = nil
	if req.DockerBuildStep != nil {
		dockerBuild := &commonmodels.DockerBuild{
			WorkDir:        req.DockerBuildStep.BuildContextDir,
			BuildArgs:      req.DockerBuildStep.BuildArgs,
			Source:         req.DockerBuildStep.DockerfileSource,
			EnableBuildkit: req.DockerBuildStep.EnableBuildkit,
			Platform:       req.DockerBuildStep.Platforms,
		}
		if req.DockerBuildStep.DockerfileSource == setting.DockerfileSourceLocal {
			dockerBuild.DockerFile = req.DockerBuildStep.DockerfileDirectory
		} else {
			dockerBuild.TemplateID = resolved.dockerfileTemplateID
			dockerBuild.TemplateName = req.DockerBuildStep.TemplateName
		}
		template.PostBuild.DockerBuild = dockerBuild
	}
}

func convertBuildTemplateToOpenAPI(template *commonmodels.BuildTemplate) (*OpenAPIBuildTemplateDetail, error) {
	if template.PreBuild == nil {
		return nil, fmt.Errorf("build template %s has incomplete build settings", template.ID.Hex())
	}
	image, err := commonrepo.NewBasicImageColl().Find(template.PreBuild.ImageID)
	if err != nil {
		return nil, fmt.Errorf("failed to find build image %s, error: %w", template.PreBuild.ImageID, err)
	}
	advanced := &types.OpenAPIAdvancedSetting{
		Timeout:             int64(template.Timeout),
		Spec:                template.PreBuild.ResReqSpec,
		UseHostDockerDaemon: template.PreBuild.UseHostDockerDaemon,
		PrivilegedMode:      template.EnablePrivilegedMode,
		CustomAnnotations:   convertOpenAPIKeyValues(template.PreBuild.CustomAnnotations),
		CustomLabels:        convertOpenAPIKeyValues(template.PreBuild.CustomLabels),
		Outputs:             outputNames(template.Outputs),
		CacheSetting:        &types.OpenAPICacheSetting{Enabled: template.CacheEnable, CacheDir: template.CacheUserDir},
	}
	if template.PreBuild.Storages != nil {
		advanced.Storages = &types.OpenAPIStorages{
			Enabled:            template.PreBuild.Storages.Enabled,
			StoragesProperties: template.PreBuild.Storages.StoragesProperties,
		}
	}
	if template.PreBuild.ClusterID != "" {
		cluster, err := commonrepo.NewK8SClusterColl().FindByID(template.PreBuild.ClusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to find cluster %s, error: %w", template.PreBuild.ClusterID, err)
		}
		advanced.ClusterName = cluster.Name
		if cluster.AdvancedConfig != nil {
			for _, strategy := range cluster.AdvancedConfig.ScheduleStrategy {
				if strategy.StrategyID == template.PreBuild.StrategyID {
					advanced.StrategyName = strategy.StrategyName
				}
			}
		}
	}
	input := OpenAPIBuildTemplateInput{
		Name:            template.Name,
		Infrastructure:  template.Infrastructure,
		VMLabels:        append(make([]string, 0, len(template.VmLabels)), template.VmLabels...),
		BuildOS:         image.Label,
		Installs:        convertInstalls(template.PreBuild.Installs),
		Parameters:      convertParameters(template.PreBuild.Envs),
		ScriptType:      template.ScriptType,
		BuildScript:     template.Scripts,
		AdvancedSetting: advanced,
	}
	if template.PostBuild != nil && template.PostBuild.DockerBuild != nil {
		input.DockerBuildStep = &types.OpenAPIDockerBuildStep{
			BuildContextDir:     template.PostBuild.DockerBuild.WorkDir,
			DockerfileSource:    template.PostBuild.DockerBuild.Source,
			DockerfileDirectory: template.PostBuild.DockerBuild.DockerFile,
			TemplateName:        template.PostBuild.DockerBuild.TemplateName,
			BuildArgs:           template.PostBuild.DockerBuild.BuildArgs,
			EnableBuildkit:      template.PostBuild.DockerBuild.EnableBuildkit,
			Platforms:           template.PostBuild.DockerBuild.Platform,
		}
	}
	return &OpenAPIBuildTemplateDetail{
		ID:                        template.ID.Hex(),
		OpenAPIBuildTemplateInput: input,
		UpdateTime:                template.UpdateTime,
		UpdateBy:                  template.UpdateBy,
	}, nil
}

func convertOpenAPIKeyValues(values []*util.KeyValue) []*types.KeyValue {
	ret := make([]*types.KeyValue, 0, len(values))
	for _, value := range values {
		if value != nil {
			ret = append(ret, &types.KeyValue{Key: value.Key, Value: value.Value})
		}
	}
	return ret
}

func convertInstalls(installs []*commonmodels.Item) []*types.OpenAPIToolItem {
	ret := make([]*types.OpenAPIToolItem, 0, len(installs))
	for _, install := range installs {
		if install != nil {
			ret = append(ret, &types.OpenAPIToolItem{Name: install.Name, Version: install.Version})
		}
	}
	return ret
}

func convertOpenAPIBuildTemplateParameters(parameters []*OpenAPIBuildTemplateParameter) commonmodels.KeyValList {
	ret := make(commonmodels.KeyValList, 0, len(parameters))
	for _, parameter := range parameters {
		ret = append(ret, &commonmodels.KeyVal{
			Key:          parameter.Key,
			Value:        parameter.DefaultValue,
			Type:         commonmodels.ParameterSettingType(parameter.Type),
			ChoiceOption: parameter.ChoiceOption,
			ChoiceValue:  parameter.ChoiceValue,
			IsCredential: parameter.IsCredential,
			Description:  parameter.Description,
			Required:     parameter.Required,
		})
	}
	return ret
}

func convertParameters(parameters commonmodels.KeyValList) []*OpenAPIBuildTemplateParameter {
	ret := make([]*OpenAPIBuildTemplateParameter, 0, len(parameters))
	for _, parameter := range parameters {
		if parameter != nil {
			ret = append(ret, &OpenAPIBuildTemplateParameter{
				Key:          parameter.Key,
				Type:         types.ParameterSettingType(parameter.Type),
				DefaultValue: parameter.Value,
				ChoiceOption: parameter.ChoiceOption,
				ChoiceValue:  parameter.ChoiceValue,
				IsCredential: parameter.IsCredential,
				Description:  parameter.Description,
				Required:     parameter.Required,
			})
		}
	}
	return ret
}

func outputNames(outputs []*commonmodels.Output) []string {
	ret := make([]string, 0, len(outputs))
	for _, output := range outputs {
		if output != nil {
			ret = append(ret, output.Name)
		}
	}
	return ret
}
