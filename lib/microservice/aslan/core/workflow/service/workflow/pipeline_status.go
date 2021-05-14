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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func getLastPreviewBystatus(tasks []*commonrepo.TaskPreview, typeString config.PipelineType, pipelineName string, status config.Status) *commonmodels.TaskInfo {
	resp := &commonmodels.TaskInfo{}
	for _, task := range tasks {
		if task.PipelineName == pipelineName && task.Type == typeString && task.Status == status && task.TaskID > resp.TaskID {
			resp.PipelineName = task.PipelineName
			resp.TaskID = task.TaskID
			break
		}
	}
	return resp
}

func getLastPreview(tasks []*commonrepo.TaskPreview, typeString config.PipelineType, pipelineName string) *commonmodels.TaskInfo {
	resp := &commonmodels.TaskInfo{}
	for _, task := range tasks {
		if task.PipelineName == pipelineName && task.Type == typeString && task.TaskID > resp.TaskID {
			resp.PipelineName = task.PipelineName
			resp.Status = task.Status
			resp.TaskID = task.TaskID
			break
		}
	}
	return resp
}

func isFavoratePipeline(favoritePipelines []*commonmodels.Favorite, pipelineName string) bool {
	resp := false
	for _, pipeline := range favoritePipelines {
		if pipeline.Name == pipelineName {
			resp = true
			break
		}
	}
	return resp
}

type TaskV2Info struct {
	TaskID     int64  `json:"task_id"`
	Status     string `json:"status"`
	CreateTime int64  `json:"create_time"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	Url        string `json:"url"`
}

func FindTasks(commitId string, log *xlog.Logger) ([]*TaskV2Info, error) {
	resp := make([]*TaskV2Info, 0)
	//获取昨天的当前时间
	yesterdayDate := time.Now().AddDate(0, 0, -1)
	tasks, err := commonrepo.NewTaskColl().List(&commonrepo.ListTaskOption{CreateTime: yesterdayDate.Unix()})
	if err != nil {
		log.Errorf("list pipeline tasks error: %v", err)
		return resp, e.ErrListPipeline
	}

	keys := sets.NewString()
	for _, workflowTask := range tasks {
		stageArray := workflowTask.Stages
		for _, subStage := range stageArray {
			if subStage.TaskType != config.TaskBuild {
				continue
			}

			subBuildTaskMap := subStage.SubTasks
			for _, subTask := range subBuildTaskMap {
				buildInfo, err := commonservice.ToBuildTask(subTask)
				if err != nil {
					log.Errorf("get buildInfo ToBuildTask failed ! err:%v", err)
					continue
				}
				for _, repoInfo := range buildInfo.JobCtx.Builds {
					if repoInfo.CommitID == commitId || strings.HasPrefix(repoInfo.CommitID, commitId) {
						key := fmt.Sprintf("%s-%s-%d", workflowTask.ProductName, workflowTask.PipelineName, workflowTask.TaskID)

						if keys.Has(key) {
							continue
						}
						keys.Insert(key)
						resp = append(resp, &TaskV2Info{
							TaskID:     workflowTask.TaskID,
							Url:        fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/multi/%s/%d", config.ENVAslanURL(), workflowTask.ProductName, workflowTask.PipelineName, workflowTask.TaskID),
							Status:     string(workflowTask.Status),
							CreateTime: workflowTask.CreateTime,
							StartTime:  workflowTask.StartTime,
							EndTime:    workflowTask.EndTime,
						})
						break
					}
				}
			}
		}
	}

	return resp, nil
}
