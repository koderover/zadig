/*
Copyright 2026 The KodeRover Authors.

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

package workflowcontroller

import (
	"fmt"
	"net/url"
	"strings"

	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/jira"
)

const (
	jiraFieldValueSourceWorkflowURL = "workflow_url"
)

func updateJiraFieldsForWorkflowTask(task *commonmodels.WorkflowTask, logger *zap.SugaredLogger) {
	if task == nil {
		return
	}

	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != string(config.JobJira) {
				continue
			}

			jobTaskSpec := &commonmodels.JobTaskJiraSpec{}
			if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
				logger.Errorf("failed to convert jira job spec for job %s: %v", job.Name, err)
				continue
			}
			if len(jobTaskSpec.Issues) == 0 || len(jobTaskSpec.FieldMappings) == 0 {
				continue
			}

			fields := buildJiraIssueFields(task, jobTaskSpec.FieldMappings)
			if len(fields) == 0 {
				continue
			}

			spec, err := commonrepo.NewProjectManagementColl().GetJiraSpec(jobTaskSpec.JiraID)
			if err != nil {
				logger.Errorf("failed to get jira spec for job %s: %v", job.Name, err)
				continue
			}
			client := jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType)

			for _, issue := range jobTaskSpec.Issues {
				if issue == nil || strings.TrimSpace(issue.Key) == "" {
					continue
				}
				if err := client.Issue.UpdateFields(issue.Key, fields); err != nil {
					logger.Errorf("failed to update jira issue %s fields for workflow %s task %d: %v", issue.Key, task.WorkflowName, task.TaskID, err)
				}
			}
		}
	}
}

func addJiraCommentForWorkflowTask(task *commonmodels.WorkflowTask, logger *zap.SugaredLogger) {
	if task == nil {
		return
	}

	for _, stage := range task.Stages {
		for _, job := range stage.Jobs {
			if job.JobType != string(config.JobJira) {
				continue
			}

			jobTaskSpec := &commonmodels.JobTaskJiraSpec{}
			if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
				logger.Errorf("failed to convert jira job spec for job %s: %v", job.Name, err)
				continue
			}
			if len(jobTaskSpec.Issues) == 0 {
				continue
			}

			spec, err := commonrepo.NewProjectManagementColl().GetJiraSpec(jobTaskSpec.JiraID)
			if err != nil {
				logger.Errorf("failed to get jira spec for job %s: %v", job.Name, err)
				continue
			}
			client := jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType)

			for _, issue := range jobTaskSpec.Issues {
				if issue == nil || strings.TrimSpace(issue.Key) == "" {
					continue
				}
				fillJiraIssueCurrentStatus(issue, client, logger)
				if err := client.Issue.AddCommentV2(issue.Key, buildJiraWorkflowTaskComment(task, issue)); err != nil {
					logger.Errorf("failed to add jira issue %s workflow task comment for workflow %s task %d: %v", issue.Key, task.WorkflowName, task.TaskID, err)
				}
			}
		}
	}
}

func buildJiraIssueFields(task *commonmodels.WorkflowTask, mappings []*commonmodels.JiraFieldMapping) map[string]interface{} {
	fields := make(map[string]interface{})
	if task == nil {
		return fields
	}

	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		fieldID := strings.TrimSpace(mapping.JiraFieldID)
		if fieldID == "" {
			continue
		}

		value, ok := jiraFieldValue(task, mapping.ValueSource)
		if !ok {
			continue
		}
		fields[fieldID] = value
	}
	return fields
}

func jiraFieldValue(task *commonmodels.WorkflowTask, valueSource string) (string, bool) {
	switch valueSource {
	case jiraFieldValueSourceWorkflowURL:
		return workflowURL(task), true
	default:
		return "", false
	}
}

func workflowDisplayName(task *commonmodels.WorkflowTask) string {
	if task.WorkflowDisplayName != "" {
		return task.WorkflowDisplayName
	}
	return task.WorkflowName
}

func workflowTaskStatusIcon(status config.Status) string {
	if status == config.StatusPassed {
		return "✅"
	}
	return "❌"
}

func workflowURL(task *commonmodels.WorkflowTask) string {
	return fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s?display_name=%s",
		configbase.SystemAddress(),
		task.ProjectName,
		task.WorkflowName,
		url.QueryEscape(workflowDisplayName(task)),
	)
}

func workflowTaskURL(task *commonmodels.WorkflowTask) string {
	return fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s",
		configbase.SystemAddress(),
		task.ProjectName,
		task.WorkflowName,
		task.TaskID,
		url.QueryEscape(workflowDisplayName(task)),
	)
}

func buildJiraWorkflowTaskComment(task *commonmodels.WorkflowTask, issue *commonmodels.IssueID) string {
	return fmt.Sprintf("JIRA 状态：%s\n%s 工作流：%s\n任务链接：%s",
		jiraIssueStatusForComment(issue),
		workflowTaskStatusIcon(task.Status),
		workflowDisplayName(task),
		workflowTaskURL(task),
	)
}

func jiraIssueStatusForComment(issue *commonmodels.IssueID) string {
	if issue == nil {
		return ""
	}
	return issue.CurrentStatus
}

func fillJiraIssueCurrentStatus(issue *commonmodels.IssueID, client *jira.Client, logger *zap.SugaredLogger) {
	if issue == nil || client == nil || strings.TrimSpace(issue.CurrentStatus) != "" {
		return
	}

	jiraIssue, err := client.Issue.GetByKeyOrID(issue.Key, "status")
	if err != nil {
		logger.Errorf("failed to get jira issue %s current status for workflow task comment: %v", issue.Key, err)
		return
	}
	if jiraIssue != nil && jiraIssue.Fields != nil && jiraIssue.Fields.Status != nil {
		issue.CurrentStatus = jiraIssue.Fields.Status.Name
	}
}
