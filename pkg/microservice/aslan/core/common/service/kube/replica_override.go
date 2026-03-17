package kube

import (
	"fmt"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

var supportedReplicaWorkloadTypes = map[string]string{
	strings.ToLower(setting.Deployment):  setting.Deployment,
	strings.ToLower(setting.StatefulSet): setting.StatefulSet,
	strings.ToLower(setting.CloneSet):    setting.CloneSet,
}

func ReplicaOverrideKey(workloadType, workloadName string) string {
	return strings.ToLower(workloadType) + "/" + workloadName
}

func NormalizeReplicaWorkloadType(workloadType string) string {
	if normalizedType, ok := supportedReplicaWorkloadTypes[strings.ToLower(workloadType)]; ok {
		return normalizedType
	}
	return workloadType
}

func ValidateReplicaWorkloadType(workloadType string) error {
	if _, ok := supportedReplicaWorkloadTypes[strings.ToLower(workloadType)]; ok {
		return nil
	}
	return fmt.Errorf("unsupported replica workload type %q", workloadType)
}

func normalizeReplicaOverrideTarget(workloadType, workloadName string) (string, string, error) {
	if err := ValidateReplicaWorkloadType(workloadType); err != nil {
		return "", "", err
	}
	workloadName = strings.TrimSpace(workloadName)
	if workloadName == "" {
		return "", "", fmt.Errorf("workload name is empty")
	}
	return NormalizeReplicaWorkloadType(workloadType), workloadName, nil
}

func UpsertWorkLoadsReplicas(origins []*commonmodels.WorkLoad, workloadType, workloadName string, replicas int32) ([]*commonmodels.WorkLoad, error) {
	normalizedType, workloadName, err := normalizeReplicaOverrideTarget(workloadType, workloadName)
	if err != nil {
		return nil, err
	}
	for _, item := range origins {
		if item == nil {
			continue
		}
		if NormalizeReplicaWorkloadType(item.WorkloadType) == normalizedType && item.WorkloadName == workloadName {
			item.WorkloadType = normalizedType
			item.WorkloadName = workloadName
			item.Replicas = replicas
			return origins, nil
		}
	}

	return append(origins, &commonmodels.WorkLoad{
		WorkloadType: normalizedType,
		WorkloadName: workloadName,
		Replicas:     replicas,
	}), nil
}

// ApplyReplicaOverrides 按归一化后的 workload 目标，把 override 写入渲染结果中的 spec.replicas。
func ApplyReplicaOverrides(renderedYaml string, overrides []*commonmodels.WorkLoad) (string, error) {
	if len(overrides) == 0 || len(strings.TrimSpace(renderedYaml)) == 0 {
		return renderedYaml, nil
	}

	overrideMap := make(map[string]int32, len(overrides))
	for _, item := range overrides {
		if item == nil {
			continue
		}
		normalizedType, workloadName, err := normalizeReplicaOverrideTarget(item.WorkloadType, item.WorkloadName)
		if err != nil {
			return "", err
		}
		overrideMap[ReplicaOverrideKey(normalizedType, workloadName)] = item.Replicas
	}

	manifests := util.SplitManifests(renderedYaml)
	updated := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		if len(strings.TrimSpace(manifest)) == 0 {
			continue
		}

		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(manifest))
		if err != nil {
			return "", err
		}

		switch NormalizeReplicaWorkloadType(u.GetKind()) {
		case setting.Deployment, setting.StatefulSet, setting.CloneSet:
			if replicas, ok := overrideMap[ReplicaOverrideKey(u.GetKind(), u.GetName())]; ok {
				if err := unstructured.SetNestedField(u.Object, int64(replicas), "spec", "replicas"); err != nil {
					return "", fmt.Errorf("failed to set replicas for %s/%s: %w", u.GetKind(), u.GetName(), err)
				}
			}
		}

		yamlBytes, err := yaml.Marshal(u.Object)
		if err != nil {
			return "", fmt.Errorf("failed to marshal %s/%s: %w", u.GetKind(), u.GetName(), err)
		}
		updated = append(updated, string(yamlBytes))
	}

	return util.JoinYamls(updated), nil
}

// ExtractWorkloadReplicas 提取支持类型 workload 的 spec.replicas，并返回标准化的 "type/name -> replicas" 映射。
func ExtractWorkloadReplicas(renderedYaml string) (map[string]int32, error) {
	ret := make(map[string]int32)
	if len(strings.TrimSpace(renderedYaml)) == 0 {
		return ret, nil
	}

	for _, manifest := range util.SplitManifests(renderedYaml) {
		if len(strings.TrimSpace(manifest)) == 0 {
			continue
		}

		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(manifest))
		if err != nil {
			return nil, err
		}

		switch NormalizeReplicaWorkloadType(u.GetKind()) {
		case setting.Deployment, setting.StatefulSet, setting.CloneSet:
			replicas, found, err := unstructured.NestedInt64(u.Object, "spec", "replicas")
			if err != nil {
				return nil, fmt.Errorf("failed to get replicas from %s/%s: %w", u.GetKind(), u.GetName(), err)
			}
			if !found {
				continue
			}
			ret[ReplicaOverrideKey(u.GetKind(), u.GetName())] = int32(replicas)
		}
	}

	return ret, nil
}

func GetWorkloadReplica(renderedYaml, workloadType, workloadName string) (int32, bool, error) {
	replicaMap, err := ExtractWorkloadReplicas(renderedYaml)
	if err != nil {
		return 0, false, err
	}

	replicas, found := replicaMap[ReplicaOverrideKey(workloadType, workloadName)]
	return replicas, found, nil
}
