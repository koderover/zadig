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
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	apolloservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/apollo"
	"github.com/koderover/zadig/v2/pkg/tool/apollo"
)

type ReleaseJobCheckResult struct {
	Passed bool     `json:"passed"`
	Errors []string `json:"errors"`
}

func checkReleasePlanJob(ctx context.Context, job *models.ReleaseJob) *ReleaseJobCheckResult {
	if job.Type != config.JobWorkflow {
		return &ReleaseJobCheckResult{Passed: true, Errors: []string{}}
	}
	spec := new(models.WorkflowReleaseJobSpec)
	if err := models.IToi(job.Spec, spec); err != nil {
		return &ReleaseJobCheckResult{Errors: []string{fmt.Sprintf("%s：工作流配置解析失败：%v", job.Name, err)}}
	}
	return checkWorkflowApolloNamespaces(ctx, spec.Workflow)
}

// Prechecks are read-only. Execution checks again under the namespace lock
// because Apollo can change after this check has completed.
func checkWorkflowApolloNamespaces(ctx context.Context, workflow *models.WorkflowV4) *ReleaseJobCheckResult {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	result := &ReleaseJobCheckResult{Errors: []string{}}
	if workflow == nil {
		result.Errors = append(result.Errors, "工作流不存在，无法检查发布配置。")
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
				result.Errors = append(result.Errors, fmt.Sprintf("%s：Apollo 任务配置解析失败：%v", job.Name, err))
				continue
			}
			var client *apollo.Client
			var clientErr error
			loaded := false
			for _, namespace := range spec.NamespaceList {
				if namespace == nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s：Apollo 命名空间配置为空，请重新编辑并保存。", job.Name))
					continue
				}
				if namespace.Action != "" && namespace.Action != models.ApolloActionUpdate {
					continue
				}
				if !loaded {
					info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(ctx, spec.ApolloID)
					clientErr = err
					if err == nil {
						client = apollo.NewClient(info.ServerAddress, info.Token).WithContext(ctx)
					}
					loaded = true
				}
				message := ""
				if clientErr != nil {
					message = fmt.Sprintf("读取 Apollo 配置失败：%v", clientErr)
				} else if _, err := apolloservice.MergeNamespace(ctx, client, namespace); err != nil {
					if strings.Contains(err.Error(), "configuration merge conflict") {
						message = "Apollo 配置已更新，自动合并发生冲突，请重新编辑并保存。"
					} else {
						message = fmt.Sprintf("配置检查失败：%v", err)
					}
				}
				if message != "" {
					result.Errors = append(result.Errors, fmt.Sprintf("%s:%s:%s:%s：%s", namespace.AppID, namespace.Env, namespace.ClusterID, namespace.Namespace, message))
				}
			}
		}
	}
	result.Passed = len(result.Errors) == 0
	return result
}
