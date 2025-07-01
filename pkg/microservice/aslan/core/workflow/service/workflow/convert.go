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

package workflow

import (
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
)

func ConvertQueueToTask(queueTask *commonmodels.Queue) *task.Task {
	return &task.Task{
		TaskID:                  queueTask.TaskID,
		ProductName:             queueTask.ProductName,
		PipelineName:            queueTask.PipelineName,
		PipelineDisplayName:     queueTask.PipelineDisplayName,
		Type:                    queueTask.Type,
		Status:                  queueTask.Status,
		Description:             queueTask.Description,
		TaskCreator:             queueTask.TaskCreator,
		TaskRevoker:             queueTask.TaskRevoker,
		CreateTime:              queueTask.CreateTime,
		StartTime:               queueTask.StartTime,
		EndTime:                 queueTask.EndTime,
		SubTasks:                queueTask.SubTasks,
		Stages:                  queueTask.Stages,
		ReqID:                   queueTask.ReqID,
		AgentHost:               queueTask.AgentHost,
		DockerHost:              queueTask.DockerHost,
		TeamName:                queueTask.TeamName,
		IsDeleted:               queueTask.IsDeleted,
		IsArchived:              queueTask.IsArchived,
		AgentID:                 queueTask.AgentID,
		MultiRun:                queueTask.MultiRun,
		Target:                  queueTask.Target,
		BuildModuleVer:          queueTask.BuildModuleVer,
		ServiceName:             queueTask.ServiceName,
		TaskArgs:                queueTask.TaskArgs,
		WorkflowArgs:            queueTask.WorkflowArgs,
		TestArgs:                queueTask.TestArgs,
		ServiceTaskArgs:         queueTask.ServiceTaskArgs,
		ArtifactPackageTaskArgs: queueTask.ArtifactPackageTaskArgs,
		ConfigPayload:           queueTask.ConfigPayload,
		Error:                   queueTask.Error,
		Services:                queueTask.Services,
		Render:                  queueTask.Render,
		StorageURI:              queueTask.StorageURI,
		TestReports:             queueTask.TestReports,
		ResetImage:              queueTask.ResetImage,
		ResetImagePolicy:        queueTask.ResetImagePolicy,
		TriggerBy:               queueTask.TriggerBy,
		Features:                queueTask.Features,
		IsRestart:               queueTask.IsRestart,
		StorageEndpoint:         queueTask.StorageEndpoint,
		ScanningArgs:            queueTask.ScanningArgs,
	}
}

func ConvertTaskToQueue(task *task.Task) *commonmodels.Queue {
	return &commonmodels.Queue{
		TaskID:                  task.TaskID,
		ProductName:             task.ProductName,
		PipelineName:            task.PipelineName,
		PipelineDisplayName:     task.PipelineDisplayName,
		Type:                    task.Type,
		Status:                  task.Status,
		Description:             task.Description,
		TaskCreator:             task.TaskCreator,
		TaskRevoker:             task.TaskRevoker,
		CreateTime:              task.CreateTime,
		StartTime:               task.StartTime,
		EndTime:                 task.EndTime,
		SubTasks:                task.SubTasks,
		Stages:                  task.Stages,
		ReqID:                   task.ReqID,
		AgentHost:               task.AgentHost,
		DockerHost:              task.DockerHost,
		TeamName:                task.TeamName,
		IsDeleted:               task.IsDeleted,
		IsArchived:              task.IsArchived,
		AgentID:                 task.AgentID,
		MultiRun:                task.MultiRun,
		Target:                  task.Target,
		BuildModuleVer:          task.BuildModuleVer,
		ServiceName:             task.ServiceName,
		TaskArgs:                task.TaskArgs,
		WorkflowArgs:            task.WorkflowArgs,
		ScanningArgs:            task.ScanningArgs,
		TestArgs:                task.TestArgs,
		ServiceTaskArgs:         task.ServiceTaskArgs,
		ArtifactPackageTaskArgs: task.ArtifactPackageTaskArgs,
		ConfigPayload:           task.ConfigPayload,
		Error:                   task.Error,
		Services:                task.Services,
		Render:                  task.Render,
		StorageURI:              task.StorageURI,
		TestReports:             task.TestReports,
		ResetImage:              task.ResetImage,
		ResetImagePolicy:        task.ResetImagePolicy,
		TriggerBy:               task.TriggerBy,
		Features:                task.Features,
		IsRestart:               task.IsRestart,
		StorageEndpoint:         task.StorageEndpoint,
	}
}
