package models

type WorkflowTarget struct {
	ProjectKey   string `json:"project_key,omitempty" jsonschema:"description=项目标识"`
	WorkflowName string `json:"workflow_name,omitempty" jsonschema:"description=工作流标识，不是展示名称"`
	TaskID       int64  `json:"task_id,omitempty" jsonschema:"description=工作流任务编号"`
	JobName      string `json:"job_name,omitempty" jsonschema:"description=可选的失败任务名称"`
}

// 诊断补问用指针区分未提供生产属性与明确选择测试环境。
type ServiceDiagnosisInput struct {
	Goal        string `json:"goal,omitempty" jsonschema:"enum=diagnose,enum=restart_service,enum=rollback_environment,description=首次调用必须根据用户明确目标选择仅诊断、重启或回滚；修复目标不能降级为仅诊断"`
	ProjectKey  string `json:"project_key,omitempty"`
	EnvName     string `json:"env_name,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	Production  *bool  `json:"production,omitempty"`
}

type ServiceTarget struct {
	ProjectKey  string `json:"project_key" jsonschema:"description=项目标识"`
	EnvName     string `json:"env_name" jsonschema:"description=环境名称"`
	ServiceName string `json:"service_name" jsonschema:"description=服务名称"`
	Production  bool   `json:"production" jsonschema:"description=是否生产环境"`
}
