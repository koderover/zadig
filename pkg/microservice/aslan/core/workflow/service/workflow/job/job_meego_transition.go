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

package job

import (
	"errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
)

type MeegoTransitionJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.MeegoTransitionJobSpec
}

func (j *MeegoTransitionJob) Instantiate() error {
	j.spec = &commonmodels.MeegoTransitionJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *MeegoTransitionJob) SetPreset() error {
	j.spec = &commonmodels.MeegoTransitionJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *MeegoTransitionJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.MeegoTransitionJobSpec{}
		if err := commonmodels.IToi(args.Spec, j.spec); err != nil {
			return err
		}
		j.job.Spec = j.spec
	}
	return nil
}

func (j *MeegoTransitionJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	meegoInfo, err := commonrepo.NewProjectManagementColl().GetMeego()
	if err != nil {
		return nil, errors.New("failed to find meego integration info")
	}
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.MeegoTransitionJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	jobTask := &commonmodels.JobTask{
		Name:    j.job.Name,
		Key:     j.job.Name,
		JobType: string(config.JobMeegoTransition),
		Spec: &commonmodels.MeegoTransitionSpec{
			Link:            meegoInfo.MeegoHost,
			Source:          j.spec.Source,
			ProjectKey:      j.spec.ProjectKey,
			ProjectName:     j.spec.ProjectName,
			WorkItemType:    j.spec.WorkItemType,
			WorkItemTypeKey: j.spec.WorkItemTypeKey,
			WorkItems:       j.spec.WorkItems,
		},
	}
	resp = append(resp, jobTask)
	return resp, nil
}

func (j *MeegoTransitionJob) LintJob() error {
	return nil
}
