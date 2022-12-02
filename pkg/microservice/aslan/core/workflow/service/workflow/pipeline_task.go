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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/scmnotify"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/pkg/tool/errors"
	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/types"
)

func CreatePipelineTask(args *commonmodels.TaskArgs, log *zap.SugaredLogger) (*CreateTaskResp, error) {
	pipeline, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: args.PipelineName})
	if err != nil {
		log.Errorf("PipelineV2.Find %s error: %v", args.PipelineName, err)
		return nil, e.ErrCreateTask.AddDesc(e.FindPipelineErrMsg)
	}

	if !pipeline.Enabled {
		log.Errorf("pipeline %s disabled", args.PipelineName)
		return nil, e.ErrCreateTask.AddDesc(e.PipelineDisabledErrMsg)
	}

	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.PipelineTaskFmt, args.PipelineName))
	if err != nil {
		log.Errorf("Counter.GetNextSeq error: %v", err)
		return nil, e.ErrGetCounter.AddDesc(err.Error())
	}

	// 如果用户使用预定义编译配置, 则从编译模块配置中生成SubTasks
	if pipeline.BuildModuleVer != "" {
		subTasks, err := BuildModuleToSubTasks(&commonmodels.BuildModuleArgs{
			Target: pipeline.Target,
		}, log)
		if err != nil {
			return nil, e.ErrCreateTask.AddErr(err)
		}

		// 将转换出的SubTasks作为pipeline的开始任务
		// 注意：在保存pipeline的时候，如果选择预定义编译配置，则Build等SubTasks不需要在pipeline中保存。
		pipeline.SubTasks = append(subTasks, pipeline.SubTasks...)
	}

	// 更新单服务工作的subtask的build_os
	// 自定义基础镜像的镜像名称可能会被更新，需要使用ID获取最新的镜像名称
	for i, subTask := range pipeline.SubTasks {
		pre, err := base.ToPreview(subTask)
		if err != nil {
			log.Errorf("subTask.ToPreview error: %v", err)
			continue
		}
		switch pre.TaskType {
		case config.TaskBuild:
			build, err := base.ToBuildTask(subTask)
			if err != nil || build == nil {
				log.Errorf("subTask.ToBuildTask error: %v", err)
				continue
			}
			if build.ImageID == "" {
				continue
			}
			basicImage, err := commonrepo.NewBasicImageColl().Find(build.ImageID)
			if err != nil {
				log.Errorf("BasicImage.Find failed, id:%s, err:%v", build.ImageID, err)
				continue
			}
			build.BuildOS = basicImage.Value

			if build.DockerBuild != nil && build.DockerBuild.Enabled {
				build.JobCtx.DockerBuildCtx = &task.DockerBuildCtx{
					WorkDir:    build.DockerBuild.WorkDir,
					DockerFile: build.DockerBuild.DockerFile,
					BuildArgs:  build.DockerBuild.BuildArgs,
				}
			}

			if build.Registries == nil {
				registries, err := commonservice.ListRegistryNamespaces("", true, log)
				if err != nil {
					log.Errorf("ListRegistryNamespaces err:%v", err)
				} else {
					build.Registries = registries
				}
			}

			// 创建任务时可以临时编辑环境变量，需要将pipeline中的环境变量更新成新的值。
			if args.BuildArgs != nil {
				build.JobCtx.EnvVars = args.BuildArgs
			}

			pipeline.SubTasks[i], err = build.ToSubTask()
			if err != nil {
				log.Errorf("build.ToSubTask error: %v", err)
				continue
			}
		case config.TaskTestingV2:
			testing, err := base.ToTestingTask(subTask)
			if err != nil || testing == nil {
				log.Errorf("subTask.ToTestingTask error: %v", err)
				continue
			}
			if testing.ImageID == "" {
				continue
			}
			basicImage, err := commonrepo.NewBasicImageColl().Find(testing.ImageID)
			if err != nil {
				log.Errorf("BasicImage.Find failed, id:%s, err:%v", testing.ImageID, err)
				continue
			}

			if testing.Registries == nil {
				registries, err := commonservice.ListRegistryNamespaces("", true, log)
				if err != nil {
					log.Errorf("ListRegistryNamespaces err:%v", err)
				} else {
					testing.Registries = registries
				}
			}

			testing.BuildOS = basicImage.Value
			pipeline.SubTasks[i], err = testing.ToSubTask()
			if err != nil {
				log.Errorf("testing.ToSubTask error: %v", err)
				continue
			}
		}
	}

	jiraInfo, _ := systemconfig.New().GetJiraInfo()
	if jiraInfo != nil {
		jiraTask, err := AddPipelineJiraSubTask(pipeline, log)
		if err != nil {
			log.Errorf("add jira task error: %v", err)
			return nil, e.ErrCreateTask.AddErr(fmt.Errorf("add jira task error: %v", err))
		}
		pipeline.SubTasks = append(pipeline.SubTasks, jiraTask)
	}

	var defaultStorageURI string
	if defaultS3, err := s3.FindDefaultS3(); err == nil {
		defaultStorageURI, err = defaultS3.GetEncryptedURL()
		if err != nil {
			return nil, e.ErrS3Storage.AddErr(err)
		}
	}

	pt := &task.Task{
		TaskID:         nextTaskID,
		ProductName:    pipeline.ProductName,
		PipelineName:   args.PipelineName,
		Type:           config.SingleType,
		TaskCreator:    args.TaskCreator,
		ReqID:          args.ReqID,
		Status:         config.StatusCreated,
		SubTasks:       pipeline.SubTasks,
		TaskArgs:       args,
		TeamName:       pipeline.TeamName,
		ConfigPayload:  commonservice.GetConfigPayload(args.CodeHostID),
		MultiRun:       pipeline.MultiRun,
		BuildModuleVer: pipeline.BuildModuleVer,
		Target:         pipeline.Target,
		StorageURI:     defaultStorageURI,
	}

	sort.Sort(ByTaskKind(pt.SubTasks))

	for i, t := range pt.SubTasks {
		preview, err := base.ToPreview(t)
		if err != nil {
			continue
		}
		if preview.TaskType != config.TaskDeploy {
			continue
		}

		t, err := base.ToDeployTask(t)
		if err == nil && t.Enabled {
			env, err := commonrepo.NewProductColl().FindEnv(&commonrepo.ProductEnvFindOptions{
				Namespace: pt.TaskArgs.Deploy.Namespace,
				Name:      pt.ProductName,
			})

			if err != nil {
				return nil, e.ErrCreateTask.AddDesc(
					e.EnvNotFoundErrMsg + ": " + pt.TaskArgs.Deploy.Namespace,
				)
			}

			t.EnvName = env.EnvName
			t.ProductName = pt.ProductName
			pt.ConfigPayload.DeployClusterID = env.ClusterID
			pt.SubTasks[i], _ = t.ToSubTask()
		}
	}

	repos, err := commonservice.ListRegistryNamespaces("", false, log)
	if err != nil {
		return nil, e.ErrCreateTask.AddErr(err)
	}

	pt.ConfigPayload.RepoConfigs = make(map[string]*commonmodels.RegistryNamespace)
	for _, repo := range repos {
		pt.ConfigPayload.RepoConfigs[repo.ID.Hex()] = repo
	}

	if err := ensurePipelineTask(&task.TaskOpt{
		Task:    pt,
		EnvName: pt.TaskArgs.Deploy.Namespace,
	}, log); err != nil {
		log.Errorf("Service.ensurePipelineTask failed %v %v", args, err)
		if err, ok := err.(*ContainerNotFound); ok {
			return nil, e.NewWithExtras(
				e.ErrCreateTaskFailed,
				"container doesn't exists", map[string]interface{}{
					"productName":   err.ProductName,
					"envName":       err.EnvName,
					"serviceName":   err.ServiceName,
					"containerName": err.Container,
				})
		}

		return nil, e.ErrCreateTask.AddDesc(err.Error())
	}

	if len(pt.SubTasks) <= 0 {
		return nil, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	if config.EnableGitCheck() {
		if err := createGitCheck(pt, log); err != nil {
			log.Error(err)
		}
	}

	// send to queue to execute task
	if err := CreateTask(pt); err != nil {
		log.Error(err)
		return nil, e.ErrCreateTask
	}

	scmnotify.NewService().UpdatePipelineWebhookComment(pt, log)

	resp := &CreateTaskResp{
		ProjectName:  args.ProductName,
		PipelineName: args.PipelineName,
		TaskID:       nextTaskID,
	}
	return resp, nil
}

// TaskResult ...
type TaskResult struct {
	Data      []*commonrepo.TaskPreview `bson:"data"             json:"data"`
	StartAt   int                       `bson:"start_at"         json:"start_at"`
	MaxResult int                       `bson:"max_result"       json:"max_result"`
	Total     int                       `bson:"total"            json:"total"`
}

// ListPipelineTasksV2Result 工作流任务分页信息
func ListPipelineTasksV2Result(name string, typeString config.PipelineType, queryType string, filters []string, maxResult, startAt int, log *zap.SugaredLogger) (*TaskResult, error) {
	ret := &TaskResult{MaxResult: maxResult, StartAt: startAt}
	var err error
	var listTaskOpt *commonrepo.ListTaskOption
	var countTaskOpt *commonrepo.CountTaskOption
	var restp []*commonrepo.TaskPreview
	if len(filters) == 0 || (len(filters) == 1 && filters[0] == "") {
		queryType = ""
	}
	switch queryType {
	case "creator":
		listTaskOpt = &commonrepo.ListTaskOption{PipelineName: name, Limit: maxResult, Skip: startAt, TaskCreators: filters, ForWorkflowTaskList: true, Type: typeString}
		countTaskOpt = &commonrepo.CountTaskOption{PipelineNames: []string{name}, TaskCreators: filters, Type: typeString}
	case "committer":
		listTaskOpt = &commonrepo.ListTaskOption{PipelineName: name, Limit: maxResult, Skip: startAt, Committers: filters, ForWorkflowTaskList: true, Type: typeString}
		countTaskOpt = &commonrepo.CountTaskOption{PipelineNames: []string{name}, Committers: filters, Type: typeString}
	case "serviceName":
		listTaskOpt = &commonrepo.ListTaskOption{PipelineName: name, Limit: maxResult, Skip: startAt, ServiceModule: filters, ForWorkflowTaskList: true, Type: typeString}
		countTaskOpt = &commonrepo.CountTaskOption{PipelineNames: []string{name}, ServiceModule: filters, Type: typeString}
	case "taskStatus":
		listTaskOpt = &commonrepo.ListTaskOption{PipelineName: name, Limit: maxResult, Skip: startAt, Statuses: filters, ForWorkflowTaskList: true, Type: typeString}
		countTaskOpt = &commonrepo.CountTaskOption{PipelineNames: []string{name}, Statuses: filters, Type: typeString}
	default:
		listTaskOpt = &commonrepo.ListTaskOption{PipelineName: name, Limit: maxResult, Skip: startAt, ForWorkflowTaskList: true, Type: typeString}
		countTaskOpt = &commonrepo.CountTaskOption{PipelineNames: []string{name}, Type: typeString}
	}
	restp, err = commonrepo.NewTaskColl().List(listTaskOpt)
	if err != nil {
		log.Errorf("PipelineTaskV2.List: %v error: %s", listTaskOpt, err)
		return ret, e.ErrListTasks
	}

	for _, t := range restp {
		if t.WorkflowArgs == nil {
			continue
		}
		t.Namespace = t.WorkflowArgs.Namespace
		serviceModuleMap := make(map[string]*commonrepo.ServiceModule)

		for _, target := range t.WorkflowArgs.Target {
			sm := &commonrepo.ServiceModule{
				ServiceName:   target.ServiceName,
				ServiceModule: target.Name,
			}
			serviceModuleMap[fmt.Sprintf("%s_%s", target.Name, target.ServiceName)] = sm
			t.ServiceModules = append(t.ServiceModules, sm)
		}

		for _, stage := range t.Stages {
			if stage.TaskType != config.TaskBuild {
				continue
			}
			for fullServiceName, sTask := range stage.SubTasks {
				buildTask, err := base.ToBuildTask(sTask)
				if err != nil {
					log.Warnf("failed to get build task for task: %s, err: %s", name, err)
					continue
				}
				if sm, ok := serviceModuleMap[fullServiceName]; ok {
					for _, buildInfo := range buildTask.JobCtx.Builds {
						sm.CodeInfo = append(sm.CodeInfo, buildInfo)
					}
				}
			}
			break
		}

		t.WorkflowArgs = nil
		t.Stages = nil
	}

	ret.Data = restp
	ret.Total, err = commonrepo.NewTaskColl().Count(countTaskOpt)
	if err != nil {
		log.Errorf("PipelineTaskV2.List Count error: %v", err)
		return ret, e.ErrCountTasks
	}
	return ret, nil
}

func GetPipelineTaskV2(taskID int64, pipelineName string, typeString config.PipelineType, log *zap.SugaredLogger) (*task.Task, error) {
	resp, err := commonrepo.NewTaskColl().Find(taskID, pipelineName, typeString)
	if err != nil {
		log.Errorf("[%d:%s] PipelineTaskV2.Find error: %v", taskID, pipelineName, err)
		return resp, e.ErrGetTask
	}

	Clean(resp)
	return resp, nil
}

func GetFiltersPipelineTaskV2(projectName, pipelineName, querytype string, typeString config.PipelineType, log *zap.SugaredLogger) ([]interface{}, error) {
	resp := []interface{}{}
	var err error
	fieldName := ""
	switch querytype {
	case "creator":
		fieldName = "task_creator"
		resp, err = commonrepo.NewTaskColl().DistinctFieldsPipelineTask(fieldName, projectName, pipelineName, typeString, false)
		if err != nil {
			log.Errorf("[%s] DistinctFeildsPipelineTask fieldName: %s error: %s", fieldName, pipelineName, err)
			return resp, e.ErrGetTask
		}
	case "committer":
		fieldName = "workflow_args.committer"
		resp, err = commonrepo.NewTaskColl().DistinctFieldsPipelineTask(fieldName, projectName, pipelineName, typeString, false)
		if err != nil {
			log.Errorf("[%s] DistinctFeildsPipelineTask fieldName: %s error: %s", fieldName, pipelineName, err)
			return resp, e.ErrGetTask
		}
	case "serviceName":
		fieldName = "workflow_args.targets.name"
		resp, err = commonrepo.NewTaskColl().DistinctFieldsPipelineTask(fieldName, projectName, pipelineName, typeString, false)
		if err != nil {
			log.Errorf("[%s] DistinctFeildsPipelineTask fieldName: %s error: %s", fieldName, pipelineName, err)
			return resp, e.ErrGetTask
		}
	default:
		return resp, fmt.Errorf("queryType parameter is invalid")
	}
	return resp, nil
}

func RestartPipelineTaskV2(userName string, taskID int64, pipelineName string, typeString config.PipelineType, log *zap.SugaredLogger) error {
	t, err := commonrepo.NewTaskColl().Find(taskID, pipelineName, typeString)
	if err != nil {
		log.Errorf("[%d:%s] find pipeline error: %v", taskID, pipelineName, err)
		return e.ErrRestartTask.AddDesc(e.FindPipelineTaskErrMsg)
	}

	// 不重试已经成功的pipelie task
	if t.Status == config.StatusRunning || t.Status == config.StatusPassed {
		log.Errorf("cannot restart running or passed task. Status: %v", t.Status)
		return e.ErrRestartTask.AddDesc(e.RestartPassedTaskErrMsg)
	}

	//更新测试的相关信息
	if t.Type == config.TestType {
		stages := make([]*commonmodels.Stage, 0)
		testName := strings.Replace(t.PipelineName, "-job", "", 1)

		if testTask, err := TestArgsToTestSubtask(&commonmodels.TestTaskArgs{ProductName: t.ProductName, TestName: testName, TestTaskCreator: userName}, t, log); err == nil {
			FmtBuilds(testTask.JobCtx.Builds, log)
			if testSubTask, err := testTask.ToSubTask(); err == nil {
				AddSubtaskToStage(&stages, testSubTask, testTask.TestModuleName)
				sort.Sort(ByStageKind(stages))
				t.Stages = stages
			}
		}
	} else if t.Type == config.WorkflowType {
		stageArray := t.Stages
		for _, subStage := range stageArray {
			taskType := subStage.TaskType
			switch taskType {
			case config.TaskBuild:
				subBuildTaskMap := subStage.SubTasks
				for serviceModule, subTask := range subBuildTaskMap {
					if buildInfo, err := base.ToBuildTask(subTask); err == nil {
						if newModules, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{Targets: []string{serviceModule}, ServiceName: buildInfo.Service, ProductName: t.ProductName}); err == nil && len(newModules) > 0 {
							newBuildInfo := newModules[0]
							buildInfo.JobCtx.BuildSteps = []*task.BuildStep{}
							if newBuildInfo.Scripts != "" {
								buildInfo.JobCtx.BuildSteps = append(buildInfo.JobCtx.BuildSteps, &task.BuildStep{BuildType: "shell", Scripts: newBuildInfo.Scripts})
							}

							if newBuildInfo.PreBuild != nil {
								buildInfo.InstallItems = newBuildInfo.PreBuild.Installs
								buildInfo.JobCtx.UploadPkg = newBuildInfo.PreBuild.UploadPkg
								buildInfo.JobCtx.CleanWorkspace = newBuildInfo.PreBuild.CleanWorkspace
								buildInfo.JobCtx.EnableProxy = newBuildInfo.PreBuild.EnableProxy

								for _, env := range buildInfo.JobCtx.EnvVars {
									for _, overwrite := range newBuildInfo.PreBuild.Envs {
										if overwrite.Key == env.Key {
											env.Value = overwrite.Value
											env.IsCredential = overwrite.IsCredential
											break
										}
									}
								}

								buildInfo.ImageID = newBuildInfo.PreBuild.ImageID
								buildInfo.BuildOS = newBuildInfo.PreBuild.BuildOS
								buildInfo.ImageFrom = newBuildInfo.PreBuild.ImageFrom
								buildInfo.ResReq = newBuildInfo.PreBuild.ResReq
								buildInfo.ResReqSpec = newBuildInfo.PreBuild.ResReqSpec
							}

							if newBuildInfo.PostBuild != nil && newBuildInfo.PostBuild.DockerBuild != nil {
								buildInfo.JobCtx.DockerBuildCtx = &task.DockerBuildCtx{
									WorkDir:    newBuildInfo.PostBuild.DockerBuild.WorkDir,
									DockerFile: newBuildInfo.PostBuild.DockerBuild.DockerFile,
									BuildArgs:  newBuildInfo.PostBuild.DockerBuild.BuildArgs,
									ImageName:  buildInfo.JobCtx.Image,
								}
							}

							if newBuildInfo.PostBuild != nil && newBuildInfo.PostBuild.FileArchive != nil {
								buildInfo.JobCtx.FileArchiveCtx = &task.FileArchiveCtx{
									FileLocation: newBuildInfo.PostBuild.FileArchive.FileLocation,
								}
							}
							buildInfo.JobCtx.Caches = newBuildInfo.Caches
							// 设置 build 安装脚本
							buildInfo.InstallCtx, err = BuildInstallCtx(buildInfo.InstallItems)
							if err != nil {
								log.Errorf("buildInstallCtx error: %v", err)
							}
							buildInfo.Timeout = newBuildInfo.Timeout * 60
							buildInfo.IsRestart = true
							if bst, err := buildInfo.ToSubTask(); err == nil {
								subBuildTaskMap[serviceModule] = bst
							}
						}
					}
				}

			case config.TaskTestingV2:
				subTestTaskMap := subStage.SubTasks
				for testName, subTask := range subTestTaskMap {
					if testInfo, err := base.ToTestingTask(subTask); err == nil {
						if newTestInfo, err := GetTesting(testInfo.TestModuleName, "", log); err == nil {
							testInfo.JobCtx.BuildSteps = []*task.BuildStep{}
							if newTestInfo.Scripts != "" {
								testInfo.JobCtx.BuildSteps = append(testInfo.JobCtx.BuildSteps, &task.BuildStep{BuildType: "shell", Scripts: newTestInfo.Scripts})
							}

							testInfo.JobCtx.TestResultPath = newTestInfo.TestResultPath
							testInfo.JobCtx.Caches = newTestInfo.Caches
							testInfo.JobCtx.ArtifactPaths = newTestInfo.ArtifactPaths

							if newTestInfo.PreTest != nil {
								testInfo.InstallItems = newTestInfo.PreTest.Installs
								testInfo.JobCtx.CleanWorkspace = newTestInfo.PreTest.CleanWorkspace
								testInfo.JobCtx.EnableProxy = newTestInfo.PreTest.EnableProxy

								for _, env := range testInfo.JobCtx.EnvVars {
									for _, overwrite := range newTestInfo.PreTest.Envs {
										if overwrite.Key == env.Key {
											env.Value = overwrite.Value
											env.IsCredential = overwrite.IsCredential
											break
										}
									}
								}

								testInfo.ImageID = newTestInfo.PreTest.ImageID
								testInfo.BuildOS = newTestInfo.PreTest.BuildOS
								testInfo.ImageFrom = newTestInfo.PreTest.ImageFrom
								testInfo.ResReq = newTestInfo.PreTest.ResReq
								testInfo.ResReqSpec = newTestInfo.PreTest.ResReqSpec
							}
							// 设置 build 安装脚本
							testInfo.InstallCtx, err = BuildInstallCtx(testInfo.InstallItems)
							if err != nil {
								log.Errorf("buildInstallCtx error: %v", err)
							}
							testInfo.Timeout = newTestInfo.Timeout * 60
							testInfo.IsRestart = true
							if testSubTask, err := testInfo.ToSubTask(); err == nil {
								subTestTaskMap[testName] = testSubTask
							}
						}
					}
				}
			case config.TaskDeploy:
				resetImage := false
				resetImagePolicy := setting.ResetImagePolicyTaskCompletedOrder
				if workflow, err := commonrepo.NewWorkflowColl().Find(t.PipelineName); err == nil {
					resetImage = workflow.ResetImage
					resetImagePolicy = workflow.ResetImagePolicy
				}
				timeout := 0
				if productTempl, err := template.NewProductColl().Find(t.ProductName); err == nil {
					timeout = productTempl.Timeout * 60
				}

				subDeployTaskMap := subStage.SubTasks
				for serviceName, subTask := range subDeployTaskMap {
					if deployInfo, err := base.ToDeployTask(subTask); err == nil {
						deployInfo.Timeout = timeout
						deployInfo.IsRestart = true
						deployInfo.ResetImage = resetImage
						deployInfo.ResetImagePolicy = resetImagePolicy
						if newDeployInfo, err := deployInfo.ToSubTask(); err == nil {
							subDeployTaskMap[serviceName] = newDeployInfo
						}
					}
				}
			}
		}
	}
	t.IsRestart = true
	t.Status = config.StatusCreated
	t.TaskCreator = userName
	if err := UpdateTask(t); err != nil {
		log.Errorf("update pipeline task error: %v", err)
		return e.ErrRestartTask.AddDesc(e.UpdatePipelineTaskErrMsg)
	}
	return nil
}

func TestArgsToTestSubtask(args *commonmodels.TestTaskArgs, pt *task.Task, log *zap.SugaredLogger) (*task.Testing, error) {
	var resp *task.Testing

	testTask := &task.Testing{
		TaskType: config.TaskTestingV2,
		Enabled:  true,
		TestName: "test",
	}

	allTestings, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{ProductName: args.ProductName, TestType: ""})
	if err != nil {
		log.Errorf("testArgsToTestSubtask TestingModule.List error: %v", err)
		return resp, e.ErrListTestModule.AddDesc(err.Error())
	}
	testArg := &commonmodels.TestArgs{}
	for _, testing := range allTestings {
		if args.TestName == testing.Name {
			EnsureTaskResp(testing)
			if len(testing.Repos) == 0 {
				testArg.Builds = make([]*types.Repository, 0)
			} else {
				testArg.Builds = testing.Repos
				pr, _ := strconv.Atoi(args.MergeRequestID)

				for i, build := range testArg.Builds {
					if build.Source == args.Source {
						testArg.Builds[i].PR = pr
					}
				}

			}

			if testing.PreTest != nil {
				testArg.Envs = testing.PreTest.Envs
			}

			testArg.TestModuleName = args.TestName

			// In some old testing configurations, the `pre_test.cluster_id` field is empty indicating that's a local cluster.
			// We do a protection here to avoid query failure.
			// Resaving the testing configuration after v1.8.0 will automatically populate this field.
			if testing.PreTest.ClusterID == "" {
				testing.PreTest.ClusterID = setting.LocalClusterID
			}

			clusterInfo, err := commonrepo.NewK8SClusterColl().Get(testing.PreTest.ClusterID)
			if err != nil {
				return resp, e.ErrListTestModule.AddDesc(err.Error())
			}
			testTask.Cache = clusterInfo.Cache

			// If the cluster is not configured with a cache medium, the cache cannot be used, so don't enable cache explicitly.
			if testTask.Cache.MediumType == "" {
				testTask.CacheEnable = false
			} else {
				testTask.CacheEnable = testing.CacheEnable
				testTask.CacheDirType = testing.CacheDirType
				testTask.CacheUserDir = testing.CacheUserDir
			}

			break
		}
	}

	testModule, err := GetRaw(args.TestName, "", log)
	if err != nil {
		log.Errorf("[%s]get TestingModule error: %v", args.TestName, err)
		return resp, err
	}
	testTask.Timeout = testModule.Timeout

	testTask.TestModuleName = testModule.Name
	testTask.JobCtx.TestType = testModule.TestType
	testTask.JobCtx.Builds = testModule.Repos
	testTask.JobCtx.BuildSteps = append(testTask.JobCtx.BuildSteps, &task.BuildStep{BuildType: "shell", Scripts: testModule.Scripts})

	testTask.JobCtx.TestResultPath = testModule.TestResultPath
	testTask.JobCtx.TestReportPath = testModule.TestReportPath
	testTask.JobCtx.TestThreshold = testModule.Threshold
	testTask.JobCtx.Caches = testModule.Caches
	testTask.JobCtx.ArtifactPaths = testModule.ArtifactPaths
	if testTask.Registries == nil {
		registries, err := commonservice.ListRegistryNamespaces("", true, log)
		if err != nil {
			log.Errorf("ListRegistryNamespaces err:%v", err)
		} else {
			testTask.Registries = registries
		}
	}
	if testModule.PreTest != nil {
		testTask.InstallItems = testModule.PreTest.Installs
		testTask.JobCtx.CleanWorkspace = testModule.PreTest.CleanWorkspace
		testTask.JobCtx.EnableProxy = testModule.PreTest.EnableProxy
		testTask.Namespace = testModule.PreTest.Namespace
		testTask.ClusterID = testModule.PreTest.ClusterID

		envs := testModule.PreTest.Envs[:]

		for _, env := range envs {
			for _, overwrite := range testArg.Envs {
				if overwrite.Key == env.Key {
					env.Value = overwrite.Value
					env.IsCredential = overwrite.IsCredential
					break
				}
			}
		}
		envs = append(envs, &commonmodels.KeyVal{Key: "TEST_URL", Value: GetLink(pt, configbase.SystemAddress(), config.TestType)})
		envs = append(envs, &commonmodels.KeyVal{Key: "WORKSPACE", Value: "/workspace"})
		testTask.JobCtx.EnvVars = envs
		testTask.ImageID = testModule.PreTest.ImageID
		testTask.BuildOS = testModule.PreTest.BuildOS
		testTask.ImageFrom = testModule.PreTest.ImageFrom
		testTask.ResReq = testModule.PreTest.ResReq
		testTask.ResReqSpec = testModule.PreTest.ResReqSpec
	}
	// 设置 build 安装脚本
	testTask.InstallCtx, err = BuildInstallCtx(testTask.InstallItems)
	if err != nil {
		log.Errorf("buildInstallCtx error: %v", err)
		return resp, err
	}
	// Iterate test jobctx builds, and replace it if params specified from task.
	// 外部触发的pipeline
	_ = setManunalBuilds(testTask.JobCtx.Builds, testArg.Builds, log)
	return testTask, nil
}

func EnsureTaskResp(mt *commonmodels.Testing) {
	if len(mt.Repos) == 0 {
		mt.Repos = make([]*types.Repository, 0)
	}

	if mt.PreTest != nil {
		if len(mt.PreTest.Installs) == 0 {
			mt.PreTest.Installs = make([]*commonmodels.Item, 0)
		}
		if len(mt.PreTest.Envs) == 0 {
			mt.PreTest.Envs = make([]*commonmodels.KeyVal, 0)
		}
	}
}

// GetRaw find the testing module with secret env not masked
func GetRaw(name, productName string, log *zap.SugaredLogger) (*commonmodels.Testing, error) {
	if len(name) == 0 {
		return nil, e.ErrGetTestModule.AddDesc("empty Name")
	}

	resp, err := commonrepo.NewTestingColl().Find(name, productName)
	if err != nil {
		log.Errorf("[Testing.Find] %s: error: %v", name, err)
		return nil, e.ErrGetTestModule.AddErr(err)
	}

	return resp, nil
}

func GetTesting(name, productName string, log *zap.SugaredLogger) (*commonmodels.Testing, error) {
	resp, err := GetRaw(name, productName, log)
	if err != nil {
		return nil, err
	}

	// 数据兼容： 4.1.2版本之前的定时器数据已经包含在workflow的schedule字段中，而4.1.3及以后的定时器数据需要在cronjob表中获取
	if resp.Schedules == nil {
		schedules, err := commonrepo.NewCronjobColl().List(&commonrepo.ListCronjobParam{
			ParentName: resp.Name,
			ParentType: config.TestingCronjob,
		})
		if err != nil {
			return nil, err
		}
		scheduleList := []*commonmodels.Schedule{}

		for _, v := range schedules {
			scheduleList = append(scheduleList, &commonmodels.Schedule{
				ID:           v.ID,
				Number:       v.Number,
				Frequency:    v.Frequency,
				Time:         v.Time,
				MaxFailures:  v.MaxFailure,
				TaskArgs:     v.TaskArgs,
				WorkflowArgs: v.WorkflowArgs,
				TestArgs:     v.TestArgs,
				Type:         config.ScheduleType(v.JobType),
				Cron:         v.Cron,
				Enabled:      v.Enabled,
			})
		}
		schedule := commonmodels.ScheduleCtrl{
			Enabled: resp.ScheduleEnabled,
			Items:   scheduleList,
		}
		resp.Schedules = &schedule
	}

	EnsureTestingResp(resp)

	return resp, nil
}

func EnsureTestingResp(mt *commonmodels.Testing) {
	if len(mt.Repos) == 0 {
		mt.Repos = make([]*types.Repository, 0)
	}
	for _, repo := range mt.Repos {
		repo.RepoNamespace = repo.GetRepoNamespace()
	}

	if mt.PreTest != nil {
		if len(mt.PreTest.Installs) == 0 {
			mt.PreTest.Installs = make([]*commonmodels.Item, 0)
		}
		if len(mt.PreTest.Envs) == 0 {
			mt.PreTest.Envs = make([]*commonmodels.KeyVal, 0)
		}
		// 隐藏用户设置的敏感信息
		for k := range mt.PreTest.Envs {
			if mt.PreTest.Envs[k].IsCredential {
				mt.PreTest.Envs[k].Value = setting.MaskValue
			}
		}
	}
}

type ProductNameWithType struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Namespace string `json:"namespace"`
}

func ListPipelineUpdatableProductNames(userName, pipelineName string, log *zap.SugaredLogger) ([]ProductNameWithType, error) {
	resp := make([]ProductNameWithType, 0)
	serviceName, err := findDeployServiceName(pipelineName, log)

	if err != nil {
		return resp, err
	}

	products, err := listPipelineUpdatableProducts(userName, serviceName, log)
	if err != nil {
		return resp, err
	}

	for _, prod := range products {
		prodNameWithType := ProductNameWithType{
			Name:      prod.EnvName,
			Namespace: prod.Namespace,
		}

		prodNameWithType.Type = setting.NormalModeProduct

		found := false
		for _, r := range resp {
			if prodNameWithType == r {
				found = true
				break
			}
		}
		if found {
			continue
		}

		resp = append(resp, prodNameWithType)
	}

	// adapt for qiniu deployment
	if config.OldEnvSupported() {
		resp = append(resp, ListOldEnvsByServiceName(serviceName, log)...)
	}

	return resp, nil
}

func findDeployServiceName(pipelineName string, log *zap.SugaredLogger) (resp string, err error) {
	pipe, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: pipelineName})
	if err != nil {
		log.Errorf("[%s] PipelineV2.Find error: %v", err)
		return resp, e.ErrGetPipeline.AddDesc(err.Error())
	}

	deploy, err := getFirstEnabledDeployTask(pipe.SubTasks)
	if err != nil {
		log.Errorf("[%s] GetFirstEnabledDeployTask error: %v", err)
		return resp, e.ErrGetTask.AddDesc(err.Error())
	}

	if deploy.ServiceName == "" {
		return resp, e.ErrGetTask.AddDesc("deploy task has no group name or service name")
	}

	return deploy.ServiceName, nil
}

func getFirstEnabledDeployTask(subTasks []map[string]interface{}) (*task.Deploy, error) {
	for _, subTask := range subTasks {
		pre, err := base.ToPreview(subTask)
		if err != nil {
			return nil, err
		}
		if pre.TaskType == config.TaskDeploy && pre.Enabled {
			return base.ToDeployTask(subTask)
		}
	}
	return nil, e.NewErrInvalidTaskType("DeployTask not found")
}

// ListUpdatableProductNames 列出用户可以deploy的产品环境, 包括自己的产品和被授权的产品
func listPipelineUpdatableProducts(userName, serviceName string, log *zap.SugaredLogger) ([]*commonmodels.Product, error) {
	resp := make([]*commonmodels.Product, 0)

	products, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{})
	if err != nil {
		log.Errorf("[%s] Collections.Product.List error: %v", userName, err)
		return resp, e.ErrListProducts.AddDesc(err.Error())
	}

	//userTeams, err := s.FindUserTeams(userName, log)
	//if err != nil {
	//	log.Errorf("FindUserTeams error: %v", err)
	//	return resp, err
	//}
	//userTeams := make([]string, 0)

	for _, prod := range products {
		//if prod.EnvName == userName || prod.IsUserAuthed(userName, userTeams, product.ProductWritePermission) {
		serviceNames := sets.NewString(GetServiceNames(prod)...)
		if serviceNames.Has(serviceName) {
			resp = append(resp, prod)
		}
		//}
	}

	return resp, nil
}

func GetServiceNames(p *commonmodels.Product) []string {
	resp := make([]string, 0)
	for _, group := range p.Services {
		for _, service := range group {
			resp = append(resp, service.ServiceName)
		}
	}
	return resp
}

func ListOldEnvsByServiceName(serviceName string, log *zap.SugaredLogger) []ProductNameWithType {
	resps := make([]ProductNameWithType, 0)
	nsMap := make(map[string]ProductNameWithType)
	kubeClient := krkubeclient.Client()
	selector := labels.Set{setting.ServiceLabel: serviceName}.AsSelector()

	if deployments, err := getter.ListDeployments("", selector, kubeClient); err != nil {
		log.Warnf("failed to list service by %s %v", serviceName, err)
	} else {
		for _, deployment := range deployments {
			for key := range deployment.Labels {
				if key == "s-product" && !strings.HasPrefix(deployment.Namespace, "koderover-") {
					nsMap[deployment.Namespace] = ProductNameWithType{
						Namespace: deployment.Namespace,
						Name:      deployment.Namespace,
						Type:      setting.NormalModeProduct,
					}
				}
			}
		}
	}

	for _, v := range nsMap {
		resps = append(resps, v)
	}

	return resps
}

func GePackageFileContent(pipelineName string, taskID int64, log *zap.SugaredLogger) ([]byte, string, error) {
	var packageFile, storageURL string
	//获取pipeline task
	resp, err := commonrepo.NewTaskColl().Find(taskID, pipelineName, config.SingleType)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get popeline")
	}

	for _, subTask := range resp.SubTasks {
		pre, err := base.ToPreview(subTask)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get preview")
		}
		switch pre.TaskType {

		case config.TaskBuild:
			build, err := base.ToBuildTask(subTask)
			if err != nil {
				return nil, "", fmt.Errorf("failed to get build")
			}
			packageFile = build.JobCtx.PackageFile
			storageURL = resp.StorageURI
		}
	}
	storage, err := s3.NewS3StorageFromEncryptedURI(storageURL)
	if err != nil {
		log.Errorf("failed to get s3 storage %s", storageURL)
		return nil, packageFile, fmt.Errorf("failed to get s3 storage %s", storageURL)
	}
	if storage.Subfolder != "" {
		storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", storage.Subfolder, pipelineName, resp.TaskID, "file")
	} else {
		storage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, resp.TaskID, "file")
	}

	tmpfile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, packageFile, fmt.Errorf("failed to open file %v", err)
	}

	_ = tmpfile.Close()

	defer func() {
		_ = os.Remove(tmpfile.Name())
	}()
	forcedPathStyle := true
	if storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
	if err != nil {
		return nil, packageFile, fmt.Errorf("failed to get s3 client to download %s, error is: %v", packageFile, err)
	}
	objectKey := storage.GetObjectPath(packageFile)
	err = client.Download(storage.Bucket, objectKey, tmpfile.Name())
	if err != nil {
		return nil, packageFile, fmt.Errorf("failed to download %s %v", packageFile, err)
	}
	fileBytes, err := ioutil.ReadFile(tmpfile.Name())
	return fileBytes, packageFile, err
}

func GetArtifactFileContent(pipelineName string, taskID int64, notHistoryFileFlag bool, log *zap.SugaredLogger) ([]byte, error) {
	s3Storage, client, artifactFiles, artifactResultOutByts, err := GetArtifactAndS3Info(pipelineName, "", taskID, notHistoryFileFlag, log)
	if err != nil {
		return nil, fmt.Errorf("download artifact err: %s", err)
	}
	if notHistoryFileFlag {
		return artifactResultOutByts, nil
	}
	tempDir, _ := ioutil.TempDir("", "")
	sourcePath := path.Join(tempDir, "artifact")
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		_ = os.MkdirAll(sourcePath, 0777)
	}

	for _, artifactFile := range artifactFiles {
		artifactFileArr := strings.Split(artifactFile, "/")
		if len(artifactFileArr) > 1 {
			artifactFileName := artifactFileArr[len(artifactFileArr)-1]
			file, err := os.Create(path.Join(sourcePath, artifactFileName))
			if err != nil {
				return nil, fmt.Errorf("failed to create file %s %v", artifactFileName, err)
			}
			defer func() {
				_ = file.Close()
			}()

			err = client.Download(s3Storage.Bucket, artifactFile, file.Name())
			if err != nil {
				return nil, fmt.Errorf("failed to download %s %v", artifactFile, err)
			}
		}
	}
	//将该目录压缩
	goCacheManager := new(GoCacheManager)
	artifactTarFileName := path.Join(sourcePath, "artifact.tar.gz")
	err = goCacheManager.Archive(sourcePath, artifactTarFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to Archive %s %v", sourcePath, err)
	}
	defer func() {
		_ = os.Remove(artifactTarFileName)
		_ = os.Remove(tempDir)
	}()

	fileBytes, err := ioutil.ReadFile(path.Join(sourcePath, "artifact.tar.gz"))
	return fileBytes, err
}

func GetArtifactAndS3Info(pipelineName, dir string, taskID int64, notHistoryFileFlag bool, log *zap.SugaredLogger) (*s3.S3, *s3tool.Client, []string, []byte, error) {
	fis := make([]string, 0)

	storage, err := s3.FindDefaultS3()
	if err != nil {
		log.Errorf("GetTestArtifactInfo FindDefaultS3 err:%v", err)
		return nil, nil, fis, nil, err
	}

	if storage.Subfolder != "" {
		storage.Subfolder = fmt.Sprintf("%s/%s/%d/%s", storage.Subfolder, pipelineName, taskID, "artifact")
	} else {
		storage.Subfolder = fmt.Sprintf("%s/%d/%s", pipelineName, taskID, "artifact")
	}
	forcedPathStyle := true
	if storage.Provider == setting.ProviderSourceAli {
		forcedPathStyle = false
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Insecure, forcedPathStyle)
	if err != nil {
		log.Errorf("GetTestArtifactInfo Create S3 client err:%+v", err)
		return nil, nil, fis, nil, err
	}

	if notHistoryFileFlag {
		objectKey := storage.GetObjectPath(fmt.Sprintf("%s/%s/%s", dir, "workspace", setting.ArtifactResultOut))
		object, err := client.GetFile(storage.Bucket, objectKey, &s3tool.DownloadOption{RetryNum: 2})
		if err != nil {
			log.Errorf("GetTestArtifactInfo GetFile err:%s", err)
			return nil, nil, fis, nil, err
		}
		fileByts, err := ioutil.ReadAll(object.Body)
		if err != nil {
			log.Errorf("GetTestArtifactInfo ioutil.ReadAll err:%s", err)
			return nil, nil, fis, nil, err
		}
		return storage, client, fis, fileByts, nil
	}

	prefix := storage.GetObjectPath(dir)
	files, err := client.ListFiles(storage.Bucket, prefix, true)
	if err != nil || len(files) <= 0 {
		log.Errorf("GetTestArtifactInfo ListFiles err:%v", err)
		return nil, nil, fis, nil, err
	}
	return storage, client, files, nil, nil
}
