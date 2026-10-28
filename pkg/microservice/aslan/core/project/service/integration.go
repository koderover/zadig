/*
Copyright 2023 The KodeRover Authors.

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
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/mongodb"
	systemconfig_codehost_service "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func CreateProjectCodeHost(projectName string, codehost *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	if codehost.Type == setting.SourceFromOther {
		codehost.IsReady = "2"
	}
	if codehost.Type == setting.SourceFromGerrit {
		codehost.IsReady = "2"
		codehost.AccessToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", codehost.Username, codehost.Password)))
	}

	if codehost.Alias != "" {
		if _, err := mongodb.NewCodehostColl().GetProjectCodeHostByAlias(projectName, codehost.Alias); err == nil {
			return nil, fmt.Errorf("alias cannot have the same name")
		}
	}

	if err := systemconfig_codehost_service.ValidateCodeHostBot(codehost); err != nil {
		return nil, err
	}

	codehost.CreatedAt = time.Now().Unix()
	codehost.UpdatedAt = time.Now().Unix()

	list, err := mongodb.NewCodehostColl().CodeHostList()
	if err != nil {
		return nil, err
	}
	codehost.ID = len(list) + 1
	return mongodb.NewCodehostColl().AddProjectCodeHost(projectName, codehost)
}

func GetProjectCodehostList(projectName, encryptedKey, address, owner, source string, log *zap.SugaredLogger) ([]*models.CodeHost, error) {
	codeHosts, err := mongodb.NewCodehostColl().List(&mongodb.ListArgs{
		IntegrationLevel: setting.IntegrationLevelProject,
		Project:          projectName,
		Address:          address,
		Owner:            owner,
		Source:           source,
	})
	if err != nil {
		log.Errorf("ListCodeHost error:%s", err)
		return nil, err
	}
	return systemconfig_codehost_service.EncypteCodeHost(encryptedKey, codeHosts, log)
}

func DeleteProjectCodeHosts(projectName string) error {
	return mongodb.NewCodehostColl().DeleteProjectCodeHosts(projectName)
}

func DeleteProjectCodeHostByID(projectName string, id int, _ *zap.SugaredLogger) error {
	return mongodb.NewCodehostColl().DeleteProjectCodeHostByID(projectName, id)
}

func UpdateProjectSystemCodeHost(projectName string, host *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	if host.Type == setting.SourceFromGerrit {
		host.AccessToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", host.Username, host.Password)))
	}

	oldCodeHost, err := mongodb.NewCodehostColl().GetProjectCodeHostByID(host.ID, projectName, false)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, e.ErrNotFound.AddDesc(fmt.Sprintf("代码源 %d 不存在", host.ID))
		}
		return nil, err
	}
	if host.Alias != "" && host.Alias != oldCodeHost.Alias {
		if _, err := mongodb.NewCodehostColl().GetProjectCodeHostByAlias(projectName, host.Alias); err == nil {
			return nil, fmt.Errorf("alias cannot have the same name")
		}
	}

	if err := systemconfig_codehost_service.ValidateCodeHostBot(host); err != nil {
		return nil, err
	}

	return mongodb.NewCodehostColl().UpdateProjectCodeHost(projectName, host)
}

func UpdateCodeHostByToken(host *models.CodeHost, _ *zap.SugaredLogger) (*models.CodeHost, error) {
	return mongodb.NewCodehostColl().UpdateCodeHostToken(host)
}

func GetProjectCodeHost(id int, projectName string, ignoreDelete bool, encryptedKey string, log *zap.SugaredLogger) (*models.CodeHost, error) {
	codeHost, err := mongodb.NewCodehostColl().GetProjectCodeHostByID(id, projectName, ignoreDelete)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, e.ErrNotFound.AddDesc(fmt.Sprintf("代码源 %d 不存在", id))
		}
		return nil, err
	}
	codeHosts, err := systemconfig_codehost_service.EncypteCodeHost(encryptedKey, []*models.CodeHost{codeHost}, log)
	if err != nil {
		return nil, err
	}
	return codeHosts[0], nil
}

func GetAvailableCodehostList(projectName, encryptedKey string, log *zap.SugaredLogger) ([]*models.CodeHost, error) {
	codeHosts, err := mongodb.NewCodehostColl().AvailableCodeHost(projectName)
	if err != nil {
		log.Errorf("ListCodeHost error:%s", err)
		return nil, err
	}
	return systemconfig_codehost_service.EncypteCodeHost(encryptedKey, codeHosts, log)
}
