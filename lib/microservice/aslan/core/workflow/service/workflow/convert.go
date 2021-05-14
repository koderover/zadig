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

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types"
)

func ConvertQueueToTask(queueTask *commonmodels.Queue) *task.Task {
	return &task.Task{
		TaskID:          queueTask.TaskID,
		ProductName:     queueTask.ProductName,
		PipelineName:    queueTask.PipelineName,
		Namespace:       queueTask.Namespace,
		Type:            queueTask.Type,
		Status:          queueTask.Status,
		Description:     queueTask.Description,
		TaskCreator:     queueTask.TaskCreator,
		TaskRevoker:     queueTask.TaskRevoker,
		CreateTime:      queueTask.CreateTime,
		StartTime:       queueTask.StartTime,
		EndTime:         queueTask.EndTime,
		SubTasks:        queueTask.SubTasks,
		Stages:          queueTask.Stages,
		ReqID:           queueTask.ReqID,
		AgentHost:       queueTask.AgentHost,
		DockerHost:      queueTask.DockerHost,
		TeamID:          queueTask.TeamID,
		TeamName:        queueTask.TeamName,
		IsDeleted:       queueTask.IsDeleted,
		IsArchived:      queueTask.IsArchived,
		AgentID:         queueTask.AgentID,
		MultiRun:        queueTask.MultiRun,
		Target:          queueTask.Target,
		BuildModuleVer:  queueTask.BuildModuleVer,
		ServiceName:     queueTask.ServiceName,
		TaskArgs:        queueTask.TaskArgs,
		WorkflowArgs:    queueTask.WorkflowArgs,
		TestArgs:        queueTask.TestArgs,
		ServiceTaskArgs: queueTask.ServiceTaskArgs,
		ConfigPayload:   queueTask.ConfigPayload,
		Error:           queueTask.Error,
		OrgID:           queueTask.OrgID,
		Services:        queueTask.Services,
		Render:          queueTask.Render,
		StorageUri:      queueTask.StorageUri,
		TestReports:     queueTask.TestReports,
		RwLock:          queueTask.RwLock,
		ResetImage:      queueTask.ResetImage,
		TriggerBy:       queueTask.TriggerBy,
		Features:        queueTask.Features,
		IsRestart:       queueTask.IsRestart,
		StorageEndpoint: queueTask.StorageEndpoint,
	}
}

func ConvertTaskToQueue(task *task.Task) *commonmodels.Queue {
	return &commonmodels.Queue{
		TaskID:          task.TaskID,
		ProductName:     task.ProductName,
		PipelineName:    task.PipelineName,
		Namespace:       task.Namespace,
		Type:            task.Type,
		Status:          task.Status,
		Description:     task.Description,
		TaskCreator:     task.TaskCreator,
		TaskRevoker:     task.TaskRevoker,
		CreateTime:      task.CreateTime,
		StartTime:       task.StartTime,
		EndTime:         task.EndTime,
		SubTasks:        task.SubTasks,
		Stages:          task.Stages,
		ReqID:           task.ReqID,
		AgentHost:       task.AgentHost,
		DockerHost:      task.DockerHost,
		TeamID:          task.TeamID,
		TeamName:        task.TeamName,
		IsDeleted:       task.IsDeleted,
		IsArchived:      task.IsArchived,
		AgentID:         task.AgentID,
		MultiRun:        task.MultiRun,
		Target:          task.Target,
		BuildModuleVer:  task.BuildModuleVer,
		ServiceName:     task.ServiceName,
		TaskArgs:        task.TaskArgs,
		WorkflowArgs:    task.WorkflowArgs,
		TestArgs:        task.TestArgs,
		ServiceTaskArgs: task.ServiceTaskArgs,
		ConfigPayload:   task.ConfigPayload,
		Error:           task.Error,
		OrgID:           task.OrgID,
		Services:        task.Services,
		Render:          task.Render,
		StorageUri:      task.StorageUri,
		TestReports:     task.TestReports,
		RwLock:          task.RwLock,
		ResetImage:      task.ResetImage,
		TriggerBy:       task.TriggerBy,
		Features:        task.Features,
		IsRestart:       task.IsRestart,
		StorageEndpoint: task.StorageEndpoint,
	}
}

func BuildModuleToSubTasks(moduleName, version, target, serviceName, productName string, envs []*commonmodels.KeyVal, pro *commonmodels.Product, log *xlog.Logger) ([]map[string]interface{}, error) {
	var (
		subTasks = make([]map[string]interface{}, 0)
	)

	serviceTmpl, _ := commonservice.GetServiceTemplate(
		serviceName, "", "", setting.ProductStatusDeleting, 0, log,
	)
	if serviceTmpl != nil && serviceTmpl.Visibility == setting.PUBLICSERVICE {
		productName = serviceTmpl.ProductName
	}

	opt := &commonrepo.BuildListOption{
		Name:        moduleName,
		Version:     version,
		ServiceName: serviceName,
		ProductName: productName,
	}

	if len(target) > 0 {
		opt.Targets = []string{target}
	}

	modules, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	registries, err := commonservice.ListRegistryNamespaces(log)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	for _, module := range modules {
		build := &task.Build{
			TaskType:     config.TaskBuild,
			Enabled:      true,
			InstallItems: module.PreBuild.Installs,
			ServiceName:  target,
			Service:      serviceName,
			JobCtx:       task.JobCtx{},
			ImageID:      module.PreBuild.ImageID,
			BuildOS:      module.PreBuild.BuildOS,
			ImageFrom:    module.PreBuild.ImageFrom,
			ResReq:       module.PreBuild.ResReq,
			Timeout:      module.Timeout,
			Registries:   registries,
		}

		if build.ImageFrom == "" {
			build.ImageFrom = commonmodels.ImageFromKoderover
		}

		if pro != nil {
			build.EnvName = pro.EnvName
		}

		if build.InstallItems == nil {
			build.InstallItems = make([]*commonmodels.Item, 0)
		}

		build.JobCtx.Builds = module.Repos
		if len(build.JobCtx.Builds) == 0 {
			build.JobCtx.Builds = make([]*types.Repository, 0)
		}

		build.JobCtx.BuildSteps = []*task.BuildStep{}
		if module.Scripts != "" {
			build.JobCtx.BuildSteps = append(build.JobCtx.BuildSteps, &task.BuildStep{BuildType: "shell", Scripts: module.Scripts})
		}

		build.JobCtx.EnvVars = module.PreBuild.Envs

		if len(module.PreBuild.Envs) == 0 {
			build.JobCtx.EnvVars = make([]*commonmodels.KeyVal, 0)
		}

		if len(envs) > 0 {
			for _, envVar := range build.JobCtx.EnvVars {
				for _, overwrite := range envs {
					if overwrite.Key == envVar.Key && overwrite.Value != setting.MaskValue {
						envVar.Value = overwrite.Value
						envVar.IsCredential = overwrite.IsCredential
						break
					}
				}
			}
		}

		build.JobCtx.UploadPkg = module.PreBuild.UploadPkg
		build.JobCtx.CleanWorkspace = module.PreBuild.CleanWorkspace
		build.JobCtx.EnableProxy = module.PreBuild.EnableProxy

		if module.PostBuild != nil && module.PostBuild.DockerBuild != nil {
			build.JobCtx.DockerBuildCtx = &task.DockerBuildCtx{
				WorkDir:    module.PostBuild.DockerBuild.WorkDir,
				DockerFile: module.PostBuild.DockerBuild.DockerFile,
				BuildArgs:  module.PostBuild.DockerBuild.BuildArgs,
			}
		}

		if module.PostBuild != nil && module.PostBuild.FileArchive != nil {
			build.JobCtx.FileArchiveCtx = &task.FileArchiveCtx{
				FileLocation: module.PostBuild.FileArchive.FileLocation,
			}
		}

		if module.PostBuild != nil && module.PostBuild.Scripts != "" {
			build.JobCtx.PostScripts = module.PostBuild.Scripts
		}

		build.JobCtx.Caches = module.Caches

		bst, err := build.ToSubTask()
		if err != nil {
			return subTasks, e.ErrConvertSubTasks.AddErr(err)
		}
		subTasks = append(subTasks, bst)
	}

	return subTasks, nil
}

type JenkinsBuildOption struct {
	version          string
	target           string
	serviceName      string
	productName      string
	jenkinsBuildArgs *commonmodels.JenkinsBuildArgs
}

// JenkinsBuildModuleToSubTasks
func JenkinsBuildModuleToSubTasks(jenkinsBuildOption *JenkinsBuildOption, log *xlog.Logger) ([]map[string]interface{}, error) {
	var (
		subTasks = make([]map[string]interface{}, 0)
	)

	opt := &commonrepo.BuildListOption{
		Version:     jenkinsBuildOption.version,
		ServiceName: jenkinsBuildOption.serviceName,
		ProductName: jenkinsBuildOption.productName,
	}

	if len(jenkinsBuildOption.target) > 0 {
		opt.Targets = []string{jenkinsBuildOption.target}
	}

	modules, err := commonrepo.NewBuildColl().List(opt)
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	jenkinsIntegrations, err := commonrepo.NewJenkinsIntegrationColl().List()
	if err != nil {
		return nil, e.ErrConvertSubTasks.AddErr(err)
	}

	jenkinsBuildParams := make([]*task.JenkinsBuildParam, 0)
	for _, jenkinsBuildParam := range jenkinsBuildOption.jenkinsBuildArgs.JenkinsBuildParams {
		jenkinsBuildParams = append(jenkinsBuildParams, &task.JenkinsBuildParam{
			Name:  jenkinsBuildParam.Name,
			Value: jenkinsBuildParam.Value,
		})
	}

	for _, module := range modules {
		build := &task.JenkinsBuild{
			TaskType:    config.TaskJenkinsBuild,
			Enabled:     true,
			ServiceName: jenkinsBuildOption.target,
			Service:     jenkinsBuildOption.serviceName,
			ResReq:      module.PreBuild.ResReq,
			Timeout:     module.Timeout,
			JenkinsBuildArgs: &task.JenkinsBuildArgs{
				JobName:            jenkinsBuildOption.jenkinsBuildArgs.JobName,
				JenkinsBuildParams: jenkinsBuildParams,
			},
			JenkinsIntegration: &task.JenkinsIntegration{
				URL:      jenkinsIntegrations[0].URL,
				Username: jenkinsIntegrations[0].Username,
				Password: jenkinsIntegrations[0].Password,
			},
		}

		bst, err := build.ToSubTask()
		if err != nil {
			return subTasks, e.ErrConvertSubTasks.AddErr(err)
		}
		subTasks = append(subTasks, bst)
	}

	return subTasks, nil
}

func AddPipelineJiraSubTask(pipeline *commonmodels.Pipeline, log *xlog.Logger) (map[string]interface{}, error) {
	jira := &task.Jira{
		TaskType: config.TaskJira,
		Enabled:  true,
	}

	for _, subTask := range pipeline.SubTasks {
		pre, err := commonservice.ToPreview(subTask)
		if err != nil {
			return nil, errors.New(e.InterfaceToTaskErrMsg)
		}

		if !pre.Enabled {
			continue
		}

		switch pre.TaskType {
		case config.TaskBuild:
			t, err := commonservice.ToBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return nil, err
			}
			jira.Builds = t.JobCtx.Builds
		}
	}
	return jira.ToSubTask()
}
