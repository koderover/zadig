/*
Copyright 2022 The KodeRover Authors.

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
	"crypto/md5"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commontypes "github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/dingtalk"
	"github.com/koderover/zadig/pkg/tool/guanceyun"
	"github.com/koderover/zadig/pkg/tool/lark"
	"github.com/koderover/zadig/pkg/types"
)

type WorkflowV4 struct {
	ID              primitive.ObjectID       `bson:"_id,omitempty"       yaml:"-"                   json:"id"`
	Name            string                   `bson:"name"                yaml:"name"                json:"name"`
	DisplayName     string                   `bson:"display_name"        yaml:"display_name"        json:"display_name"`
	Category        setting.WorkflowCategory `bson:"category"            yaml:"category"            json:"category"`
	KeyVals         []*KeyVal                `bson:"key_vals"            yaml:"key_vals"            json:"key_vals"`
	Params          []*Param                 `bson:"params"              yaml:"params"              json:"params"`
	Stages          []*WorkflowStage         `bson:"stages"              yaml:"stages"              json:"stages"`
	Project         string                   `bson:"project"             yaml:"project"             json:"project"`
	Description     string                   `bson:"description"         yaml:"description"         json:"description"`
	CreatedBy       string                   `bson:"created_by"          yaml:"created_by"          json:"created_by"`
	CreateTime      int64                    `bson:"create_time"         yaml:"create_time"         json:"create_time"`
	UpdatedBy       string                   `bson:"updated_by"          yaml:"updated_by"          json:"updated_by"`
	UpdateTime      int64                    `bson:"update_time"         yaml:"update_time"         json:"update_time"`
	NotifyCtls      []*NotifyCtl             `bson:"notify_ctls"         yaml:"notify_ctls"         json:"notify_ctls"`
	Debug           bool                     `bson:"debug"               yaml:"-"                   json:"debug"`
	HookCtls        []*WorkflowV4Hook        `bson:"hook_ctl"            yaml:"-"                   json:"hook_ctl"`
	JiraHookCtls    []*JiraHook              `bson:"jira_hook_ctls"      yaml:"-"                   json:"jira_hook_ctls"`
	MeegoHookCtls   []*MeegoHook             `bson:"meego_hook_ctls"     yaml:"-"                   json:"meego_hook_ctls"`
	GeneralHookCtls []*GeneralHook           `bson:"general_hook_ctls"   yaml:"-"                   json:"general_hook_ctls"`
	NotificationID  string                   `bson:"notification_id"     yaml:"-"                   json:"notification_id"`
	HookPayload     *HookPayload             `bson:"hook_payload"        yaml:"-"                   json:"hook_payload,omitempty"`
	BaseName        string                   `bson:"base_name"           yaml:"-"                   json:"base_name"`
	ShareStorages   []*ShareStorage          `bson:"share_storages"      yaml:"share_storages"      json:"share_storages"`
	Hash            string                   `bson:"hash"                yaml:"hash"                json:"hash"`
	// ConcurrencyLimit is the max number of concurrent runs of this workflow
	// -1 means no limit
	ConcurrencyLimit int          `bson:"concurrency_limit"   yaml:"concurrency_limit"   json:"concurrency_limit"`
	CustomField      *CustomField `bson:"custom_field"        yaml:"-"                   json:"custom_field"`
}

func (w *WorkflowV4) UpdateHash() {
	w.Hash = fmt.Sprintf("%x", w.CalculateHash())
}

func (w *WorkflowV4) CalculateHash() [md5.Size]byte {
	fieldList := make(map[string]interface{})
	ignoringFieldList := []string{"CreatedBy", "CreateTime", "UpdatedBy", "UpdateTime", "Description", "Hash"}
	ignoringFields := sets.NewString(ignoringFieldList...)

	val := reflect.ValueOf(*w)
	for i := 0; i < val.NumField(); i++ {
		fieldType := val.Type().Field(i)
		fieldName := fieldType.Name
		if !ignoringFields.Has(fieldName) {
			fieldValue := val.Field(i).Interface()
			fieldList[fieldName] = fieldValue
		}
	}

	jsonBytes, _ := json.Marshal(fieldList)
	return md5.Sum(jsonBytes)
}

// @todo job spec
type WorkflowStage struct {
	Name     string    `bson:"name"          yaml:"name"         json:"name"`
	Parallel bool      `bson:"parallel"      yaml:"parallel"     json:"parallel"`
	Approval *Approval `bson:"approval"      yaml:"approval"     json:"approval"`
	Jobs     []*Job    `bson:"jobs"          yaml:"jobs"         json:"jobs"`
}

type Approval struct {
	Enabled          bool                `bson:"enabled"                     yaml:"enabled"                       json:"enabled"`
	Status           config.Status       `bson:"status"                      yaml:"status"                        json:"status"`
	Type             config.ApprovalType `bson:"type"                        yaml:"type"                          json:"type"`
	Description      string              `bson:"description"                 yaml:"description"                   json:"description"`
	StartTime        int64               `bson:"start_time"                  yaml:"start_time,omitempty"          json:"start_time,omitempty"`
	EndTime          int64               `bson:"end_time"                    yaml:"end_time,omitempty"            json:"end_time,omitempty"`
	NativeApproval   *NativeApproval     `bson:"native_approval"             yaml:"native_approval,omitempty"     json:"native_approval,omitempty"`
	LarkApproval     *LarkApproval       `bson:"lark_approval"               yaml:"lark_approval,omitempty"       json:"lark_approval,omitempty"`
	DingTalkApproval *DingTalkApproval   `bson:"dingtalk_approval"           yaml:"dingtalk_approval,omitempty"   json:"dingtalk_approval,omitempty"`
}

type NativeApproval struct {
	Timeout         int                    `bson:"timeout"                     yaml:"timeout"                    json:"timeout"`
	ApproveUsers    []*User                `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
	NeededApprovers int                    `bson:"needed_approvers"            yaml:"needed_approvers"           json:"needed_approvers"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
	// InstanceCode: native approval instance code, save for working after restart aslan
	InstanceCode string `bson:"instance_code"               yaml:"instance_code"              json:"instance_code"`
}

type DingTalkApproval struct {
	Timeout int `bson:"timeout"                     yaml:"timeout"                    json:"timeout"`
	// ID: dintalk im app mongodb id
	ID string `bson:"approval_id"                 yaml:"approval_id"                json:"approval_id"`
	// DefaultApprovalInitiator if not set, use workflow task creator as approval initiator
	DefaultApprovalInitiator *DingTalkApprovalUser   `bson:"default_approval_initiator" yaml:"default_approval_initiator" json:"default_approval_initiator"`
	ApprovalNodes            []*DingTalkApprovalNode `bson:"approval_nodes"             yaml:"approval_nodes"             json:"approval_nodes"`
	// InstanceCode: dingtalk approval instance code
	InstanceCode string `bson:"instance_code"              yaml:"instance_code"              json:"instance_code"`
}

type DingTalkApprovalNode struct {
	ApproveUsers    []*DingTalkApprovalUser `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
	Type            dingtalk.ApprovalAction `bson:"type"                        yaml:"type"                       json:"type"`
	RejectOrApprove config.ApproveOrReject  `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
}

type DingTalkApprovalUser struct {
	ID              string                 `bson:"id"                          yaml:"id"                         json:"id"`
	Name            string                 `bson:"name"                        yaml:"name"                       json:"name"`
	Avatar          string                 `bson:"avatar"                      yaml:"avatar"                     json:"avatar"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve,omitempty"           yaml:"-"                          json:"reject_or_approve,omitempty"`
	Comment         string                 `bson:"comment,omitempty"                     yaml:"-"                          json:"comment,omitempty"`
	OperationTime   int64                  `bson:"operation_time,omitempty"              yaml:"-"                          json:"operation_time,omitempty"`
}

type LarkApproval struct {
	Timeout int `bson:"timeout"                     yaml:"timeout"                    json:"timeout"`
	// ID: lark im app mongodb id
	ID string `bson:"approval_id"                 yaml:"approval_id"                json:"approval_id"`
	// DefaultApprovalInitiator if not set, use workflow task creator as approval initiator
	DefaultApprovalInitiator *LarkApprovalUser `bson:"default_approval_initiator" yaml:"default_approval_initiator" json:"default_approval_initiator"`
	// Deprecated: use ApprovalNodes instead
	ApproveUsers  []*LarkApprovalUser `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
	ApprovalNodes []*LarkApprovalNode `bson:"approval_nodes"               yaml:"approval_nodes"              json:"approval_nodes"`
	// InstanceCode: lark approval instance code
	InstanceCode string `bson:"instance_code"               yaml:"instance_code"              json:"instance_code"`
}

// GetNodeTypeKey get node type key for deduplication
func (l LarkApproval) GetNodeTypeKey() string {
	var keys []string
	for _, node := range l.ApprovalNodes {
		keys = append(keys, string(node.Type))
	}
	return strings.Join(keys, "-")
}

// GetLarkApprovalNode convert approval node to lark sdk approval node
func (l LarkApproval) GetLarkApprovalNode() (resp []*lark.ApprovalNode) {
	for _, node := range l.ApprovalNodes {
		resp = append(resp, &lark.ApprovalNode{
			ApproverIDList: func() (re []string) {
				for _, user := range node.ApproveUsers {
					re = append(re, user.ID)
				}
				return
			}(),
			Type: node.Type,
		})
	}
	return
}

type LarkApprovalNode struct {
	ApproveUsers    []*LarkApprovalUser    `bson:"approve_users"               yaml:"approve_users"              json:"approve_users"`
	Type            lark.ApproveType       `bson:"type"                        yaml:"type"                       json:"type"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
}

type LarkApprovalUser struct {
	lark.UserInfo   `bson:",inline"  yaml:",inline"  json:",inline"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve,omitempty"           yaml:"-"                          json:"reject_or_approve,omitempty"`
	Comment         string                 `bson:"comment,omitempty"                     yaml:"-"                          json:"comment,omitempty"`
	OperationTime   int64                  `bson:"operation_time,omitempty"              yaml:"-"                          json:"operation_time,omitempty"`
}

type User struct {
	Type            string                 `bson:"type"                        yaml:"type"                       json:"type"`
	UserID          string                 `bson:"user_id,omitempty"           yaml:"user_id,omitempty"          json:"user_id,omitempty"`
	UserName        string                 `bson:"user_name,omitempty"         yaml:"user_name,omitempty"        json:"user_name,omitempty"`
	GroupID         string                 `bson:"group_id,omitempty"          yaml:"group_id,omitempty"         json:"group_id,omitempty"`
	GroupName       string                 `bson:"group_name,omitempty"        yaml:"group_name,omitempty"       json:"group_name,omitempty"`
	RejectOrApprove config.ApproveOrReject `bson:"reject_or_approve"           yaml:"-"                          json:"reject_or_approve"`
	Comment         string                 `bson:"comment"                     yaml:"-"                          json:"comment"`
	OperationTime   int64                  `bson:"operation_time"              yaml:"-"                          json:"operation_time"`
}

type Job struct {
	Name    string         `bson:"name"           yaml:"name"     json:"name"`
	JobType config.JobType `bson:"type"           yaml:"type"     json:"type"`
	// only for webhook workflow args to skip some tasks.
	Skipped        bool                     `bson:"skipped"        yaml:"skipped"    json:"skipped"`
	Spec           interface{}              `bson:"spec"           yaml:"spec"       json:"spec"`
	RunPolicy      config.JobRunPolicy      `bson:"run_policy"     yaml:"run_policy" json:"run_policy"`
	ServiceModules []*WorkflowServiceModule `bson:"service_modules"                  json:"service_modules"`
}

type WorkflowServiceModule struct {
	ServiceModule string              `bson:"service_module" json:"service_module"`
	ServiceName   string              `bson:"service_name"   json:"service_name"`
	CodeInfo      []*types.Repository `bson:"code_info"      json:"code_info"`
}

type CustomDeployJobSpec struct {
	Namespace          string `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	ClusterID          string `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	DockerRegistryID   string `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	SkipCheckRunStatus bool   `bson:"skip_check_run_status"  json:"skip_check_run_status" yaml:"skip_check_run_status"`
	// support two sources, runtime/fixed.
	Source string `bson:"source"                 json:"source"                yaml:"source"`
	// unit is minute.
	Timeout int64            `bson:"timeout"                json:"timeout"               yaml:"timeout"`
	Targets []*DeployTargets `bson:"targets"                json:"targets"               yaml:"targets"`
}

type DeployTargets struct {
	// workload_type/workload_name/container_name.
	Target    string `bson:"target"           json:"target"            yaml:"target"`
	Image     string `bson:"image,omitempty"  json:"image,omitempty"   yaml:"image,omitempty"`
	ImageName string `bson:"image_name,omitempty"  json:"image_name,omitempty"   yaml:"image_name,omitempty"`
}

type PluginJobSpec struct {
	Properties *JobProperties  `bson:"properties"               yaml:"properties"              json:"properties"`
	Plugin     *PluginTemplate `bson:"plugin"                   yaml:"plugin"                  json:"plugin"`
}

type FreestyleJobSpec struct {
	Properties *JobProperties `bson:"properties"     yaml:"properties"    json:"properties"`
	Steps      []*Step        `bson:"steps"          yaml:"steps"         json:"steps"`
	Outputs    []*Output      `bson:"outputs"        yaml:"outputs"       json:"outputs"`
}

type ZadigBuildJobSpec struct {
	DockerRegistryID string             `bson:"docker_registry_id"     yaml:"docker_registry_id"     json:"docker_registry_id"`
	ServiceAndBuilds []*ServiceAndBuild `bson:"service_and_builds"     yaml:"service_and_builds"     json:"service_and_builds"`
}

type ServiceAndBuild struct {
	ServiceName      string              `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	ServiceModule    string              `bson:"service_module"      yaml:"service_module"   json:"service_module"`
	BuildName        string              `bson:"build_name"          yaml:"build_name"       json:"build_name"`
	Image            string              `bson:"image"                   yaml:"-"                json:"image"`
	ImageName        string              `bson:"image_name"                   yaml:"image_name"                json:"image_name"`
	Package          string              `bson:"-"                   yaml:"-"                json:"package"`
	KeyVals          []*KeyVal           `bson:"key_vals"            yaml:"key_vals"         json:"key_vals"`
	Repos            []*types.Repository `bson:"repos"               yaml:"repos"            json:"repos"`
	ShareStorageInfo *ShareStorageInfo   `bson:"share_storage_info"   yaml:"share_storage_info"   json:"share_storage_info"`
}

type ZadigDeployJobSpec struct {
	Env                string `bson:"env"                      yaml:"env"                         json:"env"`
	Production         bool   `bson:"production"               yaml:"production"                  json:"production"`
	DeployType         string `bson:"deploy_type"              yaml:"deploy_type,omitempty"       json:"deploy_type"`
	SkipCheckRunStatus bool   `bson:"skip_check_run_status"    yaml:"skip_check_run_status"       json:"skip_check_run_status"`
	// fromjob/runtime, runtime 表示运行时输入，fromjob 表示从上游构建任务中获取
	Source         config.DeploySourceType `bson:"source"     yaml:"source"     json:"source"`
	DeployContents []config.DeployContent  `bson:"deploy_contents"     yaml:"deploy_contents"     json:"deploy_contents"`
	// 当 source 为 fromjob 时需要，指定部署镜像来源是上游哪一个构建任务
	JobName string `bson:"job_name"             yaml:"job_name"             json:"job_name"`
	// save the origin quoted job name
	OriginJobName    string             `bson:"origin_job_name"      yaml:"origin_job_name"      json:"origin_job_name"`
	ServiceAndImages []*ServiceAndImage `bson:"service_and_images"   yaml:"service_and_images"   json:"service_and_images"`
	Services         []*DeployService   `bson:"services"             yaml:"services"             json:"services"`
}

type ZadigHelmChartDeployJobSpec struct {
	Env                string             `bson:"env"                      yaml:"env"                         json:"env"`
	EnvSource          string             `bson:"env_source"               yaml:"env_source"                  json:"env_source"`
	SkipCheckRunStatus bool               `bson:"skip_check_run_status"    yaml:"skip_check_run_status"       json:"skip_check_run_status"`
	DeployHelmCharts   []*DeployHelmChart `bson:"deploy_helm_charts"       yaml:"deploy_helm_charts"          json:"deploy_helm_charts"`
}

type DeployHelmChart struct {
	ReleaseName  string `bson:"release_name"          yaml:"release_name"             json:"release_name"`
	ChartRepo    string `bson:"chart_repo"            yaml:"chart_repo"               json:"chart_repo"`
	ChartName    string `bson:"chart_name"            yaml:"chart_name"               json:"chart_name"`
	ChartVersion string `bson:"chart_version"         yaml:"chart_version"            json:"chart_version"`
	ValuesYaml   string `bson:"values_yaml"           yaml:"values_yaml"              json:"values_yaml"`
}

type DeployService struct {
	ServiceName string `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	// VariableConfigs added since 1.18
	VariableConfigs []*DeplopyVariableConfig `bson:"variable_configs"                 json:"variable_configs"                    yaml:"variable_configs"`
	// VariableKVs added since 1.18
	VariableKVs []*commontypes.RenderVariableKV `bson:"variable_kvs"       yaml:"variable_kvs"    json:"variable_kvs"`
	// LatestVariableKVs added since 1.18
	LatestVariableKVs []*commontypes.RenderVariableKV `bson:"latest_variable_kvs"       yaml:"latest_variable_kvs"    json:"latest_variable_kvs"`
	// VariableYaml added since 1.18, used for helm production environments
	VariableYaml string `bson:"variable_yaml" yaml:"variable_yaml" json:"variable_yaml"`
	// KeyVals Deprecated since 1.18
	KeyVals []*ServiceKeyVal `bson:"key_vals"            yaml:"key_vals"         json:"key_vals"`
	// LatestKeyVals Deprecated since 1.18
	LatestKeyVals []*ServiceKeyVal `bson:"latest_key_vals"     yaml:"latest_key_vals"  json:"latest_key_vals"`
	UpdateConfig  bool             `bson:"update_config"       yaml:"update_config"    json:"update_config"`
	Updatable     bool             `bson:"updatable"           yaml:"updatable"        json:"updatable"`
}

type DeplopyVariableConfig struct {
	VariableKey       string `bson:"variable_key"                     json:"variable_key"                        yaml:"variable_key"`
	UseGlobalVariable bool   `bson:"use_global_variable"              json:"use_global_variable"                 yaml:"use_global_variable"`
}

type ServiceKeyVal struct {
	Key          string               `bson:"key"                       json:"key"                         yaml:"key"`
	Value        interface{}          `bson:"value"                     json:"value"                       yaml:"value"`
	Type         ParameterSettingType `bson:"type,omitempty"            json:"type,omitempty"              yaml:"type"`
	ChoiceOption []string             `bson:"choice_option,omitempty"   json:"choice_option,omitempty"     yaml:"choice_option,omitempty"`
	IsCredential bool                 `bson:"is_credential"             json:"is_credential"               yaml:"is_credential"`
}

type ServiceAndImage struct {
	ServiceName   string `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	ServiceModule string `bson:"service_module"      yaml:"service_module"   json:"service_module"`
	Image         string `bson:"image"               yaml:"image"            json:"image"`
	ImageName     string `bson:"image_name"          yaml:"image_name"       json:"image_name"`
}

type ZadigDistributeImageJobSpec struct {
	// fromjob/runtime, `runtime` means runtime input, `fromjob` means that it is obtained from the upstream build job
	Source config.DeploySourceType `bson:"source"     yaml:"source"     json:"source"`
	// required when source is `fromjob`, specify which upstream build job the distribution image source is
	JobName string `bson:"job_name"                       json:"job_name"                      yaml:"job_name"`
	// not required when source is fromjob, directly obtained from upstream build job information
	SourceRegistryID string              `bson:"source_registry_id"             json:"source_registry_id"            yaml:"source_registry_id"`
	TargetRegistryID string              `bson:"target_registry_id"             json:"target_registry_id"            yaml:"target_registry_id"`
	Targets          []*DistributeTarget `bson:"targets"                        json:"targets"                       yaml:"targets"`
	// unit is minute.
	Timeout                  int64  `bson:"timeout"                        json:"timeout"                       yaml:"timeout"`
	ClusterID                string `bson:"cluster_id"                     json:"cluster_id"                    yaml:"cluster_id"`
	StrategyID               string `bson:"strategy_id"                    json:"strategy_id"                   yaml:"strategy_id"`
	EnableTargetImageTagRule bool   `bson:"enable_target_image_tag_rule" json:"enable_target_image_tag_rule" yaml:"enable_target_image_tag_rule"`
	TargetImageTagRule       string `bson:"target_image_tag_rule"        json:"target_image_tag_rule"        yaml:"target_image_tag_rule"`
}

type DistributeTarget struct {
	ServiceName   string `bson:"service_name"              yaml:"service_name"               json:"service_name"`
	ServiceModule string `bson:"service_module"            yaml:"service_module"             json:"service_module"`
	SourceTag     string `bson:"source_tag,omitempty"      yaml:"source_tag,omitempty"       json:"source_tag,omitempty"`
	TargetTag     string `bson:"target_tag,omitempty"      yaml:"target_tag,omitempty"       json:"target_tag,omitempty"`
	ImageName     string `bson:"image_name,omitempty"      yaml:"image_name,omitempty"       json:"image_name,omitempty"`
	SourceImage   string `bson:"source_image,omitempty"    yaml:"source_image,omitempty"     json:"source_image,omitempty"`
	TargetImage   string `bson:"target_image,omitempty"    yaml:"target_image,omitempty"     json:"target_image,omitempty"`
	// if UpdateTag was false, use SourceTag as TargetTag.
	UpdateTag bool `bson:"update_tag"                yaml:"update_tag"                json:"update_tag"`
}

type ZadigTestingJobSpec struct {
	TestType        config.TestModuleType   `bson:"test_type"        yaml:"test_type"        json:"test_type"`
	Source          config.DeploySourceType `bson:"source"           yaml:"source"           json:"source"`
	JobName         string                  `bson:"job_name"         yaml:"job_name"         json:"job_name"`
	OriginJobName   string                  `bson:"origin_job_name"  yaml:"origin_job_name"  json:"origin_job_name"`
	TargetServices  []*ServiceTestTarget    `bson:"target_services"  yaml:"target_services"  json:"target_services"`
	TestModules     []*TestModule           `bson:"test_modules"     yaml:"test_modules"     json:"test_modules"`
	ServiceAndTests []*ServiceAndTest       `bson:"service_and_tests" yaml:"service_and_tests" json:"service_and_tests"`
}

type ServiceAndTest struct {
	ServiceName   string `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	ServiceModule string `bson:"service_module"      yaml:"service_module"   json:"service_module"`
	TestModule    `bson:",inline"  yaml:",inline"  json:",inline"`
}

type ServiceTestTarget struct {
	ServiceName   string `bson:"service_name"        yaml:"service_name"     json:"service_name"`
	ServiceModule string `bson:"service_module"      yaml:"service_module"   json:"service_module"`
}

type TestModule struct {
	Name             string              `bson:"name"                yaml:"name"             json:"name"`
	ProjectName      string              `bson:"project_name"        yaml:"project_name"     json:"project_name"`
	KeyVals          []*KeyVal           `bson:"key_vals"            yaml:"key_vals"         json:"key_vals"`
	Repos            []*types.Repository `bson:"repos"               yaml:"repos"            json:"repos"`
	ShareStorageInfo *ShareStorageInfo   `bson:"share_storage_info"   yaml:"share_storage_info"   json:"share_storage_info"`
}

type ZadigScanningJobSpec struct {
	Scannings []*ScanningModule `bson:"scannings"     yaml:"scannings"     json:"scannings"`
}

type ScanningModule struct {
	Name             string              `bson:"name"                yaml:"name"             json:"name"`
	ProjectName      string              `bson:"project_name"        yaml:"project_name"     json:"project_name"`
	Repos            []*types.Repository `bson:"repos"               yaml:"repos"            json:"repos"`
	ShareStorageInfo *ShareStorageInfo   `bson:"share_storage_info"   yaml:"share_storage_info"   json:"share_storage_info"`
}

type BlueGreenDeployJobSpec struct {
	ClusterID        string             `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace        string             `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	DockerRegistryID string             `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	Targets          []*BlueGreenTarget `bson:"targets"                json:"targets"               yaml:"targets"`
}

type BlueGreenDeployV2JobSpec struct {
	Version    string                      `bson:"version"               json:"version"              yaml:"version"`
	Production bool                        `bson:"production"               json:"production"              yaml:"production"`
	Env        string                      `bson:"env"               json:"env"              yaml:"env"`
	Source     string                      `bson:"source"               json:"source"              yaml:"source"`
	Services   []*BlueGreenDeployV2Service `bson:"services" json:"services" yaml:"services"`
}

type BlueGreenDeployV2ServiceModuleAndImage struct {
	ServiceModule string `bson:"service_module" json:"service_module" yaml:"service_module"`
	Image         string `bson:"image"          json:"image"          yaml:"image"`
	// Following fields only save for frontend
	ImageName   string `bson:"image_name"     json:"image_name"     yaml:"image_name"`
	Name        string `bson:"name"           json:"name"           yaml:"name"`
	ServiceName string `bson:"service_name"   json:"service_name"   yaml:"service_name"`
	Value       string `bson:"value"          json:"value"          yaml:"value"`
}

type BlueGreenDeployV2Service struct {
	// ServiceName is zadig service name
	ServiceName         string                                    `bson:"service_name" json:"service_name" yaml:"service_name"`
	BlueServiceYaml     string                                    `bson:"blue_service_yaml" json:"blue_service_yaml" yaml:"blue_service_yaml"`
	BlueServiceName     string                                    `bson:"blue_service_name,omitempty" json:"blue_service_name,omitempty" yaml:"blue_service_name,omitempty"`
	BlueDeploymentYaml  string                                    `bson:"blue_deployment_yaml,omitempty" json:"blue_deployment_yaml,omitempty" yaml:"blue_deployment_yaml,omitempty"`
	BlueDeploymentName  string                                    `bson:"blue_deployment_name,omitempty" json:"blue_deployment_name,omitempty" yaml:"blue_deployment_name,omitempty"`
	GreenDeploymentName string                                    `bson:"green_deployment_name,omitempty" json:"green_deployment_name,omitempty" yaml:"green_deployment_name,omitempty"`
	GreenServiceName    string                                    `bson:"green_service_name,omitempty" json:"green_service_name,omitempty" yaml:"green_service_name,omitempty"`
	ServiceAndImage     []*BlueGreenDeployV2ServiceModuleAndImage `bson:"service_and_image" json:"service_and_image" yaml:"service_and_image"`
}

type BlueGreenReleaseJobSpec struct {
	FromJob string `bson:"from_job"               json:"from_job"              yaml:"from_job"`
}

type BlueGreenReleaseV2JobSpec struct {
	FromJob string `bson:"from_job"               json:"from_job"              yaml:"from_job"`
}

type BlueGreenTarget struct {
	K8sServiceName     string `bson:"k8s_service_name"       json:"k8s_service_name"      yaml:"k8s_service_name"`
	BlueK8sServiceName string `bson:"blue_k8s_service_name"  json:"blue_k8s_service_name" yaml:"-"`
	ContainerName      string `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Image              string `bson:"image"                  json:"image"                 yaml:"image"`
	// unit is minute.
	DeployTimeout    int64  `bson:"deploy_timeout"         json:"deploy_timeout"        yaml:"deploy_timeout"`
	WorkloadName     string `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	BlueWorkloadName string `bson:"blue_workload_name"     json:"blue_workload_name"    yaml:"-"`
	WorkloadType     string `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
	Version          string `bson:"version"                json:"version"               yaml:"-"`
}

type CanaryDeployJobSpec struct {
	ClusterID        string          `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace        string          `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	DockerRegistryID string          `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	Targets          []*CanaryTarget `bson:"targets"                json:"targets"               yaml:"targets"`
}

type CanaryReleaseJobSpec struct {
	FromJob string `bson:"from_job"               json:"from_job"              yaml:"from_job"`
	// unit is minute.
	ReleaseTimeout int64 `bson:"release_timeout"        json:"release_timeout"       yaml:"release_timeout"`
}

type CanaryTarget struct {
	K8sServiceName   string `bson:"k8s_service_name"       json:"k8s_service_name"      yaml:"k8s_service_name"`
	ContainerName    string `bson:"container_name"         json:"container_name"        yaml:"container_name"`
	Image            string `bson:"image"                  json:"image"                 yaml:"image"`
	CanaryPercentage int    `bson:"canary_percentage"      json:"canary_percentage"     yaml:"canary_percentage"`
	// unit is minute.
	DeployTimeout int64  `bson:"deploy_timeout"         json:"deploy_timeout"        yaml:"deploy_timeout"`
	WorkloadName  string `bson:"workload_name"          json:"workload_name"         yaml:"workload_name"`
	WorkloadType  string `bson:"workload_type"          json:"workload_type"         yaml:"workload_type"`
}

type GrayReleaseJobSpec struct {
	ClusterID        string `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace        string `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	DockerRegistryID string `bson:"docker_registry_id"     json:"docker_registry_id"    yaml:"docker_registry_id"`
	FromJob          string `bson:"from_job"               json:"from_job"              yaml:"from_job"`
	// unit is minute.
	DeployTimeout int64                `bson:"deploy_timeout"         json:"deploy_timeout"        yaml:"deploy_timeout"`
	GrayScale     int                  `bson:"gray_scale"             json:"gray_scale"            yaml:"gray_scale"`
	Targets       []*GrayReleaseTarget `bson:"targets"                json:"targets"               yaml:"targets"`
}

type GrayReleaseTarget struct {
	WorkloadType  string `bson:"workload_type"             json:"workload_type"            yaml:"workload_type"`
	WorkloadName  string `bson:"workload_name"             json:"workload_name"            yaml:"workload_name"`
	Replica       int    `bson:"replica,omitempty"         json:"replica,omitempty"        yaml:"replica,omitempty"`
	ContainerName string `bson:"container_name"            json:"container_name"           yaml:"container_name"`
	Image         string `bson:"image,omitempty"           json:"image,omitempty"          yaml:"image,omitempty"`
}

type K8sPatchJobSpec struct {
	ClusterID  string       `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace  string       `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	PatchItems []*PatchItem `bson:"patch_items"            json:"patch_items"           yaml:"patch_items"`
}

type PatchItem struct {
	ResourceName    string   `bson:"resource_name"                json:"resource_name"               yaml:"resource_name"`
	ResourceKind    string   `bson:"resource_kind"                json:"resource_kind"               yaml:"resource_kind"`
	ResourceGroup   string   `bson:"resource_group"               json:"resource_group"              yaml:"resource_group"`
	ResourceVersion string   `bson:"resource_version"             json:"resource_version"            yaml:"resource_version"`
	PatchContent    string   `bson:"patch_content"                json:"patch_content"               yaml:"patch_content"`
	Params          []*Param `bson:"params"                       json:"params"                      yaml:"params"`
	// support strategic-merge/merge/json
	PatchStrategy string `bson:"patch_strategy"          json:"patch_strategy"         yaml:"patch_strategy"`
}

type GrayRollbackJobSpec struct {
	ClusterID string `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	Namespace string `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	// unit is minute.
	RollbackTimeout int64                 `bson:"rollback_timeout"       json:"rollback_timeout"      yaml:"rollback_timeout"`
	Targets         []*GrayRollbackTarget `bson:"targets"                json:"targets"               yaml:"targets"`
}

type GrayRollbackTarget struct {
	WorkloadType  string `bson:"workload_type"             json:"workload_type"            yaml:"workload_type"`
	WorkloadName  string `bson:"workload_name"             json:"workload_name"            yaml:"workload_name"`
	OriginImage   string `bson:"-"                         json:"origin_image"             yaml:"origin_image,omitempty"`
	OriginReplica int    `bson:"-"                         json:"origin_replica"           yaml:"origin_replica,omitempty"`
}

type JiraJobSpec struct {
	ProjectID          string `bson:"project_id"  json:"project_id"  yaml:"project_id"`
	JiraID             string `bson:"jira_id" json:"jira_id" yaml:"jira_id"`
	JiraSystemIdentity string `bson:"jira_system_identity" json:"jira_system_identity" yaml:"jira_system_identity"`
	JiraURL            string `bson:"jira_url" json:"jira_url" yaml:"jira_url"`
	// QueryType: "" means common, "advanced" means use custom jql
	QueryType string `bson:"query_type" json:"query_type" yaml:"query_type"`
	// JQL: when query type is advanced, use this
	JQL string `bson:"jql" json:"jql" yaml:"jql"`

	IssueType    string     `bson:"issue_type"  json:"issue_type"  yaml:"issue_type"`
	Issues       []*IssueID `bson:"issues" json:"issues" yaml:"issues"`
	TargetStatus string     `bson:"target_status" json:"target_status" yaml:"target_status"`
	Source       string     `bson:"source" json:"source" yaml:"source"`
}

type IstioJobSpec struct {
	First             bool              `bson:"first"              json:"first"              yaml:"first"`
	ClusterID         string            `bson:"cluster_id"         json:"cluster_id"         yaml:"cluster_id"`
	FromJob           string            `bson:"from_job"           json:"from_job"           yaml:"from_job"`
	RegistryID        string            `bson:"registry_id"        json:"registry_id"        yaml:"registry_id"`
	Namespace         string            `bson:"namespace"          json:"namespace"          yaml:"namespace"`
	Timeout           int64             `bson:"timeout"            json:"timeout"            yaml:"timeout"`
	ReplicaPercentage int64             `bson:"replica_percentage" json:"replica_percentage" yaml:"replica_percentage"`
	Weight            int64             `bson:"weight"             json:"weight"             yaml:"weight"`
	Targets           []*IstioJobTarget `bson:"targets"            json:"targets"            yaml:"targets"`
}

type IstioRollBackJobSpec struct {
	ClusterID string            `bson:"cluster_id"  json:"cluster_id"  yaml:"cluster_id"`
	Namespace string            `bson:"namespace"   json:"namespace"   yaml:"namespace"`
	Timeout   int64             `bson:"timeout"     json:"timeout"     yaml:"timeout"`
	Targets   []*IstioJobTarget `bson:"targets"     json:"targets"     yaml:"targets"`
}

type ApolloJobSpec struct {
	ApolloID      string             `bson:"apolloID" json:"apolloID" yaml:"apolloID"`
	NamespaceList []*ApolloNamespace `bson:"namespaceList" json:"namespaceList" yaml:"namespaceList"`
}

type ApolloNamespace struct {
	AppID      string      `bson:"appID"             json:"appID"             yaml:"appID"`
	ClusterID  string      `bson:"clusterID"         json:"clusterID"         yaml:"clusterID"`
	Env        string      `bson:"env"               json:"env"               yaml:"env"`
	Namespace  string      `bson:"namespace"         json:"namespace"         yaml:"namespace"`
	Type       string      `bson:"type"              json:"type"              yaml:"type"`
	KeyValList []*ApolloKV `bson:"kv"                json:"kv"                yaml:"kv"`
}

type MeegoTransitionJobSpec struct {
	Source              string                     `bson:"source"                json:"source"`
	ProjectKey          string                     `bson:"project_key"           json:"project_key"           yaml:"project_key"`
	ProjectName         string                     `bson:"project_name"          json:"project_name"          yaml:"project_name"`
	MeegoID             string                     `bson:"meego_id"              json:"meego_id"              yaml:"meego_id"`
	MeegoSystemIdentity string                     `bson:"meego_system_identity" json:"meego_system_identity" yaml:"meego_system_identity"`
	MeegoURL            string                     `bson:"meego_url"             json:"meego_url"             yaml:"meego_url"`
	WorkItemType        string                     `bson:"work_item_type"        json:"work_item_type"        yaml:"work_item_type"`
	WorkItemTypeKey     string                     `bson:"work_item_type_key"    json:"work_item_type_key"    yaml:"work_item_type_key"`
	Link                string                     `bson:"link"                  json:"link"                  yaml:"link"`
	WorkItems           []*MeegoWorkItemTransition `bson:"work_items"            json:"work_items"            yaml:"work_items"`
}

type MeegoWorkItemTransition struct {
	ID              int    `bson:"id"                json:"id"                yaml:"id"`
	Name            string `bson:"name"              json:"name"              yaml:"name"`
	TransitionID    int64  `bson:"transition_id"     json:"transition_id"     yaml:"transition_id"`
	TargetStateKey  string `bson:"target_state_key"  json:"target_state_key"  yaml:"target_state_key"`
	TargetStateName string `bson:"target_state_name" json:"target_state_name" yaml:"target_state_name"`
	Status          string `bson:"status"            json:"status"            yaml:"status,omitempty"`
}

type IstioJobTarget struct {
	WorkloadName       string `bson:"workload_name"             json:"workload_name"             yaml:"workload_name"`
	ContainerName      string `bson:"container_name"            json:"container_name"            yaml:"container_name"`
	VirtualServiceName string `bson:"virtual_service_name"      json:"virtual_service_name"      yaml:"virtual_service_name"`
	Host               string `bson:"host"                      json:"host"                      yaml:"host"`
	Image              string `bson:"image"                     json:"image"                     yaml:"image,omitempty"`
	CurrentReplica     int    `bson:"current_replica,omitempty" json:"current_replica,omitempty" yaml:"current_replica,omitempty"`
	TargetReplica      int    `bson:"target_replica,omitempty"  json:"target_replica,omitempty"  yaml:"target_replica,omitempty"`
}

type GuanceyunCheckJobSpec struct {
	ID   string `bson:"id" json:"id" yaml:"id"`
	Name string `bson:"name" json:"name" yaml:"name"`
	// CheckTime minute
	CheckTime int64               `bson:"check_time" json:"check_time" yaml:"check_time"`
	CheckMode string              `bson:"check_mode" json:"check_mode" yaml:"check_mode"`
	Monitors  []*GuanceyunMonitor `bson:"monitors" json:"monitors" yaml:"monitors"`
}

type GuanceyunMonitor struct {
	ID   string `bson:"id" json:"id" yaml:"id"`
	Name string `bson:"name" json:"name" yaml:"name"`
	// Level is the lowest level to trigger alarm
	Level  guanceyun.Level `bson:"level" json:"level" yaml:"level"`
	Status string          `bson:"status,omitempty" json:"status,omitempty" yaml:"status,omitempty"`
	Url    string          `bson:"url,omitempty" json:"url,omitempty" yaml:"url,omitempty"`
}

type MseGrayReleaseJobSpec struct {
	Production         bool                     `bson:"production" json:"production" yaml:"production"`
	GrayTag            string                   `bson:"gray_tag" json:"gray_tag" yaml:"gray_tag"`
	BaseEnv            string                   `bson:"base_env" json:"base_env" yaml:"base_env"`
	GrayEnv            string                   `bson:"gray_env" json:"gray_env" yaml:"gray_env"`
	GrayEnvSource      string                   `bson:"gray_env_source" json:"gray_env_source" yaml:"gray_env_source"`
	DockerRegistryID   string                   `bson:"docker_registry_id" json:"docker_registry_id" yaml:"docker_registry_id"`
	SkipCheckRunStatus bool                     `bson:"skip_check_run_status" json:"skip_check_run_status" yaml:"skip_check_run_status"`
	GrayServices       []*MseGrayReleaseService `bson:"gray_services" json:"gray_services" yaml:"gray_services"`
}

type MseGrayReleaseService struct {
	ServiceName     string                                 `bson:"service_name" json:"service_name" yaml:"service_name"`
	Replicas        int                                    `bson:"replicas"     json:"replicas"     yaml:"replicas"`
	YamlContent     string                                 `bson:"yaml" json:"yaml" yaml:"yaml"`
	ServiceAndImage []*MseGrayReleaseServiceModuleAndImage `bson:"service_and_image" json:"service_and_image" yaml:"service_and_image"`
}

type MseGrayReleaseServiceModuleAndImage struct {
	ServiceModule string `bson:"service_module" json:"service_module" yaml:"service_module"`
	Image         string `bson:"image"          json:"image"          yaml:"image"`
	// Following fields only save for frontend
	ImageName   string `bson:"image_name"     json:"image_name"     yaml:"image_name"`
	Name        string `bson:"name"           json:"name"           yaml:"name"`
	ServiceName string `bson:"service_name"   json:"service_name"   yaml:"service_name"`
	Value       string `bson:"value"          json:"value"          yaml:"value"`
}

type MseGrayOfflineJobSpec struct {
	Env        string `bson:"env" json:"env" yaml:"env"`
	Source     string `bson:"source" json:"source" yaml:"source"`
	GrayTag    string `bson:"gray_tag" json:"gray_tag" yaml:"gray_tag"`
	Production bool   `bson:"production" json:"production" yaml:"production"`
}

type NacosJobSpec struct {
	NacosID           string               `bson:"nacos_id"            json:"nacos_id"            yaml:"nacos_id"`
	NamespaceID       string               `bson:"namespace_id"        json:"namespace_id"        yaml:"namespace_id"`
	NacosDatas        []*types.NacosConfig `bson:"nacos_datas"         json:"nacos_datas"         yaml:"nacos_datas"`
	NacosFilteredData []*types.NacosConfig `bson:"nacos_filtered_data" json:"nacos_filtered_data" yaml:"nacos_filtered_data"`
	NacosDataRange    []string             `bson:"nacos_data_range"    json:"nacos_data_range"    yaml:"nacos_data_range"`
	DataFixed         bool                 `bson:"data_fixed"          json:"data_fixed"          yaml:"data_fixed"`
}

type WorkflowTriggerJobSpec struct {
	IsEnableCheck bool                       `bson:"is_enable_check" json:"is_enable_check" yaml:"is_enable_check"`
	TriggerType   config.WorkflowTriggerType `bson:"trigger_type" json:"trigger_type" yaml:"trigger_type"`
	// FixedWorkflowList is the only field used for trigger_type = fixed
	FixedWorkflowList []*ServiceTriggerWorkflowInfo `bson:"fixed_workflow_list" json:"fixed_workflow_list" yaml:"fixed_workflow_list"`

	// ServiceTriggerWorkflow and other fields are used for trigger_type = common
	ServiceTriggerWorkflow []*ServiceTriggerWorkflowInfo    `bson:"service_trigger_workflow" json:"service_trigger_workflow" yaml:"service_trigger_workflow"`
	Source                 config.TriggerWorkflowSourceType `bson:"source" json:"source" yaml:"source"`
	SourceJobName          string                           `bson:"source_job_name" json:"source_job_name" yaml:"source_job_name"`
	SourceService          []*ServiceNameAndModule          `bson:"source_service" json:"source_service" yaml:"source_service"`
}

type ServiceNameAndModule struct {
	ServiceName   string `bson:"service_name" json:"service_name" yaml:"service_name"`
	ServiceModule string `bson:"service_module" json:"service_module" yaml:"service_module"`
}

type ServiceTriggerWorkflowInfo struct {
	ServiceName   string   `bson:"service_name" json:"service_name" yaml:"service_name"`
	ServiceModule string   `bson:"service_module" json:"service_module" yaml:"service_module"`
	WorkflowName  string   `bson:"workflow_name" json:"workflow_name" yaml:"workflow_name"`
	ProjectName   string   `bson:"project_name" json:"project_name" yaml:"project_name"`
	Params        []*Param `bson:"params" json:"params" yaml:"params"`
}

type OfflineServiceJobSpec struct {
	EnvType  config.EnvType `bson:"env_type" json:"env_type" yaml:"env_type"`
	EnvName  string         `bson:"env_name" json:"env_name" yaml:"env_name"`
	Source   string         `bson:"source" json:"source" yaml:"source"`
	Services []string       `bson:"services" json:"services" yaml:"services"`
}

type JobProperties struct {
	Timeout         int64               `bson:"timeout"                json:"timeout"               yaml:"timeout"`
	Retry           int64               `bson:"retry"                  json:"retry"                 yaml:"retry"`
	ResourceRequest setting.Request     `bson:"res_req"                json:"res_req"               yaml:"res_req"`
	ResReqSpec      setting.RequestSpec `bson:"res_req_spec"           json:"res_req_spec"          yaml:"res_req_spec"`
	ClusterID       string              `bson:"cluster_id"             json:"cluster_id"            yaml:"cluster_id"`
	StrategyID      string              `bson:"strategy_id"            json:"strategy_id"           yaml:"strategy_id"`
	BuildOS         string              `bson:"build_os"               json:"build_os"              yaml:"build_os,omitempty"`
	ImageFrom       string              `bson:"image_from"             json:"image_from"            yaml:"image_from,omitempty"`
	ImageID         string              `bson:"image_id"               json:"image_id"              yaml:"image_id,omitempty"`
	Namespace       string              `bson:"namespace"              json:"namespace"             yaml:"namespace"`
	Envs            []*KeyVal           `bson:"envs"                   json:"envs"                  yaml:"envs"`
	// log user-defined variables, shows in workflow task detail.
	CustomEnvs          []*KeyVal            `bson:"custom_envs"            json:"custom_envs"           yaml:"custom_envs,omitempty"`
	Params              []*Param             `bson:"params"                 json:"params"                yaml:"params"`
	Paths               string               `bson:"-"                      json:"-"                     yaml:"-"`
	LogFileName         string               `bson:"log_file_name"          json:"log_file_name"         yaml:"log_file_name"`
	DockerHost          string               `bson:"-"                      json:"docker_host,omitempty" yaml:"docker_host,omitempty"`
	Registries          []*RegistryNamespace `bson:"registries"             json:"registries"            yaml:"registries"`
	Cache               types.Cache          `bson:"cache"                  json:"cache"                 yaml:"cache"`
	CacheEnable         bool                 `bson:"cache_enable"           json:"cache_enable"          yaml:"cache_enable"`
	CacheDirType        types.CacheDirType   `bson:"cache_dir_type"         json:"cache_dir_type"        yaml:"cache_dir_type"`
	CacheUserDir        string               `bson:"cache_user_dir"         json:"cache_user_dir"        yaml:"cache_user_dir"`
	ShareStorageInfo    *ShareStorageInfo    `bson:"share_storage_info"     json:"share_storage_info"    yaml:"share_storage_info"`
	ShareStorageDetails []*StorageDetail     `bson:"share_storage_details"  json:"share_storage_details" yaml:"-"`
	UseHostDockerDaemon bool                 `bson:"use_host_docker_daemon,omitempty" json:"use_host_docker_daemon,omitempty" yaml:"use_host_docker_daemon"`
}

type Step struct {
	Name     string          `bson:"name"           json:"name"             yaml:"name"`
	Timeout  int64           `bson:"timeout"        json:"timeout"          yaml:"timeout"`
	StepType config.StepType `bson:"type"           json:"type"             yaml:"type"`
	Spec     interface{}     `bson:"spec"           json:"spec"             yaml:"spec"`
}

type Output struct {
	Name        string `bson:"name"           json:"name"             yaml:"name"`
	Description string `bson:"description"    json:"description"      yaml:"description"`
}

type WorkflowV4Hook struct {
	Name                string              `bson:"name"                      json:"name"`
	AutoCancel          bool                `bson:"auto_cancel"               json:"auto_cancel"`
	CheckPatchSetChange bool                `bson:"check_patch_set_change"    json:"check_patch_set_change"`
	Enabled             bool                `bson:"enabled"                   json:"enabled"`
	MainRepo            *MainHookRepo       `bson:"main_repo"                 json:"main_repo"`
	Description         string              `bson:"description,omitempty"     json:"description,omitempty"`
	Repos               []*types.Repository `bson:"-"                         json:"repos,omitempty"`
	IsManual            bool                `bson:"is_manual"                 json:"is_manual"`
	WorkflowArg         *WorkflowV4         `bson:"workflow_arg"              json:"workflow_arg"`
}

type JiraHook struct {
	Name        string      `bson:"name" json:"name"`
	Enabled     bool        `bson:"enabled" json:"enabled"`
	Description string      `bson:"description" json:"description"`
	WorkflowArg *WorkflowV4 `bson:"workflow_arg" json:"workflow_arg"`
}

type MeegoHook struct {
	Name                string      `bson:"name" json:"name"`
	MeegoID             string      `bson:"meego_id" json:"meego_id"`
	MeegoSystemIdentity string      `bson:"meego_system_identity" json:"meego_system_identity"`
	MeegoURL            string      `bson:"meego_url"             json:"meego_url"`
	Enabled             bool        `bson:"enabled" json:"enabled"`
	Description         string      `bson:"description" json:"description"`
	WorkflowArg         *WorkflowV4 `bson:"workflow_arg" json:"workflow_arg"`
}

type GeneralHook struct {
	Name        string      `bson:"name" json:"name"`
	Enabled     bool        `bson:"enabled" json:"enabled"`
	Description string      `bson:"description" json:"description"`
	WorkflowArg *WorkflowV4 `bson:"workflow_arg" json:"workflow_arg"`
}

type Param struct {
	Name        string `bson:"name"             json:"name"             yaml:"name"`
	Description string `bson:"description"      json:"description"      yaml:"description"`
	// support string/text/choice/repo type
	ParamsType   string                 `bson:"type"                      json:"type"                        yaml:"type"`
	Value        string                 `bson:"value"                     json:"value"                       yaml:"value,omitempty"`
	Repo         *types.Repository      `bson:"repo"                     json:"repo"                         yaml:"repo,omitempty"`
	ChoiceOption []string               `bson:"choice_option,omitempty"   json:"choice_option,omitempty"     yaml:"choice_option,omitempty"`
	Default      string                 `bson:"default"                   json:"default"                     yaml:"default"`
	IsCredential bool                   `bson:"is_credential"             json:"is_credential"               yaml:"is_credential"`
	Source       config.ParamSourceType `bson:"source,omitempty" json:"source,omitempty" yaml:"source,omitempty"`
}

type ShareStorage struct {
	Name string `bson:"name"             json:"name"             yaml:"name"`
	Path string `bson:"path"             json:"path"             yaml:"path"`
}

type ShareStorageInfo struct {
	Enabled       bool            `bson:"enabled"             json:"enabled"             yaml:"enabled"`
	ShareStorages []*ShareStorage `bson:"share_storages"      json:"share_storages"      yaml:"share_storages"`
}

type StorageDetail struct {
	Type      types.MediumType `bson:"type"             json:"type"             yaml:"type"`
	Name      string           `bson:"name"             json:"name"             yaml:"name"`
	SubPath   string           `bson:"sub_path"         json:"sub_path"         yaml:"sub_path"`
	MountPath string           `bson:"mount_path"       json:"mount_path"       yaml:"mount_path"`
}

func IToiYaml(before interface{}, after interface{}) error {
	b, err := yaml.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %v", err)
	}

	if err := yaml.Unmarshal(b, after); err != nil {
		return fmt.Errorf("unmarshal task error: %v", err)
	}
	return nil
}

func (WorkflowV4) TableName() string {
	return "workflow_v4"
}

// CustomField use to display custom field of workflow history tasks
type CustomField struct {
	TaskID                 int            `bson:"task_id"                             json:"task_id"`
	Status                 int            `bson:"status"                              json:"status"`
	Duration               int            `bson:"duration"                            json:"duration"`
	Executor               int            `bson:"executor"                            json:"executor"`
	BuildServiceComponent  map[string]int `bson:"build_service_component,omitempty"   json:"build_service_component,omitempty"`
	BuildCodeMsg           map[string]int `bson:"build_code_msg,omitempty"            json:"build_code_msg,omitempty"`
	DeployServiceComponent map[string]int `bson:"deploy_service_component,omitempty"  json:"deploy_service_component,omitempty"`
	DeployEnv              map[string]int `bson:"deploy_env,omitempty"                json:"deploy_env,omitempty"`
	TestResult             map[string]int `bson:"test_result,omitempty"               json:"test_result,omitempty"`
}
