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

package models

import (
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
)

type Pipeline struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty"           json:"id,omitempty"`
	Name        string              `bson:"name"                         json:"name"`
	Type        config.PipelineType `bson:"type"                         json:"type"`
	Enabled     bool                `bson:"enabled"                      json:"enabled"`
	TeamName    string              `bson:"team"                         json:"team"`
	ProductName string              `bson:"product_name"                 json:"product_name"`
	// OrgID 单租户ID
	OrgID int `bson:"org_id"                     json:"org_id"`
	// target 服务名称, k8s为容器名称, 物理机为服务名
	Target string `bson:"target"                    json:"target"`
	// 使用预定义编译管理模块中的内容生成SubTasks,
	// 查询条件为 服务名称: Target, 版本: BuildModuleVer
	// 如果为空，则使用pipeline自定义SubTasks
	BuildModuleVer string                   `bson:"build_module_ver"             json:"build_module_ver"`
	SubTasks       []map[string]interface{} `bson:"sub_tasks"                    json:"sub_tasks"`
	Description    string                   `bson:"description,omitempty"        json:"description,omitempty"`
	UpdateBy       string                   `bson:"update_by"                    json:"update_by,omitempty"`
	CreateTime     int64                    `bson:"create_time"                  json:"create_time,omitempty"`
	UpdateTime     int64                    `bson:"update_time"                  json:"update_time,omitempty"`
	Schedules      ScheduleCtrl             `bson:"schedules,omitempty"          json:"schedules,omitempty"`
	Hook           *Hook                    `bson:"hook,omitempty"               json:"hook,omitempty"`
	Notifiers      []string                 `bson:"notifiers,omitempty"          json:"notifiers,omitempty"`
	RunCount       int                      `bson:"run_count,omitempty"          json:"run_count,omitempty"`
	DailyRunCount  float32                  `bson:"daily_run_count,omitempty"    json:"daily_run_count,omitempty"`
	PassRate       float32                  `bson:"pass_rate,omitempty"          json:"pass_rate,omitempty"`
	IsDeleted      bool                     `bson:"is_deleted"                   json:"is_deleted"`
	Slack          *Slack                   `bson:"slack"                        json:"slack"`
	NotifyCtl      *NotifyCtl               `bson:"notify_ctl"                   json:"notify_ctl"`
	IsFavorite     bool                     `bson:"-"                            json:"is_favorite"`
	// 是否允许同时运行多次
	MultiRun bool `bson:"multi_run"                    json:"multi_run"`
}

// Stage ...
type Stage struct {
	// 注意: 同一个stage暂时不能运行不同类型的Task
	TaskType    config.TaskType                   `bson:"type"               json:"type"`
	Status      config.Status                     `bson:"status"             json:"status"`
	RunParallel bool                              `bson:"run_parallel"       json:"run_parallel"`
	Desc        string                            `bson:"desc,omitempty"     json:"desc,omitempty"`
	SubTasks    map[string]map[string]interface{} `bson:"sub_tasks"          json:"sub_tasks"`
	AfterAll    bool                              `json:"after_all" bson:"after_all"`
}

type Hook struct {
	Enabled  bool      `bson:"enabled"             json:"enabled"`
	GitHooks []GitHook `bson:"git_hooks"           json:"git_hooks,omitempty"`
}

// GitHook Events: push, pull_request
// MatchFolders: 包含目录或者文件后缀
// 以!开头的目录或者后缀名为不运行pipeline的过滤条件
type GitHook struct {
	Owner        string   `bson:"repo_owner"               json:"repo_owner"`
	Repo         string   `bson:"repo"                     json:"repo"`
	Branch       string   `bson:"branch"                   json:"branch"`
	Events       []string `bson:"events"                   json:"events"`
	MatchFolders []string `bson:"match_folders"            json:"match_folders,omitempty"`
	CodehostId   int      `bson:"codehost_id"              json:"codehost_id"`
	AutoCancel   bool     `bson:"auto_cancel"              json:"auto_cancel"`
}

type SubTask map[string]interface{}

type Preview struct {
	TaskType config.TaskType `json:"type"`
	Enabled  bool            `json:"enabled"`
}

func (sb *SubTask) ToPreview() (*Preview, error) {
	var pre *Preview
	if err := IToi(sb, &pre); err != nil {
		return nil, fmt.Errorf("convert interface to SubTaskPreview error: %v", err)
	}
	return pre, nil
}

// ToBuildTask ...
func (sb *SubTask) ToBuildTask() (*Build, error) {
	var task *Build
	if err := IToi(sb, &task); err != nil {
		return nil, fmt.Errorf("convert interface to BuildTaskV2 error: %v", err)
	}
	return task, nil
}

// ToTestingTask ...
func (sb *SubTask) ToTestingTask() (*Testing, error) {
	var task *Testing
	if err := IToi(sb, &task); err != nil {
		return nil, fmt.Errorf("convert interface to Testing error: %v", err)
	}
	return task, nil
}

func IToi(before interface{}, after interface{}) error {
	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %v", err)
	}

	if err := json.Unmarshal(b, &after); err != nil {
		return fmt.Errorf("unmarshal task error: %v", err)
	}

	return nil
}

func (Pipeline) TableName() string {
	return "pipeline_v2"
}
