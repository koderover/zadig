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
	"errors"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/types"
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
		RwLock:                  queueTask.RwLock,
		ResetImage:              queueTask.ResetImage,
		ResetImagePolicy:        queueTask.ResetImagePolicy,
		TriggerBy:               queueTask.TriggerBy,
		Features:                queueTask.Features,
		IsRestart:               queueTask.IsRestart,
		StorageEndpoint:         queueTask.StorageEndpoint,
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
		TestArgs:                task.TestArgs,
		ServiceTaskArgs:         task.ServiceTaskArgs,
		ArtifactPackageTaskArgs: task.ArtifactPackageTaskArgs,
		ConfigPayload:           task.ConfigPayload,
		Error:                   task.Error,
		Services:                task.Services,
		Render:                  task.Render,
		StorageURI:              task.StorageURI,
		TestReports:             task.TestReports,
		RwLock:                  task.RwLock,
		ResetImage:              task.ResetImage,
		ResetImagePolicy:        task.ResetImagePolicy,
		TriggerBy:               task.TriggerBy,
		Features:                task.Features,
		IsRestart:               task.IsRestart,
		StorageEndpoint:         task.StorageEndpoint,
	}
}

type JenkinsBuildOption struct {
	BuildName        string
	Version          string
	Target           string
	ServiceName      string
	ProductName      string
	JenkinsBuildArgs *commonmodels.JenkinsBuildArgs
}

func JenkinsBuildModuleToSubTasks(jenkinsBuildOption *JenkinsBuildOption, log *zap.SugaredLogger) ([]map[string]interface{}, error) {
	var (
		subTasks = make([]map[string]interface{}, 0)
	)

	opt := &commonrepo.BuildListOption{
		Name:        jenkinsBuildOption.BuildName,
		ServiceName: jenkinsBuildOption.ServiceName,
		ProductName: jenkinsBuildOption.ProductName,
	}

	if len(jenkinsBuildOption.Target) > 0 {
		opt.Targets = []string{jenkinsBuildOption.Target}
	}

	modules, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	registries, err := commonservice.ListRegistryNamespaces("", true, log)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	jenkinsBuildParams := make([]*types.JenkinsBuildParam, 0)
	for _, jenkinsBuildParam := range jenkinsBuildOption.JenkinsBuildArgs.JenkinsBuildParams {
		jenkinsBuildParams = append(jenkinsBuildParams, &types.JenkinsBuildParam{
			Name:         jenkinsBuildParam.Name,
			Value:        jenkinsBuildParam.Value,
			AutoGenerate: jenkinsBuildParam.AutoGenerate,
		})
	}

	for _, module := range modules {
		jenkinsIntegration := getJenkinsIntegration(module.JenkinsBuild)
		if jenkinsIntegration == nil {
			return nil, e.ErrConvertSubTasks.AddDesc("not found jenkins client")
		}
		build := &task.JenkinsBuild{
			TaskType:    config.TaskJenkinsBuild,
			Enabled:     true,
			ServiceName: jenkinsBuildOption.Target,
			Service:     jenkinsBuildOption.ServiceName,
			ResReq:      module.PreBuild.ResReq,
			ResReqSpec:  module.PreBuild.ResReqSpec,
			Timeout:     module.Timeout,
			JenkinsBuildArgs: &task.JenkinsBuildArgs{
				JobName:            jenkinsBuildOption.JenkinsBuildArgs.JobName,
				JenkinsBuildParams: jenkinsBuildParams,
			},
			JenkinsIntegration: &task.JenkinsIntegration{
				URL:      jenkinsIntegration.URL,
				Username: jenkinsIntegration.Username,
				Password: jenkinsIntegration.Password,
			},
			Registries: registries,
		}

		bst, err := build.ToSubTask()
		if err != nil {
			return subTasks, e.ErrConvertSubTasks.AddErr(err)
		}
		subTasks = append(subTasks, bst)
	}

	return subTasks, nil
}

func getJenkinsIntegration(jenkinsBuild *commonmodels.JenkinsBuild) *commonmodels.JenkinsIntegration {
	if jenkinsBuild == nil {
		return nil
	}
	jenkinsIntegration, err := commonrepo.NewJenkinsIntegrationColl().Get(jenkinsBuild.JenkinsID)
	if err != nil {
		return nil
	}
	return jenkinsIntegration
}

func AddPipelineJiraSubTask(pipeline *commonmodels.Pipeline, log *zap.SugaredLogger) (map[string]interface{}, error) {
	jira := &task.Jira{
		TaskType: config.TaskJira,
		Enabled:  true,
	}

	for _, subTask := range pipeline.SubTasks {
		pre, err := base.ToPreview(subTask)
		if err != nil {
			return nil, errors.New(e.InterfaceToTaskErrMsg)
		}

		if !pre.Enabled {
			continue
		}

		switch pre.TaskType {
		case config.TaskBuild:
			t, err := base.ToBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return nil, err
			}
			jira.Builds = t.JobCtx.Builds
		}
	}
	return jira.ToSubTask()
}
