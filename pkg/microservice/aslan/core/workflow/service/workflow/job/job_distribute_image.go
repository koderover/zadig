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

package job

import (
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/tool/log"
)

type ImageDistributeJob struct {
	job      *commonmodels.Job
	workflow *commonmodels.WorkflowV4
	spec     *commonmodels.ZadigDistributeImageJobSpec
}

func (j *ImageDistributeJob) Instantiate() error {
	j.spec = &commonmodels.ZadigDistributeImageJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ImageDistributeJob) SetPreset() error {
	j.spec = &commonmodels.ZadigDistributeImageJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return err
	}
	if j.spec.Source == config.SourceFromJob {
		jobSpec, err := getQuoteBuildJobSpec(j.spec.JobName, j.workflow)
		if err != nil {
			log.Error(err)
		}
		targets := []*commonmodels.DistributeTarget{}
		for _, svc := range jobSpec.ServiceAndBuilds {
			targets = append(targets, &commonmodels.DistributeTarget{
				ServiceName:   svc.ServiceName,
				ServiceModule: svc.ServiceModule,
			})
		}
		j.spec.Tatgets = targets
	}
	j.job.Spec = j.spec
	return nil
}

func (j *ImageDistributeJob) MergeArgs(args *commonmodels.Job) error {
	if j.job.Name == args.Name && j.job.JobType == args.JobType {
		j.spec = &commonmodels.ZadigDistributeImageJobSpec{}
		if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
			return err
		}
		argsSpec := &commonmodels.ZadigDistributeImageJobSpec{}
		if err := commonmodels.IToi(args.Spec, argsSpec); err != nil {
			return err
		}
		if j.spec.Source == config.SourceRuntime {
			j.spec.Tatgets = argsSpec.Tatgets
		}
		j.job.Spec = j.spec
	}
	return nil
}

func (j *ImageDistributeJob) ToJobs(taskID int64) ([]*commonmodels.JobTask, error) {
	logger := log.SugaredLogger()
	resp := []*commonmodels.JobTask{}

	j.spec = &commonmodels.ZadigDistributeImageJobSpec{}
	if err := commonmodels.IToi(j.job.Spec, j.spec); err != nil {
		return resp, err
	}
	// get distribute targets from previous build job.
	if j.spec.Source == config.SourceFromJob {
		refJobSpec, err := getQuoteBuildJobSpec(j.spec.JobName, j.workflow)
		if err != nil {
			log.Error(err)
		}
		j.spec.SourceRegistryID = refJobSpec.DockerRegistryID
		targetTagMap := map[string]string{}
		for _, target := range j.spec.Tatgets {
			if !target.UpdateTag {
				continue
			}
			targetTagMap[getServiceKey(target.ServiceName, target.ServiceModule)] = target.TargetTag
		}
		newTargets := []*commonmodels.DistributeTarget{}
		for _, svc := range refJobSpec.ServiceAndBuilds {
			newTargets = append(newTargets, &commonmodels.DistributeTarget{
				ServiceName:   svc.ServiceName,
				ServiceModule: svc.ServiceModule,
				SourceTag:     getImageTag(svc.Image),
				TargetTag:     getTargetTag(svc.ServiceName, svc.ServiceModule, getImageTag(svc.Image), targetTagMap),
			})
		}
		j.spec.Tatgets = newTargets
	}
	jobSpec := &commonmodels.JobTaskImageDistributeSpec{
		SourceRegistryID: j.spec.SourceRegistryID,
		TargetRegistryID: j.spec.TargetRegistryID,
	}
	sourceReg, _, err := commonservice.FindRegistryById(j.spec.SourceRegistryID, true, logger)
	if err != nil {
		return resp, fmt.Errorf("source image registry: %s not found: %v", j.spec.SourceRegistryID, err)
	}
	targetReg, _, err := commonservice.FindRegistryById(j.spec.TargetRegistryID, true, logger)
	if err != nil {
		return resp, fmt.Errorf("target image registry: %s not found: %v", j.spec.TargetRegistryID, err)
	}
	jobTask := &commonmodels.JobTask{
		Name:    j.job.Name,
		JobType: string(config.JobZadigDistributeImage),
		Spec:    jobSpec,
	}
	for _, target := range j.spec.Tatgets {
		jobSpec.DistributeTarget = append(jobSpec.DistributeTarget, &commonmodels.DistributeTaskTarget{
			SoureImage:    getImage(target.SourceTag, sourceReg),
			TargetImage:   getImage(target.TargetTag, targetReg),
			ServiceName:   target.ServiceName,
			ServiceModule: target.ServiceModule,
			UpdateTag:     target.UpdateTag,
		})
	}
	resp = append(resp, jobTask)
	j.job.Spec = j.spec
	return resp, nil
}

func (j *ImageDistributeJob) LintJob() error {
	j.spec = &commonmodels.ZadigDistributeImageJobSpec{}
	if err := commonmodels.IToiYaml(j.job.Spec, j.spec); err != nil {
		return err
	}
	return nil
}

func getQuoteBuildJobSpec(jobName string, workflow *commonmodels.WorkflowV4) (*commonmodels.ZadigBuildJobSpec, error) {
	resp := &commonmodels.ZadigBuildJobSpec{}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.Name != jobName {
				continue
			}
			if job.JobType != config.JobZadigBuild {
				return resp, fmt.Errorf("cannot reference job: %s that is not a build", jobName)
			}
			if err := commonmodels.IToi(job.Spec, resp); err != nil {
				return resp, err
			}
			return resp, nil
		}
	}
	return resp, fmt.Errorf("reference job: %s not found", jobName)
}

func getServiceKey(serviceName, serviceModule string) string {
	return fmt.Sprintf("%s/%s", serviceName, serviceModule)
}

func getImageTag(image string) string {
	strs := strings.Split(image, ":")
	return strs[len(strs)-1]
}

func getTargetTag(serviceName, serviceModule, sourceTag string, tagMap map[string]string) string {
	targetTag := sourceTag
	if tag, ok := tagMap[getServiceKey(serviceName, serviceModule)]; ok {
		targetTag = tag
	}
	return targetTag
}

func getImage(tag string, reg *commonmodels.RegistryNamespace) string {
	image := fmt.Sprintf("%s/%s/%s", reg.RegAddr, reg.Namespace, tag)
	image = strings.TrimPrefix(image, "http://")
	image = strings.TrimPrefix(image, "https://")
	return image
}
