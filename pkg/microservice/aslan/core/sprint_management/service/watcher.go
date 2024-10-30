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
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
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
				serviceModuleMap[key].CodeInfo = append(serviceModuleMap[key].CodeInfo, serviceModule.CodeInfo...)
				serviceModuleMap[key].Images = append(serviceModuleMap[key].Images, serviceModule.Images...)
			}
		}
	}

	for _, stage := range workflowTask.WorkflowArgs.Stages {
		for _, job := range stage.Jobs {
			if job.Skipped {
				continue
			}
			switch job.JobType {
			case config.JobZadigBuild:
				build := new(models.ZadigBuildJobSpec)
				if err := models.IToi(job.Spec, build); err != nil {
					ctx.Logger.Errorf("failed to convert job spec to build job spec, error: %s", err)
					continue
				}
				serviceModules := make([]*models.WorkflowServiceModule, 0)
				for _, serviceAndBuild := range build.ServiceAndBuilds {
					sm := &models.WorkflowServiceModule{
						ServiceName:   serviceAndBuild.ServiceName,
						ServiceModule: serviceAndBuild.ServiceModule,
					}
					for _, repo := range serviceAndBuild.Repos {
						sm.CodeInfo = append(sm.CodeInfo, repo)
					}
					serviceModules = append(serviceModules, sm)
				}
				addToServiceModuleMap(serviceModules)
			case config.JobZadigDeploy:
				deploy := new(models.ZadigDeployJobSpec)
				if err := models.IToi(job.Spec, deploy); err != nil {
					ctx.Logger.Errorf("failed to convert job spec to deploy job spec, error: %s", err)
					continue
				}
				serviceModules := make([]*models.WorkflowServiceModule, 0)
				for _, svc := range deploy.Services {
					for _, module := range svc.Modules {
						sm := &models.WorkflowServiceModule{
							ServiceName:   svc.ServiceName,
							ServiceModule: module.ServiceModule,
							Images:        []string{module.Image},
						}
						serviceModules = append(serviceModules, sm)
					}
				}
				addToServiceModuleMap(serviceModules)
			case config.JobZadigDistributeImage:
				distribute := new(models.ZadigDistributeImageJobSpec)
				if err := models.IToi(job.Spec, distribute); err != nil {
					ctx.Logger.Errorf("failed to convert job spec to distribute image job spec, error: %s", err)
					continue
				}
				serviceModules := make([]*models.WorkflowServiceModule, 0)
				for _, target := range distribute.Targets {
					sm := &models.WorkflowServiceModule{
						ServiceName:   target.ServiceName,
						ServiceModule: target.ServiceModule,
						Images:        []string{target.TargetImage},
					}
					serviceModules = append(serviceModules, sm)
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
