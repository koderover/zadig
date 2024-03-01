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

	kubernetes "github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type StatefulSetAnalyzer struct{}

func (StatefulSetAnalyzer) Analyze(a Analyzer) ([]Result, error) {

	kind := "StatefulSet"
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

	list, err := a.Client.GetClient().AppsV1().StatefulSets(a.Namespace).List(a.Context, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var preAnalysis = map[string]PreAnalysis{}

	for _, sts := range list.Items {
		var failures []Failure

		// get serviceName
		serviceName := sts.Spec.ServiceName
		_, err := a.Client.GetClient().CoreV1().Services(sts.Namespace).Get(a.Context, serviceName, metav1.GetOptions{})
		if err != nil {
			doc := apiDoc.GetApiDocV2("spec.serviceName")

			failures = append(failures, Failure{
				Text: fmt.Sprintf(
					"StatefulSet uses the service %s/%s which does not exist.",
					sts.Namespace,
					serviceName,
				),
				KubernetesDoc: doc,
				Sensitive: []Sensitive{
					{
						Unmasked: sts.Namespace,
						Masked:   MaskString(sts.Namespace),
					},
					{
						Unmasked: serviceName,
						Masked:   MaskString(serviceName),
					},
				},
			})
		}
		if len(sts.Spec.VolumeClaimTemplates) > 0 {
			for _, volumeClaimTemplate := range sts.Spec.VolumeClaimTemplates {
				if volumeClaimTemplate.Spec.StorageClassName != nil {
					_, err := a.Client.GetClient().StorageV1().StorageClasses().Get(a.Context, *volumeClaimTemplate.Spec.StorageClassName, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, Failure{
							Text: fmt.Sprintf("StatefulSet uses the storage class %s which does not exist.", *volumeClaimTemplate.Spec.StorageClassName),
							Sensitive: []Sensitive{
								{
									Unmasked: *volumeClaimTemplate.Spec.StorageClassName,
									Masked:   MaskString(*volumeClaimTemplate.Spec.StorageClassName),
								},
							},
						})
					}
				}
			}
		}
		if len(failures) > 0 {
			preAnalysis[fmt.Sprintf("%s/%s", sts.Namespace, sts.Name)] = PreAnalysis{
				StatefulSet:    sts,
				FailureDetails: failures,
			}
			AnalyzerErrorsMetric.WithLabelValues(kind, sts.Name, sts.Namespace).Set(float64(len(failures)))
		}
	}

	for key, value := range preAnalysis {
		var currentAnalysis = Result{
			Kind:  kind,
			Name:  key,
			Error: value.FailureDetails,
		}

		parent, _ := GetParent(a.Client, value.StatefulSet.ObjectMeta)
		currentAnalysis.ParentObject = parent
		a.Results = append(a.Results, currentAnalysis)
	}

	return a.Results, nil
}
