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
	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func DisableCronjob(name, ptype string, log *xlog.Logger) error {
	jobList, err := commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
		ParentName: name,
		ParentType: ptype,
	})
	if err != nil {
		log.Errorf("Failed to get job list, the error is: %v", err)
		return err
	}
	for _, job := range jobList {
		err := commonrepo.NewCronjobColl().Update(&commonmodels.Cronjob{
			ID:           job.ID,
			Name:         job.Name,
			Type:         job.Type,
			Number:       job.Number,
			Frequency:    job.Frequency,
			Time:         job.Time,
			Cron:         job.Cron,
			MaxFailure:   job.MaxFailure,
			TaskArgs:     job.TaskArgs,
			WorkflowArgs: job.WorkflowArgs,
			TestArgs:     job.TestArgs,
			JobType:      job.JobType,
			Enabled:      false,
		})
		if err != nil {
			log.Errorf("Failed to update document with ID: %s", job.ID.String())
			continue
		}
	}
	return nil
}

func ListActiveCronjobFailsafe() ([]*commonmodels.Cronjob, error) {
	var ret []*commonmodels.Cronjob
	workflowList, err := commonrepo.NewWorkflowColl().ListWithScheduleEnabled()
	if err != nil {
		return ret, err
	}

	for _, workflow := range workflowList {
		jobList, err := commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
			ParentName: workflow.Name,
			ParentType: config.WorkflowCronjob,
		})
		if err != nil {
			return ret, err
		}
		ret = append(ret, jobList...)
	}

	testList, err := commonrepo.NewTestingColl().ListWithScheduleEnabled()
	if err != nil {
		return []*commonmodels.Cronjob{}, err
	}
	for _, test := range testList {
		jobList, err := commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
			ParentName: test.Name,
			ParentType: config.TestingCronjob,
		})
		if err != nil {
			return []*commonmodels.Cronjob{}, err
		}
		ret = append(ret, jobList...)
	}

	return ret, nil
}

func ListActiveCronjob() ([]*commonmodels.Cronjob, error) {
	return commonrepo.NewCronjobColl().ListActiveJob()
}

func ListCronjob(name, jobType string) ([]*commonmodels.Cronjob, error) {
	return commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
		ParentName: name,
		ParentType: jobType,
	})
}
