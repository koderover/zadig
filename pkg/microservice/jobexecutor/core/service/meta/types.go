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

package meta

type JobContext struct {
	Name string `yaml:"name"`
	// Workspace 容器工作目录 [必填]
	Workspace string `yaml:"workspace"`
	// Envs 用户注入环境变量, 包括安装脚本环境变量 [optional]
	Envs EnvVar `yaml:"envs"`
	// SecretEnvs 用户注入敏感信息环境变量, value不能在stdout stderr中输出 [optional]
	SecretEnvs EnvVar `yaml:"secret_envs"`
	// WorkflowName
	WorkflowName string `yaml:"workflow_name"`
	// TaskID
	TaskID int64 `yaml:"task_id"`
	// Paths 执行脚本Path
	Paths string `yaml:"paths"`

	Steps   []*Step  `yaml:"steps"`
	Outputs []string `yaml:"outputs"`
}

type Step struct {
	Name      string      `yaml:"name"`
	StepType  string      `yaml:"type"`
	Onfailure bool        `yaml:"on_failure"`
	Spec      interface{} `yaml:"spec"`
}

type EnvVar []string
