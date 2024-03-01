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
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kubernetes "github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
)

// DeploymentAnalyzer is an analyzer that checks for misconfigured Deployments
type DeploymentAnalyzer struct {
}

// Analyze scans all namespaces for Deployments with misconfigurations
func (d DeploymentAnalyzer) Analyze(a Analyzer) ([]Result, error) {

	kind := "Deployment"
	apiDoc := kubernetes.K8sApiReference{
		Kind: kind,
		ApiVersion: schema.GroupVersion{
			Group:   "apps",
			Version: "v1",
		},
		OpenapiSchema: a.OpenapiSchema,
	}

	AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	deployments, err := a.Client.GetClient().AppsV1().Deployments(a.Namespace).List(context.Background(), v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var preAnalysis = map[string]PreAnalysis{}

	for _, deployment := range deployments.Items {
		var failures []Failure
		if *deployment.Spec.Replicas != deployment.Status.Replicas {
			doc := apiDoc.GetApiDocV2("spec.replicas")

			failures = append(failures, Failure{
				Text:          fmt.Sprintf("Deployment %s/%s has %d replicas but %d are available", deployment.Namespace, deployment.Name, *deployment.Spec.Replicas, deployment.Status.Replicas),
				KubernetesDoc: doc,
				Sensitive: []Sensitive{
					{
						Unmasked: deployment.Namespace,
						Masked:   MaskString(deployment.Namespace),
					},
					{
						Unmasked: deployment.Name,
						Masked:   MaskString(deployment.Name),
					},
				}})
		}
		if len(failures) > 0 {
			preAnalysis[fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)] = PreAnalysis{
				FailureDetails: failures,
				Deployment:     deployment,
			}
			AnalyzerErrorsMetric.WithLabelValues(kind, deployment.Name, deployment.Namespace).Set(float64(len(failures)))
		}

	}

	for key, value := range preAnalysis {
		var currentAnalysis = Result{
			Kind:  kind,
			Name:  key,
			Error: value.FailureDetails,
		}

		a.Results = append(a.Results, currentAnalysis)
	}

	return a.Results, nil
}
