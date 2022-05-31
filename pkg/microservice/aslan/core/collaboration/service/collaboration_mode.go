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
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/collaboration/repository/mongodb"
)

func CreateCollaborationMode(userName string, collaborationMode *models.CollaborationMode, logger *zap.SugaredLogger) error {
	err := mongodb.NewCollaborationModeColl().Create(userName, collaborationMode)
	if err != nil {
		logger.Errorf("CreateCollaborationMode error, err msg:%s", err)
		return err
	}
	return nil
}

func UpdateCollaborationMode(userName string, collaborationMode *models.CollaborationMode, logger *zap.SugaredLogger) error {
	err := mongodb.NewCollaborationModeColl().Update(userName, collaborationMode)
	if err != nil {
		logger.Errorf("UpdateCollaborationMode error, err msg:%s", err)
		return err
	}
	return nil
}

func DeleteCollaborationMode(username, projectName, name string, logger *zap.SugaredLogger) error {
	err := mongodb.NewCollaborationModeColl().Delete(username, projectName, name)
	if err != nil {
		logger.Errorf("UpdateCollaborationMode error, err msg:%s", err)
		return err
	}
	return mongodb.NewCollaborationInstanceColl().ResetRevision(name, projectName)
}
