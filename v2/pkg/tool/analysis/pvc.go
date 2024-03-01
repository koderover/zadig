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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PvcAnalyzer struct{}

func (PvcAnalyzer) Analyze(a Analyzer) ([]Result, error) {

	kind := "PersistentVolumeClaim"

	AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	// search all namespaces for pods that are not running
	list, err := a.Client.GetClient().CoreV1().PersistentVolumeClaims(a.Namespace).List(a.Context, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var preAnalysis = map[string]PreAnalysis{}

	for _, pvc := range list.Items {
		var failures []Failure

		// Check for empty rs
		if pvc.Status.Phase == "Pending" {

			// parse the event log and append details
			evt, err := FetchLatestEvent(a.Context, a.Client, pvc.Namespace, pvc.Name)
			if err != nil || evt == nil {
				continue
			}
			if evt.Reason == "ProvisioningFailed" && evt.Message != "" {
				failures = append(failures, Failure{
					Text:      evt.Message,
					Sensitive: []Sensitive{},
				})
			}
		}
		if len(failures) > 0 {
			preAnalysis[fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)] = PreAnalysis{
				PersistentVolumeClaim: pvc,
				FailureDetails:        failures,
			}
			AnalyzerErrorsMetric.WithLabelValues(kind, pvc.Name, pvc.Namespace).Set(float64(len(failures)))
		}
	}

	for key, value := range preAnalysis {
		var currentAnalysis = Result{
			Kind:  kind,
			Name:  key,
			Error: value.FailureDetails,
		}

		parent, _ := GetParent(a.Client, value.PersistentVolumeClaim.ObjectMeta)
		currentAnalysis.ParentObject = parent
		a.Results = append(a.Results, currentAnalysis)
	}

	return a.Results, nil
}
