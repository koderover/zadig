/*
 * Copyright 2023 The KodeRover Authors.
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
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

type OfflineServiceJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.OfflineServiceJobSpec
}

func (j *OfflineServiceJob) Instantiate() error {
	j.spec = &commonmodels.OfflineServiceJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *OfflineServiceJob) SetPreset() error {
	j.spec = &commonmodels.OfflineServiceJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}

	j.job.Spec = j.spec
	return nil
}

func (j *OfflineServiceJob) MergeArgs(args *commonmodels.Job) error {
	j.spec = &commonmodels.OfflineServiceJobSpec{}
	if err := commonmodels.IToi(args.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *OfflineServiceJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.OfflineServiceJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec

	jobTask := &commonmodels.JobTask{
		Name:    j.job.Name,
		Key:     j.job.Name,
		JobType: string(config.JobOfflineService),
		Spec: &commonmodels.JobTaskOfflineServiceSpec{
			EnvType: j.spec.EnvType,
			EnvName: j.spec.EnvName,
			ServiceEvents: func() (resp []*commonmodels.JobTaskOfflineServiceEvent) {
				for _, serviceName := range j.spec.Services {
					resp = append(resp, &commonmodels.JobTaskOfflineServiceEvent{
						ServiceName: serviceName,
					})
				}
				return
			}(),
		},
		Timeout: 0,
	}
	return []*commonmodels.JobTask{jobTask}, nil
}

func (j *OfflineServiceJob) LintJob() error {
	j.spec = &commonmodels.OfflineServiceJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec

	return nil
}
