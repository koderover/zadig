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

package task

import (
	"encoding/json"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
)

type Task struct {
	ID                  primitive.ObjectID       `bson:"_id,omitempty"             json:"id,omitempty"`
	TaskID              int64                    `bson:"task_id"                   json:"task_id"`
	ProductName         string                   `bson:"product_name"              json:"product_name"`
	PipelineName        string                   `bson:"pipeline_name"             json:"pipeline_name"`
	PipelineDisplayName string                   `bson:"pipeline_display_name"     json:"pipeline_display_name"`
	Type                config.PipelineType      `bson:"type"                      json:"type"`
	Status              config.Status            `bson:"status"                    json:"status,omitempty"`
	Description         string                   `bson:"description,omitempty"     json:"description,omitempty"`
	TaskCreator         string                   `bson:"task_creator"              json:"task_creator,omitempty"`
	TaskRevoker         string                   `bson:"task_revoker,omitempty"    json:"task_revoker,omitempty"`
	CreateTime          int64                    `bson:"create_time"               json:"create_time,omitempty"`
	StartTime           int64                    `bson:"start_time"                json:"start_time,omitempty"`
	EndTime             int64                    `bson:"end_time"                  json:"end_time,omitempty"`
	SubTasks            []map[string]interface{} `bson:"sub_tasks"                 json:"sub_tasks"`
	Stages              []*models.Stage          `bson:"stages"                    json:"stages"`
	ReqID               string                   `bson:"req_id,omitempty"          json:"req_id,omitempty"`
	AgentHost           string                   `bson:"agent_host,omitempty"      json:"agent_host,omitempty"`
	DockerHost          string                   `bson:"-"                         json:"docker_host,omitempty"`
	TeamName            string                   `bson:"team,omitempty"            json:"team,omitempty"`
	IsDeleted           bool                     `bson:"is_deleted"                json:"is_deleted"`
	IsArchived          bool                     `bson:"is_archived"               json:"is_archived"`
	AgentID             string                   `bson:"agent_id"                  json:"agent_id"`
	// 是否允许同时运行多次
	MultiRun bool `bson:"multi_run"                 json:"multi_run"`
	// target 服务名称, k8s为容器名称, 物理机为服务名
	Target string `bson:"target,omitempty"                    json:"target"`
	// 使用预定义编译管理模块中的内容生成SubTasks,
	// 查询条件为 服务模板名称: ServiceTmpl, 版本: BuildModuleVer
	// 如果为空，则使用pipeline自定义SubTasks
	BuildModuleVer string `bson:"build_module_ver,omitempty"                 json:"build_module_ver"`
	ServiceName    string `bson:"service_name,omitempty"              json:"service_name,omitempty"`
	// TaskArgs 单服务工作流任务参数
	TaskArgs *models.TaskArgs `bson:"task_args,omitempty"         json:"task_args,omitempty"`
	// WorkflowArgs 多服务工作流任务参数
	WorkflowArgs *models.WorkflowTaskArgs `bson:"workflow_args"         json:"workflow_args,omitempty"`
	// TestArgs 测试任务参数
	TestArgs *models.TestTaskArgs `bson:"test_args,omitempty"         json:"test_args,omitempty"`
	// ScanningArgs argument for scanning tasks
	ScanningArgs *models.ScanningArgs `bson:"scanning_args,omitempty" json:"scanning_args,omitempty"`
	// ServiceTaskArgs 脚本部署工作流任务参数
	ServiceTaskArgs *models.ServiceTaskArgs `bson:"service_args,omitempty"         json:"service_args,omitempty"`
	// ArtifactPackageTaskArgs arguments for artifact-package type tasks
	ArtifactPackageTaskArgs *models.ArtifactPackageTaskArgs `bson:"artifact_package_args,omitempty"         json:"artifact_package_args,omitempty"`
	// ConfigPayload 系统配置信息
	ConfigPayload *models.ConfigPayload      `bson:"configpayload"                  json:"config_payload,omitempty"`
	Error         string                     `bson:"error,omitempty"                json:"error,omitempty"`
	Services      [][]*models.ProductService `bson:"services"                       json:"services"`
	Render        *models.RenderInfo         `bson:"render"                         json:"render"`
	StorageURI    string                     `bson:"storage_uri,omitempty"          json:"storage_uri,omitempty"`
	// interface{} 为types.TestReport
	TestReports      map[string]interface{}       `bson:"test_reports,omitempty" json:"test_reports,omitempty"`
	RwLock           sync.Mutex                   `bson:"-"                      json:"-"`
	ResetImage       bool                         `bson:"resetImage"             json:"resetImage"`
	ResetImagePolicy setting.ResetImagePolicyType `bson:"reset_image_policy"     json:"reset_image_policy"`
	TriggerBy        *models.TriggerBy            `bson:"trigger_by,omitempty"   json:"trigger_by,omitempty"`
	Features         []string                     `bson:"features"               json:"features"`
	IsRestart        bool                         `bson:"is_restart"             json:"is_restart"`
	StorageEndpoint  string                       `bson:"storage_endpoint"       json:"storage_endpoint"`
}

func (Task) TableName() string {
	return "pipeline_task_v2"
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

type TestReport struct {
	FunctionTestSuite     *TestSuite                `bson:"function_test_suite,omitempty"         json:"functionTestSuite,omitempty"`
	PerformanceTestSuites []*PerformanceTestSuite   `bson:"performance_test_suite,omitempty"      json:"performanceTestSuite,omitempty"`
	Security              map[string]map[string]int `bson:"security,omitempty"                    json:"security,omitempty"`
}

type TestSuite struct {
	// totalNum=tests+skips successNum=tests-failures-errors
	Tests     int        `bson:"tests"                   json:"tests"                    xml:"tests,attr"`
	Failures  int        `bson:"failures"                json:"failures"                 xml:"failures,attr"`
	Successes int        `bson:"successes,omitempty"     json:"successes,omitempty"      xml:"successes,attr,omitempty"`
	Skips     int        `bson:"skips"                   json:"skips"                    xml:"skips,attr"`
	Errors    int        `bson:"errors,omitempty"        json:"errors"                   xml:"errors,attr,omitempty"`
	Time      float64    `bson:"time"                    json:"time"                     xml:"time,attr"`
	SystemOut string     `bson:"system_out,omitempty"    json:"system_out"               xml:"system-out,omitempty"`
	SystemErr string     `bson:"system-err,omitempty"    json:"system_err"               xml:"system-err,omitempty"`
	TestCases []TestCase `bson:"testcase"                json:"testcase"                 xml:"testcase"`
	SuiteType string     `bson:"-"                       json:"-"                        xml:"-"`
	Name      string     `bson:"name"                    json:"-"                        xml:"-"`
}

type PerformanceTestSuite struct {
	Label      string `bson:"label"        json:"label"`
	Samples    string `bson:"samples"      json:"samples"`
	Average    string `bson:"average"      json:"average"`
	Min        string `bson:"min"          json:"min"`
	Max        string `bson:"max"          json:"max"`
	Line       string `bson:"line"         json:"line"`
	StdDev     string `bson:"std_dev"      json:"stdDev"`
	Error      string `bson:"error"        json:"error"`
	Throughput string `bson:"throughput"   json:"throughput"`
	ReceivedKb string `bson:"received_kb"  json:"receivedKb"`
	AvgByte    string `bson:"avg_byte"     json:"avgByte"`
}

type Skipped struct {
}

type Failure struct {
	Message string `bson:"message"  json:"message" xml:"message,attr"`
	Type    string `bson:"type"     json:"type"    xml:"type,attr"`
	Text    string `bson:"text"     json:"text"    xml:",chardata"`
}

type TestCase struct {
	Name      string   `bson:"tc_name"                 json:"tc_name"      xml:"name,attr"`
	ClassName string   `bson:"classname"               json:"classname"    xml:"classname,attr"`
	Time      float64  `bson:"time"                    json:"time"         xml:"time,attr"`
	Failure   *Failure `bson:"failure,omitempty"       json:"failure"      xml:"failure,omitempty"`
	Skipped   *Skipped `bson:"skipped,omitempty"       json:"skipped"      xml:"skipped,omitempty"`
	SystemOut string   `bson:"system_out,omitempty"    json:"system_out"   xml:"system-out,omitempty"`
	SystemErr string   `bson:"system-err,omitempty"    json:"system_err"   xml:"system-err,omitempty"`
	Error     *Error   `bson:"error,omitempty"         json:"error"        xml:"error,omitempty"`
}

type Error struct {
	Message string `bson:"message"  json:"message" xml:"message,attr"`
	Type    string `bson:"type"     json:"type"    xml:"type,attr"`
	Text    string `bson:"text"     json:"text"    xml:",chardata"`
}

type TaskOpt struct {
	Task           *Task           `json:"task"`
	EnvName        string          `json:"env_name"`
	ServiceName    string          `json:"service_name"`
	ServiceInfos   *[]*ServiceInfo `json:"service_infos"`
	IsWorkflowTask bool            `json:"is_workflow_task"`
	ImageName      string          `json:"image_name"`
}
