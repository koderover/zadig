/*
 * Copyright 2026 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	apolloservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/apollo"
	"github.com/koderover/zadig/v2/pkg/tool/apollo"
)

type ReleaseJobCheckResult struct {
	Passed bool                    `json:"passed"`
	Errors []*ReleaseJobCheckError `json:"errors"`
}

type ReleaseJobCheckError struct {
	JobName   string `json:"job_name"`
	ApolloID  string `json:"apolloID,omitempty"`
	AppID     string `json:"appID,omitempty"`
	Env       string `json:"env,omitempty"`
	ClusterID string `json:"clusterID,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Message   string `json:"message"`
}

func checkReleasePlanJob(ctx context.Context, job *models.ReleaseJob) *ReleaseJobCheckResult {
	if job.Type != config.JobWorkflow {
		return &ReleaseJobCheckResult{Passed: true, Errors: []*ReleaseJobCheckError{}}
	}
	spec := new(models.WorkflowReleaseJobSpec)
	if err := models.IToi(job.Spec, spec); err != nil {
		return &ReleaseJobCheckResult{Errors: []*ReleaseJobCheckError{{
			JobName: job.Name, Message: fmt.Sprintf("invalid workflow spec: %v", err),
		}}}
	}
	return checkWorkflowApolloNamespaces(ctx, spec.Workflow)
}

// Prechecks are read-only. Execution checks again under the namespace lock
// because Apollo can change after this check has completed.
func checkWorkflowApolloNamespaces(ctx context.Context, workflow *models.WorkflowV4) *ReleaseJobCheckResult {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	result := &ReleaseJobCheckResult{Errors: []*ReleaseJobCheckError{}}
	if workflow == nil {
		result.Errors = append(result.Errors, &ReleaseJobCheckError{Message: "workflow is nil"})
		return result
	}
	for _, stage := range workflow.Stages {
		if stage == nil {
			continue
		}
		for _, job := range stage.Jobs {
			if job == nil || job.Skipped || job.JobType != config.JobApollo {
				continue
			}
			spec := new(models.ApolloJobSpec)
			if err := models.IToi(job.Spec, spec); err != nil {
				result.Errors = append(result.Errors, &ReleaseJobCheckError{
					JobName: job.Name, Message: fmt.Sprintf("invalid Apollo spec: %v", err),
				})
				continue
			}
			var client *apollo.Client
			var clientErr error
			loaded := false
			for _, namespace := range spec.NamespaceList {
				if namespace == nil {
					result.Errors = append(result.Errors, &ReleaseJobCheckError{
						JobName: job.Name, ApolloID: spec.ApolloID, Message: "Apollo namespace is nil",
					})
					continue
				}
				if namespace.Action != "" && namespace.Action != models.ApolloActionUpdate {
					continue
				}
				item := &ReleaseJobCheckError{
					JobName: job.Name, ApolloID: spec.ApolloID,
					AppID: namespace.AppID, Env: namespace.Env,
					ClusterID: namespace.ClusterID, Namespace: namespace.Namespace,
				}
				if !loaded {
					info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(ctx, spec.ApolloID)
					clientErr = err
					if err == nil {
						client = apollo.NewClient(info.ServerAddress, info.Token).WithContext(ctx)
					}
					loaded = true
				}
				if clientErr != nil {
					item.Message = fmt.Sprintf("get Apollo configuration failed: %v", clientErr)
				} else if _, err := apolloservice.MergeNamespace(ctx, client, namespace); err != nil {
					item.Message = err.Error()
				}
				if item.Message != "" {
					result.Errors = append(result.Errors, item)
				}
			}
		}
	}
	result.Passed = len(result.Errors) == 0
	return result
}
