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
	"encoding/json"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/nsq"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type CronjobPayload struct {
	Name        string             `json:"name"`
	ProductName string             `json:"product_name"`
	Action      string             `json:"action"`
	JobType     string             `json:"job_type"`
	DeleteList  []string           `json:"delete_list,omitempty"`
	JobList     []*models.Schedule `json:"job_list,omitempty"`
}

func RemoveCronjob(workflowName string, log *xlog.Logger) error {
	err := repo.NewCronjobColl().Delete(&repo.CronjobDeleteOption{ParentName: workflowName, ParentType: config.WorkflowCronjob})
	if err != nil {
		// FIXME: HOW TO DO THIS
		log.Errorf("Failed to delete %s 's cronjob, the error is: %v", workflowName, err)
		//return e.ErrDeleteWorkflow.AddDesc(err.Error())
	}
	payload := CronjobPayload{
		Name:    workflowName,
		JobType: config.WorkflowCronjob,
		Action:  setting.TypeDisableCronjob,
	}
	pl, _ := json.Marshal(payload)
	err = nsq.Publish(config.TopicCronjob, pl)
	if err != nil {
		log.Errorf("Failed to publish to nsq topic: %s, the error is: %v", config.TopicCronjob, err)
		return e.ErrUpsertCronjob.AddDesc(err.Error())
	}

	return nil
}
