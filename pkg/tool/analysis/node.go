/*
Copyright 2023 The K8sGPT Authors.
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

// Some parts of this file have been modified to make it functional in Zadig

package analysis

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeAnalyzer struct{}

func (NodeAnalyzer) Analyze(a Analyzer) ([]Result, error) {
	if a.Namespace != "" {
		return a.Results, nil
	}

	kind := "Node"

	AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	list, err := a.Client.GetClient().CoreV1().Nodes().List(a.Context, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var preAnalysis = map[string]PreAnalysis{}

	for _, node := range list.Items {
		var failures []Failure
		for _, nodeCondition := range node.Status.Conditions {
			// https://kubernetes.io/docs/concepts/architecture/nodes/#condition
			switch nodeCondition.Type {
			case v1.NodeReady:
				if nodeCondition.Status == v1.ConditionTrue {
					break
				}
				failures = addNodeConditionFailure(failures, node.Name, nodeCondition)
			default:
				if nodeCondition.Status != v1.ConditionFalse {
					failures = addNodeConditionFailure(failures, node.Name, nodeCondition)
				}
			}
		}

		if len(failures) > 0 {
			preAnalysis[node.Name] = PreAnalysis{
				Node:           node,
				FailureDetails: failures,
			}
			AnalyzerErrorsMetric.WithLabelValues(kind, node.Name, "").Set(float64(len(failures)))

		}
	}

	for key, value := range preAnalysis {
		var currentAnalysis = Result{
			Kind:  kind,
			Name:  key,
			Error: value.FailureDetails,
		}

		parent, _ := GetParent(a.Client, value.Node.ObjectMeta)
		currentAnalysis.ParentObject = parent
		a.Results = append(a.Results, currentAnalysis)
	}

	return a.Results, err
}

func addNodeConditionFailure(failures []Failure, nodeName string, nodeCondition v1.NodeCondition) []Failure {
	failures = append(failures, Failure{
		Text: fmt.Sprintf("%s has condition of type %s, reason %s: %s", nodeName, nodeCondition.Type, nodeCondition.Reason, nodeCondition.Message),
		Sensitive: []Sensitive{
			{
				Unmasked: nodeName,
				Masked:   MaskString(nodeName),
			},
		},
	})
	return failures
}
