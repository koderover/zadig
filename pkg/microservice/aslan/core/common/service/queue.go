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

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

func List(log *zap.SugaredLogger) []*models.Queue {
	tasks, err := mongodb.NewQueueColl().List(&mongodb.ListQueueOption{})
	if err != nil {
		log.Errorf("pqColl.List error: %v", err)
		return nil
	}
	return tasks
}

func Remove(task *models.Queue, log *zap.SugaredLogger) error {
	if err := mongodb.NewQueueColl().Delete(task); err != nil {
		log.Errorf("pqColl.Delete error: %v", err)
		return err
	}
	return nil
}
