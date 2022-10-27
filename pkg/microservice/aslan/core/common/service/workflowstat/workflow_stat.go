/*
Copyright 2022 The KodeRover Authors.

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

package workflowstat

import (
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/log"
)

func UpdateWorkflowStat(workflowName, workflowType, status, projectName string, duration int64, isRestart bool) error {
	if isRestart {
		return nil
	}
	if status != string(config.StatusPassed) && status != string(config.StatusFailed) && status != string(config.StatusTimeout) {
		return nil
	}

	totalSuccess := 0
	totalFailure := 0
	if status == string(config.StatusPassed) {
		totalSuccess = 1
		totalFailure = 0
	} else {
		totalSuccess = 0
		totalFailure = 1
	}
	workflowStat := &commonmodels.WorkflowStat{
		ProductName:   projectName,
		Name:          workflowName,
		Type:          workflowType,
		TotalDuration: duration,
		TotalSuccess:  totalSuccess,
		TotalFailure:  totalFailure,
	}
	if err := upsertWorkflowStat(workflowStat); err != nil {
		return err
	}

	return nil
}

func upsertWorkflowStat(args *commonmodels.WorkflowStat) error {
	if workflowStats, _ := commonrepo.NewWorkflowStatColl().FindWorkflowStat(&commonrepo.WorkflowStatArgs{Names: []string{args.Name}, Type: args.Type}); len(workflowStats) > 0 {
		currentWorkflowStat := workflowStats[0]
		currentWorkflowStat.TotalDuration += args.TotalDuration
		currentWorkflowStat.TotalSuccess += args.TotalSuccess
		currentWorkflowStat.TotalFailure += args.TotalFailure
		if err := commonrepo.NewWorkflowStatColl().Upsert(currentWorkflowStat); err != nil {
			log.Errorf("WorkflowStat upsert err:%v", err)
			return err
		}
	} else {
		args.CreatedAt = time.Now().Unix()
		args.UpdatedAt = time.Now().Unix()
		if err := commonrepo.NewWorkflowStatColl().Create(args); err != nil {
			log.Errorf("WorkflowStat create err:%v", err)
			return err
		}
	}
	return nil
}
