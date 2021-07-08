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
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

type PipelinePreview struct {
	ProductName     string                   `bson:"product_name"               json:"product_name"`
	Name            string                   `bson:"name"                       json:"name"`
	TeamName        string                   `bson:"team_name"                  json:"team_name"`
	SubTasks        []map[string]interface{} `bson:"sub_tasks"                  json:"sub_tasks"`
	Types           []string                 `bson:"-"                          json:"types"`
	UpdateBy        string                   `bson:"update_by"                  json:"update_by,omitempty"`
	UpdateTime      int64                    `bson:"update_time"                json:"update_time,omitempty"`
	IsFavorite      bool                     `bson:"is_favorite"                json:"is_favorite"`
	LastestTask     *commonmodels.TaskInfo   `bson:"-"                          json:"lastest_task"`
	LastSucessTask  *commonmodels.TaskInfo   `bson:"-"                          json:"last_task_success"`
	LastFailureTask *commonmodels.TaskInfo   `bson:"-"                          json:"last_task_failure"`
	TotalDuration   int64                    `bson:"-"                          json:"total_duration"`
	TotalNum        int                      `bson:"-"                          json:"total_num"`
	TotalSuccess    int                      `bson:"-"                          json:"total_success"`
}

func ListPipelinesPreview(userID int, log *zap.SugaredLogger) ([]*PipelinePreview, error) {
	resp := make([]*PipelinePreview, 0)

	pipelineList, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{})
	if err != nil {
		log.Errorf("list PipelineV2 error: %v", err)
		return resp, e.ErrListPipeline
	}

	favoritePipeline, err := commonrepo.NewFavoriteColl().List(&commonrepo.FavoriteArgs{UserID: userID, Type: string(config.SingleType)})
	if err != nil {
		log.Errorf("list favorite pipeline error: %v", err)
		return resp, e.ErrListPipeline
	}

	workflowStats, err := commonrepo.NewWorkflowStatColl().FindWorkflowStat(&commonrepo.WorkflowStatArgs{Type: string(config.SingleType)})
	if err != nil {
		log.Errorf("list workflow stat error: %v", err)
		return resp, fmt.Errorf("列出工作流统计失败")
	}

	for i, pipeline := range pipelineList {
		EnsureSubTasksResp(pipelineList[i].SubTasks)
		preview := &PipelinePreview{}
		preview.Name = pipeline.Name
		for _, subtask := range pipeline.SubTasks {
			if value, ok := subtask["type"]; ok && value != "" {
				task := make(map[string]interface{})
				task["type"] = value
				if value, ok := subtask["enabled"]; ok && value != "" {
					task["enabled"] = value
					if v, ok := value.(bool); v && ok {
						if tType, ok := task["type"].(string); ok {
							preview.Types = append(preview.Types, tType)
						}
					}
				}

				//返回subtask 部署环境，前端会用到
				if value, ok := subtask["service_name"]; ok && value != "" {
					task["service_name"] = value
				}
				if value, ok := subtask["product_name"]; ok && value != "" {
					task["product_name"] = value
				}
				preview.SubTasks = append(preview.SubTasks, task)
			}
		}
		preview.ProductName = pipeline.ProductName
		preview.TeamName = pipeline.TeamName
		preview.UpdateBy = pipeline.UpdateBy
		preview.UpdateTime = pipeline.UpdateTime
		preview.IsFavorite = isFavoratePipeline(favoritePipeline, pipeline.Name)

		latestTask, _ := commonrepo.NewTaskColl().FindLatestTask(&commonrepo.FindTaskOption{PipelineName: pipeline.Name, Type: config.SingleType})
		if latestTask != nil {
			preview.LastestTask = &commonmodels.TaskInfo{PipelineName: latestTask.PipelineName, TaskID: latestTask.TaskID, Status: latestTask.Status}
		}

		latestPassedTask, _ := commonrepo.NewTaskColl().FindLatestTask(&commonrepo.FindTaskOption{PipelineName: pipeline.Name, Type: config.SingleType, Status: config.StatusPassed})
		if latestPassedTask != nil {
			preview.LastSucessTask = &commonmodels.TaskInfo{PipelineName: latestPassedTask.PipelineName, TaskID: latestPassedTask.TaskID}
		}

		latestFailedTask, _ := commonrepo.NewTaskColl().FindLatestTask(&commonrepo.FindTaskOption{PipelineName: pipeline.Name, Type: config.SingleType, Status: config.StatusFailed})
		if latestFailedTask != nil {
			preview.LastFailureTask = &commonmodels.TaskInfo{PipelineName: latestFailedTask.PipelineName, TaskID: latestFailedTask.TaskID}
		}

		preview.TotalDuration, preview.TotalNum, preview.TotalSuccess = findPipelineStat(preview, workflowStats)

		resp = append(resp, preview)
	}
	sort.SliceStable(resp, func(i, j int) bool { return resp[i].Name < resp[j].Name })
	return resp, nil
}

func findPipelineStat(pipeline *PipelinePreview, workflowStats []*commonmodels.WorkflowStat) (int64, int, int) {
	for _, workflowStat := range workflowStats {
		if workflowStat.Name == pipeline.Name {
			return workflowStat.TotalDuration, workflowStat.TotalSuccess + workflowStat.TotalFailure, workflowStat.TotalSuccess
		}
	}
	return 0, 0, 0
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
	URL        string `json:"url"`
}

func FindTasks(commitID string, log *zap.SugaredLogger) ([]*TaskV2Info, error) {
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
				buildInfo, err := base.ToBuildTask(subTask)
				if err != nil {
					log.Errorf("get buildInfo ToBuildTask failed ! err:%v", err)
					continue
				}
				for _, repoInfo := range buildInfo.JobCtx.Builds {
					if repoInfo.CommitID == commitID || strings.HasPrefix(repoInfo.CommitID, commitID) {
						key := fmt.Sprintf("%s-%s-%d", workflowTask.ProductName, workflowTask.PipelineName, workflowTask.TaskID)

						if keys.Has(key) {
							continue
						}
						keys.Insert(key)
						resp = append(resp, &TaskV2Info{
							TaskID:     workflowTask.TaskID,
							URL:        fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/multi/%s/%d", configbase.SystemAddress(), workflowTask.ProductName, workflowTask.PipelineName, workflowTask.TaskID),
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
