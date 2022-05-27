package meta

type JobContext struct {
	Name string `yaml:"name"`
	// Workspace 容器工作目录 [必填]
	Workspace string `yaml:"workspace"`
	Proxy     *Proxy `yaml:"proxy"`
	// Envs 用户注入环境变量, 包括安装脚本环境变量 [optional]
	Envs EnvVar `yaml:"envs"`
	// SecretEnvs 用户注入敏感信息环境变量, value不能在stdout stderr中输出 [optional]
	SecretEnvs EnvVar `yaml:"secret_envs"`
	// WorkflowName
	WorkflowName string `yaml:"workflow_name"`
	// TaskID
	TaskID int64 `yaml:"task_id"`
	// Paths 执行脚本Path
	Paths string `yaml:"-"`

	Steps   []*Step  `yaml:"steps"`
	Outputs []string `yaml:"outputs"`
}

type Step struct {
	Name     string      `yaml:"name"`
	StepType string      `yaml:"type"`
	Spec     interface{} `yaml:"spec"`
}

type EnvVar []string

// Proxy 翻墙配置信息
type Proxy struct {
	Type                   string `yaml:"type"`
	Address                string `yaml:"address"`
	Port                   int    `yaml:"port"`
	NeedPassword           bool   `yaml:"need_password"`
	Username               string `yaml:"username"`
	Password               string `yaml:"password"`
	EnableRepoProxy        bool   `yaml:"enable_repo_proxy"`
	EnableApplicationProxy bool   `yaml:"enable_application_proxy"`
}
