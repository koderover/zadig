/*
 * Copyright 2022 The KodeRover Authors.
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
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/tool/meego"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	config2 "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	jira2 "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/jira"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/jira"
	"github.com/koderover/zadig/pkg/tool/log"
)

func ListProjectManagement(log *zap.SugaredLogger) ([]*models.ProjectManagement, error) {
	list, err := mongodb.NewProjectManagementColl().List()
	if err != nil {
		log.Errorf("List project management error: %v", err)
		return nil, e.ErrListProjectManagement.AddErr(err)
	}
	return list, nil
}

func CreateProjectManagement(pm *models.ProjectManagement, log *zap.SugaredLogger) error {
	if err := checkType(pm.Type); err != nil {
		return err
	}
	if err := mongodb.NewProjectManagementColl().Create(pm); err != nil {
		log.Errorf("Create project management error: %v", err)
		return e.ErrCreateProjectManagement.AddErr(err)
	}
	return nil
}

func UpdateProjectManagement(idHex string, pm *models.ProjectManagement, log *zap.SugaredLogger) error {
	if err := checkType(pm.Type); err != nil {
		return err
	}
	if err := mongodb.NewProjectManagementColl().UpdateByID(idHex, pm); err != nil {
		log.Errorf("Update project management error: %v", err)
		return e.ErrUpdateProjectManagement.AddErr(err)
	}
	return nil
}

func DeleteProjectManagement(idHex string, log *zap.SugaredLogger) error {
	if err := mongodb.NewProjectManagementColl().DeleteByID(idHex); err != nil {
		log.Errorf("Delete project management error: %v", err)
		return e.ErrDeleteProjectManagement.AddErr(err)
	}
	return nil
}

func ValidateJira(info *models.ProjectManagement) error {
	_, err := jira.NewJiraClient(info.JiraUser, info.JiraToken, info.JiraHost).Project.ListProjects()
	if err != nil {
		log.Errorf("Validate jira error: %v", err)
		return e.ErrValidateProjectManagement.AddDesc("failed to validate jira")
	}
	return nil
}

func ValidateMeego(info *models.ProjectManagement) error {
	token, _, err := meego.GetPluginToken(info.MeegoHost, info.MeegoPluginID, info.MeegoPluginSecret)
	if err != nil {
		log.Errorf("Failed to create meego client, error: %s", err)
		return e.ErrValidateProjectManagement.AddDesc("failed to validate meego")
	}
	client := meego.Client{
		Host:         info.MeegoHost,
		PluginID:     info.MeegoPluginID,
		PluginSecret: info.MeegoPluginSecret,
		PluginToken:  token,
		UserKey:      info.MeegoUserKey,
	}
	_, err = client.GetProjectList()
	if err != nil {
		return e.ErrValidateProjectManagement.AddDesc("failed to validate meego")
	}
	return nil
}

func ListJiraProjects() ([]string, error) {
	info, err := mongodb.NewProjectManagementColl().GetJira()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return jira.NewJiraClient(info.JiraUser, info.JiraToken, info.JiraHost).Project.ListProjects()
}

func GetJiraTypes(project string) ([]*jira.IssueTypeWithStatus, error) {
	info, err := mongodb.NewProjectManagementColl().GetJira()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return jira.NewJiraClient(info.JiraUser, info.JiraToken, info.JiraHost).Issue.GetTypes(project)
}

func SearchJiraIssues(project, _type, status, summary string, ne bool) ([]*jira.Issue, error) {
	info, err := mongodb.NewProjectManagementColl().GetJira()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	var jql []string
	if project != "" {
		jql = append(jql, fmt.Sprintf(`project = "%s"`, project))
	}
	if _type != "" {
		jql = append(jql, fmt.Sprintf(`type = "%s"`, _type))
	}
	if summary != "" {
		jql = append(jql, fmt.Sprintf(`summary ~ "%s"`, summary))
	}
	if status != "" {
		if ne {
			jql = append(jql, fmt.Sprintf(`status != "%s"`, status))
		} else {
			jql = append(jql, fmt.Sprintf(`status = "%s"`, status))
		}
	}
	// Search all results only if the summary query exist
	return jira.NewJiraClient(info.JiraUser, info.JiraToken, info.JiraHost).Issue.SearchByJQL(strings.Join(jql, " AND "), summary != "")
}

func HandleJiraHookEvent(workflowName, hookName string, event *jira.Event, logger *zap.SugaredLogger) error {
	if event.Issue == nil || event.Issue.Key == "" {
		logger.Errorf("HandleJiraHookEvent: nil issue or issue key, skip")
		return nil
	}
	workflowInfo, err := mongodb.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	var jiraHook *models.JiraHook
	for _, hook := range workflowInfo.JiraHookCtls {
		if hook.Name == hookName {
			jiraHook = hook
			break
		}
	}
	if jiraHook == nil {
		errMsg := fmt.Sprintf("Failed to find Jira hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	if !jiraHook.Enabled {
		errMsg := fmt.Sprintf("Not enabled Jira hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	taskInfo, err := workflow.CreateWorkflowTaskV4ByBuildInTrigger(setting.JiraHookTaskCreator, jiraHook.WorkflowArg, logger)
	if err != nil {
		errMsg := fmt.Sprintf("HandleJiraHookEvent: failed to create workflow task: %s", err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	logger.With(
		"issue-key", event.Issue.Key,
		"workflow", workflowName,
		"hook", hookName,
	).Infof("HandleJiraHookEvent: create workflow success")
	go func() {
		for {
			time.Sleep(5 * time.Second)
			task, err := mongodb.NewworkflowTaskv4Coll().Find(taskInfo.WorkflowName, taskInfo.TaskID)
			if err != nil {
				log.Errorf("HandleJiraHookEventWaiter: failed to find task %s-%d, err: %v", taskInfo.WorkflowName, taskInfo.TaskID, err)
				return
			}
			if lo.Contains([]config.Status{config.StatusPassed, config.StatusFailed, config.StatusTimeout, config.StatusCancelled, config.StatusReject}, task.Status) {
				statusMap := map[config.Status]string{
					config.StatusPassed:    "成功",
					config.StatusFailed:    "失败",
					config.StatusTimeout:   "超时",
					config.StatusCancelled: "取消",
					config.StatusReject:    "拒绝",
				}
				msg := ""
				if task.Status == config.StatusPassed {
					msg += "✅ "
				} else {
					msg += "❌ "
				}
				link := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", config2.SystemAddress(),
					task.ProjectName,
					task.WorkflowName,
					task.TaskID,
					url.QueryEscape(task.WorkflowDisplayName),
				)
				msg += fmt.Sprintf("Zadig 工作流执行%s: [%s|%s]", statusMap[task.Status], task.WorkflowDisplayName, link)
				err := jira2.SendComment(event.Issue.Key, msg)
				if err != nil {
					log.Errorf("HandleJiraHookEventWaiter: send jira comment error: %v", err)
					return
				}
				log.Infof("HandleJiraHookEventWaiter: send jira issue %s comment success", event.Issue.Key)
				return
			}
		}
	}()
	return nil
}

func HandleMeegoHookEvent(workflowName, hookName string, event *meego.GeneralWebhookRequest, logger *zap.SugaredLogger) error {
	workflowInfo, err := mongodb.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	var meegoHook *models.MeegoHook
	for _, hook := range workflowInfo.MeegoHookCtls {
		if hook.Name == hookName {
			meegoHook = hook
			break
		}
	}
	if meegoHook == nil {
		errMsg := fmt.Sprintf("Failed to find Jira hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	if !meegoHook.Enabled {
		errMsg := fmt.Sprintf("Not enabled Jira hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	taskInfo, err := workflow.CreateWorkflowTaskV4ByBuildInTrigger(setting.MeegoHookTaskCreator, meegoHook.WorkflowArg, logger)
	if err != nil {
		errMsg := fmt.Sprintf("HandleMeegoHookEvent: failed to create workflow task: %s", err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	logger.With(
		"work item id:", event.Payload.ID,
		"workflow", workflowName,
		"hook", hookName,
	).Infof("HandleMeegoHookEvent: create workflow success")
	go func() {
		meegoInfo, err := mongodb.NewProjectManagementColl().GetMeego()
		if err != nil {
			log.Errorf("failed to get meego info to create comment, error: %s", err)
			return
		}
		meegoClient, err := meego.NewClient(meegoInfo.MeegoHost, meegoInfo.MeegoPluginID, meegoInfo.MeegoPluginSecret, meegoInfo.MeegoUserKey)
		if err != nil {
			log.Errorf("failed to create meego client to create comment, error: %s", err)
			return
		}
		for {
			time.Sleep(5 * time.Second)
			task, err := mongodb.NewworkflowTaskv4Coll().Find(taskInfo.WorkflowName, taskInfo.TaskID)
			if err != nil {
				log.Errorf("HandleMeegoHookEventWaiter: failed to find task %s-%d, err: %v", taskInfo.WorkflowName, taskInfo.TaskID, err)
				return
			}
			if lo.Contains([]config.Status{config.StatusPassed, config.StatusFailed, config.StatusTimeout, config.StatusCancelled, config.StatusReject}, task.Status) {
				statusMap := map[config.Status]string{
					config.StatusPassed:    "成功",
					config.StatusFailed:    "失败",
					config.StatusTimeout:   "超时",
					config.StatusCancelled: "取消",
					config.StatusReject:    "拒绝",
				}
				msg := ""
				if task.Status == config.StatusPassed {
					msg += "✅ "
				} else {
					msg += "❌ "
				}
				link := fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", config2.SystemAddress(),
					task.ProjectName,
					task.WorkflowName,
					task.TaskID,
					url.QueryEscape(task.WorkflowDisplayName),
				)
				msg += fmt.Sprintf("Zadig 工作流执行%s: %s", statusMap[task.Status], link)
				_, err := meegoClient.Comment(event.Payload.ProjectKey, event.Payload.WorkItemTypeKey, event.Payload.ID, msg)
				if err != nil {
					log.Errorf("HandleMeegoHookEventWaiter: send meego comment error: %v", err)
				}
				log.Infof("HandleMeegoHookEventWaiter: send meego item %s comment success", event.Payload.ID)
				return
			}
		}
	}()
	return nil
}

func checkType(_type string) error {
	switch _type {
	case setting.PMJira:
	case setting.PMLark:
	case setting.PMMeego:
	default:
		return errors.New("invalid pm type")
	}
	return nil
}
