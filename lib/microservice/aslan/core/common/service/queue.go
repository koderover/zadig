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
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func List(log *xlog.Logger) []*models.Queue {
	tasks, err := repo.NewQueueColl().List(&repo.ListQueueOption{})
	if err != nil {
		log.Errorf("pqColl.List error: %v", err)
		return nil
	}
	return tasks
}

func Remove(task *models.Queue, log *xlog.Logger) error {
	if err := repo.NewQueueColl().Delete(task); err != nil {
		log.Errorf("pqColl.Delete error: %v", err)
		return err
	}
	return nil
}
