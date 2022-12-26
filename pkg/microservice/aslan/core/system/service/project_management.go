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
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/jira"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetJira(log *zap.SugaredLogger) (*models.ProjectManagement, error) {
	info, err := mongodb.NewProjectManagementColl().GetJira()
	if err != nil {
		log.Errorf("GeJira error:%s", err)
		return nil, err
	}
	return info, nil
}

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

func ListJiraProjects() ([]string, error) {
	info, err := mongodb.NewProjectManagementColl().GetJira()
	if err != nil {
		return nil, err
	}
	return jira.NewJiraClient(info.JiraUser, info.JiraToken, info.JiraHost).Project.ListProjects()
}

func GetJiraTypes(project string) ([]*jira.IssueTypeWithStatus, error) {
	info, err := mongodb.NewProjectManagementColl().GetJira()
	if err != nil {
		return nil, err
	}
	return jira.NewJiraClient(info.JiraUser, info.JiraToken, info.JiraHost).Issue.GetTypes(project)
}

func SearchJiraIssues(project, _type, status string, ne bool) ([]*jira.Issue, error) {
	info, err := mongodb.NewProjectManagementColl().GetJira()
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
	if status != "" {
		if ne {
			jql = append(jql, fmt.Sprintf(`status != "%s"`, status))
		} else {
			jql = append(jql, fmt.Sprintf(`status = "%s"`, status))
		}
	}
	return jira.NewJiraClient(info.JiraUser, info.JiraToken, info.JiraHost).Issue.SearchByJQL(strings.Join(jql, " AND "))
}

func checkType(_type string) error {
	switch _type {
	case setting.PMJira:
	case setting.PMLark:
	default:
		return errors.New("invalid pm type")
	}
	return nil
}
