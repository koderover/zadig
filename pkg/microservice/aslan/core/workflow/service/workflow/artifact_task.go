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
	"sort"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	taskmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

// get global config payload
func CreateArtifactPackageTask(args *commonmodels.ArtifactPackageTaskArgs, taskCreator string, log *zap.SugaredLogger) (int64, error) {
	configPayload := commonservice.GetConfigPayload(0)
	repos, err := commonservice.ListRegistryNamespaces(true, log)

	if err != nil {
		log.Errorf("CreateArtifactPackageTask query registries failed, err: %s", err)
		return 0, fmt.Errorf("failed to query registries")
	}

	registriesInvolved := sets.NewString()
	registriesInvolved.Insert(args.SourceRegistries...)
	registriesInvolved.Insert(args.TargetRegistries...)

	configPayload.RepoConfigs = make(map[string]*commonmodels.RegistryNamespace)
	for _, repo := range repos {
		if !registriesInvolved.Has(repo.ID.Hex()) {
			continue
		}
		configPayload.RepoConfigs[repo.ID.Hex()] = repo
	}

	defaultS3, err := s3.FindDefaultS3()
	if err != nil {
		err = e.ErrFindDefaultS3Storage.AddDesc("default storage is required by distribute task")
		return 0, err
	}

	defaultURL, err := defaultS3.GetEncryptedURL()
	if err != nil {
		err = e.ErrS3Storage.AddErr(err)
		return 0, err
	}

	task := &taskmodels.Task{
		Type:                    config.ArtifactType,
		ProductName:             args.ProjectName,
		Status:                  config.StatusCreated,
		ArtifactPackageTaskArgs: args,
		TaskCreator:             taskCreator,
		ConfigPayload:           configPayload,
		StorageURI:              defaultURL,
	}

	subTask, err := (&taskmodels.ArtifactPackage{
		TaskType:         config.TaskArtifactPackage,
		Enabled:          true,
		TaskStatus:       "",
		Timeout:          0,
		StartTime:        0,
		EndTime:          0,
		LogFile:          "",
		Images:           args.Images,
		SourceRegistries: args.SourceRegistries,
		TargetRegistries: args.TargetRegistries,
	}).ToSubTask()

	if err != nil {
		return 0, err
	}

	task.SubTasks = []map[string]interface{}{subTask}

	if err := ensurePipelineTask(&taskmodels.TaskOpt{
		Task: task,
	}, log); err != nil {
		log.Errorf("CreateServiceTask ensurePipelineTask err : %v", err)
		return 0, err
	}

	stages := make([]*commonmodels.Stage, 0)
	for _, subTask := range task.SubTasks {
		AddSubtaskToStage(&stages, subTask, args.EnvName)
	}
	sort.Sort(ByStageKind(stages))
	task.Stages = stages
	if len(task.Stages) == 0 {
		return 0, e.ErrCreateTask.AddDesc(e.PipelineSubTaskNotFoundErrMsg)
	}

	pipelineName := fmt.Sprintf("%s-%s-%s", args.ProjectName, args.EnvName, "artifact")
	nextTaskID, err := commonrepo.NewCounterColl().GetNextSeq(fmt.Sprintf(setting.ServiceTaskFmt, pipelineName))
	if err != nil {
		log.Errorf("CreateServiceTask Counter.GetNextSeq error: %v", err)
		return 0, e.ErrGetCounter.AddDesc(err.Error())
	}

	task.SubTasks = []map[string]interface{}{}
	task.TaskID = nextTaskID
	task.PipelineName = pipelineName

	if err := CreateTask(task); err != nil {
		log.Error(err)
		return 0, e.ErrCreateTask
	}

	return nextTaskID, nil
}
