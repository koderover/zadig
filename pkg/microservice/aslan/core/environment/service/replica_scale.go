package service

import (
	"fmt"
	"regexp"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/koderover/zadig/v2/pkg/util/converter"
	"sigs.k8s.io/yaml"
)

var (
	directReplicaVariableRegexp = regexp.MustCompile(`(?m)^[ \t]*replicas:[ \t]*\{\{\s*\.([A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*)\s*\}\}[ \t]*$`)
	templatedReplicaRegexp      = regexp.MustCompile(`(?m)^[ \t]*replicas:[^\n]*\{\{.*\}\}[ \t]*$`)
)

type replicaSourceKind string

const (
	replicaSourceLiteral replicaSourceKind = "literal"
	replicaSourceService replicaSourceKind = "service"
	replicaSourceGlobal  replicaSourceKind = "global"
)

type replicaSource struct {
	Kind    replicaSourceKind
	RootKey string
	SubPath string
}

// resolveScaleReplicaSource 判定目标 workload 的 replicas 来源（字面量/服务变量/全局变量），并在变量场景返回变量路径。
func resolveScaleReplicaSource(prod *commonmodels.Product, currentSvc *commonmodels.ProductService, currentTmpl *commonmodels.Service, workloadType, workloadName string) (*replicaSource, error) {
	renderedYaml, err := renderServiceWithOverrides(prod, currentSvc, currentTmpl, nil)
	if err != nil {
		return nil, err
	}

	path, templated, replicaRefCount, err := resolveReplicaVariablePath(renderedYaml, currentTmpl.Yaml, workloadType, workloadName)
	if err != nil {
		return nil, err
	}
	if !templated {
		return &replicaSource{Kind: replicaSourceLiteral}, nil
	}

	if replicaRefCount > 1 {
		return nil, fmt.Errorf("replicas of workload %s/%s is shared by multiple replica locations or cannot be updated safely", workloadType, workloadName)
	}

	rootKey, subPath := path, ""
	if parts := strings.SplitN(path, ".", 2); len(parts) == 2 {
		rootKey = parts[0]
		subPath = parts[1]
	}
	mergedRenderKVs, err := mergeServiceRenderVariableKVs(currentTmpl.ServiceVariableKVs, currentSvc.GetServiceRender().OverrideYaml.RenderVariableKVs)
	if err != nil {
		return nil, fmt.Errorf("failed to merge render variables for service %s: %w", currentSvc.ServiceName, err)
	}
	var targetKV *commontypes.RenderVariableKV
	for _, kv := range mergedRenderKVs {
		if kv != nil && kv.Key == rootKey {
			targetKV = kv
			break
		}
	}
	if targetKV == nil {
		return nil, fmt.Errorf("failed to find replicas variable %s for workload %s/%s", path, workloadType, workloadName)
	}

	if targetKV.UseGlobalVariable {
		return &replicaSource{Kind: replicaSourceGlobal, RootKey: rootKey, SubPath: subPath}, nil
	}

	return &replicaSource{Kind: replicaSourceService, RootKey: rootKey, SubPath: subPath}, nil
}

// resolveReplicaVariablePath 在模板原文中定位目标 workload 的 replicas 变量路径。
// 返回值中的 bool 表示 replicas 是否来自模板表达式，int 表示同变量路径在模板中被 replicas 直接引用的次数。
func resolveReplicaVariablePath(renderedYaml, rawTemplateYaml, workloadType, workloadName string) (string, bool, int, error) {
	manifestIndex := -1
	normalizedType := kube.NormalizeReplicaWorkloadType(workloadType)
	manifests := util.SplitManifests(renderedYaml)
	for index, manifest := range manifests {
		if len(strings.TrimSpace(manifest)) == 0 {
			continue
		}
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(manifest))
		if err != nil {
			return "", false, 0, err
		}
		if kube.NormalizeReplicaWorkloadType(u.GetKind()) == normalizedType && u.GetName() == workloadName {
			manifestIndex = index
			break
		}
	}
	if manifestIndex < 0 {
		return "", false, 0, fmt.Errorf("failed to find workload %s/%s in rendered yaml", workloadType, workloadName)
	}

	rawManifests := util.SplitManifests(rawTemplateYaml)
	if manifestIndex >= len(rawManifests) {
		return "", true, 0, fmt.Errorf("failed to map workload %s/%s to template manifest; replicas source cannot be determined safely", workloadType, workloadName)
	}

	rawManifest := rawManifests[manifestIndex]
	matches := directReplicaVariableRegexp.FindStringSubmatch(rawManifest)
	if len(matches) == 2 {
		replicaPath := matches[1]
		replicaRefCount := 0
		for _, manifest := range rawManifests {
			matches := directReplicaVariableRegexp.FindStringSubmatch(manifest)
			if len(matches) == 2 && matches[1] == replicaPath {
				replicaRefCount++
			}
		}
		return replicaPath, true, replicaRefCount, nil
	}
	if templatedReplicaRegexp.MatchString(rawManifest) {
		return "", true, 0, fmt.Errorf("replicas of workload %s/%s is derived from a complex template expression", workloadType, workloadName)
	}
	return "", false, 0, nil
}

// updateRenderVariableReplicaValue 更新 replicas 对应变量值；更新前会克隆 render kv，避免原对象被就地修改。
func updateRenderVariableReplicaValue(renderVars []*commontypes.RenderVariableKV, rootKey, subPath string, replicas int) ([]*commontypes.RenderVariableKV, error) {
	cloned := cloneRenderVariableKVs(renderVars)
	for _, kv := range cloned {
		if kv == nil || kv.Key != rootKey {
			continue
		}
		if subPath == "" {
			if kv.Type == commontypes.ServiceVariableKVTypeYaml {
				renderedValue, err := yaml.Marshal(replicas)
				if err != nil {
					return nil, err
				}
				kv.Value = strings.TrimSpace(string(renderedValue))
				return cloned, nil
			}
			kv.Value = replicas
			return cloned, nil
		}

		if kv.Type != commontypes.ServiceVariableKVTypeYaml {
			return nil, fmt.Errorf("variable %s does not support nested replica path %s", kv.Key, subPath)
		}
		yamlValue, ok := kv.Value.(string)
		if !ok {
			return nil, fmt.Errorf("variable %s is not a valid yaml value", kv.Key)
		}

		flatMap, err := converter.YamlToFlatMap([]byte(yamlValue))
		if err != nil {
			return nil, fmt.Errorf("failed to flatten variable %s: %w", kv.Key, err)
		}
		flatMap[subPath] = replicas

		expanded, err := converter.Expand(flatMap)
		if err != nil {
			return nil, fmt.Errorf("failed to expand variable %s: %w", kv.Key, err)
		}
		renderedValue, err := yaml.Marshal(expanded)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal variable %s: %w", kv.Key, err)
		}
		kv.Value = string(renderedValue)
		return cloned, nil
	}
	return nil, fmt.Errorf("failed to find render variable %s", rootKey)
}

// buildPreviewCandidateOverrides 仅用于预览：基于候选变量/版本计算预期的副本 override，不修改当前环境状态。
func buildPreviewCandidateOverrides(prod *commonmodels.Product, serviceName string, updateServiceRevision bool, variableKVs []*commontypes.RenderVariableKV) ([]*commonmodels.WorkLoad, error) {
	currentSvc := cloneProductService(prod.GetServiceMap()[serviceName])
	if currentSvc == nil {
		return nil, nil
	}
	if !updateServiceRevision {
		// Keep environment-recorded replica overrides when service template revision is unchanged.
		return cloneWorkLoads(currentSvc.WorkLoads), nil
	}

	currentTmpl, err := loadServiceTemplateByRevision(currentSvc, prod.Production)
	if err != nil {
		return nil, err
	}
	candidateTmpl := currentTmpl
	if updateServiceRevision {
		candidateTmpl, err = repository.QueryTemplateService(&commonrepo.ServiceFindOption{
			ServiceName: serviceName,
			ProductName: prod.ProductName,
			Revision:    0,
		}, prod.Production)
		if err != nil {
			return nil, err
		}
	}

	candidateSvc := cloneProductService(currentSvc)
	candidateSvc.Revision = candidateTmpl.Revision
	candidateRenderKVs, err := mergeServiceRenderVariableKVs(candidateTmpl.ServiceVariableKVs, currentSvc.GetServiceRender().OverrideYaml.RenderVariableKVs, variableKVs)
	if err != nil {
		return nil, err
	}
	candidateSvc.GetServiceRender().OverrideYaml.RenderVariableKVs = candidateRenderKVs
	candidateSvc.GetServiceRender().OverrideYaml.YamlContent, err = commontypes.RenderVariableKVToYaml(candidateRenderKVs, true)
	if err != nil {
		return nil, err
	}

	return syncServiceReplicaOverrides(prod, currentSvc, candidateSvc, nil)
}
