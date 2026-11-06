package models

import (
	"encoding/json"
	"maps"
)

type GitConfig struct {
	CodehostID int    `json:"codehost_id,omitempty" jsonschema:"description=从代码源目录查询得到的标识"`
	Owner      string `json:"owner,omitempty" jsonschema:"description=仓库组织或拥有者"`
	Namespace  string `json:"namespace,omitempty" jsonschema:"description=GitLab 多级命名空间，不是 Kubernetes namespace"`
	Repo       string `json:"repo,omitempty" jsonschema:"description=仓库名称"`
	Branch     string `json:"branch,omitempty" jsonschema:"description=分支"`
	Path       string `json:"path,omitempty" jsonschema:"description=已有 YAML 或 Chart 路径"`
	IsDir      bool   `json:"is_dir,omitempty" jsonschema:"description=是否把目录配置归为同一个服务"`
}

type ServiceInput struct {
	Name                string            `json:"name" jsonschema:"description=服务名，不能与已有服务重名"`
	Source              string            `json:"source,omitempty" jsonschema:"enum=yaml_git,enum=yaml_template,enum=helm_git,enum=helm_repository,enum=helm_template,description=已有部署配置或已有模板来源"`
	Git                 *GitConfig        `json:"git,omitempty"`
	TemplateID          string            `json:"template_id,omitempty" jsonschema:"description=已有 YAML 模板 ID 或 Chart 模板名称"`
	Variables           map[string]string `json:"variables,omitempty" jsonschema:"description=模板定义的变量，不接受脚本或原始 YAML"`
	ChartRepo           string            `json:"chart_repo,omitempty"`
	ChartName           string            `json:"chart_name,omitempty"`
	ChartVersion        string            `json:"chart_version,omitempty"`
	ValuesGit           *GitConfig        `json:"values_git,omitempty" jsonschema:"description=可选的已有 values Git 文件来源"`
	TemplateFingerprint string            `json:"template_fingerprint,omitempty"`
}

type BuildInput struct {
	Name                string            `json:"name"`
	TemplateID          string            `json:"template_id,omitempty" jsonschema:"description=已有构建模板 ID，未选择时跳过构建"`
	ServiceName         string            `json:"service_name,omitempty"`
	ServiceModule       string            `json:"service_module,omitempty" jsonschema:"description=服务中的构建组件或容器名称"`
	Repositories        []GitConfig       `json:"repositories,omitempty" jsonschema:"description=构建源码来源，与部署配置来源分开"`
	Variables           map[string]string `json:"variables,omitempty"`
	TemplateFingerprint string            `json:"template_fingerprint,omitempty"`
}

type EnvironmentInput struct {
	Name       string            `json:"name,omitempty"`
	ClusterID  string            `json:"cluster_id,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	RegistryID string            `json:"registry_id,omitempty"`
	Services   []string          `json:"services,omitempty"`
	Variables  map[string]string `json:"variables,omitempty"`
}

type OnboardingInput struct {
	ExpectedServiceCount int               `json:"expected_service_count,omitempty" jsonschema:"description=用户明确要求的服务总数，一到二十；只说三个服务时也必须记录，未提供时沿用草稿"`
	RemoveServices       []string          `json:"remove_services,omitempty" jsonschema:"description=仅在用户明确要求移除或替换服务时提供已有服务名；省略服务不会删除"`
	ProjectKey           string            `json:"project_key,omitempty" jsonschema:"description=项目标识，小写字母数字及短横线，最长三十二位"`
	ProjectName          string            `json:"project_name,omitempty" jsonschema:"description=新项目展示名称"`
	ProjectType          string            `json:"project_type,omitempty" jsonschema:"enum=k8s,enum=helm,description=项目部署类型"`
	Description          string            `json:"description,omitempty"`
	GroupName            string            `json:"group_name,omitempty"`
	Admins               []string          `json:"admins,omitempty" jsonschema:"description=管理员用户标识，默认当前用户"`
	Public               bool              `json:"public,omitempty" jsonschema:"description=默认私有，只有用户明确要求时才设为公开"`
	Services             []ServiceInput    `json:"services,omitempty"`
	Builds               []BuildInput      `json:"builds,omitempty"`
	Environment          *EnvironmentInput `json:"environment,omitempty"`
	CreateWorkflow       bool              `json:"create_workflow,omitempty" jsonschema:"description=是否创建现有逻辑生成的默认工作流，不执行工作流"`
	providedFields       map[string]json.RawMessage
}

// 保留实际提供的字段，区分局部修改中的省略、false、空字符串及 null。
func (a *OnboardingInput) UnmarshalJSON(data []byte) error {
	type plain OnboardingInput
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*a = OnboardingInput(value)
	a.providedFields = fields
	return nil
}

func (a OnboardingInput) ProvidedFields() map[string]json.RawMessage {
	if a.providedFields != nil {
		return maps.Clone(a.providedFields)
	}
	data, _ := json.Marshal(a)
	var fields map[string]json.RawMessage
	json.Unmarshal(data, &fields)
	return fields
}
