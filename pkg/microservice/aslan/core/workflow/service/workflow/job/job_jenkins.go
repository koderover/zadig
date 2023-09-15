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

package job

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/jenkins"
	"github.com/koderover/zadig/pkg/tool/log"
)

type JenkinsJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.JenkinsJobSpec
}

func (j *JenkinsJob) Instantiate() error {
	j.spec = &commonmodels.JenkinsJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *JenkinsJob) SetPreset() error {
	j.spec = &commonmodels.JenkinsJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	info, err := mongodb.NewJenkinsIntegrationColl().Get(j.spec.JenkinsID)
	if err != nil {
		return errors.Errorf("failed to get Jenkins info from mongo: %v", err)
	}

	client := jenkins.NewClient(info.URL, info.Username, info.Password)
	for _, job := range j.spec.Jobs {
		result, err := client.GetJob(job.JobName)
		if err != nil {
			log.Warnf("Preset JenkinsJob: get job %s error: %v", job.JobName, err)
			continue
		}
		if parameters := result.GetParameters(); len(parameters) > 0 {
			newParameter := make([]*commonmodels.JenkinsJobParameter, len(parameters))
			rawParametersMap := make(map[string]*commonmodels.JenkinsJobParameter)
			for _, parameter := range job.Parameter {
				rawParametersMap[parameter.Name] = parameter
			}
			for _, parameter := range parameters {
				if rawParameter, ok := rawParametersMap[parameter.Name]; !ok {
					newParameter = append(newParameter, &commonmodels.JenkinsJobParameter{
						Name:    parameter.Name,
						Value:   fmt.Sprintf("%v", parameter.DefaultParameterValue.Value),
						Type:    jenkins.ParameterTypeMap[parameter.Type],
						Choices: parameter.Choices,
					})
				} else {
					newParameter = append(newParameter, rawParameter)
				}
			}
			job.Parameter = newParameter
		}
	}
	j.job.Spec = j.spec
	return nil
}

func (j *JenkinsJob) MergeArgs(args *commonmodels.Job) error {
	j.spec = &commonmodels.JenkinsJobSpec{}
	if err := commonmodels.IToi(args.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *JenkinsJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	resp := []*commonmodels.JobTask{}
	j.spec = &commonmodels.JenkinsJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	j.job.Spec = j.spec
	if len(j.spec.Jobs) == 0 {
		return nil, errors.New("Jenkins job list is empty")
	}
	for _, job := range j.spec.Jobs {
		resp = append(resp, &commonmodels.JobTask{
			Name: j.job.Name,
			JobInfo: map[string]string{
				JobNameKey: j.job.Name,
			},
			Key:     j.job.Name,
			JobType: string(config.JobJenkins),
			Spec: &commonmodels.JobTaskJenkinsSpec{
				JenkinsID: j.spec.JenkinsID,
				Job: commonmodels.JobTaskJenkinsJobInfo{
					JobName:   job.JobName,
					Parameter: job.Parameter,
				},
			},
			Timeout: 0,
		})
	}

	return resp, nil
}

func (j *JenkinsJob) LintJob() error {
	j.spec = &commonmodels.JenkinsJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	if _, err := mongodb.NewJenkinsIntegrationColl().Get(j.spec.JenkinsID); err != nil {
		return errors.Errorf("not found Jenkins in mongo, err: %v", err)
	}
	return nil
}
