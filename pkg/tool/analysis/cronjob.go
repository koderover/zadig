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
	"time"

	kubernetes "github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	cron "github.com/robfig/cron/v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CronJobAnalyzer struct{}

func (analyzer CronJobAnalyzer) Analyze(a Analyzer) ([]Result, error) {

	kind := "CronJob"
	apiDoc := kubernetes.K8sApiReference{
		Kind: kind,
		ApiVersion: schema.GroupVersion{
			Group:   "batch",
			Version: "v1",
		},
		OpenapiSchema: a.OpenapiSchema,
	}

	AnalyzerErrorsMetric.DeletePartialMatch(map[string]string{
		"analyzer_name": kind,
	})

	cronJobList, err := a.Client.GetClient().BatchV1().CronJobs(a.Namespace).List(a.Context, v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var preAnalysis = map[string]PreAnalysis{}

	for _, cronJob := range cronJobList.Items {
		var failures []Failure
		if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
			doc := apiDoc.GetApiDocV2("spec.suspend")

			failures = append(failures, Failure{
				Text:          fmt.Sprintf("CronJob %s is suspended", cronJob.Name),
				KubernetesDoc: doc,
				Sensitive: []Sensitive{
					{
						Unmasked: cronJob.Namespace,
						Masked:   MaskString(cronJob.Namespace),
					},
					{
						Unmasked: cronJob.Name,
						Masked:   MaskString(cronJob.Name),
					},
				},
			})
		} else {
			// check the schedule format
			if _, err := CheckCronScheduleIsValid(cronJob.Spec.Schedule); err != nil {
				doc := apiDoc.GetApiDocV2("spec.schedule")

				failures = append(failures, Failure{
					Text:          fmt.Sprintf("CronJob %s has an invalid schedule: %s", cronJob.Name, err.Error()),
					KubernetesDoc: doc,
					Sensitive: []Sensitive{
						{
							Unmasked: cronJob.Namespace,
							Masked:   MaskString(cronJob.Namespace),
						},
						{
							Unmasked: cronJob.Name,
							Masked:   MaskString(cronJob.Name),
						},
					},
				})
			}

			// check the starting deadline
			if cronJob.Spec.StartingDeadlineSeconds != nil {
				deadline := time.Duration(*cronJob.Spec.StartingDeadlineSeconds) * time.Second
				if deadline < 0 {
					doc := apiDoc.GetApiDocV2("spec.startingDeadlineSeconds")

					failures = append(failures, Failure{
						Text:          fmt.Sprintf("CronJob %s has a negative starting deadline", cronJob.Name),
						KubernetesDoc: doc,
						Sensitive: []Sensitive{
							{
								Unmasked: cronJob.Namespace,
								Masked:   MaskString(cronJob.Namespace),
							},
							{
								Unmasked: cronJob.Name,
								Masked:   MaskString(cronJob.Name),
							},
						},
					})

				}
			}

		}

		if len(failures) > 0 {
			preAnalysis[fmt.Sprintf("%s/%s", cronJob.Namespace, cronJob.Name)] = PreAnalysis{
				FailureDetails: failures,
			}
			AnalyzerErrorsMetric.WithLabelValues(kind, cronJob.Name, cronJob.Namespace).Set(float64(len(failures)))

		}

		for key, value := range preAnalysis {
			currentAnalysis := Result{
				Kind:  kind,
				Name:  key,
				Error: value.FailureDetails,
			}
			a.Results = append(a.Results, currentAnalysis)
		}
	}

	return a.Results, nil
}

// Check CRON schedule format
func CheckCronScheduleIsValid(schedule string) (bool, error) {
	_, err := cron.ParseStandard(schedule)
	if err != nil {
		return false, err
	}

	return true, nil
}
