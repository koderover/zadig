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

package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/types"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
	"k8s.io/apimachinery/pkg/util/sets"
)

func WatchExecutingSprintWorkItemTask() {
	ctx := handler.NewBackgroupContext()
	ctx.Logger = ctx.Logger.With("service", "WatchExecutingSprintWorkItemTask")
	for {
		time.Sleep(time.Second * 3)

		listLock := cache.NewRedisLockWithExpiry(fmt.Sprint("sprint-management-watch-lock"), time.Minute*5)
		err := listLock.TryLock()
		if err != nil {
			continue
		}

		t := time.Now()
		list, _, err := mongodb.NewSprintWorkItemTaskColl().List(ctx, &mongodb.SprintWorkItemTaskListOption{
			Status: config.InCompletedStatus(),
		})
		if err != nil {
			ctx.Logger.Errorf("list executing sprint workitem task error: %v", err)
			listLock.Unlock()
			continue
		}
		for _, task := range list {
			updateSprintWorkItemTask(ctx, task)
		}
		if time.Since(t) > time.Millisecond*200 {
			ctx.Logger.Warnf("watch executing sprint workitem task cost %s", time.Since(t))
		}
		listLock.Unlock()
	}
}

func updateSprintWorkItemTask(ctx *handler.Context, workItemTask *models.SprintWorkItemTask) {
	workflowTask, err := mongodb.NewworkflowTaskv4Coll().Find(workItemTask.WorkflowName, workItemTask.WorkflowTaskID)
	if err != nil {
		ctx.Logger.Errorf("find workflow task error: %v", err)
		return
	}

	workItemTask.Status = workflowTask.Status
	workItemTask.StartTime = workflowTask.StartTime
	workItemTask.EndTime = workflowTask.EndTime

	serviceModuleMap := make(map[string]*models.WorkflowServiceModule)
	addToServiceModuleMap := func(serviceModules []*models.WorkflowServiceModule) {
		for _, serviceModule := range serviceModules {
			key := serviceModule.GetKey()
			if _, ok := serviceModuleMap[key]; !ok {
				serviceModuleMap[key] = serviceModule
			} else {
				artifactSet := sets.NewString(serviceModuleMap[key].Artifacts...)
				artifactSet.Insert(serviceModule.Artifacts...)
				serviceModuleMap[key].Artifacts = artifactSet.List()
				serviceModuleMap[key].CodeInfo = append(serviceModuleMap[key].CodeInfo, serviceModule.CodeInfo...)
			}
		}
	}

	for _, stage := range workflowTask.Stages {
		for _, jobTask := range stage.Jobs {
			switch jobTask.JobType {
			case string(config.JobZadigBuild):
				taskJobSpec := &models.JobTaskFreestyleSpec{}
				if err := models.IToi(jobTask.Spec, taskJobSpec); err != nil {
					ctx.Logger.Errorf("failed to convert job spec to build job spec, error: %s", err)
					continue
				}

				serviceModule := &models.WorkflowServiceModule{}
				for _, arg := range taskJobSpec.Properties.Envs {
					if arg.Key == "SERVICE_NAME" {
						serviceModule.ServiceName = arg.Value
						continue
					}
					if arg.Key == "SERVICE_MODULE" {
						serviceModule.ServiceModule = arg.Value
						continue
					}
				}
				for _, step := range taskJobSpec.Steps {
					if step.StepType == config.StepGit {
						stepSpec := &stepspec.StepGitSpec{}
						if err := models.IToi(step.Spec, &stepSpec); err != nil {
							ctx.Logger.Errorf("failed to convert step spec to step git spec, error: %s", err)
							continue
						}
						if serviceModule.CodeInfo == nil {
							serviceModule.CodeInfo = make([]*types.Repository, 0)
						}
						serviceModule.CodeInfo = append(serviceModule.CodeInfo, stepSpec.Repos...)
						continue
					}
					if step.StepType == config.StepPerforce {
						stepSpec := &stepspec.StepP4Spec{}
						if err := models.IToi(step.Spec, &stepSpec); err != nil {
							ctx.Logger.Errorf("failed to convert step spec to step perforce spec, error: %s", err)
							continue
						}
						if serviceModule.CodeInfo == nil {
							serviceModule.CodeInfo = make([]*types.Repository, 0)
						}
						serviceModule.CodeInfo = append(serviceModule.CodeInfo, stepSpec.Repos...)
						continue
					}
				}

				addToServiceModuleMap([]*models.WorkflowServiceModule{serviceModule})
			case string(config.JobZadigDeploy):
				taskDeploySpec := &models.JobTaskDeploySpec{}
				if err := models.IToi(jobTask.Spec, taskDeploySpec); err != nil {
					ctx.Logger.Errorf("failed to convert job spec to deploy job spec, error: %s", err)
					continue
				}

				serviceModules := make([]*models.WorkflowServiceModule, 0)
				// for compatibility
				if taskDeploySpec.ServiceModule != "" {
					serviceModules = append(serviceModules, &models.WorkflowServiceModule{
						ServiceName:   taskDeploySpec.ServiceName,
						ServiceModule: taskDeploySpec.ServiceModule,
						Artifacts:     []string{taskDeploySpec.Image},
					})
				}

				for _, imageAndmodule := range taskDeploySpec.ServiceAndImages {
					serviceModules = append(serviceModules, &models.WorkflowServiceModule{
						ServiceName:   taskDeploySpec.ServiceName,
						ServiceModule: imageAndmodule.ServiceModule,
						Artifacts:     []string{imageAndmodule.Image},
					})
				}

				addToServiceModuleMap(serviceModules)
			case string(config.JobZadigHelmDeploy):
				taskDeploySpec := &models.JobTaskHelmDeploySpec{}
				if err := models.IToi(jobTask.Spec, taskDeploySpec); err != nil {
					ctx.Logger.Errorf("failed to convert job spec to helm deploy job spec, error: %s", err)
					continue
				}

				serviceModules := make([]*models.WorkflowServiceModule, 0)

				for _, imageAndmodule := range taskDeploySpec.ImageAndModules {
					serviceModules = append(serviceModules, &models.WorkflowServiceModule{
						ServiceName:   taskDeploySpec.ServiceName,
						ServiceModule: imageAndmodule.ServiceModule,
						Artifacts:     []string{imageAndmodule.Image},
					})
				}

				addToServiceModuleMap(serviceModules)
			case string(config.JobZadigVMDeploy):
				taskJobSpec := &models.JobTaskFreestyleSpec{}
				if err := models.IToi(jobTask.Spec, taskJobSpec); err != nil {
					ctx.Logger.Errorf("failed to convert job spec to vm deploy job spec, error: %s", err)
					continue
				}

				serviceModule := &models.WorkflowServiceModule{}
				for _, arg := range taskJobSpec.Properties.Envs {
					if arg.Key == "SERVICE_NAME" {
						serviceModule.ServiceName = arg.Value
						continue
					}
					if arg.Key == "SERVICE_MODULE" {
						serviceModule.ServiceModule = arg.Value
						continue
					}
				}
				for _, step := range taskJobSpec.Steps {
					if step.StepType == config.StepDownloadArchive {
						stepSpec := &stepspec.StepDownloadArchiveSpec{}
						if err := models.IToi(step.Spec, &stepSpec); err != nil {
							ctx.Logger.Errorf("failed to convert step spec to step download archive spec, error: %s", err)
							continue
						}

						url := stepSpec.S3.Endpoint + "/" + stepSpec.S3.Bucket + "/"
						if len(stepSpec.S3.Subfolder) > 0 {
							url += strings.TrimLeft(stepSpec.S3.Subfolder, "/")
						}
						url += "/" + stepSpec.FileName
						serviceModule.Artifacts = append(serviceModule.Artifacts, url)
					}
				}

				addToServiceModuleMap([]*models.WorkflowServiceModule{serviceModule})
			case string(config.JobZadigDistributeImage):
				distribute := &models.JobTaskFreestyleSpec{}
				if err := models.IToi(jobTask.Spec, distribute); err != nil {
					ctx.Logger.Errorf("failed to convert job spec to distribute image job spec, error: %s", err)
					continue
				}
				serviceModules := make([]*models.WorkflowServiceModule, 0)

				for _, step := range distribute.Steps {
					if step.StepType == config.StepDistributeImage {
						stepSpec := &stepspec.StepImageDistributeSpec{}
						if err := models.IToi(step.Spec, &stepSpec); err != nil {
							ctx.Logger.Errorf("failed to convert step spec to step distribute spec, error: %s", err)
							break
						}

						for _, target := range stepSpec.DistributeTarget {
							sm := &models.WorkflowServiceModule{
								ServiceName:   target.ServiceName,
								ServiceModule: target.ServiceModule,
								Artifacts:     []string{target.TargetImage},
							}
							serviceModules = append(serviceModules, sm)
						}
						break
					}
				}

				addToServiceModuleMap(serviceModules)
			}
		}
	}

	serviceModules := []*models.WorkflowServiceModule{}
	for _, serviceModule := range serviceModuleMap {
		serviceModules = append(serviceModules, serviceModule)
	}
	sort.Slice(serviceModules, func(i, j int) bool {
		return serviceModules[i].GetKey() < serviceModules[j].GetKey()
	})
	workItemTask.ServiceModuleDatas = serviceModules

	err = mongodb.NewSprintWorkItemTaskColl().Update(ctx, workItemTask)
	if err != nil {
		ctx.Logger.Errorf("update sprint workitem task error: %v", err)
	}
}
