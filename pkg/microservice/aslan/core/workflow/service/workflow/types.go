package workflow

type WorkflowV3 struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	ProjectName string                   `json:"project_name"`
	Description string                   `json:"description"`
	Parameters  []*ParameterSetting      `json:"parameters"`
	SubTasks    []map[string]interface{} `bson:"sub_tasks"`
}

type WorkflowV3Brief struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectName string `json:"project_name"`
}

type ParameterSetting struct {
	// External type parameter will NOT use this key.
	Key string `json:"key"`
	// Type list：
	// string
	// choice
	// external
	Type string `json:"type"`
	//DefaultValue is the
	DefaultValue string `json:"default_value"`
	// choiceOption 是枚举的所有选项
	ChoiceOption []string `json:"choice_option"`
	// ExternalSetting 是外部系统获取变量的配置
	ExternalSetting *ExternalSetting `json:"external_setting"`
}

type ExternalSetting struct {
	// 外部系统ID
	SystemID string `json:"system_id"`
	// Endpoint路径
	Endpoint string `json:"endpoint"`
	// 请求方法
	Method string `json:"method"`
	// 请求头
	Headers []*KV `json:"headers"`
	// 请求体
	Body string `json:"body"`
	// 外部变量配置
	Params []*ExternalParamMapping `json:"params"`
}

type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ExternalParamMapping struct {
	// zadig变量名称
	ParamKey string `json:"param_key"`
	// 返回中的key的位置
	ResponseKey string `json:"response_key"`
	Display     bool   `json:"display"`
}
