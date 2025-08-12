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

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	config2 "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	jira2 "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/jira"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/jira"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/meego"
	"github.com/koderover/zadig/v2/pkg/tool/pingcode"
	"github.com/koderover/zadig/v2/pkg/tool/tapd"
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
	if _, err := mongodb.NewProjectManagementColl().GetBySystemIdentity(pm.SystemIdentity); err == nil {
		return fmt.Errorf("can't set the same system identity")
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

	var oldSystemIdentity string
	oldPm, err := mongodb.NewProjectManagementColl().GetByID(idHex)
	if err == nil {
		oldSystemIdentity = oldPm.SystemIdentity
	}
	if oldPm.SystemIdentity != "" && pm.SystemIdentity != oldSystemIdentity {
		if _, err := mongodb.NewProjectManagementColl().GetBySystemIdentity(pm.SystemIdentity); err == nil {
			return fmt.Errorf("can't set the same system identity")
		}
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
	spec := &models.ProjectManagementJiraSpec{}
	err := models.IToi(info.Spec, spec)
	if err != nil {
		return e.ErrValidateProjectManagement.AddDesc(fmt.Sprintf("failed to convert jira spec, err: %v", err))
	}

	_, err = jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Project.ListProjects()
	if err != nil {
		log.Errorf("Validate jira error: %v", err)
		return e.ErrValidateProjectManagement.AddDesc("failed to validate jira")
	}
	return nil
}

func ValidateMeego(info *models.ProjectManagement) error {
	spec := &models.ProjectManagementMeegoSpec{}
	err := models.IToi(info.Spec, spec)
	if err != nil {
		return e.ErrValidateProjectManagement.AddDesc(fmt.Sprintf("failed to convert meego spec, err: %v", err))
	}

	token, _, err := meego.GetPluginToken(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret)
	if err != nil {
		log.Errorf("Failed to create meego client, error: %s", err)
		return e.ErrValidateProjectManagement.AddDesc("failed to validate meego")
	}
	client := meego.Client{
		Host:         spec.MeegoHost,
		PluginID:     spec.MeegoPluginID,
		PluginSecret: spec.MeegoPluginSecret,
		PluginToken:  token,
		UserKey:      spec.MeegoUserKey,
	}
	_, err = client.GetProjectList()
	if err != nil {
		return e.ErrValidateProjectManagement.AddDesc("failed to validate meego")
	}
	return nil
}

func ValidatePingCode(info *models.ProjectManagement) error {
	spec := &models.ProjectManagementPingCodeSpec{}
	err := models.IToi(info.Spec, spec)
	if err != nil {
		return e.ErrValidateProjectManagement.AddDesc(fmt.Sprintf("failed to convert pingcode spec, err: %v", err))
	}

	client, err := pingcode.NewClient(spec.PingCodeAddress, spec.PingCodeClientID, spec.PingCodeClientSecret)
	if err != nil {
		fmtErr := fmt.Sprintf("failed to new pingcode client, err: %v", err)
		log.Errorf(fmtErr)
		return e.ErrValidateProjectManagement.AddDesc(fmtErr)
	}
	_, err = client.ListProjects()
	if err != nil {
		fmtErr := fmt.Sprintf("failed to list projects, err: %v", err)
		log.Errorf(fmtErr)
		return e.ErrValidateProjectManagement.AddDesc(fmtErr)
	}
	return nil
}

func ValidateTapd(info *models.ProjectManagement) error {
	spec := &models.ProjectManagementTapdSpec{}
	err := models.IToi(info.Spec, spec)
	if err != nil {
		return e.ErrValidateProjectManagement.AddDesc(fmt.Sprintf("failed to convert tapd spec, err: %v", err))
	}

	client, err := tapd.NewClient(spec.TapdAddress, spec.TapdClientID, spec.TapdClientSecret, spec.TapdCompanyID)
	if err != nil {
		fmtErr := fmt.Sprintf("failed to new tapd client, err: %v", err)
		log.Errorf(fmtErr)
		return e.ErrValidateProjectManagement.AddDesc(fmtErr)
	}
	_, err = client.ListProjects()
	if err != nil {
		fmtErr := fmt.Sprintf("failed to list projects, err: %v", err)
		log.Errorf(fmtErr)
		return e.ErrValidateProjectManagement.AddDesc(fmtErr)
	}
	return nil
}

type JiraProjectResp struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func ListJiraProjects(id string) ([]JiraProjectResp, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	list, err := jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Project.ListProjects()
	if err != nil {
		return nil, err
	}

	var resp []JiraProjectResp
	for _, project := range list {
		resp = append(resp, JiraProjectResp{
			Name: project.Name,
			Key:  project.Key,
		})
	}
	return resp, nil
}

type JiraBoardResp struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func ListJiraBoards(id, projectKey string) ([]JiraBoardResp, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	boards, err := jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Board.ListBoards(projectKey, "scrum")
	if err != nil {
		return nil, err
	}

	var resp []JiraBoardResp
	for _, board := range boards {
		resp = append(resp, JiraBoardResp{
			ID:   board.ID,
			Name: board.Name,
			Type: board.Type,
		})
	}
	return resp, nil
}

type JiraSprintResp struct {
	ID            int       `json:"id,omitempty"`
	State         string    `json:"state,omitempty"`
	Name          string    `json:"name,omitempty"`
	StartDate     time.Time `json:"startDate,omitempty"`
	EndDate       time.Time `json:"endDate,omitempty"`
	CompleteDate  time.Time `json:"completeDate,omitempty"`
	OriginBoardID int       `json:"originBoardId,omitempty"`
}

func ListJiraSprints(id string, boardID int) ([]JiraSprintResp, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	sprints, err := jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Sprint.ListSprints(boardID, "")
	if err != nil {
		return nil, err
	}

	var resp []JiraSprintResp
	for _, sprint := range sprints {
		resp = append(resp, JiraSprintResp{
			ID:            sprint.ID,
			State:         sprint.State,
			Name:          sprint.Name,
			StartDate:     sprint.StartDate,
			EndDate:       sprint.EndDate,
			CompleteDate:  sprint.CompleteDate,
			OriginBoardID: sprint.OriginBoardID,
		})
	}
	return resp, nil
}

func GetJiraSprint(id string, sprintID int) (*JiraSprintResp, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	sprint, err := jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Sprint.GetSrpint(sprintID)
	if err != nil {
		return nil, err
	}

	resp := &JiraSprintResp{
		ID:            sprint.ID,
		State:         sprint.State,
		Name:          sprint.Name,
		StartDate:     sprint.StartDate,
		EndDate:       sprint.EndDate,
		CompleteDate:  sprint.CompleteDate,
		OriginBoardID: sprint.OriginBoardID,
	}
	return resp, nil
}

type JiraSprintIssuesResp struct {
	SprintID      int    `json:"sprintID"`
	Name          string `json:"name"`
	StartDate     int64  `json:"startDate"`
	EndDate       int64  `json:"endDate"`
	New           int    `json:"new"`
	Indeterminate int    `json:"indeterminate"`
	Done          int    `json:"done"`
}

func ListJiraSprintIssues(id string, sprintID int) (*JiraSprintIssuesResp, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	sprintSerivce := jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Sprint

	resp := &JiraSprintIssuesResp{}
	sprint, err := sprintSerivce.GetSrpint(sprintID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint, sprintID: %v, err: %v", sprintID, err)
	}
	resp.SprintID = sprint.ID
	resp.Name = sprint.Name
	resp.StartDate = sprint.StartDate.Unix()
	resp.EndDate = sprint.EndDate.Unix()

	issues, err := sprintSerivce.ListSprintIssues(sprintID, "status")
	if err != nil {
		return nil, fmt.Errorf("failed to get sprint issues, srpintID: %v, err: %v", sprintID, err)
	}

	for _, issue := range issues {
		if issue.Fields == nil {
			continue
		}
		if issue.Fields.Status == nil {
			continue
		}
		if issue.Fields.Status.StatusCategory == nil {
			continue
		}

		switch issue.Fields.Status.StatusCategory.Key {
		case "new":
			resp.New++
		case "indeterminate":
			resp.Indeterminate++
		case "done":
			resp.Done++
		}
	}

	return resp, nil
}

func GetJiraTypes(id, project string) ([]*jira.IssueTypeWithStatus, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	return jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Issue.GetTypes(project)
}

func GetJiraProjectStatus(id, project string) ([]string, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	return jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Project.ListProjectStatues(project)
}

func GetJiraAllStatus(id string) ([]*jira.Status, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	return jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Platform.ListAllStatues()
}

func SearchJiraIssues(id, project, _type, status, summary string, ne bool) ([]*jira.Issue, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
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
	return jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Issue.SearchByJQL(strings.Join(jql, " AND "), summary != "")
}

func SearchJiraProjectIssuesWithJQL(id, project, jql, summary string) ([]*jira.Issue, error) {
	spec, err := mongodb.NewProjectManagementColl().GetJiraSpec(id)
	if err != nil {
		return nil, err
	}

	jql = fmt.Sprintf(`project = "%s" AND (%s)`, project, jql)
	if summary != "" {
		jql += fmt.Sprintf(` AND summary ~ "%s"`, summary)
	}
	return jira.NewJiraClientWithAuthType(spec.JiraHost, spec.JiraUser, spec.JiraToken, spec.JiraPersonalAccessToken, spec.JiraAuthType).Issue.SearchByJQL(jql, true)
}

func HandleJiraHookEvent(workflowName, hookName string, event *jira.Event, logger *zap.SugaredLogger) error {
	if event.Issue == nil || event.Issue.Key == "" {
		logger.Errorf("HandleJiraHookEvent: nil issue or issue key, skip")
		return nil
	}
	_, err := mongodb.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}

	jiraHook, err := mongodb.NewWorkflowV4JiraHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find Jira hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}

	if !jiraHook.Enabled {
		errMsg := fmt.Sprintf("Not enabled Jira hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}

	if jiraHook.EnabledIssueStatusChange {
		if event.ChangeLog == nil || len(event.ChangeLog.Items) == 0 {
			logger.Errorf("HandleJiraHookEvent: nil change log or change log items, skip")
			return nil
		}

		statusChangeMatched := false
		changelog := &jira.ChangeLogItem{}
		for _, item := range event.ChangeLog.Items {
			if item.Field != "status" || item.FieldType != "jira" {
				continue
			}

			if jiraHook.FromStatus.ID == "000000" {
				if item.To == jiraHook.ToStatus.ID {
					statusChangeMatched = true
				}
			} else {
				if item.From == jiraHook.FromStatus.ID && item.To == jiraHook.ToStatus.ID {
					statusChangeMatched = true
				}
			}

			changelog = item
			break
		}

		if !statusChangeMatched {
			logger.Infof("HandleJiraHookEvent: hook %s/%s status change not matched, skip. status changelog: %+v", workflowName, hookName, changelog)
			return nil
		}
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
	_, err := mongodb.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find WorkflowV4: %s, the error is: %v", workflowName, err)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}

	meegoHook, err := mongodb.NewWorkflowV4MeegoHookColl().Get(internalhandler.NewBackgroupContext(), workflowName, hookName)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to find Meego hook %s", hookName)
		logger.Error(errMsg)
		return errors.New(errMsg)
	}
	if !meegoHook.Enabled {
		errMsg := fmt.Sprintf("Not enabled Meego hook %s", hookName)
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
		spec, err := mongodb.NewProjectManagementColl().GetMeegoSpec(meegoHook.MeegoID)
		if err != nil {
			log.Errorf("failed to get meego info to create comment, error: %s", err)
			return
		}

		meegoClient, err := meego.NewClient(spec.MeegoHost, spec.MeegoPluginID, spec.MeegoPluginSecret, spec.MeegoUserKey)
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

func checkType(_type setting.ProjectManagementType) error {
	switch _type {
	case setting.ProjectManagementTypeJira, setting.ProjectManagementTypeMeego, setting.ProjectManagementTypePingCode, setting.ProjectManagementTypeTapd:
	default:
		return errors.New("invalid pm type")
	}
	return nil
}
