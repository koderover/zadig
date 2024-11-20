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

package jobcontroller

import (
	"gopkg.in/yaml.v2"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

type JobContext struct {
	Name        string `yaml:"name"`
	Key         string `yaml:"key"`
	OriginName  string `yaml:"origin_name"`
	DisplayName string `yaml:"display_name"`
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
	Paths string `yaml:"paths"`
	// ConfigMapName save the name of the configmap in which the jobContext resides
	ConfigMapName string `yaml:"config_map_name"`

	Steps   []*commonmodels.StepTask `yaml:"steps"`
	Outputs []string                 `yaml:"outputs"`
	// used to vm job
	Cache *JobCacheConfig `yaml:"cache"`
}

func (j *JobContext) Decode(job string) error {
	if err := yaml.Unmarshal([]byte(job), j); err != nil {
		return err
	}
	return nil
}

type JobCacheConfig struct {
	CacheEnable  bool               `json:"cache_enable"`
	CacheDirType types.CacheDirType `json:"cache_dir_type"`
	CacheUserDir string             `json:"cache_user_dir"`
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
