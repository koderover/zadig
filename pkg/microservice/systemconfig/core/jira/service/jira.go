/*
Copyright 2021 The KodeRover Authors.

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

package service

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/models"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/jira/repository/mongodb"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/koderover/zadig/pkg/tool/jira"
)

func GeJira(encryptedKey string, log *zap.SugaredLogger) (*models.Jira, error) {
	jira, err := mongodb.NewJiraColl().GetJira()
	if err != nil {
		log.Errorf("GeJira error:%s", err)
		return nil, err
	}
	if jira == nil {
		return nil, nil
	}
	aesKey, err := aslan.New(config.AslanServiceAddress()).GetTextFromEncryptedKey(encryptedKey)
	if err != nil {
		log.Errorf("GeJira GetTextFromEncryptedKey erorr:%s", err)
		return nil, err
	}
	jira.AccessToken, err = crypto.AesEncryptByKey(jira.AccessToken, aesKey.PlainText)
	if err != nil {
		log.Errorf("GeJira AesEncryptByKey erorr:%s", err)
		return nil, err
	}
	return jira, nil
}

func GeJiraInternal(_ *zap.SugaredLogger) (*models.Jira, error) {
	return mongodb.NewJiraColl().GetJira()
}

func CreateJira(jira *models.Jira, _ *zap.SugaredLogger) (*models.Jira, error) {
	jira.CreatedAt = time.Now().Unix()
	jira.UpdatedAt = time.Now().Unix()
	return mongodb.NewJiraColl().AddJira(jira)
}

func UpdateJira(jira *models.Jira, _ *zap.SugaredLogger) (*models.Jira, error) {
	jira.UpdatedAt = time.Now().Unix()
	return mongodb.NewJiraColl().UpdateJira(jira)
}

func DeleteJira(_ *zap.SugaredLogger) error {
	return mongodb.NewJiraColl().DeleteJira()
}

func ValidateJira(info *models.Jira) error {
	_, err := jira.NewJiraClient(info.User, info.AccessToken, info.Host).Project.ListProjects()
	return err
}

func ListJiraProjects() ([]string, error) {
	info, err := mongodb.NewJiraColl().GetJira()
	if err != nil {
		return nil, err
	}
	return jira.NewJiraClient(info.User, info.AccessToken, info.Host).Project.ListProjects()
}

func GetJiraTypes(project string) ([]*jira.IssueTypeWithStatus, error) {
	info, err := mongodb.NewJiraColl().GetJira()
	if err != nil {
		return nil, err
	}
	return jira.NewJiraClient(info.User, info.AccessToken, info.Host).Issue.GetTypes(project)
}

func SearchJiraIssues(project, _type, status string, ne bool) ([]*jira.Issue, error) {
	info, err := mongodb.NewJiraColl().GetJira()
	if err != nil {
		return nil, err
	}
	jql := []string{}
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
	return jira.NewJiraClient(info.User, info.AccessToken, info.Host).Issue.SearchByJQL(strings.Join(jql, " AND "))
}
