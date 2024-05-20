/*
 * Copyright 2024 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package blueking

// 以下 comment 皆为蓝鲸官方 API 称呼，留存以作为对应关系留档，不要删除！！！！！
const (
	// 查询业务
	ListBusinessAPI string = "api/c/compapi/v2/cc/search_business/"
	// 查询执行方案列表
	ListExecutionPlanAPI string = "api/c/compapi/v2/jobv3/get_job_plan_list/"
	// 查询执行方案详情
	GetExecutionPlanDetailAPI string = "api/c/compapi/v2/jobv3/get_job_plan_detail/"
	// 执行作业执行方案
	RunExecutionPlanAPI string = "api/c/compapi/v2/jobv3/execute_job_plan/"
	// 根据作业实例 ID 查询作业执行状态
	GetJobInstanceAPI string = "api/c/compapi/v2/jobv3/get_job_instance_status/"
	// 查询业务实例拓扑
	GetTopologyAPI string = "api/c/compapi/v2/cc/search_biz_inst_topo/"
	// 查询拓扑节点下的主机
	GetTopologyNodeHostAPI = "api/c/compapi/v2/cc/find_host_by_topo/"
)

type AuthorizationHeader struct {
	AppCode   string `json:"bk_app_code"`
	AppSecret string `json:"bk_app_secret"`
	Username  string `json:"bk_username"`
}

// GeneralResponse the global response from blueking system, the response detail is in the Data field
type GeneralResponse struct {
	Result    bool        `json:"result"`
	Code      int64       `json:"code"`
	Message   string      `json:"message"`
	RequestID string      `json:"request_id"`
	Data      interface{} `json:"data"`
}

type PagingReq struct {
	Start int64  `json:"start"`
	Limit int64  `json:"limit"`
	Sort  string `json:"sort,omitempty"`
}

type BusinessList struct {
	Count int64       `json:"count"`
	Info  []*Business `json:"info"`
}

type Business struct {
	ID   int64  `json:"bk_biz_id"`
	Name string `json:"bk_biz_name"`
	// 业务类型字段
	Default int64 `json:"default"`
}

type ExecutionPlanBrief struct {
	ID             int64  `json:"id"`
	BusinessID     int64  `json:"bk_biz_id"`
	Name           string `json:"name"`
	JobTemplateID  int64  `json:"job_template_id"`
	Creator        string `json:"creator"`
	CreateTime     int64  `json:"create_time"`
	LastModifyUser string `json:"last_modify_user"`
	LastModifyTime int64  `json:"last_modify_time"`
}

type ExecutionPlanDetail struct {
	BusinessID         int64             `json:"bk_biz_id"`
	ExecutionPlanID    int64             `json:"job_plan_id"`
	Name               string            `json:"name,omitempty"`
	Creator            string            `json:"creator,omitempty"`
	CreateTime         int64             `json:"create_time,omitempty"`
	LastModifyUser     string            `json:"last_modify_user,omitempty"`
	LastModifyTime     int64             `json:"last_modify_time,omitempty"`
	GlobalVariableList []*GlobalVariable `json:"global_var_list,omitempty"`
}

type GlobalVariable struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Type        int64           `json:"type"`
	Required    int64           `json:"required"`
	Value       string          `json:"value"`
	Server      *ServerOverview `json:"server"`
}

type ServerOverview struct {
	IPList   []*ServerInfo       `json:"ip_list"`
	NodeList []*TopologyNodeInfo `json:"topo_node_list"`
	// Variable 引用的变量名
	Variable string `json:"variable,omitempty"`
}

type ServerInfo struct {
	CloudID int64  `json:"bk_cloud_id"`
	IP      string `json:"ip,omitempty"`
	HostID  int64  `json:"bk_host_id,omitempty"`
	InnerIP string `json:"bk_host_innerip,omitempty"`
}

type TopologyNodeInfo struct {
	ID       int64  `json:"id"`
	NodeType string `json:"node_type"`
}

type JobInstanceBrief struct {
	JobInstanceName string `json:"job_instance_name"`
	JobInstanceID   int64  `json:"job_instance_id"`
}

type JobInstanceDetail struct {
	Finished    bool             `json:"finished"`
	JobInstance *JobInstanceInfo `json:"job_instance"`
}

type JobInstanceInfo struct {
	JobInstanceID int64  `json:"job_instance_id"`
	BusinessID    int64  `json:"bk_biz_id"`
	Name          string `json:"name"`
	CreateTime    int64  `json:"create_time"`
	Status        int64  `json:"status"`
	StartTime     int64  `json:"start_time"`
	EndTime       int64  `json:"end_time"`
	TotalTime     int    `json:"total_time"`
}

type TopologyNode struct {
	InstanceID   int             `json:"bk_inst_id"`
	InstanceName string          `json:"bk_inst_name"`
	ObjectID     string          `json:"bk_obj_id"`
	ObjectName   string          `json:"bk_obj_name"`
	Child        []*TopologyNode `json:"child"`
}

type HostList struct {
	Count      int64         `json:"count"`
	ServerList []*ServerInfo `json:"info"`
}
