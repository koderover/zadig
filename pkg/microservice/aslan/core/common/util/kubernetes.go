/*
Copyright 2024 The KodeRover Authors.

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

package util

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func BuildTolerations(clusterConfig *models.AdvancedConfig, strategyID string) []corev1.Toleration {
	ret := make([]corev1.Toleration, 0)
	if clusterConfig == nil || len(clusterConfig.ScheduleStrategy) == 0 {
		return ret
	}

	var tolerations string
	for _, strategy := range clusterConfig.ScheduleStrategy {
		if strategyID != "" && strategy.StrategyID == strategyID {
			tolerations = strategy.Tolerations
			break
		} else if strategyID == "" && strategy.Default {
			tolerations = strategy.Tolerations
			break
		}
	}
	err := yaml.Unmarshal([]byte(tolerations), &ret)
	if err != nil {
		log.Errorf("failed to parse toleration config, err: %s", err)
		return nil
	}
	return ret
}

func AddNodeAffinity(clusterConfig *models.AdvancedConfig, strategyID string) *corev1.Affinity {
	if clusterConfig == nil || len(clusterConfig.ScheduleStrategy) == 0 {
		return nil
	}

	var strategy *models.ScheduleStrategy
	for _, s := range clusterConfig.ScheduleStrategy {
		if strategyID != "" && s.StrategyID == strategyID {
			strategy = s
			break
		} else if strategyID == "" && s.Default {
			strategy = s
			break
		}
	}
	if strategy == nil {
		return nil
	}

	switch strategy.Strategy {
	case setting.RequiredSchedule:
		nodeSelectorTerms := make([]corev1.NodeSelectorTerm, 0)
		for _, nodeLabel := range strategy.NodeLabels {
			var matchExpressions []corev1.NodeSelectorRequirement
			matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
				Key:      nodeLabel.Key,
				Operator: nodeLabel.Operator,
				Values:   nodeLabel.Value,
			})
			nodeSelectorTerms = append(nodeSelectorTerms, corev1.NodeSelectorTerm{
				MatchExpressions: matchExpressions,
			})
		}

		affinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: nodeSelectorTerms,
				},
			},
		}
		return affinity
	case setting.PreferredSchedule:
		preferredScheduleTerms := make([]corev1.PreferredSchedulingTerm, 0)
		for _, nodeLabel := range strategy.NodeLabels {
			var matchExpressions []corev1.NodeSelectorRequirement
			matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
				Key:      nodeLabel.Key,
				Operator: nodeLabel.Operator,
				Values:   nodeLabel.Value,
			})
			nodeSelectorTerm := corev1.NodeSelectorTerm{
				MatchExpressions: matchExpressions,
			}
			preferredScheduleTerms = append(preferredScheduleTerms, corev1.PreferredSchedulingTerm{
				Weight:     10,
				Preference: nodeSelectorTerm,
			})
		}
		affinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: preferredScheduleTerms,
			},
		}
		return affinity
	default:
		return nil
	}
}
